package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open 打开（必要时创建）SQLite 数据库并初始化表结构
func Open(path string) (*sql.DB, error) {
	// _pragma=busy_timeout 等待锁；_pragma=journal_mode WAL 提升并发读
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite 单写：限制连接数避免 SQLITE_BUSY
	d.SetMaxOpenConns(1)
	if err := migrate(d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func migrate(d *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			name          TEXT NOT NULL UNIQUE,
			bduss         TEXT NOT NULL,
			stoken        TEXT NOT NULL DEFAULT '',
			cookies_json  TEXT NOT NULL DEFAULT '{}',
			quota_used    INTEGER NOT NULL DEFAULT 0,
			quota_total   INTEGER NOT NULL DEFAULT 0,
			status        TEXT NOT NULL DEFAULT 'active',
			last_check_at DATETIME,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id                    INTEGER PRIMARY KEY AUTOINCREMENT,
			name                  TEXT NOT NULL,
			account_id            INTEGER NOT NULL REFERENCES accounts(id),
			share_url             TEXT NOT NULL,
			pwd                   TEXT NOT NULL DEFAULT '',
			save_dir              TEXT NOT NULL DEFAULT '/来自：分享',
			folder_paths          TEXT NOT NULL DEFAULT '[]',
			folder_renames        TEXT NOT NULL DEFAULT '{}',
			folder_filter         TEXT NOT NULL DEFAULT '',
			exclude_folder_filter TEXT NOT NULL DEFAULT '',
			regex_pattern         TEXT NOT NULL DEFAULT '',
			regex_replace         TEXT NOT NULL DEFAULT '',
			cron_expr             TEXT NOT NULL DEFAULT '0 8 * * *',
			enabled               INTEGER NOT NULL DEFAULT 1,
			status                TEXT NOT NULL DEFAULT 'idle',
			last_run_at           DATETIME,
			next_run_at           DATETIME,
			created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS task_files (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id        INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			path           TEXT NOT NULL,
			md5            TEXT NOT NULL,
			size           INTEGER NOT NULL,
			transferred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(task_id, md5)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_files_md5 ON task_files(task_id, md5)`,
		`CREATE TABLE IF NOT EXISTS transfer_logs (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id    INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			run_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			result     TEXT NOT NULL,
			message    TEXT NOT NULL DEFAULT '',
			file_count INTEGER NOT NULL DEFAULT 0,
			skip_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// 数据修复：早期 Quota 适配 bug 曾把 (总量,已用) 写反入库；正常情况已用不会大于总量，
	// 启动时对调这类反常行（幂等，已修复的行不受影响）
	if _, err := d.Exec(`UPDATE accounts SET
		quota_used = quota_total,
		quota_total = quota_used
		WHERE quota_used > quota_total AND quota_total > 0`); err != nil {
		return fmt.Errorf("migrate: repair reversed quota: %w", err)
	}
	// 轻量列补丁：已有库补 folder_renames 列（CREATE IF NOT EXISTS 不会对旧表补列）
	col := "folder_renames"
	rows, err := d.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	has := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("migrate: %w", err)
		}
		if name == col {
			has = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if !has {
		if _, err := d.Exec(`ALTER TABLE tasks ADD COLUMN folder_renames TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}
