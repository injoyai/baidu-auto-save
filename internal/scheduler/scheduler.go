// Package scheduler 调度器：基于 cron 管理任务调度，触发转存引擎执行
package scheduler

import (
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"baidu-auto-save/internal/db"
	"baidu-auto-save/internal/engine"
	"baidu-auto-save/internal/notify"
)

// Scheduler 动态管理各任务的 cron 调度
type Scheduler struct {
	db     *db.DB
	engine *engine.Engine
	notify *notify.Notifier

	c       *cron.Cron
	mu      sync.Mutex
	entries map[int64]cron.EntryID // taskID → cron entry
	running map[int64]bool         // 执行中任务（防止重复触发）
}

func New(database *db.DB, eng *engine.Engine, n *notify.Notifier) *Scheduler {
	return &Scheduler{
		db:      database,
		engine:  eng,
		notify:  n,
		c:       cron.New(),
		entries: map[int64]cron.EntryID{},
		running: map[int64]bool{},
	}
}

// Start 启动调度：恢复中断状态并加载全部启用任务
func (s *Scheduler) Start() {
	// 上次进程退出时可能遗留 running 状态
	if _, err := s.db.Exec(`UPDATE tasks SET status='idle' WHERE status='running'`); err != nil {
		log.Printf("[scheduler] 恢复任务状态失败: %v", err)
	}
	tasks, err := s.db.ListEnabledTasks()
	if err != nil {
		log.Printf("[scheduler] 加载任务失败: %v", err)
	}
	for _, t := range tasks {
		s.AddTask(t)
	}
	s.c.Start()
	log.Printf("[scheduler] 已启动，注册 %d 个任务", len(tasks))
}

// Stop 停止调度并等待执行中的任务结束
func (s *Scheduler) Stop() {
	ctx := s.c.Stop()
	<-ctx.Done()
}

// ValidateCron 校验 cron 表达式（空值合法 = 仅手动执行）
func ValidateCron(expr string) error {
	if expr == "" {
		return nil
	}
	_, err := cron.ParseStandard(expr)
	return err
}

// NextRuns 返回表达式接下来 n 次触发时间（本地时区）；表达式非法返回 nil
func NextRuns(expr string, n int) []time.Time {
	sched, err := cron.ParseStandard(expr)
	if err != nil || n <= 0 {
		return nil
	}
	out := make([]time.Time, 0, n)
	next := sched.Next(time.Now())
	for i := 0; i < n; i++ {
		out = append(out, next)
		next = sched.Next(next)
	}
	return out
}

// AddTask 注册或重建单个任务的调度（任务创建/更新/启停后调用）
func (s *Scheduler) AddTask(t *db.Task) {
	s.RemoveTask(t.ID)
	if !t.Enabled || t.CronExpr == "" {
		// 仅手动执行：清空下次运行时间
		_ = s.db.SetTaskNextRun(t.ID, nil)
		return
	}
	entryID, err := s.c.AddFunc(t.CronExpr, func() { s.launch(t.ID) })
	if err != nil {
		log.Printf("[scheduler] 任务 %d cron 表达式非法 %q: %v", t.ID, t.CronExpr, err)
		_ = s.db.SetTaskStatus(t.ID, "error")
		return
	}
	s.mu.Lock()
	s.entries[t.ID] = entryID
	s.mu.Unlock()
	s.updateNextRun(t.ID, entryID)
}

// RemoveTask 移除调度（任务删除/禁用后调用）
func (s *Scheduler) RemoveTask(taskID int64) {
	s.mu.Lock()
	entryID, ok := s.entries[taskID]
	if ok {
		delete(s.entries, taskID)
	}
	s.mu.Unlock()
	if ok {
		s.c.Remove(entryID)
	}
}

// RunNow 手动立即执行；任务已在执行中返回 false
func (s *Scheduler) RunNow(taskID int64) bool {
	return s.launch(taskID)
}

// Running 任务是否执行中
func (s *Scheduler) Running(taskID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[taskID]
}

// launch 若任务空闲则异步执行，返回是否提交
func (s *Scheduler) launch(taskID int64) bool {
	s.mu.Lock()
	if s.running[taskID] {
		s.mu.Unlock()
		log.Printf("[scheduler] 任务 %d 正在执行，跳过本次触发", taskID)
		return false
	}
	s.running[taskID] = true
	s.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[scheduler] 任务 %d panic: %v", taskID, r)
				_ = s.db.SetTaskStatus(taskID, "error")
			}
			s.mu.Lock()
			delete(s.running, taskID)
			s.mu.Unlock()
			// 刷新下次运行时间
			s.mu.Lock()
			entryID, ok := s.entries[taskID]
			s.mu.Unlock()
			if ok {
				s.updateNextRun(taskID, entryID)
			}
		}()
		t, err := s.db.GetTask(taskID)
		if err != nil || t == nil {
			return
		}
		s.execute(t)
	}()
	return true
}

// execute 执行一次任务并落状态、发通知（cron 与手动触发共用）
func (s *Scheduler) execute(t *db.Task) {
	now := time.Now()
	_ = s.db.SetTaskRunInfo(t.ID, "running", &now, nil)

	res, err := s.engine.RunTask(t)
	if err != nil {
		_ = s.db.SetTaskStatus(t.ID, "error")
		s.notify.Send(notify.EventFailed, "任务「"+t.Name+"」执行异常", err.Error())
		return
	}

	switch res.ErrClass {
	case engine.ErrClassNone:
		_ = s.db.SetTaskStatus(t.ID, "idle")
		if res.NewFiles > 0 {
			s.notify.Send(notify.EventSuccess, "任务「"+t.Name+"」转存完成", res.Message)
		}
	case engine.ErrClassCookieBad:
		_ = s.db.SetTaskStatus(t.ID, "error")
		_ = s.db.SetAccountStatus(t.AccountID, "invalid")
		s.notify.Send(notify.EventCookieInvalid, "账号 Cookie 失效",
			"任务「"+t.Name+"」执行时发现账号 Cookie 已失效，请尽快到账号页更新。")
	case engine.ErrClassLinkDead:
		_ = s.db.SetTaskStatus(t.ID, "link_invalid")
		s.notify.Send(notify.EventFailed, "任务「"+t.Name+"」分享链接失效", res.Message)
	default:
		_ = s.db.SetTaskStatus(t.ID, "error")
		s.notify.Send(notify.EventFailed, "任务「"+t.Name+"」转存失败", res.Message)
	}
}

// updateNextRun 根据 cron 计划刷新任务的下次运行时间
func (s *Scheduler) updateNextRun(taskID int64, entryID cron.EntryID) {
	e := s.c.Entry(entryID)
	if e.Schedule == nil {
		return
	}
	next := e.Schedule.Next(time.Now())
	_ = s.db.SetTaskNextRun(taskID, &next)
}
