package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/auth"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ==================== 微信小程序一键登录 ====================
// 流程：小程序 wx.login → code → POST /api/v1/wx-login
//   已绑定 openid → 直接签发 JWT（免密登录）
//   未绑定 → 返回 wx_ticket（10分钟有效），前端再走账号密码 POST /api/v1/wx-bind 完成绑定

// wxTickets 内存票据表：ticket → {openid, expire}
var wxTickets = struct {
	m map[string]wxTicket
}{m: map[string]wxTicket{}}

type wxTicket struct {
	OpenID string
	Expire time.Time
}

type jscode2sessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// code2Session 用临时 code 向微信换 openid
func (h *Handler) code2Session(code string) (string, error) {
	if h.WxAppID == "" || h.WxAppSecret == "" {
		return "", fmt.Errorf("服务端未配置微信小程序凭据")
	}
	u := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		url.QueryEscape(h.WxAppID), url.QueryEscape(h.WxAppSecret), url.QueryEscape(code))
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var r jscode2sessionResp
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("微信响应解析失败")
	}
	if r.ErrCode != 0 || r.OpenID == "" {
		return "", fmt.Errorf("微信登录失败(errcode=%d): %s", r.ErrCode, r.ErrMsg)
	}
	return r.OpenID, nil
}

// WxLogin 小程序一键登录：code 换 openid，已绑定则免密签发 JWT
func (h *Handler) WxLogin(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		response.BadRequest(c, "缺少 code 参数")
		return
	}
	openID, err := h.code2Session(req.Code)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var u model.User
	if err := h.DB.Where("wx_openid = ?", openID).First(&u).Error; err != nil {
		// 未绑定：发放一次性票据，10分钟内完成账号绑定
		buf := make([]byte, 16)
		rand.Read(buf)
		tk := hex.EncodeToString(buf)
		wxTickets.m[tk] = wxTicket{OpenID: openID, Expire: time.Now().Add(10 * time.Minute)}
		// 顺手清理过期票据
		for k, v := range wxTickets.m {
			if time.Now().After(v.Expire) {
				delete(wxTickets.m, k)
			}
		}
		response.Success(c, gin.H{"bound": false, "wx_ticket": tk, "msg": "该微信尚未绑定平台账号，请用账号密码完成绑定"})
		return
	}
	if u.Status != 1 {
		response.Forbidden(c, "账号已停用")
		return
	}
	h.issueWxToken(c, &u, openID)
}

// WxBind 绑定微信：票据 + 账号密码验证通过后写 openid
func (h *Handler) WxBind(c *gin.Context) {
	var req struct {
		WxTicket string `json:"wx_ticket"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.WxTicket == "" || req.Username == "" || req.Password == "" {
		response.BadRequest(c, "参数不完整")
		return
	}
	tk, ok := wxTickets.m[req.WxTicket]
	if !ok || time.Now().After(tk.Expire) {
		delete(wxTickets.m, req.WxTicket)
		response.BadRequest(c, "绑定票据无效或已过期，请重新发起微信登录")
		return
	}
	var u model.User
	if err := h.DB.Where("username = ?", req.Username).First(&u).Error; err != nil {
		response.BadRequest(c, "用户名或密码错误")
		return
	}
	if u.Status != 1 {
		response.Forbidden(c, "账号已停用")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		response.BadRequest(c, "用户名或密码错误")
		return
	}
	// 检查该 openid 是否已被其他账号绑定
	var cnt int64
	h.DB.Model(&model.User{}).Where("wx_openid = ? AND id <> ?", tk.OpenID, u.ID).Count(&cnt)
	if cnt > 0 {
		response.BadRequest(c, "该微信已绑定其他账号")
		return
	}
	if err := h.DB.Model(&u).Update("wx_openid", tk.OpenID).Error; err != nil {
		response.ServerError(c, "绑定失败")
		return
	}
	delete(wxTickets.m, req.WxTicket)
	middleware.WriteAudit(h.DB, c, "wx_bind", u.Username, "微信小程序绑定成功")
	h.issueWxToken(c, &u, tk.OpenID)
}

// WxUnbind 解绑当前用户的微信（已登录）
func (h *Handler) WxUnbind(c *gin.Context) {
	uid, _ := c.Get("user_id")
	res := h.DB.Model(&model.User{}).Where("id = ?", uid).Update("wx_openid", "")
	if res.Error != nil {
		response.ServerError(c, "解绑失败")
		return
	}
	caller, _ := middleware.GetCaller(c)
	middleware.WriteAudit(h.DB, c, "wx_unbind", caller, "微信小程序解绑")
	response.Success(c, gin.H{"ok": true})
}

// issueWxToken 签发 JWT（与账号密码登录同构）
func (h *Handler) issueWxToken(c *gin.Context, u *model.User, openID string) {
	now := time.Now()
	h.DB.Model(u).Updates(map[string]interface{}{"last_login_at": now})
	siteIDs := parseSiteIDs(u.SiteIDs)
	if u.Role == "global_admin" {
		siteIDs = nil
	}
	token, err := auth.GenerateToken(u.ID, u.Username, u.Role, siteIDs, h.JWTExpire)
	if err != nil {
		response.ServerError(c, "生成令牌失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "wx_login", u.Username, "微信一键登录成功")
	response.Success(c, gin.H{
		"bound":    true,
		"token":    token,
		"user":     u,
		"site_ids": siteIDs,
		"expire_h": h.JWTExpire,
		"openid_tail": func() string { // 只回显尾4位便于前端确认绑定态，不泄露完整openid
			if len(openID) > 4 {
				return openID[len(openID)-4:]
			}
			return ""
		}(),
	})
}
