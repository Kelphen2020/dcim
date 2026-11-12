package api

// mobile.go — M1 移动端聚合接口
// GET  /api/v1/mobile/overview      工作台一屏聚合（KPI+待办+今日巡检+值班+PUE）
// POST /api/v1/mobile/upload        通用附件上传（工单/巡检拍照）
// GET  /api/v1/mobile/push-channels 我的推送通道绑定
// POST /api/v1/mobile/push-channels 绑定/更新推送通道（device_id 幂等）
// DELETE /api/v1/mobile/push-channels/:id 解绑

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// PushChannel 用户设备推送通道绑定
type PushChannel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	DeviceID  string    `gorm:"size:128;uniqueIndex;not null" json:"device_id"` // 设备唯一标识（幂等键）
	Channel   string    `gorm:"size:20;not null" json:"channel"`               // unipush / wxsub / feishu
	Token     string    `gorm:"size:500" json:"-"`                             // 推送 token（不回显）
	Platform  string    `gorm:"size:20" json:"platform"`                       // android / ios / wxmp / feishu
	Events    string    `gorm:"size:255;default:'*'" json:"events"`            // 订阅事件逗号分隔，*=全部
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PushChannel) TableName() string { return "t_push_channel" }

// MobileOverview GET /mobile/overview?site_id=0
// 一次返回移动端工作台所需全部数据，避免小程序端多次请求
func (h *Handler) MobileOverview(c *gin.Context) {
	uid := middleware.GetUserID(c)
	siteID := uint(0)
	if s := c.Query("site_id"); s != "" && s != "0" {
		fmt.Sscanf(s, "%d", &siteID)
	}
	now := time.Now()
	today0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	// ---- 活跃告警（status<3，排除被抑制成员）----
	type sevRow struct {
		Severity int   `json:"severity"`
		N        int64 `json:"n"`
	}
	var sevRows []sevRow
	aq := h.DB.Model(&model.Alert{}).Where("status < 3 AND suppressed = ?", false)
	if siteID > 0 {
		aq = aq.Where("site_id = ?", siteID)
	}
	aq.Select("severity, COUNT(*) as n").Group("severity").Scan(&sevRows)
	alertBySev := map[int]int64{1: 0, 2: 0, 3: 0}
	var alertTotal int64
	for _, r := range sevRows {
		alertBySev[r.Severity] = r.N
		alertTotal += r.N
	}
	// 风暴数
	var stormCount int64
	h.DB.Model(&model.Alert{}).Where("status < 3 AND storm_id > 0 AND suppressed = ?", false).
		Distinct("storm_id").Count(&stormCount)

	// 最新 5 条活跃告警（附测点名）
	type alertRow struct {
		ID        uint      `json:"id"`
		Severity  int       `json:"severity"`
		Message   string    `json:"message"`
		Status    int       `json:"status"`
		PointName string    `json:"point_name"`
		StormID   uint      `json:"storm_id"`
		Triggered time.Time `json:"triggered_at"`
	}
	var latestAlerts []alertRow
	laq := h.DB.Table("t_alert a").
		Select("a.id, a.severity, a.message, a.status, a.storm_id, a.triggered_at, COALESCE(p.name,'') as point_name").
		Joins("LEFT JOIN t_monitoring_point p ON p.id = a.point_id").
		Where("a.status < 3 AND a.suppressed = ?", false)
	if siteID > 0 {
		laq = laq.Where("a.site_id = ?", siteID)
	}
	laq.Order("a.severity DESC, a.triggered_at DESC").Limit(5).Scan(&latestAlerts)
	if latestAlerts == nil {
		latestAlerts = []alertRow{}
	}

	// ---- 待办工单 ----
	// 待我处理：指派给我且未完成（status 1待受理 2处理中）
	var myTodo, myTodoBreach int64
	wq := h.DB.Model(&model.WorkOrder{}).Where("assignee_id = ? AND status IN (1,2)", uid)
	if siteID > 0 {
		wq = wq.Where("site_id = ?", siteID)
	}
	wq.Count(&myTodo)
	wq2 := h.DB.Model(&model.WorkOrder{}).
		Where("assignee_id = ? AND status IN (1,2) AND (sla_respond_breach = ? OR sla_resolve_breach = ?)", uid, true, true)
	if siteID > 0 {
		wq2 = wq2.Where("site_id = ?", siteID)
	}
	wq2.Count(&myTodoBreach)
	// 全站未闭环工单
	var woOpen int64
	wq3 := h.DB.Model(&model.WorkOrder{}).Where("status IN (1,2)")
	if siteID > 0 {
		wq3 = wq3.Where("site_id = ?", siteID)
	}
	wq3.Count(&woOpen)

	// 我的待办工单 top5（SLA 紧迫优先）
	type woRow struct {
		ID            uint       `json:"id"`
		Code          string     `json:"code"`
		Title         string     `json:"title"`
		Priority      int        `json:"priority"`
		Status        int        `json:"status"`
		SlaResolveDue *time.Time `json:"sla_resolve_due"`
		SlaBreach     bool       `json:"sla_breach"`
		CreatedAt     time.Time  `json:"created_at"`
	}
	var myWos []woRow
	mwq := h.DB.Model(&model.WorkOrder{}).
		Select("id, code, title, priority, status, sla_resolve_due, (sla_respond_breach OR sla_resolve_breach) as sla_breach, created_at").
		Where("assignee_id = ? AND status IN (1,2)", uid)
	if siteID > 0 {
		mwq = mwq.Where("site_id = ?", siteID)
	}
	mwq.Order("sla_breach DESC, priority ASC, created_at ASC").Limit(5).Scan(&myWos)
	if myWos == nil {
		myWos = []woRow{}
	}

	// ---- 今日巡检 ----
	var planTotal, doneToday int64
	pq := h.DB.Model(&model.InspectionPlan{}).Where("status = 1")
	if siteID > 0 {
		pq = pq.Where("site_id = ? OR site_id = 0", siteID)
	}
	pq.Count(&planTotal)
	rq := h.DB.Model(&model.InspectionRecord{}).Where("inspected_at >= ?", today0)
	if siteID > 0 {
		rq = rq.Where("site_id = ?", siteID)
	}
	rq.Distinct("plan_id").Count(&doneToday)

	// ---- 值班 ----
	duty := gin.H{"on_duty": false, "shift": currentShift(now), "mine": false}
	ds := h.OnDutyUser(siteID)
	if ds != nil {
		var u model.User
		h.DB.First(&u, ds.UserID)
		duty = gin.H{
			"on_duty": true, "shift": ds.Shift, "user_id": u.ID,
			"user_name": u.DisplayName, "username": u.Username,
			"mine": ds.UserID == uid,
		}
	}

	// ---- 测点在线率 ----
	var ptTotal, ptOnline int64
	pqq := h.DB.Model(&model.MonitoringPoint{})
	if siteID > 0 {
		pqq = pqq.Where("site_id = ?", siteID)
	}
	pqq.Count(&ptTotal)
	pq2 := h.DB.Model(&model.MonitoringPoint{}).Where("status = 1")
	if siteID > 0 {
		pq2 = pq2.Where("site_id = ?", siteID)
	}
	pq2.Count(&ptOnline)
	onlineRate := 0.0
	if ptTotal > 0 {
		onlineRate = float64(ptOnline) / float64(ptTotal) * 100
	}

	// ---- 最新 PUE ----
	var pueRow struct {
		PUE        float64   `json:"pue"`
		TotalPower float64   `json:"total_power"`
		RecordedAt time.Time `json:"recorded_at"`
	}
	pueQ := h.DB.Model(&model.PUERecord{})
	if siteID > 0 {
		pueQ = pueQ.Where("site_id = ?", siteID)
	}
	pueQ.Order("recorded_at DESC").Limit(1).Scan(&pueRow)

	// ---- 未读通知 ----
	var unread int64
	h.DB.Model(&model.Notification{}).
		Where("(user_id = ? OR user_id = 0) AND read = ?", uid, false).Count(&unread)

	// ---- 站点列表（切换器）----
	var sites []model.Site
	h.DB.Where("status > 0").Order("id").Find(&sites)
	if sites == nil {
		sites = []model.Site{}
	}
	siteOpts := make([]gin.H, 0, len(sites))
	for _, s := range sites {
		siteOpts = append(siteOpts, gin.H{"id": s.ID, "name": s.Name, "code": s.Code})
	}

	response.Success(c, gin.H{
		"alerts": gin.H{
			"total": alertTotal, "by_severity": alertBySev,
			"storm_count": stormCount, "latest": latestAlerts,
		},
		"work_orders": gin.H{
			"my_todo": myTodo, "my_todo_breach": myTodoBreach,
			"open_total": woOpen, "my_list": myWos,
		},
		"inspection": gin.H{"plan_total": planTotal, "done_today": doneToday},
		"duty":       duty,
		"points": gin.H{
			"total": ptTotal, "online": ptOnline,
			"online_rate": float64(int(onlineRate*10)) / 10,
		},
		"assets":        h.mobileAssetStats(siteID), // M6: 设备统计块
		"today_plans":   h.mobileTodayPlans(siteID, uid), // M6-E2: 今日巡检计划
		"spare_warn":    h.mobileSpareWarn(),              // M6-E4: 备件低库存预警
		"pue":           pueRow,
		"unread":        unread,
		"sites":         siteOpts,
		"generated_at":  now.Format(time.RFC3339),
	})
}

