// Package engine 转存编排层：解析链接 → 验证 → 遍历 → 过滤 → 去重 → 分批转存 → 重命名 → 记录 → 通知
package engine

import (
	"fmt"
	"log"
	"math/rand"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"baidu-auto-save/internal/baidu"
	"baidu-auto-save/internal/db"
)

// ErrClass 错误分类（设计文档 §5）
type ErrClass string

const (
	ErrClassNone       ErrClass = ""
	ErrClassNetwork    ErrClass = "network"
	ErrClassRateLimit  ErrClass = "rate_limit"
	ErrClassCookieBad  ErrClass = "cookie_invalid"
	ErrClassLinkDead   ErrClass = "link_invalid"
	ErrClassPwdWrong   ErrClass = "pwd_wrong"
	ErrClassOther      ErrClass = "other"
)

// Result 单次运行结果
type Result struct {
	LogID     int64
	NewFiles  int64
	Skipped   int64
	ErrClass  ErrClass
	Message   string
}

// Engine 转存引擎
type Engine struct {
	db     *db.DB
	newCli func(cookieStr string) (baidu.Client, error) // 便于测试注入
	// 同一时刻最多一个转存在执行（设计 §5 并发约束）
	sem chan struct{}
}

// New 创建引擎
func New(database *db.DB) *Engine {
	return &Engine{
		db:     database,
		newCli: baidu.NewClient,
		sem:    make(chan struct{}, 1),
	}
}

// setNewClient 仅供测试
func (e *Engine) setNewClient(f func(string) (baidu.Client, error)) { e.newCli = f }

// RunTask 执行一个任务（串行闸门 + panic 保护由调用方 scheduler 处理）
func (e *Engine) RunTask(t *db.Task) (*Result, error) {
	e.sem <- struct{}{}
	defer func() { <-e.sem }()

	res := e.runTask(t)
	// 写日志
	logRow := &db.TransferLog{
		TaskID:    t.ID,
		Result:    resultString(res),
		Message:   res.Message,
		FileCount: res.NewFiles,
		SkipCount: res.Skipped,
	}
	id, err := e.db.InsertTransferLog(logRow)
	if err != nil {
		log.Printf("[engine] 写日志失败 task=%d: %v", t.ID, err)
	}
	res.LogID = id
	return res, nil
}

func resultString(r *Result) string {
	switch r.ErrClass {
	case ErrClassNone:
		if r.NewFiles == 0 && r.Skipped > 0 {
			return "skipped"
		}
		return "success"
	case ErrClassLinkDead:
		return "failed"
	default:
		return "failed"
	}
}

