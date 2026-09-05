package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// auth 单密码 → JWT 认证（设计文档 §6：内网部署，简单登录）
type auth struct {
	password string
	secret   []byte
}

func newAuth(password string) *auth {
	return &auth{password: password, secret: []byte(password + "|baidu-auto-save-jwt")}
}

type loginReq struct {
	Password string `json:"password"`
}

func (a *auth) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		failBadRequest(c, "缺少 password")
		return
	}
	if req.Password != a.password {
		fail(c, http.StatusUnauthorized, 1003, "密码错误")
		return
	}
	token, err := a.issueToken()
	if err != nil {
		failErr(c, err)
		return
	}
	ok(c, gin.H{"token": token})
}

func (a *auth) issueToken() (string, error) {
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   "admin",
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
}

// middleware JWT 校验
func (a *auth) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		tokenStr, found := strings.CutPrefix(h, "Bearer ")
		if !found || tokenStr == "" {
			fail(c, http.StatusUnauthorized, 1003, "未登录")
			c.Abort()
			return
		}
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("非法签名算法")
			}
			return a.secret, nil
		})
		if err != nil || !token.Valid {
			fail(c, http.StatusUnauthorized, 1003, "登录已过期，请重新登录")
			c.Abort()
			return
		}
		c.Next()
	}
}
