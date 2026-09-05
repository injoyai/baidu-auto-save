package api

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"baidu-auto-save/internal/db"
	"baidu-auto-save/internal/scheduler"
)

// registerOpenRoutes 开放推送端点（X-API-Token 认证，外部工具调用）
func (s *Server) registerOpenRoutes() *gin.RouterGroup {
	g := s.r.Group("/open")
	g.Use(s.openTokenAuth())
	g.POST("/links", s.openPush)
	return g
}

// openTokenAuth 校验 X-API-Token 头（常量时间比较）
func (s *Server) openTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := s.db.GetSetting(db.KeyOpenAPIToken)
		if token == "" {
			fail(c, http.StatusServiceUnavailable, 1004, "未配置开放推送 Token，请先到设置页开启")
			c.Abort()
			return
		}
		got := c.GetHeader("X-API-Token")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			fail(c, http.StatusUnauthorized, 1003, "Token 无效")
			c.Abort()
			return
		}
		c.Next()
	}
}

type openPushReq struct {
	URL     string `json:"url"`
	Pwd     string `json:"pwd"`
	SaveDir string `json:"save_dir"`
	// 可选：覆盖默认 cron（默认每日 08:00）
	CronExpr string `json:"cron_expr"`
}

// openPush 创建任务并立即执行一次（设计文档 §6 推送语义）
func (s *Server) openPush(c *gin.Context) {
	var req openPushReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: "+err.Error())
		return
	}
	if req.URL == "" || !isBaiduShareURL(req.URL) {
		failBadRequest(c, "url 必须是百度网盘分享链接")
		return
	}
	if req.CronExpr == "" {
		req.CronExpr = "0 8 * * *"
	}
	if err := scheduler.ValidateCron(req.CronExpr); err != nil {
		failBadRequest(c, "cron_expr 非法")
		return
	}
	saveDir := req.SaveDir
	if saveDir == "" {
		saveDir, _ = s.db.GetSetting(db.KeyGlobalSaveDir)
	}
	if saveDir == "" {
		saveDir = "/来自：分享"
	}
	// 取第一个可用账号（推送场景通常单账号）
	accounts, err := s.db.ListAccounts()
	if err != nil {
		failErr(c, err)
		return
	}
	if len(accounts) == 0 {
		failBadRequest(c, "尚未添加任何转存账号，请先在网页端添加")
		return
	}
	id, err := s.db.CreateTask(&db.Task{
		Name:     "推送-" + time.Now().Format("0102-1504"),
		AccountID: accounts[0].ID,
		ShareURL: req.URL,
		Pwd:      req.Pwd,
		SaveDir:  saveDir,
		CronExpr: req.CronExpr,
		Enabled:  true,
	})
	if err != nil {
		failErr(c, err)
		return
	}
	nt, _ := s.db.GetTask(id)
	if nt != nil {
		s.sched.AddTask(nt)
		s.sched.RunNow(id)
	}
	ok(c, gin.H{"task_id": id})
}

var _ = strconv.Itoa
var _ = strings.TrimSpace
