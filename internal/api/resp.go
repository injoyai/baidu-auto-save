// Package api HTTP API 层：路由、JWT 认证中间件、REST 端点
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 统一响应格式（设计文档 §6）
type resp struct {
	Code    int    `json:"code"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message"`
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, resp{Code: 0, Data: data, Message: "ok"})
}

func fail(c *gin.Context, status, code int, message string) {
	c.JSON(status, resp{Code: code, Message: message})
}

// failErr 内部错误统一 500/1000
func failErr(c *gin.Context, err error) {
	fail(c, http.StatusInternalServerError, 1000, err.Error())
}

// failBadRequest 参数/业务校验失败统一 400/1001
func failBadRequest(c *gin.Context, message string) {
	fail(c, http.StatusBadRequest, 1001, message)
}

// failNotFound 404/1002
func failNotFound(c *gin.Context, message string) {
	fail(c, http.StatusNotFound, 1002, message)
}
