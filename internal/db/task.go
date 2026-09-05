package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Task 转存任务
type Task struct {
	ID                  int64            `json:"id"`
	Name                string           `json:"name"`
	AccountID           int64            `json:"account_id"`
	ShareURL            string           `json:"share_url"`
	Pwd                 string           `json:"pwd"`
	SaveDir             string           `json:"save_dir"`
	FolderPaths         []string         `json:"folder_paths"`
	FolderRenames       map[string]string `json:"folder_renames"` // 勾选目录 → 转存后的文件夹名（空值=不改名）
	FolderFilter        string           `json:"folder_filter"`
	ExcludeFolderFilter string           `json:"exclude_folder_filter"`
	RegexPattern        string           `json:"regex_pattern"`
	RegexReplace        string           `json:"regex_replace"`
	CronExpr            string           `json:"cron_expr"`
	Enabled             bool             `json:"enabled"`
	Status              string           `json:"status"` // idle | running | error | link_invalid
	LastRunAt           *time.Time       `json:"last_run_at"`
	NextRunAt           *time.Time       `json:"next_run_at"`
	CreatedAt           time.Time        `json:"created_at"`
}

const taskColumns = `id, name, account_id, share_url, pwd, save_dir, folder_paths, folder_renames, folder_filter,
	exclude_folder_filter, regex_pattern, regex_replace, cron_expr, enabled, status, last_run_at, next_run_at, created_at`

func scanTask(row interface{ Scan(...any) error }) (*Task, error) {
	var t Task
	var folderPaths, folderRenames, enabled string
	var lastRun, nextRun sql.NullTime
	if err := row.Scan(&t.ID, &t.Name, &t.AccountID, &t.ShareURL, &t.Pwd, &t.SaveDir,
		&folderPaths, &folderRenames, &t.FolderFilter, &t.ExcludeFolderFilter,
		&t.RegexPattern, &t.RegexReplace, &t.CronExpr, &enabled, &t.Status,
		&lastRun, &nextRun, &t.CreatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(folderPaths), &t.FolderPaths)
	if t.FolderPaths == nil {
		t.FolderPaths = []string{}
	}
	_ = json.Unmarshal([]byte(folderRenames), &t.FolderRenames)
	if t.FolderRenames == nil {
		t.FolderRenames = map[string]string{}
	}
	t.Enabled = enabled == "1" || enabled == "true"
	if lastRun.Valid {
		t.LastRunAt = &lastRun.Time
	}
	if nextRun.Valid {
		t.NextRunAt = &nextRun.Time
	}
	return &t, nil
}

func listTasks(d *DB, where string, args ...any) ([]*Task, error) {
	rows, err := d.Query(`SELECT `+taskColumns+` FROM tasks `+where+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListTasks 全部任务
func (d *DB) ListTasks() ([]*Task, error) { return listTasks(d, ``) }

// ListEnabledTasks 启用的任务
func (d *DB) ListEnabledTasks() ([]*Task, error) { return listTasks(d, `WHERE enabled=1`) }

// GetTask 按 id 查任务
func (d *DB) GetTask(id int64) (*Task, error) {
	row := d.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=?`, id)
	t, err := scanTask(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// CreateTask 新建任务，返回 id
func (d *DB) CreateTask(t *Task) (int64, error) {
	fp, _ := json.Marshal(t.FolderPaths)
	fr, _ := json.Marshal(t.FolderRenames)
	if t.FolderRenames == nil {
		fr = []byte("{}")
	}
	res, err := d.Exec(`INSERT INTO tasks(name, account_id, share_url, pwd, save_dir, folder_paths, folder_renames,
		folder_filter, exclude_folder_filter, regex_pattern, regex_replace, cron_expr, enabled, status)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.Name, t.AccountID, t.ShareURL, t.Pwd, t.SaveDir, string(fp), string(fr),
		t.FolderFilter, t.ExcludeFolderFilter, t.RegexPattern, t.RegexReplace, t.CronExpr,
		boolToInt(t.Enabled), "idle")
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateTask 更新任务全部可编辑字段
func (d *DB) UpdateTask(t *Task) error {
	fp, _ := json.Marshal(t.FolderPaths)
	fr, _ := json.Marshal(t.FolderRenames)
	if t.FolderRenames == nil {
		fr = []byte("{}")
	}
	_, err := d.Exec(`UPDATE tasks SET name=?, account_id=?, share_url=?, pwd=?, save_dir=?, folder_paths=?, folder_renames=?,
		folder_filter=?, exclude_folder_filter=?, regex_pattern=?, regex_replace=?, cron_expr=?, enabled=?
		WHERE id=?`,
		t.Name, t.AccountID, t.ShareURL, t.Pwd, t.SaveDir, string(fp), string(fr),
		t.FolderFilter, t.ExcludeFolderFilter, t.RegexPattern, t.RegexReplace, t.CronExpr,
		boolToInt(t.Enabled), t.ID)
	return err
}

// SetTaskStatus 更新任务状态
func (d *DB) SetTaskStatus(id int64, status string) error {
	_, err := d.Exec(`UPDATE tasks SET status=? WHERE id=?`, status, id)
	return err
}

// SetTaskRunInfo 更新任务状态与最近/下次运行时间
func (d *DB) SetTaskRunInfo(id int64, status string, lastRun, nextRun *time.Time) error {
	_, err := d.Exec(`UPDATE tasks SET status=?, last_run_at=?, next_run_at=? WHERE id=?`,
		status, lastRun, nextRun, id)
	return err
}

// SetTaskEnabled 启用/禁用
func (d *DB) SetTaskEnabled(id int64, enabled bool) error {
	_, err := d.Exec(`UPDATE tasks SET enabled=? WHERE id=?`, boolToInt(enabled), id)
	return err
}

// SetTaskNextRun 更新下次运行时间（nil 表示清空）
func (d *DB) SetTaskNextRun(id int64, next *time.Time) error {
	_, err := d.Exec(`UPDATE tasks SET next_run_at=? WHERE id=?`, next, id)
	return err
}

// DeleteTask 删除任务
func (d *DB) DeleteTask(id int64) error {
	_, err := d.Exec(`DELETE FROM tasks WHERE id=?`, id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
