package db

import (
	"database/sql"
	"strings"
	"time"
)

// Account 百度网盘账号
type Account struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	BDUSS       string     `json:"-"`            // 不直接暴露给前端
	STOKEN      string     `json:"-"`            // 不直接暴露给前端
	CookiesJSON string     `json:"-"`            // 其余补充 Cookie
	QuotaUsed   int64      `json:"quota_used"`   // 已用容量(字节)
	QuotaTotal  int64      `json:"quota_total"`  // 总容量(字节)
	Status      string     `json:"status"`       // active | invalid
	LastCheckAt *time.Time `json:"last_check_at"`
	CreatedAt   time.Time  `json:"created_at"`
	// Cookie 完整 Cookie（BDUSS=xxx; STOKEN=yyy 格式），供编辑回显
	Cookie string `json:"cookie"`
	// CookieMasked 供前端展示的脱敏 Cookie 摘要
	CookieMasked string `json:"cookie_masked"`
}

const accountColumns = `id, name, bduss, stoken, cookies_json, quota_used, quota_total, status, last_check_at, created_at`

func scanAccount(row interface{ Scan(...any) error }) (*Account, error) {
	var a Account
	var lastCheck sql.NullTime
	if err := row.Scan(&a.ID, &a.Name, &a.BDUSS, &a.STOKEN, &a.CookiesJSON,
		&a.QuotaUsed, &a.QuotaTotal, &a.Status, &lastCheck, &a.CreatedAt); err != nil {
		return nil, err
	}
	if lastCheck.Valid {
		a.LastCheckAt = &lastCheck.Time
	}
	a.CookieMasked = maskCookie(a.BDUSS)
	// 组装完整 Cookie 串（与 engine.BuildCookieStr 同格式），供编辑回显
	var sb strings.Builder
	sb.WriteString("BDUSS=" + a.BDUSS)
	if a.STOKEN != "" {
		sb.WriteString("; STOKEN=" + a.STOKEN)
	}
	a.Cookie = sb.String()
	return &a, nil
}

// maskCookie 脱敏展示：仅保留前 6 后 4 位
func maskCookie(v string) string {
	if len(v) <= 10 {
		return "****"
	}
	return v[:6] + "****" + v[len(v)-4:]
}

// ListAccounts 返回全部账号
func (d *DB) ListAccounts() ([]*Account, error) {
	rows, err := d.Query(`SELECT ` + accountColumns + ` FROM accounts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAccount 按 id 查账号（含完整 Cookie，仅供内部转存使用）
func (d *DB) GetAccount(id int64) (*Account, error) {
	row := d.QueryRow(`SELECT `+accountColumns+` FROM accounts WHERE id = ?`, id)
	a, err := scanAccount(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// CreateAccount 新增账号
func (d *DB) CreateAccount(a *Account) (int64, error) {
	res, err := d.Exec(`INSERT INTO accounts(name, bduss, stoken, cookies_json) VALUES(?,?,?,?)`,
		a.Name, a.BDUSS, a.STOKEN, a.CookiesJSON)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateAccountCookie 更新账号 Cookie（修复失效）
func (d *DB) UpdateAccountCookie(id int64, bduss, stoken, cookiesJSON string) error {
	_, err := d.Exec(`UPDATE accounts SET bduss=?, stoken=?, cookies_json=?, status='active' WHERE id=?`,
		bduss, stoken, cookiesJSON, id)
	return err
}

// RenameAccount 修改账号名称
func (d *DB) RenameAccount(id int64, name string) error {
	_, err := d.Exec(`UPDATE accounts SET name=? WHERE id=?`, name, id)
	return err
}

// SetAccountStatus 更新账号状态与最后检查时间
func (d *DB) SetAccountStatus(id int64, status string) error {
	_, err := d.Exec(`UPDATE accounts SET status=?, last_check_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

// UpdateAccountQuota 更新账号容量信息
func (d *DB) UpdateAccountQuota(id int64, used, total int64) error {
	_, err := d.Exec(`UPDATE accounts SET quota_used=?, quota_total=?, last_check_at=CURRENT_TIMESTAMP WHERE id=?`,
		used, total, id)
	return err
}

// DeleteAccount 删除账号
func (d *DB) DeleteAccount(id int64) error {
	_, err := d.Exec(`DELETE FROM accounts WHERE id=?`, id)
	return err
}

// CountTasksByAccount 任务引用计数
func (d *DB) CountTasksByAccount(accountID int64) (int64, error) {
	var n int64
	err := d.QueryRow(`SELECT COUNT(*) FROM tasks WHERE account_id=?`, accountID).Scan(&n)
	return n, err
}
