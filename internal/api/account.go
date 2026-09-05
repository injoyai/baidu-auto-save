package api

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"baidu-auto-save/internal/baidu"
	"baidu-auto-save/internal/db"
	"baidu-auto-save/internal/engine"
)

// registerAccountRoutes 账号管理端点
func (s *Server) registerAccountRoutes(g *gin.RouterGroup) {
	g.GET("/accounts", s.listAccounts)
	g.POST("/accounts", s.createAccount)
	g.PUT("/accounts/:id", s.updateAccount)
	g.DELETE("/accounts/:id", s.deleteAccount)
	g.POST("/accounts/:id/check", s.checkAccount)
}

func (s *Server) listAccounts(c *gin.Context) {
	list, err := s.db.ListAccounts()
	if err != nil {
		failErr(c, err)
		return
	}
	if list == nil {
		list = []*db.Account{}
	}
	ok(c, list)
}

type accountReq struct {
	Name  string `json:"name"`
	BDUSS string `json:"bduss"`
	// 兼容粘贴完整 Cookie 字符串的场景：从中提取 BDUSS/STOKEN
	Cookie string `json:"cookie"`
}

// extractCookie 解析 cookie 字段，返回 bduss/stoken。
// 支持三种输入：整段 Cookie 串（含 BDUSS=）、单对 BDUSS=xxx、裸 BDUSS 值。
func extractCookie(req *accountReq) (bduss, stoken string, err string) {
	raw := strings.TrimSpace(req.Cookie)
	if raw != "" {
		if !strings.Contains(raw, "=") && !strings.Contains(raw, ";") {
			// 直接粘贴了单个 BDUSS 值
			bduss = raw
		} else {
			for _, kv := range strings.Split(raw, ";") {
				k, v, _ := strings.Cut(strings.TrimSpace(kv), "=")
				switch k {
				case "BDUSS":
					bduss = v
				case "STOKEN":
					stoken = v
				}
			}
		}
	}
	if bduss == "" {
		bduss = req.BDUSS
	}
	if bduss == "" {
		return "", "", "无法识别 BDUSS：请粘贴浏览器复制的整段 Cookie，或单独填写 BDUSS 值"
	}
	return bduss, stoken, ""
}

func (s *Server) createAccount(c *gin.Context) {
	var req accountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: "+err.Error())
		return
	}
	if req.Name == "" {
		failBadRequest(c, "账号名称不能为空")
		return
	}
	bduss, stoken, errMsg := extractCookie(&req)
	if errMsg != "" {
		failBadRequest(c, errMsg)
		return
	}
	id, err := s.db.CreateAccount(&db.Account{
		Name: req.Name, BDUSS: bduss, STOKEN: stoken, CookiesJSON: "{}",
	})
	if err != nil {
		failErr(c, err)
		return
	}
	ok(c, gin.H{"id": id})
}

// updateAccount 更新 Cookie（失效后修复；Cookie 已在 UpdateAccountCookie 中重置为 active）
func (s *Server) updateAccount(c *gin.Context) {
	acc, err := s.pathIDAccount(c)
	if err != nil {
		return
	}
	var req accountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadRequest(c, "参数错误: "+err.Error())
		return
	}
	bduss, stoken, errMsg := extractCookie(&req)
	if errMsg != "" {
		failBadRequest(c, errMsg)
		return
	}
	if err := s.db.UpdateAccountCookie(acc.ID, bduss, stoken, "{}"); err != nil {
		failErr(c, err)
		return
	}
	ok(c, nil)
}

func (s *Server) deleteAccount(c *gin.Context) {
	acc, err := s.pathIDAccount(c)
	if err != nil {
		return
	}
	n, err := s.db.CountTasksByAccount(acc.ID)
	if err != nil {
		failErr(c, err)
		return
	}
	if n > 0 {
		failBadRequest(c, "该账号下还有 "+itoa(n)+" 个任务，请先删除或改绑任务")
		return
	}
	if err := s.db.DeleteAccount(acc.ID); err != nil {
		failErr(c, err)
		return
	}
	ok(c, nil)
}

// checkAccount 校验 Cookie 有效性并刷新容量
func (s *Server) checkAccount(c *gin.Context) {
	acc, err := s.pathIDAccount(c)
	if err != nil {
		return
	}
	cli, err := baidu.NewClient(engine.BuildCookieStr(acc))
	if err != nil {
		failErr(c, err)
		return
	}
	used, total, err := cli.Quota()
	if err != nil {
		// -6 等错误 → Cookie 失效
		if strings.Contains(err.Error(), "-6") || strings.Contains(err.Error(), "登录") {
			_ = s.db.SetAccountStatus(acc.ID, "invalid")
			failBadRequest(c, "Cookie 已失效，请更新")
			return
		}
		failErr(c, err)
		return
	}
	if err := s.db.UpdateAccountQuota(acc.ID, used, total); err != nil {
		log.Printf("[api] 更新账号容量失败: %v", err)
	}
	if err := s.db.SetAccountStatus(acc.ID, "active"); err != nil {
		log.Printf("[api] 更新账号状态失败: %v", err)
	}
	ok(c, gin.H{"quota_used": used, "quota_total": total})
}

// pathIDAccount 解析 :id 并加载账号，出错时已写响应
func (s *Server) pathIDAccount(c *gin.Context) (*db.Account, error) {
	id, okID := parseID(c)
	if !okID {
		failBadRequest(c, "无效的 id")
		return nil, errStop
	}
	acc, err := s.db.GetAccount(id)
	if err != nil {
		failErr(c, err)
		return nil, errStop
	}
	if acc == nil {
		failNotFound(c, "账号不存在")
		return nil, errStop
	}
	return acc, nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
