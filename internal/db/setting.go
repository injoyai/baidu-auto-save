package db

import (
	"database/sql"
)

// DB 封装 *sql.DB，提供领域方法
type DB struct {
	*sql.DB
}

// GetSetting 读取设置，缺失返回 ("", nil)
func (d *DB) GetSetting(key string) (string, error) {
	var v string
	err := d.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// SetSetting 写入设置
func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec(`INSERT INTO settings(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// GetAllSettings 全部设置
func (d *DB) GetAllSettings() (map[string]string, error) {
	rows, err := d.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// 设置键约定（设计文档 §4）
const (
	KeyNotifyWecomWebhook   = "notify.wecom_webhook"
	KeyNotifyServerChanKey  = "notify.serverchan_sendkey"
	KeyNotifyTelegramToken  = "notify.telegram_bot_token"
	KeyNotifyTelegramChatID = "notify.telegram_chat_id"
	KeyNotifyCustomWebhook  = "notify.custom_webhook"
	KeyNotifyEvents         = "notify.events"
	KeyOpenAPIToken         = "open_api_token"
	KeyGlobalSaveDir        = "global.save_dir"
)
