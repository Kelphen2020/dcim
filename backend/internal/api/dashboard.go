package api

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// Dashboard 仪表盘首页统计
func (h *Handler) Dashboard(c *gin.Context) {
	scope := middleware.GetSiteScope(c)
	q := func(table string) *gin.H { return nil }
	_ = q

	count := func(m interface{}, where string, args ...interface{}) int64 {
		var n int64
		q := h.DB.Model(m)
		if scope != nil {
			q = q.Where("site_id IN ?", scope)
		}
		if where != "" {
			q = q.Where(where, args...)
		}
		q.Count(&n)
		return n
	}

	sites := count(&model.Site{}, "")
	rooms := count(&model.Room{}, "")
	cabinets := count(&model.Cabinet{}, "")
	cabActive := count(&model.Cabinet{}, "status = 1")
	gws := count(&model.Gateway{}, "")
	gwOnline := count(&model.Gateway{}, "online = true")
	devices := count(&model.Asset{}, "")
	devicesMounted := count(&model.Asset{}, "status = 1")

	// P2 运维统计
	ordersOpen := count(&model.WorkOrder{}, "status IN ?", []int{1, 2})
	changesPending := count(&model.ChangeRequest{}, "status = ?", 2)
	changesRunning := count(&model.ChangeRequest{}, "status = ?", 5)
	powerAlarm := count(&model.PowerChain{}, "status IN ?", []int{2, 3})

	// P3 动环统计
	monitorPoints := count(&model.MonitoringPoint{}, "")
	alertRules := count(&model.AlertRule{}, "enabled = ?", true)
	activeAlerts := count(&model.Alert{}, "status = ?", 0)

	// ===== V43 运行焦点：告警分级 / 测点状态 / 今日告警 / 最近活跃告警 =====
	var monNormal, monAlerting, monOffline int64
	monNormal = count(&model.MonitoringPoint{}, "status = 1")
	monAlerting = count(&model.MonitoringPoint{}, "status IN ?", []int{2, 3})
	monOffline = count(&model.MonitoringPoint{}, "status = 0")

	type sevRow struct {
		S1 int64
		S2 int64
		S3 int64
	}
	var sev sevRow
	sevQ := h.DB.Model(&model.Alert{}).Where("status = 0").
		Select("COUNT(*) FILTER (WHERE severity = 1) as s1, COUNT(*) FILTER (WHERE severity = 2) as s2, COUNT(*) FILTER (WHERE severity = 3) as s3")
	if scope != nil {
		sevQ = sevQ.Where("site_id IN ?", scope)
	}
	sevQ.Scan(&sev)

	var todayAlerts int64
	taQ := h.DB.Model(&model.Alert{}).Where("created_at::date = ?", time.Now().Format("2006-01-02"))
	if scope != nil {
		taQ = taQ.Where("site_id IN ?", scope)
	}
	taQ.Count(&todayAlerts)

	type alertRow struct {
		ID          uint      `json:"id"`
		Severity    int       `json:"severity"`
		Message     string    `json:"message"`
		PointID     *uint     `json:"point_id"`
		PointName   string    `json:"point_name"`
		TriggeredAt time.Time `json:"triggered_at"`
	}
	var recentAlerts []alertRow
	today0 := time.Now().Format("2006-01-02")
	raQ := h.DB.Table("t_alert a").
		Select("a.id, a.severity, a.message, a.point_id, COALESCE(p.name,'') as point_name, a.triggered_at").
		Joins("LEFT JOIN t_monitoring_point p ON p.id = a.point_id").
		Where("a.status = 0").Order("a.severity DESC, a.id DESC").Limit(6)
	if scope != nil {
		raQ = raQ.Where("a.site_id IN ?", scope)
	}
	raQ.Scan(&recentAlerts)
	if recentAlerts == nil {
		recentAlerts = []alertRow{}
	}

	// 最近审计
	var audits []model.Audit
	h.DB.Order("id DESC").Limit(8).Find(&audits)

	// 最近工单（仪表盘速览）
	var orders []model.WorkOrder
	oq := h.DB.Model(&model.WorkOrder{}).Order("id DESC").Limit(5)
	if scope != nil {
		oq = oq.Where("site_id IN ?", scope)
	}
	oq.Find(&orders)

	// V6 能耗速览
	energy := gin.H{}
	{
		pq := h.DB.Model(&model.PUERecord{})
		if scope != nil {
			pq = pq.Where("site_id IN ?", scope)
		}
		type pueRow struct {
			PUE        float64
			TotalPower float64
			ITPower    float64
		}
		var latest pueRow
		pq.Order("recorded_at DESC").Limit(1).Scan(&latest)
		energy["latest_pue"] = round2(latest.PUE)
		energy["it_power_kw"] = round2(latest.ITPower)
		energy["total_power_kw"] = round2(latest.TotalPower)

		// 近 7 天能耗（kWh，每小时一条采样积分近似）
		since := time.Now().Add(-7 * 24 * time.Hour)
		var pwrRow struct {
			Cnt int64
			Avg float64
		}
		sq := h.DB.Model(&model.PUERecord{}).
			Select("COUNT(*) as cnt, COALESCE(AVG(total_power),0) as avg").
			Where("recorded_at >= ?", since)
		if scope != nil {
			sq = sq.Where("site_id IN ?", scope)
		}
		sq.Scan(&pwrRow)
		energy["energy_7d_kwh"] = round2(float64(pwrRow.Cnt) * pwrRow.Avg)

		// 近 24 小时 PUE 走势（每小时均值，最多 24 点）
		type trendRow struct {
			Hour string
			PUE  float64
		}
		var trend []trendRow
		tq := h.DB.Model(&model.PUERecord{}).
			Select("to_char(date_trunc('hour', recorded_at), 'MM-DD HH24:MI') as hour, ROUND(AVG(pue)::numeric, 3) as pue").
			Where("recorded_at >= ?", time.Now().Add(-24*time.Hour)).
			Group("date_trunc('hour', recorded_at)").Order("date_trunc('hour', recorded_at)")
		if scope != nil {
			tq = tq.Where("site_id IN ?", scope)
		}
		tq.Find(&trend)
		labels := make([]string, 0, len(trend))
		vals := make([]float64, 0, len(trend))
		for _, t := range trend {
			labels = append(labels, t.Hour)
			vals = append(vals, t.PUE)
		}
		energy["trend_labels"] = labels
		energy["trend_values"] = vals
	}

	response.Success(c, gin.H{
		"stats": gin.H{
			"sites":           sites,
			"rooms":           rooms,
			"cabinets":        cabinets,
			"cabinets_active": cabActive,
			"gateways":        gws,
			"gateways_online": gwOnline,
			"alarms_active":   0, // P3 接入
			"devices":         devices,
			"devices_mounted": devicesMounted,
			"orders_open":     ordersOpen,
			"changes_pending": changesPending,
			"changes_running": changesRunning,
			"power_alarm":     powerAlarm,
				"active_alerts":   activeAlerts,
				"monitor_points":  monitorPoints,
				"alert_rules":     alertRules,
				"monitor_normal":  monNormal,
				"monitor_alert":   monAlerting,
				"monitor_offline": monOffline,
				"today_alerts":    todayAlerts,
				"sev1":            sev.S1,
				"sev2":            sev.S2,
				"sev3":            sev.S3,
				"today_orders":    count(&model.WorkOrder{}, "created_at::date = ?", today0),
				},
				"recent_alerts": recentAlerts,
		"recent_orders": orders,
		"recent_audits": audits,
		"energy":        energy,
	})
}

