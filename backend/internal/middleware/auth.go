package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"dcim/backend/pkg/auth"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// Auth JWT 鉴权，注入 user_id/username/role/site_ids
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Unauthorized(c, "缺少或无效的 Authorization 头")
			c.Abort()
			return
		}
		claims, err := auth.ParseToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			response.Unauthorized(c, "token 无效或已过期")
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("site_ids", claims.SiteIDs) // []uint，空=全局
		c.Next()
	}
}

// RequireRoles 角色校验
func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		for _, r := range roles {
			if roleStr == r {
				c.Next()
				return
			}
		}
		response.Forbidden(c, "权限不足")
		c.Abort()
	}
}

// DenyViewerWrites viewer 只读守卫
func DenyViewerWrites() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role == "viewer" {
			m := c.Request.Method
			if m != http.MethodGet && m != http.MethodOptions && m != http.MethodHead {
				response.Forbidden(c, "只读角色无权执行此操作")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// SiteScope 多站点数据过滤：
// 把当前用户可见的站点ID列表写入 site_scope（[]uint，nil=全局不限制）。
// 业务 handler 统一通过 GetSiteScope 取出并作为查询条件。
// 若请求带 ?site_id= 且不在可见范围内则直接 403。
func SiteScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		ids, _ := c.Get("site_ids")
		siteIDs, _ := ids.([]uint)
		if len(siteIDs) == 0 {
			c.Set("site_scope", []uint(nil)) // 全局
			c.Next()
			return
		}
		// 校验 query 指定的 site_id 是否在权限内
		if sid := c.Query("site_id"); sid != "" {
			n, err := strconv.ParseUint(sid, 10, 64)
			if err != nil {
				response.BadRequest(c, "site_id 参数无效")
				c.Abort()
				return
			}
			ok := false
			for _, id := range siteIDs {
				if uint(n) == id {
					ok = true
					break
				}
			}
			if !ok {
				response.Forbidden(c, "无权访问该站点数据")
				c.Abort()
				return
			}
			c.Set("site_scope", []uint{uint(n)})
			c.Next()
			return
		}
		c.Set("site_scope", siteIDs)
		c.Next()
	}
}

// GetSiteScope 返回当前请求可见站点范围；nil 表示全局不限制
func GetSiteScope(c *gin.Context) []uint {
	v, _ := c.Get("site_scope")
	scope, _ := v.([]uint)
	return scope
}

// GetUserID 当前登录用户 ID（JWT claims.user_id，中间件已 Set）
func GetUserID(c *gin.Context) uint {
	v, ok := c.Get("user_id")
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case uint:
		return n
	case float64:
		return uint(n)
	case int64:
		return uint(n)
	}
	return 0
}

// GetCaller 审计用：返回用户名与IP
func GetCaller(c *gin.Context) (string, string) {
	u, _ := c.Get("username")
	name, _ := u.(string)
	ip := c.ClientIP()
	if fwd := c.GetHeader("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
	}
	return name, ip
}

// InSiteScope 判断 siteID 是否在当前用户可见范围内（nil scope=全局，恒真）
func InSiteScope(c *gin.Context, siteID uint) bool {
	scope := GetSiteScope(c)
	if scope == nil {
		return true
	}
	for _, id := range scope {
		if id == siteID {
			return true
		}
	}
	return false
}

// CORSMiddleware 跨域
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
