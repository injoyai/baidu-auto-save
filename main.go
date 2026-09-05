// baidu-auto-save：百度网盘分享自动转存服务
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"baidu-auto-save/internal/api"
	"baidu-auto-save/internal/config"
	"baidu-auto-save/internal/db"
	"baidu-auto-save/internal/engine"
	"baidu-auto-save/internal/notify"
	"baidu-auto-save/internal/scheduler"
)

//go:embed all:web/dist
var webFS embed.FS

func main() {
	// 加载配置：config/config.yaml 不存在时自动生成模板并退出提示
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	cfg.ApplyEnv() // 环境变量覆盖（Docker 场景）
	if err := cfg.ApplyDefaults(); err != nil {
		log.Fatal(err)
	}

	// 数据目录
	if err := os.MkdirAll(cfg.Storage.DataDir, 0o755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}
	database, err := db.Open(filepath.Join(cfg.Storage.DataDir, "baidusave.db"))
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer database.Close()

	wrapper := &db.DB{DB: database}

	// 通知器 → 引擎 → 调度器
	notifier := notify.New(wrapper.GetSetting)
	eng := engine.New(wrapper)
	sched := scheduler.New(wrapper, eng, notifier)

	srv := api.NewServer(wrapper, eng, sched, notifier, cfg.Auth.Password)
	srv.Register(staticHandler())

	sched.Start()
	log.Printf("baidu-auto-save 已启动，监听 %s", cfg.Server.Addr)
	if err := srv.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}

// staticHandler 返回 SPA 静态资源（embed），未知路径回退 index.html
func staticHandler() gin.HandlerFunc {
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		log.Fatalf("嵌入前端资源不可用: %v", err)
	}
	httpFS := http.FS(sub)
	fileServer := http.StripPrefix("/", http.FileServer(httpFS))
	return func(c *gin.Context) {
		p := strings.TrimPrefix(c.Request.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			// SPA 路由回退
			c.Request.URL.Path = "/"
		}
		// index.html 禁缓存（引用带哈希的 js/css）；带哈希资源长缓存
		if strings.HasSuffix(c.Request.URL.Path, "index.html") || c.Request.URL.Path == "/" {
			c.Header("Cache-Control", "no-cache")
		} else if strings.HasPrefix(filepath.Base(c.Request.URL.Path), "index-") ||
			strings.Contains(c.Request.URL.Path, "/static/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}
