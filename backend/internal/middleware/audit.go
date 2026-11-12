package middleware

import (
	"dcim/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Audit 写审计日志的辅助（在 handler 内调用，非中间件拦截式）
func WriteAudit(db *gorm.DB, c *gin.Context, action, target, detail string) {
	username, ip := GetCaller(c)
	db.Create(&model.Audit{
		Username: username,
		Action:   action,
		Target:   target,
		IP:       ip,
		Detail:   detail,
	})
}