// ==================== 站点网关管理 ====================

type gatewayReq struct {
	SiteID uint   `json:"site_id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Remark string `json:"remark"`
}

// ListGateways 网关列表
func (h *Handler) ListGateways(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.Gateway{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	var total int64
	q.Count(&total)
	var items []model.Gateway
	q.Order("id").Offset((page - 1) * size).Limit(size).Find(&items)
	response.Paginated(c, items, total, page, size)
}

// CreateGateway 注册网关（生成接入密钥）
func (h *Handler) CreateGateway(c *gin.Context) {
	var req gatewayReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.SiteID == 0 || req.Code == "" {
		response.BadRequest(c, "站点和网关编码不能为空")
		return
	}
	var site model.Site
	if err := h.DB.First(&site, req.SiteID).Error; err != nil {
		response.BadRequest(c, "所属站点不存在")
		return
	}
	if !middleware.InSiteScope(c, req.SiteID) {
		response.Forbidden(c, "无权在该站点注册网关")
		return
	}
	var cnt int64
	h.DB.Model(&model.Gateway{}).Where("code = ?", req.Code).Count(&cnt)
	if cnt > 0 {
		response.BadRequest(c, "网关编码已存在")
		return
	}
	buf := make([]byte, 16)
	rand.Read(buf)
	gw := model.Gateway{
		SiteID: req.SiteID, Code: req.Code, Name: req.Name,
		Secret: hex.EncodeToString(buf), Status: 1, Remark: req.Remark,
	}
	if err := h.DB.Create(&gw).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_gateway", req.Code, "站点="+site.Name)
	// 密钥仅创建时返回一次
	response.Created(c, gin.H{"gateway": gw, "secret": gw.Secret})
}

// UpdateGateway 更新网关基本信息（不含密钥）
func (h *Handler) UpdateGateway(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var gw model.Gateway
	if err := h.DB.First(&gw, id).Error; err != nil {
		response.NotFound(c, "网关不存在")
		return
	}
	if !middleware.InSiteScope(c, gw.SiteID) {
		response.Forbidden(c, "无权访问该站点数据")
		return
	}
	var req struct {
		gatewayReq
		Status  *int   `json:"status"`
		IP      string `json:"ip"`
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Remark != "" {
		updates["remark"] = req.Remark
	}
	if req.SiteID != 0 && req.SiteID != gw.SiteID {
		if !middleware.InSiteScope(c, req.SiteID) {
			response.Forbidden(c, "无权迁移到目标站点")
			return
		}
		updates["site_id"] = req.SiteID
	}
	if req.Code != "" && req.Code != gw.Code {
		var cnt int64
		h.DB.Model(&model.Gateway{}).Where("code = ? AND id <> ?", req.Code, id).Count(&cnt)
		if cnt > 0 {
			response.BadRequest(c, "网关编码已存在")
			return
		}
		updates["code"] = req.Code
	}
	if req.Status != nil && (*req.Status == 0 || *req.Status == 1) {
		updates["status"] = *req.Status
	}
	if req.IP != "" {
		updates["ip"] = req.IP
	}
	if req.Version != "" {
		updates["version"] = req.Version
	}
	if len(updates) > 0 {
		if err := h.DB.Model(&gw).Updates(updates).Error; err != nil {
			response.ServerError(c, "更新失败")
			return
		}
		middleware.WriteAudit(h.DB, c, "update_gateway", gw.Code, "")
	}
	h.DB.First(&gw, id)
	response.Success(c, gw)
}

// DeleteGateway 删除网关
func (h *Handler) DeleteGateway(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var gw model.Gateway
	if err := h.DB.First(&gw, id).Error; err != nil {
		response.NotFound(c, "网关不存在")
		return
	}
	if !middleware.InSiteScope(c, gw.SiteID) {
		response.Forbidden(c, "无权访问该站点数据")
		return
	}
	h.DB.Delete(&gw)
	middleware.WriteAudit(h.DB, c, "delete_gateway", gw.Code, "")
	response.Success(c, gin.H{"ok": true})
}

// ==================== 网关接入端点（边缘网关调用） ====================

// GatewayRegister 网关上线注册
func (h *Handler) GatewayRegister(c *gin.Context) {
	var req struct {
		Code    string `json:"code"`
		Secret  string `json:"secret"`
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	var gw model.Gateway
	if err := h.DB.Where("code = ?", req.Code).First(&gw).Error; err != nil {
		response.Unauthorized(c, "网关未注册")
		return
	}
	if gw.Status != 1 {
		response.Forbidden(c, "网关已停用")
		return
	}
	if gw.Secret != req.Secret {
		response.Unauthorized(c, "接入密钥错误")
		return
	}
	now := time.Now()
	h.DB.Model(&gw).Updates(map[string]interface{}{
		"online": true, "last_seen_at": now, "version": req.Version, "ip": c.ClientIP(),
	})
	response.Success(c, gin.H{
		"gateway_id": gw.ID,
		"site_id":    gw.SiteID,
		"hb_interval": 15, // 心跳间隔秒
	})
}

// GatewayHeartbeat 网关心跳
func (h *Handler) GatewayHeartbeat(c *gin.Context) {
	var req struct {
		Code   string `json:"code"`
		Secret string `json:"secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	var gw model.Gateway
	if err := h.DB.Where("code = ? AND secret = ?", req.Code, req.Secret).First(&gw).Error; err != nil {
		response.Unauthorized(c, "网关认证失败")
		return
	}
	now := time.Now()
	h.DB.Model(&gw).Updates(map[string]interface{}{"online": true, "last_seen_at": now})
	response.Success(c, gin.H{"ok": true})
}

// GatewayCommands 拉取待执行指令（P0 骨架：返回空；P3 接入指令队列）
func (h *Handler) GatewayCommands(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cmds []model.GatewayCommand
	h.DB.Where("gateway_id = ?", id).Order("id desc").Limit(50).Find(&cmds)
	response.Success(c, gin.H{"commands": cmds})
}

// GatewayOfflineCheck 后台任务：心跳超时置离线（由 main 定时调用）
func (h *Handler) GatewayOfflineCheck() {
	threshold := time.Now().Add(-45 * time.Second)
	h.DB.Model(&model.Gateway{}).
		Where("online = ? AND last_seen_at < ?", true, threshold).
		Update("online", false)
}
