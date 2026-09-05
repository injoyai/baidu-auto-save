package engine

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/qjfoidnh/BaiduPCS-Go/baidupcs"

	"baidu-auto-save/internal/baidu"
	"baidu-auto-save/internal/db"
)

// baiduErrLinkInvalid 与 baidu.ErrLinkInvalid 同值（测试用别名）
var baiduErrLinkInvalid = baidu.ErrLinkInvalid

// mockCli 可编程的 baidu.Client 假实现
type mockCli struct {
	shareDirs   map[string][]*baidu.ShareFile // dir → entries
	transferErr map[string]error              // saveDir → error（模拟批次失败）
	transferred [][]int64                     // 每次 Transfer 收到的 fsid 批次
	mkdirs      []string
	renames     [][2]string
	accessErr   error
	verifyErr   error
	listErr     error
	quotaErr    error
}

func (m *mockCli) AccessSharePage(surl string) (map[string]string, error) {
	if m.accessErr != nil {
		return nil, m.accessErr
	}
	return map[string]string{"ErrMsg": "0", "shareid": "1", "share_uk": "2", "bdstoken": "t"}, nil
}

func (m *mockCli) VerifyPwd(surl string, tokens map[string]string, pwd string) error {
	return m.verifyErr
}

func (m *mockCli) ListShareDir(surl, dir string) ([]*baidu.ShareFile, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.shareDirs[dir], nil
}

func (m *mockCli) Transfer(fsids []int64, saveDir string) error {
	m.transferred = append(m.transferred, fsids)
	if m.transferErr != nil {
		if err := m.transferErr[saveDir]; err != nil {
			// 只失败第一次，模拟部分成功
			m.transferErr[saveDir] = nil
			return err
		}
	}
	return nil
}

func (m *mockCli) Rename(from, to string) error {
	m.renames = append(m.renames, [2]string{from, to})
	return nil
}

func (m *mockCli) Quota() (int64, int64, error) { return 0, 0, m.quotaErr }

func (m *mockCli) Mkdir(p string) error {
	m.mkdirs = append(m.mkdirs, p)
	return nil
}

func (m *mockCli) ListDir(p string) ([]*baidupcs.FileDirectory, error) { return nil, nil }

// newTestEngine 建内存库 + 注入 mock 客户端的引擎
func newTestEngine(t *testing.T, mc *mockCli) (*Engine, *db.DB) {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	wrapper := &db.DB{DB: database}
	if _, err := wrapper.CreateAccount(&db.Account{Name: "test", BDUSS: "bduss", STOKEN: "stoken", CookiesJSON: "{}"}); err != nil {
		t.Fatalf("建账号失败: %v", err)
	}
	e := New(wrapper)
	e.setNewClient(func(cookieStr string) (baidu.Client, error) {
		return mc, nil
	})
	return e, wrapper
}

func mkTask(accountID int64) *db.Task {
	return &db.Task{
		Name: "t1", AccountID: accountID,
		ShareURL: "https://pan.baidu.com/s/1abc?pwd=x", Pwd: "",
		SaveDir: "/save", CronExpr: "0 8 * * *", Enabled: true,
	}
}

func file(fsID int64, p, md5 string, size int64) *baidu.ShareFile {
	return &baidu.ShareFile{FsID: fsID, Path: p, ServerName: path.Base(p), MD5: md5, Size: size}
}

func dir(fsID int64, p string) *baidu.ShareFile {
	return &baidu.ShareFile{FsID: fsID, Path: p, ServerName: path.Base(p), IsDir: true}
}

func TestRunTask_Success(t *testing.T) {
	mc := &mockCli{shareDirs: map[string][]*baidu.ShareFile{
		"": {file(1, "/a.txt", "aaa", 10), file(2, "/b.txt", "bbb", 20)},
	}}
	e, database := newTestEngine(t, mc)
	id, _ := database.CreateTask(mkTask(1))
	task, _ := database.GetTask(id)

	res, err := e.RunTask(task)
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if res.ErrClass != ErrClassNone || res.NewFiles != 2 {
		t.Fatalf("结果不符: %+v", res)
	}
	if len(mc.transferred) != 1 || len(mc.transferred[0]) != 2 {
		t.Fatalf("转存批次不符: %v", mc.transferred)
	}
	// 日志已写入
	n, _ := database.CountTransferLogs(0)
	if n != 1 {
		t.Fatalf("日志数 %d != 1", n)
	}
}

func TestRunTask_MDDedup(t *testing.T) {
	mc := &mockCli{shareDirs: map[string][]*baidu.ShareFile{
		"": {file(1, "/a.txt", "aaa", 10)},
	}}
	e, database := newTestEngine(t, mc)
	id, _ := database.CreateTask(mkTask(1))
	task, _ := database.GetTask(id)

	if res, _ := e.RunTask(task); res.NewFiles != 1 {
		t.Fatalf("首次应转存 1 个: %+v", res)
	}
	// 同一 MD5 第二次运行应跳过
	res, _ := e.RunTask(task)
	if res.NewFiles != 0 || res.Skipped != 1 {
		t.Fatalf("二次运行应跳过: %+v", res)
	}
}

