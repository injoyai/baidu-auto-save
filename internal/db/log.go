package db

import (
	"database/sql"
	"time"
)

// TransferLog 转存运行日志（ID 即对外 run id）
type TransferLog struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	TaskName  string    `json:"task_name,omitempty"`
	RunAt     time.Time `json:"run_at"`
	Result    string    `json:"result"` // success | partial | failed | skipped
	Message   string    `json:"message"`
	FileCount int64     `json:"file_count"`
	SkipCount int64     `json:"skip_count"`
}

// TaskFile 已转存文件记录
type TaskFile struct {
	ID            int64     `json:"id"`
	TaskID        int64     `json:"task_id"`
	Path          string    `json:"path"`
	MD5           string    `json:"md5"`
	Size          int64     `json:"size"`
	TransferredAt time.Time `json:"transferred_at"`
}

// InsertTransferLog 写入运行日志，返回 log id（run id）
func (d *DB) InsertTransferLog(l *TransferLog) (int64, error) {
	res, err := d.Exec(`INSERT INTO transfer_logs(task_id, result, message, file_count, skip_count)
		VALUES(?,?,?,?,?)`, l.TaskID, l.Result, l.Message, l.FileCount, l.SkipCount)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListTransferLogs 分页日志，taskID>0 时按任务过滤
func (d *DB) ListTransferLogs(taskID int64, offset, limit int) ([]*TransferLog, error) {
	where, args := ``, []any{}
	if taskID > 0 {
		where = ` WHERE l.task_id=?`
		args = append(args, taskID)
	}
	args = append(args, limit, offset)
	rows, err := d.Query(`SELECT l.id, l.task_id, COALESCE(t.name,''), l.run_at, l.result, l.message,
		l.file_count, l.skip_count
		FROM transfer_logs l LEFT JOIN tasks t ON t.id=l.task_id`+where+` ORDER BY l.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TransferLog
	for rows.Next() {
		var l TransferLog
		if err := rows.Scan(&l.ID, &l.TaskID, &l.TaskName, &l.RunAt, &l.Result, &l.Message,
			&l.FileCount, &l.SkipCount); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

// CountTransferLogs 日志总数
func (d *DB) CountTransferLogs(taskID int64) (int64, error) {
	var n int64
	var err error
	if taskID > 0 {
		err = d.QueryRow(`SELECT COUNT(*) FROM transfer_logs WHERE task_id=?`, taskID).Scan(&n)
	} else {
		err = d.QueryRow(`SELECT COUNT(*) FROM transfer_logs`).Scan(&n)
	}
	return n, err
}

// InsertTaskFile 记录已转存文件（IGNORE 处理并发重复）
func (d *DB) InsertTaskFile(f *TaskFile) error {
	_, err := d.Exec(`INSERT OR IGNORE INTO task_files(task_id, path, md5, size) VALUES(?,?,?,?)`,
		f.TaskID, f.Path, f.MD5, f.Size)
	return err
}

// FilterKnownMD5 返回 md5s 中未在任务内出现过的子集（去重主判据）
func (d *DB) FilterKnownMD5(taskID int64, md5s []string) (map[string]bool, error) {
	known := make(map[string]bool, len(md5s))
	if len(md5s) == 0 {
		return known, nil
	}
	// 逐个查询（数据量小，IN 动态拼接在 sqlite 驱动参数上限内也可，这里从简）
	stmt, err := d.Prepare(`SELECT 1 FROM task_files WHERE task_id=? AND md5=?`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	for _, m := range md5s {
		var one int
		err := stmt.QueryRow(taskID, m).Scan(&one)
		if err == nil {
			known[m] = true
		} else if err != sql.ErrNoRows {
			return nil, err
		}
	}
	return known, nil
}

// RecentTransferCount 近 N 天累计转存文件数（仪表盘）
func (d *DB) RecentTransferCount(days int) (int64, error) {
	var n int64
	err := d.QueryRow(`SELECT COALESCE(SUM(file_count),0) FROM transfer_logs
		WHERE result IN ('success','partial') AND run_at >= datetime('now', ?)`,
		"-"+itoa(days)+" days").Scan(&n)
	return n, err
}

// StatusCount 任务状态分布（仪表盘）
func (d *DB) StatusCount() (map[string]int64, error) {
	rows, err := d.Query(`SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var s string
		var n int64
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
