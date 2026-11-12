package api

import (
	"sync"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/auth"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Handler struct {
	DB          *gorm.DB
	JWTSecret   string
	JWTExpire   int
	WxAppID     string // 微信小程序 AppID（一键登录）
	WxAppSecret string // 微信小程序 AppSecret（仅服务端）
	AIAPIKey    string // V53 AI 助手（百炼 OpenAI 兼容接口密钥，仅服务端）；V58 起受 aiMu 保护，读用 getAI()
	AIBaseURL   string // V53 AI 助手接口地址
	AIModel     string // V53 AI 助手模型名
	// V58 大模型配置界面：DB 配置热更新（RWMutex 保护上方三字段）+ env 原值留档（恢复用）
	aiMu        sync.RWMutex
	AIProvider  string // 供应商标识（bailian/openai/deepseek/zhipu/moonshot/ollama/custom）
	AIEnvKey    string // 启动时 ai.env 原值（DELETE 恢复用）
	AIEnvBase   string
	AIEnvModel  string
}

func NewHandler(db *gorm.DB, secret string, expireHours int) *Handler {
	return &Handler{DB: db, JWTSecret: secret, JWTExpire: expireHours}
}

// ==================== 登录 / 个人信息 ====================

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Captcha  string `json:"captcha"`
	CapID    string `json:"captcha_id"`
}

func parseSiteIDs(s string) []uint {
	var ids []uint
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if n, err := strconv.ParseUint(p, 10, 64); err == nil && n > 0 {
			ids = append(ids, uint(n))
		}
	}
	return ids
}

// Login 登录（验证码 + 密码 + 限流）
func (h *Handler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Username == "" || req.Password == "" {
		response.BadRequest(c, "用户名和密码不能为空")
		return
	}
	if req.CapID == "" || !verifyCaptcha(req.CapID, req.Captcha) {
		response.BadRequest(c, "验证码错误或已过期")
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
	now := time.Now()
	h.DB.Model(&u).Update("last_login_at", now)

	siteIDs := parseSiteIDs(u.SiteIDs)
	if u.Role == "global_admin" {
		siteIDs = nil // 全局管理员不受站点限制
	}
	token, err := auth.GenerateToken(u.ID, u.Username, u.Role, siteIDs, h.JWTExpire)
	if err != nil {
		response.ServerError(c, "生成令牌失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "login", u.Username, "登录成功")
	response.Success(c, gin.H{
		"token":      token,
		"user":       u,
		"site_ids":   siteIDs,
		"expire_h":   h.JWTExpire,
	})
}

// Profile 当前用户信息
func (h *Handler) Profile(c *gin.Context) {
	uid, _ := c.Get("user_id")
	var u model.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, gin.H{"user": u, "site_ids": parseSiteIDs(u.SiteIDs), "wx_bound": u.WxOpenID != ""})
}

// ChangePassword 修改自己的密码
func (h *Handler) ChangePassword(c *gin.Context) {
	var req struct {
		Old string `json:"old_password"`
		New string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Old == "" || req.New == "" {
		response.BadRequest(c, "参数不完整")
		return
	}
	if len(req.New) < 8 {
		response.BadRequest(c, "新密码至少 8 位")
		return
	}
	uid, _ := c.Get("user_id")
	var u model.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Old)) != nil {
		response.BadRequest(c, "原密码错误")
		return
	}
	if ok, msg := ValidatePasswordStrength(req.New); !ok {
		response.BadRequest(c, msg)
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
	h.DB.Model(&u).Update("password_hash", string(hash))
	middleware.WriteAudit(h.DB, c, "change_password", u.Username, "修改密码")
	response.Success(c, gin.H{"ok": true})
}

// ==================== 用户管理（全局管理员） ====================

type userReq struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	DisplayName   string `json:"display_name"`
	Role          string `json:"role"`
	SiteIDs       string `json:"site_ids"`
	Phone         string `json:"phone"`
	Status        *int   `json:"status"`
	AllowedGroups string `json:"allowed_groups"`
}

var validRoles = map[string]bool{"global_admin": true, "site_admin": true, "operator": true, "viewer": true}

// validGroups 菜单权限分组白名单
var validGroups = map[string]bool{"dashboard": true, "infra": true, "asset": true, "ops": true, "monitor": true, "energy": true, "visual": true, "network": true, "report": true, "system": true}

func normalizeGroups(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return "*", true
	}
	parts := strings.Split(s, ",")
	seen := map[string]bool{}
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !validGroups[p] {
			return "", false
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return "*", true
	}
	return strings.Join(out, ","), true
}

