package api

import (
	"github.com/gin-gonic/gin"

	"baidu-auto-save/internal/db"
	"baidu-auto-save/internal/engine"
	"baidu-auto-save/internal/notify"
	"baidu-auto-save/internal/scheduler"
)

// Server API 服务器：聚合各依赖并注册路由
type Server struct {
	db       *db.DB
	engine   *engine.Engine
	sched    *scheduler.Scheduler
	notifier *notify.Notifier
	password string // 登录密码（来自配置）
	r        *gin.Engine
}

func NewServer(database *db.DB, eng *engine.Engine, sched *scheduler.Scheduler, n *notify.Notifier, password string) *Server {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	return &Server{
		db: database, engine: eng, sched: sched, notifier: n, password: password, r: r,
	}
}

// Register 注册全部路由（含静态资源，由 main 注入 handler）
func (s *Server) Register(staticHandler gin.HandlerFunc) {
	s.r.NoRoute(staticHandler)

	v1 := s.r.Group("/api/v1")

	// 登录（无认证）；密码为空时 main 启动校验已拦截，不会走到这里
	a := newAuth(s.password)
	v1.POST("/login", a.login)

	authed := v1.Group("")
	authed.Use(a.middleware())
	s.registerAccountRoutes(authed)
	s.registerTaskRoutes(authed)
	s.registerShareRoutes(authed)
	s.registerSettingRoutes(authed)
	s.registerDashboardRoutes(authed)

	// 开放推送（X-API-Token）
	s.registerOpenRoutes()
}

// Run 启动 HTTP 服务
func (s *Server) Run(addr string) error {
	return s.r.Run(addr)
}
