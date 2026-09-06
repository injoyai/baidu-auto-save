package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"baidu-auto-save/internal/db"
	"baidu-auto-save/internal/scheduler"
)

// errStop 内部哨兵：pathID* 已写响应，调用方直接返回
var errStop = errors.New("stop")

// parseID 解析路径参数 :id
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	return id, err == nil && id > 0
}

// pathIDTask 解析 :id 并加载任务，出错时已写响应
func (s *Server) pathIDTask(c *gin.Context) (*db.Task, bool) {
	id, okID := parseID(c)
	if !okID {
		failBadRequest(c, "无效的 id")
		return nil, false
	}
	t, err := s.db.GetTask(id)
	if err != nil {
		failErr(c, err)
		return nil, false
	}
	if t == nil {
		failNotFound(c, "任务不存在")
		return nil, false
	}
	return t, true
}

type taskReq struct {
	Name                string            `json:"name"`
	AccountID           int64             `json:"account_id"`
	ShareURL            string            `json:"share_url"`
	Pwd                 string            `json:"pwd"`
	SaveDir             string            `json:"save_dir"`
	FolderPaths         []string          `json:"folder_paths"`
	FolderRenames       map[string]string `json:"folder_renames"`
	FolderFilter        string            `json:"folder_filter"`
	ExcludeFolderFilter string            `json:"exclude_folder_filter"`
	RegexPattern        string            `json:"regex_pattern"`
	RegexReplace        string            `json:"regex_replace"`
	CronExpr            string            `json:"cron_expr"`
	Enabled             *bool             `json:"enabled"`
}

func (r *taskReq) toTask(t *db.Task) *db.Task {
	t.Name = r.Name
	t.AccountID = r.AccountID
	t.ShareURL = r.ShareURL
	t.Pwd = r.Pwd
	if r.SaveDir == "" {
		r.SaveDir = "/来自：分享"
	}
	t.SaveDir = r.SaveDir
	if r.FolderPaths == nil {
		r.FolderPaths = []string{}
	}
	t.FolderPaths = r.FolderPaths
	if r.FolderRenames == nil {
		r.FolderRenames = map[string]string{}
	}
	t.FolderRenames = r.FolderRenames
	t.FolderFilter = r.FolderFilter
	t.ExcludeFolderFilter = r.ExcludeFolderFilter
	t.RegexPattern = r.RegexPattern
	t.RegexReplace = r.RegexReplace
	t.CronExpr = r.CronExpr
	if r.Enabled != nil {
		t.Enabled = *r.Enabled
	}
	return t
}

// validateTask 创建/更新任务公共校验
func (s *Server) validateTask(t *db.Task) string {
	if t.Name == "" {
		return "任务名称不能为空"
	}
	if t.ShareURL == "" || !isBaiduShareURL(t.ShareURL) {
		return "分享链接格式非法"
	}
	if t.AccountID <= 0 {
		return "必须选择转存账号"
	}
	acc, err := s.db.GetAccount(t.AccountID)
	if err != nil {
		return "查询账号失败: " + err.Error()
	}
	if acc == nil {
		return "账号不存在"
	}
	if err := scheduler.ValidateCron(t.CronExpr); err != nil {
		return "cron 表达式非法（5 段格式，如 0 8 * * *；留空表示仅手动执行）"
	}
	return ""
}

func isBaiduShareURL(u string) bool {
	return len(u) >= len("https://pan.baidu.com/s/") && contains(u, "pan.baidu.com/s/")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// registerTaskRoutes 任务端点
func (s *Server) registerTaskRoutes(g *gin.RouterGroup) {
	g.GET("/tasks", s.listTasks)
	g.POST("/tasks", s.createTask)
	g.PUT("/tasks/:id", s.updateTask)
	g.DELETE("/tasks/:id", s.deleteTask)
	g.POST("/tasks/:id/run", s.runTask)
	g.POST("/tasks/:id/toggle", s.toggleTask)
	g.GET("/tasks/:id/logs", s.taskLogs)
	g.GET("/tasks/:id/status", s.taskStatus)
	g.GET("/logs", s.allLogs)
}

func (s *Server) listTasks(c *gin.Context) {
	list, err := s.db.ListTasks()
	if err != nil {
		failErr(c, err)
		return
	}
	if list == nil {
		list = []*db.Task{}
	}
	ok(c, list)
}

func (s *Server) createTask(c *gin.Context) {
	var req taskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: "+err.Error())
		return
	}
	t := req.toTask(&db.Task{Enabled: true})
	if msg := s.validateTask(t); msg != "" {
		failBadRequest(c, msg)
		return
	}
	id, err := s.db.CreateTask(t)
	if err != nil {
		failErr(c, err)
		return
	}
	nt, _ := s.db.GetTask(id)
	if nt != nil {
		s.sched.AddTask(nt)
	}
	ok(c, gin.H{"id": id})
}

