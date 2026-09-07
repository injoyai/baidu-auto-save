// Package baidu 封装与百度网盘的交互（底层复用 qjfoidnh/BaiduPCS-Go 的 baidupcs 包，
// 编排所需的分享目录遍历等能力自行实现），对上层暴露干净的接口以便测试 mock。
package baidu

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/qjfoidnh/BaiduPCS-Go/baidupcs"
	"github.com/qjfoidnh/BaiduPCS-Go/baidupcs/pcserror"
)

// ShareFile 分享目录中的文件/子目录条目
type ShareFile struct {
	FsID       int64  `json:"fs_id"`
	Path       string `json:"path"` // 分享内相对路径（从根目录起）
	ServerName string `json:"server_name"`
	MD5        string `json:"md5"`
	Size       int64  `json:"size"`
	IsDir      bool   `json:"is_dir"`
}

// Client 与百度网盘交互的抽象（供 engine mock）
type Client interface {
	// AccessSharePage 访问分享页，返回 shareid/share_uk/bdstoken
	AccessSharePage(surl string) (tokens map[string]string, err error)
	// VerifyPwd 提取码验证，成功返回 nil（内部已带 randsk 重访）
	VerifyPwd(surl string, tokens map[string]string, pwd string) error
	// ListShareDir 分享目录遍历：返回 dir 下条目（dir 为分享内相对路径，根目录为 ""）
	ListShareDir(surl, dir string) ([]*ShareFile, error)
	// Transfer 转存指定 fs_id 列表到 saveDir，返回百度 errno（0 成功）
	Transfer(fsids []int64, saveDir string) error
	// Rename 重命名（转存后的重命名两步实现）
	Rename(from, to string) error
	// Quota 查询容量
	Quota() (used, total int64, err error)
	// Mkdir 创建目录
	Mkdir(pcspath string) error
	// ListDir 列出自己网盘目录
	ListDir(path string) ([]*baidupcs.FileDirectory, error)
}

type client struct {
	pcs     *baidupcs.BaiduPCS
	surl    string // 1xxxxxxxx 形式的特征串
	referer string
}

// pcsAppID PCS 接口（quota/mkdir/rename 等）使用的 app_id。
// 网页版 app_id=250528（PanAppID）已被百度对纯 Cookie 请求封禁（31030 pcs token not exist），
// BaiduPCS-Go 默认的 266719 实测可用；share/* 网页接口仍使用 250528。
const pcsAppID = 266719

// NewClient 用 Cookie 构建客户端。cookieStr 形如 "BDUSS=xxx; STOKEN=yyy; ..."
func NewClient(cookieStr string) (Client, error) {
	pcs := baidupcs.NewPCSWithCookieStr(pcsAppID, cookieStr)
	return &client{pcs: pcs}, nil
}

func featureStr(shareURL string) (string, error) {
	u, err := url.Parse(shareURL)
	if err != nil {
		return "", fmt.Errorf("链接格式非法: %w", err)
	}
	fs := path_Base(strings.TrimSuffix(u.Path, "/"))
	if fs == "init" {
		fs = "1" + u.Query().Get("surl")
	}
	if len(fs) > 23 || !strings.HasPrefix(fs, "1") {
		return "", fmt.Errorf("链接地址非法")
	}
	return fs, nil
}