// MobileUpload POST /mobile/upload  (multipart: file, ref_type, ref_id)
// 通用附件上传：工单处理拍照 / 巡检异常拍照，图片 ≤10MB
func (h *Handler) MobileUpload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少 file 字段")
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}
	if !allowed[ext] {
		response.BadRequest(c, "仅支持 jpg/png/webp/gif 图片")
		return
	}
	if fh.Size > 10<<20 {
		response.BadRequest(c, "图片超过 10MB 限制")
		return
	}
	day := time.Now().Format("20060102")
	dir := filepath.Join("uploads", "attachments", day)
	_ = os.MkdirAll(dir, 0o755)
	name := fmt.Sprintf("a%d_%d%s", time.Now().UnixMilli(), middleware.GetUserID(c), ext)
	dst := filepath.Join(dir, name)
	src, err := fh.Open()
	if err != nil {
		response.ServerError(c, "读取失败")
		return
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		response.ServerError(c, "写入失败")
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		response.ServerError(c, "保存失败")
		return
	}
	url := "/" + filepath.ToSlash(dst)
	refType := c.PostForm("ref_type")
	refID := c.PostForm("ref_id")
	h.saveAttachment(c, url, fh.Filename, fh.Size) // M6: 附件落库（工单/巡检照片真闭环）
	go middleware.WriteAudit(h.DB, c, "upload", "attachment",
		fmt.Sprintf("mobile attachment %s ref=%s:%s size=%d", url, refType, refID, fh.Size))
	response.Success(c, gin.H{"url": url, "name": fh.Filename, "size": fh.Size})
}