func (s *Server) updateTask(c *gin.Context) {
	t, okT := s.pathIDTask(c)
	if !okT {
		return
	}
	var req taskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.toTask(t)
	if msg := s.validateTask(t); msg != "" {
		failBadRequest(c, msg)
		return
	}
	if err := s.db.UpdateTask(t); err != nil {
		failErr(c, err)
		return
	}
	s.sched.AddTask(t)
	ok(c, nil)
}

func (s *Server) deleteTask(c *gin.Context) {
	t, okT := s.pathIDTask(c)
	if !okT {
		return
	}
	s.sched.RemoveTask(t.ID)
	if err := s.db.DeleteTask(t.ID); err != nil {
		failErr(c, err)
		return
	}
	ok(c, nil)
}

// runTask 手动触发一次（异步），返回是否提交
func (s *Server) runTask(c *gin.Context) {
	t, okT := s.pathIDTask(c)
	if !okT {
		return
	}
	if !s.sched.RunNow(t.ID) {
		failBadRequest(c, "任务正在执行中，请稍后再试")
		return
	}
	ok(c, gin.H{"started": true})
}

// taskStatus 任务实时运行态（供详情弹窗轮询）
func (s *Server) taskStatus(c *gin.Context) {
	t, okT := s.pathIDTask(c)
	if !okT {
		return
	}
	lv := s.engine.Status(t.ID)
	out := gin.H{
		"running":  s.sched.Running(t.ID),
		"dbStatus": t.Status,
		"stage":    "",
		"done":     0,
		"total":    0,
		"result":   "",
		"logs":     []string{},
	}
	if lv != nil {
		logs := lv.Logs
		if logs == nil {
			logs = []string{}
		}
		out["stage"] = lv.Stage
		out["done"] = lv.Done
		out["total"] = lv.Total
		out["result"] = lv.Result
		out["resultMsg"] = lv.ResultMsg
		out["logs"] = logs
		out["finished"] = lv.Finished
	}
	ok(c, out)
}

func (s *Server) toggleTask(c *gin.Context) {
	t, okT := s.pathIDTask(c)
	if !okT {
		return
	}
	nt := !t.Enabled
	if err := s.db.SetTaskEnabled(t.ID, nt); err != nil {
		failErr(c, err)
		return
	}
	t.Enabled = nt
	s.sched.AddTask(t)
	ok(c, gin.H{"enabled": nt})
}

// taskLogs 任务日志分页（?page=1&size=20）
func (s *Server) taskLogs(c *gin.Context) {
	t, okT := s.pathIDTask(c)
	if !okT {
		return
	}
	s.serveLogs(c, t.ID)
}

// allLogs 全部日志分页（转存日志页未选任务时）
func (s *Server) allLogs(c *gin.Context) {
	s.serveLogs(c, 0)
}

func (s *Server) serveLogs(c *gin.Context, taskID int64) {
	page, size := paging(c)
	logs, err := s.db.ListTransferLogs(taskID, (page-1)*size, size)
	if err != nil {
		failErr(c, err)
		return
	}
	total, err := s.db.CountTransferLogs(taskID)
	if err != nil {
		failErr(c, err)
		return
	}
	ok(c, gin.H{"list": logs, "total": total, "page": page, "size": size})
}

// paging 解析分页参数（默认 page=1 size=20，上限 100）
func paging(c *gin.Context) (page, size int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ = strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}