func TestRunTask_LinkInvalid(t *testing.T) {
	mc := &mockCli{accessErr: baiduErrLinkInvalid}
	e, database := newTestEngine(t, mc)
	id, _ := database.CreateTask(mkTask(1))
	task, _ := database.GetTask(id)

	res, _ := e.RunTask(task)
	if res.ErrClass != ErrClassLinkDead || res.Message != "分享链接已失效" {
		t.Fatalf("应识别链接失效: %+v", res)
	}
}

func TestRunTask_CookieInvalid(t *testing.T) {
	mc := &mockCli{accessErr: errors.New("Cookie 缺少网盘 STOKEN，请补充后重试")}
	e, database := newTestEngine(t, mc)
	id, _ := database.CreateTask(mkTask(1))
	task, _ := database.GetTask(id)

	res, _ := e.RunTask(task)
	if res.ErrClass != ErrClassCookieBad {
		t.Fatalf("应识别 Cookie 失效: %+v", res)
	}
}

func TestRunTask_Batches(t *testing.T) {
	var files []*baidu.ShareFile
	for i := 1; i <= 120; i++ {
		files = append(files, file(int64(i), fmt.Sprintf("/f%d.txt", i), fmt.Sprintf("md5-%d", i), 1))
	}
	mc := &mockCli{shareDirs: map[string][]*baidu.ShareFile{"" : files}}
	e, database := newTestEngine(t, mc)
	id, _ := database.CreateTask(mkTask(1))
	task, _ := database.GetTask(id)

	res, _ := e.RunTask(task)
	if res.NewFiles != 120 {
		t.Fatalf("应转存 120 个: %+v", res)
	}
	// 120 / 50 = 3 批
	if len(mc.transferred) != 3 {
		t.Fatalf("应分 3 批: %v", lenBatches(mc.transferred))
	}
}

func lenBatches(b [][]int64) int { return len(b) }

func TestRunTask_FolderPathsFilter(t *testing.T) {
	mc := &mockCli{shareDirs: map[string][]*baidu.ShareFile{
		"":      {dir(9, "/dir1"), dir(10, "/dir2")},
		"/dir1": {file(1, "/dir1/x.txt", "x1", 1)},
		"/dir2": {file(2, "/dir2/y.txt", "y1", 1)},
	}}
	e, database := newTestEngine(t, mc)
	tk := mkTask(1)
	tk.FolderPaths = []string{"/dir1"}
	id, _ := database.CreateTask(tk)
	task, _ := database.GetTask(id)

	res, _ := e.RunTask(task)
	if res.NewFiles != 1 {
		t.Fatalf("勾选 dir1 时应只转存 1 个: %+v", res)
	}
}

func TestDestDirFor_FolderRename(t *testing.T) {
	tk := mkTask(1)
	tk.FolderPaths = []string{"/A", "/A/sub"}
	tk.FolderRenames = map[string]string{"/A": "B"}

	cases := map[string]string{
		"/A/x.txt":             "/save/B",             // 勾选目录 A 改名 B
		"/A/sub/y.txt":         "/save/B/sub",         // A 的子级保留层级
		"/A/sub/deep/z.txt":    "/save/B/sub/deep",    // 深层嵌套
		"/other/w.txt":         "/save/other",         // 未勾选命中 → 保存目录 + 分享内路径（实际运行会被过滤）
	}
	for p, want := range cases {
		f := &baidu.ShareFile{Path: p}
		if got := destDirFor(tk, f); got != want {
			t.Errorf("destDirFor(%q) = %q, 期望 %q", p, got, want)
		}
	}
}

func TestDestDirFor_PreservesSubdirs(t *testing.T) {
	// 整树场景：分享 /资金流向/每日更新/东财/2025-08/x.csv 勾选 /资金流向 改名 B
	tk := mkTask(1)
	tk.FolderPaths = []string{"/资金流向"}
	tk.FolderRenames = map[string]string{"/资金流向": "B"}

	f := &baidu.ShareFile{Path: "/资金流向/每日更新/东财/2025-08/x.csv"}
	if got := destDirFor(tk, f); got != "/save/B/每日更新/东财/2025-08" {
		t.Errorf("应保留子目录层级: %q", got)
	}

	// 无勾选（整分享转存）：保留完整分享内层级
	tk2 := mkTask(1)
	tk2.FolderPaths = nil
	if got := destDirFor(tk2, f); got != "/save/资金流向/每日更新/东财/2025-08" {
		t.Errorf("整分享转存应保留层级: %q", got)
	}
}