// ListMyPushChannels GET /mobile/push-channels
func (h *Handler) ListMyPushChannels(c *gin.Context) {
	uid := middleware.GetUserID(c)
	var items []PushChannel
	h.DB.Where("user_id = ?", uid).Order("id DESC").Find(&items)
	if items == nil {
		items = []PushChannel{}
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

type pushChannelReq struct {
	DeviceID string `json:"device_id"`
	Channel  string `json:"channel"`
	Token    string `json:"token"`
	Platform string `json:"platform"`
	Events   string `json:"events"`
	Enabled  *bool  `json:"enabled"`
}

// UpsertPushChannel POST /mobile/push-channels — device_id 幂等
func (h *Handler) UpsertPushChannel(c *gin.Context) {
	var req pushChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		response.BadRequest(c, "device_id 必填")
		return
	}
	if !map[string]bool{"unipush": true, "wxsub": true, "feishu": true}[req.Channel] {
		response.BadRequest(c, "channel 须为 unipush/wxsub/feishu")
		return
	}
	uid := middleware.GetUserID(c)
	var pc PushChannel
	err := h.DB.Where("device_id = ?", req.DeviceID).First(&pc).Error
	updates := map[string]interface{}{
		"channel":  req.Channel,
		"platform": req.Platform,
		"enabled":  true,
	}
	if req.Token != "" {
		updates["token"] = req.Token
	}
	if strings.TrimSpace(req.Events) != "" {
		updates["events"] = req.Events
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if err != nil { // 新建
		pc = PushChannel{UserID: uid, DeviceID: req.DeviceID}
		if err := h.DB.Create(&pc).Error; err != nil {
			response.ServerError(c, "创建失败: "+err.Error())
			return
		}
	} else if pc.UserID != uid { // 设备易主：换绑到当前用户
		updates["user_id"] = uid
	}
	if err := h.DB.Model(&pc).Updates(updates).Error; err != nil {
		response.ServerError(c, "更新失败: "+err.Error())
		return
	}
	go middleware.WriteAudit(h.DB, c, "bind", "push_channel",
		fmt.Sprintf("device=%s channel=%s platform=%s", req.DeviceID, req.Channel, req.Platform))
	h.DB.First(&pc, pc.ID)
	response.Success(c, pc)
}

// DeletePushChannel DELETE /mobile/push-channels/:id — 只能解绑自己的
func (h *Handler) DeletePushChannel(c *gin.Context) {
	uid := middleware.GetUserID(c)
	var pc PushChannel
	if err := h.DB.First(&pc, c.Param("id")).Error; err != nil {
		response.NotFound(c, "通道不存在")
		return
	}
	if pc.UserID != uid {
		response.Forbidden(c, "只能解绑自己的设备")
		return
	}
	h.DB.Delete(&pc)
	go middleware.WriteAudit(h.DB, c, "unbind", "push_channel", fmt.Sprintf("device=%s", pc.DeviceID))
	response.Success(c, gin.H{"deleted": pc.ID})
}