// path_Base 兼容封装，避免直接依赖 filepath（URL 路径应用 / 分隔）
func path_Base(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func (c *client) AccessSharePage(surl string) (map[string]string, error) {
	fs, err := featureStr(surl)
	if err != nil {
		return nil, err
	}
	c.surl = fs
	c.referer = "https://pan.baidu.com/s/" + fs
	tokens := c.pcs.AccessSharePage(fs, true)
	if tokens["ErrMsg"] != "0" {
		msg := tokens["ErrMsg"]
		if strings.Contains(msg, "已失效") || strings.Contains(msg, "页面不存在") {
			return nil, ErrLinkInvalid
		}
		if strings.Contains(msg, "STOKEN") {
			return nil, fmt.Errorf("Cookie 缺少网盘 STOKEN，请补充后重试")
		}
		return nil, fmt.Errorf("访问分享页失败: %s", msg)
	}
	return tokens, nil
}

func (c *client) VerifyPwd(surl string, tokens map[string]string, pwd string) error {
	if pwd == "" {
		return nil
	}
	verifyURL := c.pcs.GenerateShareQueryURL("verify", map[string]string{
		"shareid":    tokens["shareid"],
		"time":       strconv.FormatInt(time.Now().UnixMilli(), 10),
		"clienttype": "1",
		"uk":         tokens["share_uk"],
	}).String()
	res := c.pcs.PostShareQuery(verifyURL, c.referer, map[string]string{
		"pwd":       pwd,
		"vcode":     "null",
		"vcode_str": "null",
		"bdstoken":  tokens["bdstoken"],
	})
	if res["ErrMsg"] == "0" {
		return nil
	}
	if res["ErrMsg"] == "提取码错误" {
		return ErrPwdWrong
	}
	return fmt.Errorf("提取码验证失败: %s", res["ErrMsg"])
}

// shareListEntry /share/list 接口返回条目
// 注意：百度该接口 fs_id/size/isdir 可能返回字符串或数字，统一用 json.Number 兼容
type shareListEntry struct {
	FsID       json.Number `json:"fs_id"`
	Path       string      `json:"path"`
	ServerName string      `json:"server_filename"`
	MD5        string      `json:"md5"`
	Size       json.Number `json:"size"`
	Isdir      json.Number `json:"isdir"`
}

// ListShareDir 自研：调用网页版 /share/list 遍历分享目录
// （上游 baidupcs 包无此实现，参考 web 端行为：root=1 取根，否则 dir=分享内路径）
func (c *client) ListShareDir(surl, dir string) ([]*ShareFile, error) {
	listURL := c.pcs.GenerateShareQueryURL("list", map[string]string{
		"app_id":     baidupcs.PanAppID,
		"channel":    "chunlei",
		"clienttype": "0",
		"web":        "1",
		"num":        "1000",
		"shorturl":   c.surl[1:],
	})
	if dir == "" {
		listURL.RawQuery = withParam(listURL.RawQuery, "root", "1")
	} else {
		listURL.RawQuery = withParam(listURL.RawQuery, "dir", dir)
	}

	body, err := c.getJSON(listURL.String())
	if err != nil {
		return nil, err
	}
	errno := jsonGet(body, "errno").Int()
	if errno != 0 {
		return nil, fmt.Errorf("分享目录遍历失败 errno=%d", errno)
	}
	listRaw := jsonGet(body, "list").Raw()
	var entries []shareListEntry
	if err := json.Unmarshal([]byte(listRaw), &entries); err != nil {
		return nil, fmt.Errorf("解析分享目录失败: %w", err)
	}
	out := make([]*ShareFile, 0, len(entries))
	for _, e := range entries {
		fsID, _ := e.FsID.Int64()
		size, _ := e.Size.Int64()
		isdir, _ := e.Isdir.Int64()
		out = append(out, &ShareFile{
			FsID:       fsID,
			Path:       e.Path,
			ServerName: e.ServerName,
			MD5:        e.MD5,
			Size:       size,
			IsDir:      isdir == 1,
		})
	}
	return out, nil
}

func (c *client) getJSON(rawURL string) (string, error) {
	resp, err := c.pcs.GetClient().Req(http.MethodGet, rawURL, nil, map[string]string{
		"User-Agent": "netdisk",
		"Referer":    c.referer,
	})
	if err != nil {
		return "", fmt.Errorf("网络错误: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func withParam(rawQuery, k, v string) string {
	q, _ := url.ParseQuery(rawQuery)
	q.Set(k, v)
	return q.Encode()
}

// jsonGet 避免直接引入 gjson 的小工具：解析顶层 json 并按点路径取值
// 实现为最简实现，仅支持对象成员访问
func jsonGet(body, path string) jsonResult {
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return jsonResult{}
	}
	cur := v
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return jsonResult{}
		}
		cur, ok = m[seg]
		if !ok {
			return jsonResult{}
		}
	}
	return jsonResult{v: cur}
}

type jsonResult struct{ v any }

func (r jsonResult) Int() int64 {
	switch n := r.v.(type) {
	case float64:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	}
	return 0
}