func (e *Engine) runTask(t *db.Task) *Result {
	res := &Result{}

	acc, err := e.db.GetAccount(t.AccountID)
	if err != nil || acc == nil {
		res.ErrClass, res.Message = ErrClassOther, "账号不存在"
		return res
	}
	if acc.Status == "invalid" {
		res.ErrClass, res.Message = ErrClassCookieBad, "账号 Cookie 已失效，请更新后再试"
		return res
	}

	cookieStr := buildCookieStr(acc)
	cli, err := e.newCli(cookieStr)
	if err != nil {
		res.ErrClass, res.Message = ErrClassOther, "创建客户端失败: "+err.Error()
		return res
	}

	// 1. 解析 surl & 访问分享页
	tokens, err := cli.AccessSharePage(t.ShareURL)
	if err != nil {
		return classify(res, err)
	}
	// 2. 提取码验证
	if err := cli.VerifyPwd(t.ShareURL, tokens, t.Pwd); err != nil {
		return classify(res, err)
	}

	// 3. 遍历分享目录树 → 收集待转存文件
	files, err := e.walkShare(cli, t)
	if err != nil {
		return classify(res, err)
	}
	if len(files) == 0 {
		res.Message = "分享中无符合条件的文件"
		return res
	}

	// 4. MD5 去重（任务内）
	md5s := make([]string, 0, len(files))
	for _, f := range files {
		if f.MD5 != "" {
			md5s = append(md5s, f.MD5)
		}
	}
	known, err := e.db.FilterKnownMD5(t.ID, md5s)
	if err != nil {
		res.ErrClass, res.Message = ErrClassOther, "去重查询失败: "+err.Error()
		return res
	}
	pending := make([]*baidu.ShareFile, 0, len(files))
	seenMD5 := make(map[string]bool, len(files)) // 运行内去重：分享不同目录可能存在相同 MD5 的重复文件
	for _, f := range files {
		if f.IsDir || f.MD5 == "" {
			continue
		}
		if known[f.MD5] || seenMD5[f.MD5] {
			res.Skipped++
			continue
		}
		seenMD5[f.MD5] = true
		pending = append(pending, f)
	}
	if len(pending) == 0 {
		res.Message = "全部文件均已转存"
		return res
	}

	// 5. 按目标目录分组（folder_renames 改名的文件落到 保存目录/B）
	groups := groupByDest(t, pending)

	// 6. 分组转存：先确保目录存在，再分批转存
	var transferred []*baidu.ShareFile
	var lastErr error
	mkdirCache := make(map[string]bool) // 本次运行内共享：同名目录只 Mkdir 一次
	for dest, group := range groups {
		if err := ensureDirCached(cli, dest, mkdirCache); err != nil {
			res.ErrClass, res.Message = ErrClassOther, "创建保存目录失败: "+err.Error()
			return res
		}
		for _, batch := range splitBatches(group, defaultBatchSize) {
			if err := withRetry(func() error {
				return cli.Transfer(fsidsOf(batch), dest)
			}); err != nil {
				if cls := classifyErr(err); cls == ErrClassLinkDead || cls == ErrClassCookieBad || cls == ErrClassPwdWrong {
					// 不可恢复错误，直接终止
					return classify(res, err)
				}
				lastErr = err
				continue
			}
			transferred = append(transferred, batch...)
			// 限速：批次间稍作间隔，降低风控
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 7. 记录已转存文件（记录实际落盘路径）
	for _, f := range transferred {
		fullPath := path.Join(destDirFor(t, f), f.ServerName)
		_ = e.db.InsertTaskFile(&db.TaskFile{
			TaskID: t.ID,
			Path:   fullPath,
			MD5:    f.MD5,
			Size:   f.Size,
		})
	}
	res.NewFiles = int64(len(transferred))

	// 8. 重命名（两步实现：转存成功后处理）
	if t.RegexReplace != "" && t.RegexPattern != "" {
		e.renameTransferred(cli, t, transferred)
	}

	if len(transferred) == 0 && lastErr != nil {
		res.ErrClass, res.Message = classifyErr(lastErr), lastErr.Error()
		return res
	}
	if lastErr != nil {
		res.Message = fmt.Sprintf("部分批次失败: %s", lastErr.Error())
		res.ErrClass = ErrClassNetwork // partial 视作可重试
	} else {
		res.Message = fmt.Sprintf("转存 %d 个文件", res.NewFiles)
	}
	return res
}

const defaultBatchSize = 50

// splitBatches 按上限切分批次
func splitBatches(files []*baidu.ShareFile, n int) [][]*baidu.ShareFile {
	if n <= 0 {
		n = defaultBatchSize
	}
	var out [][]*baidu.ShareFile
	for i := 0; i < len(files); i += n {
		end := i + n
		if end > len(files) {
			end = len(files)
		}
		out = append(out, files[i:end])
	}
	return out
}

func fsidsOf(files []*baidu.ShareFile) []int64 {
	ids := make([]int64, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.FsID)
	}
	return ids
}

// walkShare 遍历分享目录树，应用 folder_paths 勾选与过滤规则，返回文件列表
func (e *Engine) walkShare(cli baidu.Client, t *db.Task) ([]*baidu.ShareFile, error) {
	var includeRe, excludeRe, fileRe *regexp.Regexp
	var err error
	if t.FolderFilter != "" {
		if includeRe, err = regexp.Compile(t.FolderFilter); err != nil {
			return nil, fmt.Errorf("包含文件夹正则非法: %w", err)
		}
	}
	if t.ExcludeFolderFilter != "" {
		if excludeRe, err = regexp.Compile(t.ExcludeFolderFilter); err != nil {
			return nil, fmt.Errorf("排除文件夹正则非法: %w", err)
		}
	}
	if t.RegexPattern != "" {
		if fileRe, err = regexp.Compile(t.RegexPattern); err != nil {
			return nil, fmt.Errorf("文件名正则非法: %w", err)
		}
	}

	var out []*baidu.ShareFile
	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := cli.ListShareDir(t.ShareURL, dir)
		if err != nil {
			return err
		}
		for _, ent := range entries {
			if ent.IsDir {
				rel := ent.Path
				// 过滤规则基于分享内相对路径
				if includeRe != nil && !includeRe.MatchString(rel) {
					continue
				}
				if excludeRe != nil && excludeRe.MatchString(rel) {
					continue
				}
				if err := walk(rel); err != nil {
					return err
				}
			} else {
				// 文件级过滤：regex_pattern 匹配文件名
				if fileRe != nil && !fileRe.MatchString(ent.ServerName) {
					continue
				}
				// folder_paths 勾选：空=全部；否则文件必须位于勾选目录（含子级）
				if len(t.FolderPaths) > 0 && !underAny(ent.Path, t.FolderPaths) {
					continue
				}
				out = append(out, ent)
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	return out, nil
}

// underAny 判断 p 是否等于 sel 或位于 sel 之下
func underAny(p string, sel []string) bool {
	for _, s := range sel {
		s = strings.TrimSuffix(s, "/")
		if p == s || strings.HasPrefix(p, s+"/") {
			return true
		}
	}
	return false
}

// destDirFor 计算文件实际落盘目录（保留分享内子目录层级）：
// 找出文件命中且配置了改名的最深勾选目录 sel（级联勾选会产生无改名的子级选中项，不作为基准），
// 文件落到 保存目录/B/<相对 sel 的子目录>；无任何改名命中时落 保存目录/<分享内完整路径>。
// 百度 Transfer 不会按 fsid 保留原路径，层级必须由目标目录显式携带。
func destDirFor(t *db.Task, f *baidu.ShareFile) string {
	best := "" // 命中且配置了改名的最深勾选目录
	for _, sel := range t.FolderPaths {
		sel = strings.TrimSuffix(sel, "/")
		if f.Path != sel && !strings.HasPrefix(f.Path, sel+"/") {
			continue
		}
		if newName := strings.TrimSpace(t.FolderRenames[sel]); newName != "" && len(sel) > len(best) {
			best = sel
		}
	}
	// 文件相对基准目录的子路径（目录部分）；无基准时用分享内完整路径
	rel := f.Path
	if best != "" {
		rel = strings.TrimPrefix(f.Path, best+"/")
	}
	relDir := path.Dir(rel)
	if relDir == "." || relDir == "/" {
		relDir = ""
	}
	if newName := strings.TrimSpace(t.FolderRenames[best]); best != "" && newName != "" {
		return path.Join(t.SaveDir, newName, relDir)
	}
	return path.Join(t.SaveDir, relDir)
}

// groupByDest 按目标目录分组待转存文件
func groupByDest(t *db.Task, files []*baidu.ShareFile) map[string][]*baidu.ShareFile {
	groups := make(map[string][]*baidu.ShareFile)
	for _, f := range files {
		dest := destDirFor(t, f)
		groups[dest] = append(groups[dest], f)
	}
	return groups
}

// ensureDir 逐级创建网盘目录（内存缓存：跨组/跨批次复用，避免重复 Mkdir 请求拖慢转存）
func ensureDir(cli baidu.Client, dir string) error {
	return ensureDirCached(cli, dir, make(map[string]bool))
}

func ensureDirCached(cli baidu.Client, dir string, created map[string]bool) error {
	dir = strings.Trim(dir, "/")
	if dir == "" {
		return nil
	}
	parts := strings.Split(dir, "/")
	cur := ""
	for _, p := range parts {
		cur = cur + "/" + p
		if created[cur] {
			continue
		}
		if err := cli.Mkdir(cur); err != nil {
			// 已存在等情况忽略
			continue
		}
		created[cur] = true
	}
	return nil
}

// renameTransferred 对转存成功的文件执行重命名（正则捕获组模板）
func (e *Engine) renameTransferred(cli baidu.Client, t *db.Task, files []*baidu.ShareFile) {
	re, err := regexp.Compile(t.RegexPattern)
	if err != nil {
		return
	}
	for _, f := range files {
		newName := renameTemplate(re, f.ServerName, t.RegexReplace)
		if newName == "" || newName == f.ServerName {
			continue
		}
		from := path.Join(t.SaveDir, f.ServerName)
		to := path.Join(t.SaveDir, newName)
		if err := cli.Rename(from, to); err != nil {
			log.Printf("[engine] 重命名失败 %s → %s: %v", from, to, err)
		}
	}
}

// renameTemplate 应用捕获组模板生成新文件名（模板即完整新文件名）
// 两种占位符等价：\1 与 $1 均展开为第 N 个捕获组（Go 原生 ReplaceAllString
// 只替换匹配子串，不符合重命名语义，故统一手动展开）
func renameTemplate(re *regexp.Regexp, name, tpl string) string {
	m := re.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	if !strings.Contains(tpl, "\\") && !strings.Contains(tpl, "$") {
		return ""
	}
	expand := func(token string) string {
		// token 形如 "\1" 或 "$1"（支持多位数字）
		digits := token[1:]
		n, err := strconv.Atoi(digits)
		if err != nil || n >= len(m) {
			return ""
		}
		return m[n]
	}
	reToken := regexp.MustCompile(`\\(\d+)|\$(\d+)`)
	out := reToken.ReplaceAllStringFunc(tpl, func(s string) string {
		if strings.HasPrefix(s, "\\") {
			return expand(s)
		}
		return expand("$" + s[1:])
	})
	if out == tpl {
		return ""
	}
	return out
}

// classify 按错误分类填充 Result
func classify(res *Result, err error) *Result {
	res.ErrClass = classifyErr(err)
	res.Message = err.Error()
	return res
}

// classifyErr 错误 → 分类
func classifyErr(err error) ErrClass {
	if err == nil {
		return ErrClassNone
	}
	switch err {
	case baidu.ErrLinkInvalid:
		return ErrClassLinkDead
	case baidu.ErrPwdWrong:
		return ErrClassPwdWrong
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "提取码错误"):
		return ErrClassPwdWrong
	case strings.Contains(msg, "已失效") || strings.Contains(msg, "页面不存在"):
		return ErrClassLinkDead
	case strings.Contains(msg, "Cookie") || strings.Contains(msg, "STOKEN") || strings.Contains(msg, "登录"):
		return ErrClassCookieBad
	case strings.Contains(msg, "网络错误") || strings.Contains(msg, "timeout") || strings.Contains(msg, "EOF"):
		return ErrClassNetwork
	case strings.Contains(msg, "稍后再试") || strings.Contains(msg, "验证"):
		return ErrClassRateLimit
	default:
		return ErrClassOther
	}
}

// withRetry 网络类退避重试（1s/4s/16s + 抖动）
func withRetry(fn func() error) error {
	var err error
	backoff := time.Second
	for i := 0; i < 3; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if cls := classifyErr(err); cls != ErrClassNetwork && cls != ErrClassRateLimit {
			return err // 非瞬时错误不重试
		}
		jitter := time.Duration(rand.Int63n(int64(backoff) / 4))
		time.Sleep(backoff + jitter)
		backoff *= 4
	}
	return err
}

// buildCookieStr 组装 Cookie 字符串（API 层校验账号等场景复用）
func BuildCookieStr(a *db.Account) string {
	parts := []string{"BDUSS=" + a.BDUSS}
	if a.STOKEN != "" {
		parts = append(parts, "STOKEN="+a.STOKEN)
	}
	if a.CookiesJSON != "" && a.CookiesJSON != "{}" {
		var extra map[string]string
		if err := jsonUnmarshal(a.CookiesJSON, &extra); err == nil {
			for k, v := range extra {
				parts = append(parts, k+"="+v)
			}
		}
	}
	return strings.Join(parts, "; ")
}

// buildCookieStr 内部别名
func buildCookieStr(a *db.Account) string { return BuildCookieStr(a) }