// userView 用户列表返回视图（含站点范围数组）
type userView struct {
	model.User
	SiteIDArr []uint `json:"site_id_arr"`
}

// ListUserOptions 精简用户下拉（id/username/display_name/role），供改派/责任人选择，所有登录用户可用
func (h *Handler) ListUserOptions(c *gin.Context) {
	var items []model.User
	h.DB.Where("status = 1").Order("id").Find(&items)
	out := make([]gin.H, 0, len(items))
	for _, u := range items {
		name := u.DisplayName
		if name == "" {
			name = u.Username
		}
		out = append(out, gin.H{"id": u.ID, "username": u.Username, "display_name": name, "role": u.Role})
	}
	response.Success(c, gin.H{"items": out})
}

// ListUsers 用户列表
func (h *Handler) ListUsers(c *gin.Context) {
	var items []model.User
	h.DB.Order("id").Find(&items)
	views := make([]userView, 0, len(items))
	for _, u := range items {
		views = append(views, userView{User: u, SiteIDArr: parseSiteIDs(u.SiteIDs)})
	}
	response.Success(c, gin.H{"items": views, "total": len(views), "groups": groupOptions()})
}

// groupOptions 返回可分配的菜单分组定义（前端渲染权限矩阵用）
func groupOptions() []gin.H {
	defs := []struct{ Key, Name string }{
		{"dashboard", "运行总览"},
		{"infra", "基础设施"},
		{"asset", "资产管理"},
		{"ops", "运维管理"},
		{"monitor", "动环监控"},
		{"energy", "能耗管理"},
		{"visual", "可视化"},
		{"network", "网络管理"},
		{"report", "报表与API"},
		{"system", "系统管理"},
	}
	out := make([]gin.H, 0, len(defs))
	for _, d := range defs {
		out = append(out, gin.H{"key": d.Key, "name": d.Name})
	}
	return out
}

// CreateUser 新增用户
func (h *Handler) CreateUser(c *gin.Context) {
	var req userReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Username == "" || req.Password == "" {
		response.BadRequest(c, "用户名和密码不能为空")
		return
	}
	if !validRoles[req.Role] {
		response.BadRequest(c, "角色无效（global_admin/site_admin/operator/viewer）")
		return
	}
	if req.Role != "global_admin" && req.SiteIDs == "" {
		response.BadRequest(c, "非全局管理员必须指定站点范围")
		return
	}
	if ok, msg := ValidatePasswordStrength(req.Password); !ok {
		response.BadRequest(c, msg)
		return
	}
	groups, ok := normalizeGroups(req.AllowedGroups)
	if !ok {
		response.BadRequest(c, "菜单权限分组无效")
		return
	}
	var cnt int64
	h.DB.Model(&model.User{}).Where("username = ?", req.Username).Count(&cnt)
	if cnt > 0 {
		response.BadRequest(c, "用户名已存在")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	u := model.User{
		Username:      req.Username,
		PasswordHash:  string(hash),
		DisplayName:   req.DisplayName,
		Role:          req.Role,
		SiteIDs:       req.SiteIDs,
		Phone:         req.Phone,
		AllowedGroups: groups,
		Status:        1,
	}
	if req.Status != nil {
		u.Status = *req.Status
	}
	if err := h.DB.Create(&u).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_user", req.Username, "角色="+req.Role)
	response.Created(c, u)
}

// UpdateUser 更新用户（不改密码；密码重置走独立接口）
func (h *Handler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID 无效")
		return
	}
	var u model.User
	if err := h.DB.First(&u, id).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	var req userReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Role != "" {
		if !validRoles[req.Role] {
			response.BadRequest(c, "角色无效")
			return
		}
		u.Role = req.Role
	}
	if u.Role == "global_admin" {
		u.SiteIDs = "" // 全局管理员不受站点限制，清空范围
	} else if strings.TrimSpace(req.SiteIDs) != "" {
		u.SiteIDs = req.SiteIDs
	} else if strings.TrimSpace(u.SiteIDs) == "" {
		response.BadRequest(c, "非全局管理员必须指定站点范围")
		return
	}
	if req.DisplayName != "" {
		u.DisplayName = req.DisplayName
	}
	if req.Phone != "" {
		u.Phone = req.Phone
	}
	if req.Status != nil {
		if *req.Status != 0 && *req.Status != 1 {
			response.BadRequest(c, "状态值无效")
			return
		}
		u.Status = *req.Status
	}
	if req.AllowedGroups != "" {
		groups, ok := normalizeGroups(req.AllowedGroups)
		if !ok {
			response.BadRequest(c, "菜单权限分组无效")
			return
		}
		u.AllowedGroups = groups
	}
	h.DB.Save(&u)
	middleware.WriteAudit(h.DB, c, "update_user", u.Username, "")
	response.Success(c, u)
}

