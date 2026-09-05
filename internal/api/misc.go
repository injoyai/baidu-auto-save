package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"baidu-auto-save/internal/baidu"
	"baidu-auto-save/internal/db"
	"baidu-auto-save/internal/engine"
	"baidu-auto-save/internal/scheduler"
)

// registerShareRoutes 分享预览端点（任务向导：懒加载目录树）
func (s *Server) registerShareRoutes(g *gin.RouterGroup) {
	g.POST("/shares/preview", s.sharePreview)
	g.POST("/cron/preview", s.cronPreview)
}

type sharePreviewReq struct {
	ShareURL string `json:"share_url"`
	Pwd      string `json:"pwd"`
	AccountID int64  `json:"account_id"`
	// Path 非空时仅返回该子目录内容（懒加载），为空返回根目录
	Path string `json:"path"`
}

type shareNode struct {
	FsID       int64  `json:"fs_id"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	MD5        string `json:"md5"`
	Size       int64  `json:"size"`
	IsDir      bool   `json:"is_dir"`
	HasChildren bool  `json:"has_children"`
}

// sharePreview 解析分享链接，返回指定层级目录内容
func (s *Server) sharePreview(c *gin.Context) {
	var req sharePreviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: "+err.Error())
		return
	}
	if req.ShareURL == "" || !isBaiduShareURL(req.ShareURL) {
		failBadRequest(c, "分享链接格式非法")
		return
	}
	acc, err := s.db.GetAccount(req.AccountID)
	if err != nil {
		failErr(c, err)
		return
	}
	if acc == nil {
		failBadRequest(c, "请先选择账号")
		return
	}
	cli, err := baidu.NewClient(engine.BuildCookieStr(acc))
	if err != nil {
		failErr(c, err)
		return
	}
	tokens, err := cli.AccessSharePage(req.ShareURL)
	if err != nil {
		respondBaiduErr(c, err)
		return
	}
	if err := cli.VerifyPwd(req.ShareURL, tokens, req.Pwd); err != nil {
		respondBaiduErr(c, err)
		return
	}
	entries, err := cli.ListShareDir(req.ShareURL, req.Path)
	if err != nil {
		respondBaiduErr(c, err)
		return
	}
	nodes := make([]shareNode, 0, len(entries))
	for _, e := range entries {
		nodes = append(nodes, shareNode{
			FsID: e.FsID, Path: e.Path, Name: e.ServerName,
			MD5: e.MD5, Size: e.Size, IsDir: e.IsDir,
			HasChildren: e.IsDir,
		})
	}
	ok(c, gin.H{"list": nodes})
}

// respondBaiduErr 把百度侧错误映射为可读的 HTTP 响应
func respondBaiduErr(c *gin.Context, err error) {
	switch err {
	case baidu.ErrLinkInvalid:
		failBadRequest(c, err.Error())
	case baidu.ErrPwdWrong:
		failBadRequest(c, err.Error())
	default:
		msg := err.Error()
		if strings.Contains(msg, "提取码错误") {
			failBadRequest(c, "提取码错误")
			return
		}
		if strings.Contains(msg, "STOKEN") || strings.Contains(msg, "Cookie") {
			failBadRequest(c, msg)
			return
		}
		failErr(c, err)
	}
}

// registerDashboardRoutes 仪表盘统计
func (s *Server) registerDashboardRoutes(g *gin.RouterGroup) {
	g.GET("/dashboard", s.dashboard)
}

// cronPreviewReq cron 预览请求
type cronPreviewReq struct {
	Expr string `json:"expr"`
}

// cronPreview 返回 cron 表达式未来 5 次触发时间（空表达式返回空列表）
func (s *Server) cronPreview(c *gin.Context) {
	var req cronPreviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误")
		return
	}
	if req.Expr == "" {
		ok(c, gin.H{"valid": true, "next": []string{}})
		return
	}
	if err := scheduler.ValidateCron(req.Expr); err != nil {
		ok(c, gin.H{"valid": false, "next": []string{}})
		return
	}
	times := scheduler.NextRuns(req.Expr, 5)
	strs := make([]string, len(times))
	for i, t := range times {
		strs[i] = t.Format("2006-01-02 15:04:05")
	}
	ok(c, gin.H{"valid": true, "next": strs})
}

type dashboardData struct {
	Accounts     []*db.Account     `json:"accounts"`
	TaskStatus   map[string]int64  `json:"task_status"`
	Recent7Days  int64             `json:"recent_7d_files"`
	RecentLogs   []*db.TransferLog `json:"recent_logs"`
}

func (s *Server) dashboard(c *gin.Context) {
	data := dashboardData{}
	var err error
	if data.Accounts, err = s.db.ListAccounts(); err != nil {
		failErr(c, err)
		return
	}
	if data.TaskStatus, err = s.db.StatusCount(); err != nil {
		failErr(c, err)
		return
	}
	if data.Recent7Days, err = s.db.RecentTransferCount(7); err != nil {
		failErr(c, err)
		return
	}
	if data.RecentLogs, err = s.db.ListTransferLogs(0, 0, 10); err != nil {
		failErr(c, err)
		return
	}
	if data.RecentLogs == nil {
		data.RecentLogs = []*db.TransferLog{}
	}
	ok(c, data)
}

// registerSettingRoutes 设置端点
func (s *Server) registerSettingRoutes(g *gin.RouterGroup) {
	g.GET("/settings", s.getSettings)
	g.PUT("/settings", s.putSettings)
	g.POST("/settings/notify/test", s.testNotify)
}

// settingKeys 可通过 API 修改的设置键（与 db 常量对应）
var settingKeys = []string{
	db.KeyNotifyWecomWebhook, db.KeyNotifyServerChanKey,
	db.KeyNotifyTelegramToken, db.KeyNotifyTelegramChatID,
	db.KeyNotifyCustomWebhook, db.KeyNotifyEvents,
	db.KeyOpenAPIToken, db.KeyGlobalSaveDir,
}

func (s *Server) getSettings(c *gin.Context) {
	all, err := s.db.GetAllSettings()
	if err != nil {
		failErr(c, err)
		return
	}
	out := make(map[string]string, len(settingKeys))
	for _, k := range settingKeys {
		out[k] = all[k]
	}
	// Token 仅返回是否已设置
	if v := out[db.KeyOpenAPIToken]; v != "" {
		out[db.KeyOpenAPIToken] = "已设置"
	}
	ok(c, out)
}

func (s *Server) putSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: "+err.Error())
		return
	}
	valid := map[string]bool{}
	for _, k := range settingKeys {
		valid[k] = true
	}
	for k, v := range req {
		if !valid[k] {
			failBadRequest(c, "未知的设置键: "+k)
			return
		}
		// Token 占位值不覆盖真实值
		if k == db.KeyOpenAPIToken && v == "已设置" {
			continue
		}
		if err := s.db.SetSetting(k, strings.TrimSpace(v)); err != nil {
			failErr(c, err)
			return
		}
	}
	ok(c, nil)
}

func (s *Server) testNotify(c *gin.Context) {
	s.notifier.SendTest()
	// Send 是异步的，稍作等待让响应更接近真实结果
	time.Sleep(200 * time.Millisecond)
	ok(c, nil)
}

// 供日志分页参数转换复用
var _ = strconv.Itoa