func (r jsonResult) Raw() string {
	if r.v == nil {
		return "null"
	}
	b, _ := json.Marshal(r.v)
	return string(b)
}

func (c *client) Transfer(fsids []int64, saveDir string) error {
	if len(fsids) == 0 {
		return nil
	}
	tokens := c.pcs.AccessSharePage(c.surl, false)
	if tokens["ErrMsg"] != "0" {
		if strings.Contains(tokens["ErrMsg"], "已失效") || strings.Contains(tokens["ErrMsg"], "页面不存在") {
			return ErrLinkInvalid
		}
		return fmt.Errorf("访问分享页失败: %s", tokens["ErrMsg"])
	}
	fsidStr := make([]string, 0, len(fsids))
	for _, id := range fsids {
		fsidStr = append(fsidStr, strconv.FormatInt(id, 10))
	}
	transMetas := map[string]string{
		"bdstoken": tokens["bdstoken"],
		"from":     tokens["share_uk"],
		"shareid":  tokens["shareid"],
		"fs_id":    "[" + strings.Join(fsidStr, ",") + "]",
		"path":     saveDir,
		"referer":  c.referer,
	}
	// 上游 GenerateRequestQuery 通过 transMetas["shareUrl"] 携带转存请求地址，
	// fs_id/path 等作为 POST body；与 internal/pcscommand 用法一致
	shareURL := &url.URL{
		Scheme: "https",
		Host:   "pan.baidu.com",
		Path:   "/share/transfer",
	}
	uv := shareURL.Query()
	uv.Set("app_id", baidupcs.PanAppID)
	uv.Set("channel", "chunlei")
	uv.Set("clienttype", "0")
	uv.Set("web", "1")
	for k, v := range transMetas {
		uv.Set(k, v)
	}
	shareURL.RawQuery = uv.Encode()
	transMetas["shareUrl"] = shareURL.String()

	reqResult := c.pcs.GenerateRequestQuery("POST", transMetas)

	switch {
	case reqResult["ErrNo"] == "0":
		return nil
	case reqResult["ErrNo"] == "4", strings.Contains(reqResult["ErrMsg"], "文件重复"), strings.Contains(reqResult["ErrMsg"], "已存在"):
		// 文件重复/已存在 → 视作已存在（目标目录已有同名或同内容文件，百度整批拒绝时 ErrMsg
		// 为「文件重复」而 errno 非 4；去重兜底场景，等价跳过）
		return nil
	default:
		return fmt.Errorf("%s", reqResult["ErrMsg"])
	}
}

func (c *client) Rename(from, to string) error {
	return c.pcs.Rename(from, to)
}

func (c *client) Quota() (int64, int64, error) {
	// 上游 QuotaInfo 返回顺序为 (总量, 已用)，适配本接口 (used, total) 约定
	total, used, err := c.pcs.QuotaInfo()
	return used, total, err
}

func (c *client) Mkdir(pcspath string) error {
	return c.pcs.Mkdir(pcspath)
}

func (c *client) ListDir(path string) ([]*baidupcs.FileDirectory, error) {
	data, pcsErr := c.pcs.FilesDirectoriesList(path, nil)
	if pcsErr != nil {
		// pcserror.Error → error 适配
		if pe, ok := pcsErr.(pcserror.Error); ok && pe != nil {
			return nil, pe
		}
		return nil, pcsErr
	}
	return data, nil
}

// IsNotExist 判断错误是否为「文件或目录不存在」类错误（用于区分目录被删除与网络等临时故障）
func IsNotExist(err error) bool {
	if err == nil {
		return false
	}
	if pe, ok := err.(pcserror.Error); ok && pe != nil && pe.GetErrType() == pcserror.ErrTypeRemoteError {
		switch pe.GetRemoteErrCode() {
		case -9, 12, 31066: // pan api 与 pcs api 的「文件/目录不存在」错误码
			return true
		}
	}
	return strings.Contains(err.Error(), "不存在")
}

// 领域错误（供编排层分类处理）
var (
	ErrLinkInvalid = fmt.Errorf("分享链接已失效")
	ErrPwdWrong    = fmt.Errorf("提取码错误")
)
