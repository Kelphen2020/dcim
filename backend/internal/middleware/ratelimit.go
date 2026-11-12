package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LoginRateLimit 登录频率限制（同一IP 5次/10分钟 → 锁定15分钟）
type LoginRateLimit struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

type loginAttempt struct {
	count    int
	firstAt  time.Time
	lockedUntil *time.Time
}

var loginLimiter = &LoginRateLimit{attempts: make(map[string]*loginAttempt)}

func (l *LoginRateLimit) Check(ip string) (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	a, exists := l.attempts[ip]
	if !exists || now.Sub(a.firstAt) > 10*time.Minute {
		l.attempts[ip] = &loginAttempt{count: 1, firstAt: now}
		return true, ""
	}

	if a.lockedUntil != nil {
		if now.Before(*a.lockedUntil) {
			remaining := a.lockedUntil.Sub(now).Round(time.Second)
			return false, "登录过于频繁，请在 " + remaining.String() + " 后重试"
		}
		// 锁定已过期，重置
		a.count = 1
		a.firstAt = now
		a.lockedUntil = nil
		return true, ""
	}

	a.count++
	if a.count > 5 {
		lockUntil := now.Add(15 * time.Minute)
		a.lockedUntil = &lockUntil
		return false, "登录失败次数过多，已锁定 15 分钟"
	}
	return true, ""
}

func (l *LoginRateLimit) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// LoginRateLimitMiddleware 登录限流中间件
func LoginRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		allowed, msg := loginLimiter.Check(ip)
		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429, "msg": msg,
			})
			return
		}
		c.Next()
		// 登录成功后重置计数
		if c.Writer.Status() == http.StatusOK {
			loginLimiter.Reset(ip)
		}
	}
}