func TestDestDirFor_NestedRenameDeepestWins(t *testing.T) {
	// 勾选 /A（改名 B）与 /A/sub（改名 C）：sub 下文件落 C/<层级>，其余落 B/<层级>
	tk := mkTask(1)
	tk.FolderPaths = []string{"/A", "/A/sub"}
	tk.FolderRenames = map[string]string{"/A": "B", "/A/sub": "C"}

	if got := destDirFor(tk, &baidu.ShareFile{Path: "/A/sub/y.txt"}); got != "/save/C" {
		t.Errorf("最深匹配应胜出: %q", got)
	}
	if got := destDirFor(tk, &baidu.ShareFile{Path: "/A/sub/nest/y.txt"}); got != "/save/C/nest" {
		t.Errorf("sub 深层文件应保留层级: %q", got)
	}
	if got := destDirFor(tk, &baidu.ShareFile{Path: "/A/x.txt"}); got != "/save/B" {
		t.Errorf("非 sub 文件应落 B: %q", got)
	}
}

func TestRunTask_FolderRenameTransfer(t *testing.T) {
	mc := &mockCli{shareDirs: map[string][]*baidu.ShareFile{
		"":      {dir(9, "/dir1"), dir(10, "/dir2")},
		"/dir1": {file(1, "/dir1/x.txt", "x1", 1)},
		"/dir2": {file(2, "/dir2/y.txt", "y1", 1)},
	}}
	e, database := newTestEngine(t, mc)
	tk := mkTask(1)
	tk.SaveDir = "/save"
	tk.FolderPaths = []string{"/dir1"}
	tk.FolderRenames = map[string]string{"/dir1": " renamed "} // 带空格应被 TrimSpace
	id, _ := database.CreateTask(tk)
	task, _ := database.GetTask(id)

	res, _ := e.RunTask(task)
	if res.NewFiles != 1 {
		t.Fatalf("应转存 1 个: %+v", res)
	}
	// dir1 的文件应转存到 /save/renamed（根级文件无子目录层级）
	// 通过 mkdirs 确认 /save/renamed 已创建
	found := false
	for _, m := range mc.mkdirs {
		if m == "/save/renamed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("应创建改名目录 /save/renamed: %v", mc.mkdirs)
	}
	// task_files 记录实际落盘路径
	var gotPath string
	if err := database.QueryRow(`SELECT path FROM task_files WHERE task_id=? AND md5='x1'`, task.ID).Scan(&gotPath); err != nil {
		t.Fatalf("查询落盘记录失败: %v", err)
	}
	if gotPath != "/save/renamed/x.txt" {
		t.Fatalf("落盘路径记录不符: %q", gotPath)
	}
}

func TestRenameTemplate(t *testing.T) {
	re := regexp.MustCompile(`第(\d+)课`)
	if got := renameTemplate(re, "第01课.mp4", "EP\\1.mp4"); got != "EP01.mp4" {
		t.Fatalf("\\1 模板失败: %q", got)
	}
	if got := renameTemplate(re, "第01课.mp4", "EP$1.mp4"); got != "EP01.mp4" {
		t.Fatalf("$1 模板失败: %q", got)
	}
	if got := renameTemplate(re, "nomatch.mp4", "EP\\1.mp4"); got != "" {
		t.Fatalf("不匹配应返回空: %q", got)
	}
}

func TestSplitBatches(t *testing.T) {
	files := make([]*baidu.ShareFile, 75)
	got := splitBatches(files, 50)
	if len(got) != 2 || len(got[0]) != 50 || len(got[1]) != 25 {
		t.Fatalf("切分错误: %v", []int{len(got), len(got[0]), len(got[1])})
	}
}

func TestClassifyErr(t *testing.T) {
	cases := map[string]ErrClass{
		"分享链接已失效":              ErrClassLinkDead,
		"提取码错误":                  ErrClassPwdWrong,
		"Cookie 缺少网盘 STOKEN":   ErrClassCookieBad,
		"网络错误: timeout":           ErrClassNetwork,
		"请求过快，请稍后再试":          ErrClassRateLimit,
		"未知错误":                    ErrClassOther,
	}
	for msg, want := range cases {
		if got := classifyErr(errors.New(msg)); got != want {
			t.Errorf("%q → %s, 期望 %s", msg, got, want)
		}
	}
	if got := classifyErr(baidu.ErrLinkInvalid); got != ErrClassLinkDead {
		t.Errorf("哨兵错误应映射 LinkDead, got %s", got)
	}
}

func TestWithRetry_NonRetryable(t *testing.T) {
	calls := 0
	err := withRetry(func() error {
		calls++
		return errors.New("提取码错误")
	})
	if calls != 1 || !strings.Contains(err.Error(), "提取码") {
		t.Fatalf("非瞬时错误不应重试: calls=%d", calls)
	}
}

func TestBuildCookieStr(t *testing.T) {
	a := &db.Account{BDUSS: "B1", STOKEN: "S1", CookiesJSON: `{"BAIDUID":"X"}`}
	got := buildCookieStr(a)
	for _, want := range []string{"BDUSS=B1", "STOKEN=S1", "BAIDUID=X"} {
		if !strings.Contains(got, want) {
			t.Fatalf("缺少 %s: %q", want, got)
		}
	}
}