// ResetUserPassword 管理员重置指定用户密码
func (h *Handler) ResetUserPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID 无效")
		return
	}
	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NewPassword == "" {
		response.BadRequest(c, "请提供新密码")
		return
	}
	if ok, msg := ValidatePasswordStrength(req.NewPassword); !ok {
		response.BadRequest(c, msg)
		return
	}
	var u model.User
	if err := h.DB.First(&u, id).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	h.DB.Model(&u).Update("password_hash", string(hash))
	middleware.WriteAudit(h.DB, c, "reset_password", u.Username, "管理员重置密码")
	response.Success(c, gin.H{"ok": true})
}

// UpdateProfile 用户修改自己的显示名/电话
func (h *Handler) UpdateProfile(c *gin.Context) {
	uid, _ := c.Get("user_id")
	var u model.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	var req struct {
		DisplayName *string `json:"display_name"`
		Phone       *string `json:"phone"`
		Email       *string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.DisplayName != nil {
		u.DisplayName = *req.DisplayName
		updates["display_name"] = u.DisplayName
	}
	if req.Phone != nil {
		u.Phone = *req.Phone
		updates["phone"] = u.Phone
	}
	if req.Email != nil {
		em := *req.Email
		if em != "" && !regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`).MatchString(em) {
			response.BadRequest(c, "邮箱格式不正确，示例：name@company.com")
			return
		}
		u.Email = em
		updates["email"] = em
	}
	if len(updates) == 0 {
		response.BadRequest(c, "没有需要更新的字段")
		return
	}
	h.DB.Model(&u).Updates(updates)
	middleware.WriteAudit(h.DB, c, "update_profile", u.Username, "更新个人资料")
	response.Success(c, gin.H{"user": u})
}

// DeleteUser 删除用户
func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID 无效")
		return
	}
	uid, _ := c.Get("user_id")
	if uint(id) == uid {
		response.BadRequest(c, "不能删除自己")
		return
	}
	var u model.User
	if err := h.DB.First(&u, id).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	if u.Username == "admin" {
		response.Forbidden(c, "内置管理员不可删除")
		return
	}
	h.DB.Delete(&u)
	middleware.WriteAudit(h.DB, c, "delete_user", u.Username, "")
	response.Success(c, gin.H{"ok": true})
}

// ==================== 审计日志 ====================

// ListAudits 审计日志分页
func (h *Handler) ListAudits(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", c.DefaultQuery("page_size", "20")))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	q := h.DB.Model(&model.Audit{})
	if action := c.Query("action"); action != "" {
		q = q.Where("action = ?", action)
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("action ILIKE ? OR target ILIKE ? OR username ILIKE ? OR detail ILIKE ?", like, like, like, like)
	}
	if start := c.Query("start"); start != "" {
		q = q.Where("created_at >= ?", start)
	}
	if end := c.Query("end"); end != "" {
		q = q.Where("created_at < ?", end+" 23:59:59")
	}
	var total int64
	q.Count(&total)
	var items []model.Audit
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

// ValidatePasswordStrength 密码强度校验：至少8位，含大小写字母+数字+特殊字符
func ValidatePasswordStrength(pw string) (bool, string) {
	if len(pw) < 8 {
		return false, "密码至少8位"
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r): hasUpper = true
		case unicode.IsLower(r): hasLower = true
		case unicode.IsDigit(r): hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r): hasSpecial = true
		}
	}
	missing := ""
	if !hasUpper { missing += "大写字母 " }
	if !hasLower { missing += "小写字母 " }
	if !hasDigit { missing += "数字 " }
	if !hasSpecial { missing += "特殊字符 " }
	if missing != "" {
		return false, "密码需包含：" + missing
	}
	return true, ""
}
