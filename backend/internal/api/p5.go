package api

import (
	"fmt"
	"strconv"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== PUE 能效 ====================

func (h *Handler) ListPUERecords(c *gin.Context) {
	siteID := c.DefaultQuery("site_id", "1")
	days := c.DefaultQuery("days", "7")
	q := h.DB.Model(&model.PUERecord{}).Where("site_id = ?", siteID)
	if days != "" {
		cutoff := time.Now().AddDate(0, 0, -30)
		q = q.Where("recorded_at >= ?", cutoff)
	}
	var records []model.PUERecord
	q.Order("recorded_at").Limit(720).Find(&records)
	response.Success(c, gin.H{"items": records, "total": len(records)})
}

func (h *Handler) AddPUERecord(c *gin.Context) {
	var req struct {
		SiteID     uint    `json:"site_id"`
		TotalPower float64 `json:"total_power"`
		ITPower    float64 `json:"it_power"`
	}
	if err := c.ShouldBindJSON(&req); err != nil { response.Error(c, 400, "参数错误"); return }
	if req.ITPower <= 0 { response.Error(c, 400, "IT功率必须大于0"); return }
	pue := req.TotalPower / req.ITPower
	rec := model.PUERecord{
		SiteID:     req.SiteID,
		TotalPower: req.TotalPower,
		ITPower:    req.ITPower,
		PUE:        pue,
		RecordedAt: time.Now(),
	}
	h.DB.Create(&rec)
	response.Success(c, rec)
}

// ==================== 大屏聚合 ====================

func (h *Handler) BigScreenData(c *gin.Context) {
	siteID := c.DefaultQuery("site_id", "1")

	// 设备统计（1=在架 2=在库）
	var devices, mounted int64
	h.DB.Model(&model.Asset{}).Where("site_id = ?", siteID).Count(&devices)
	h.DB.Model(&model.Asset{}).Where("site_id = ? AND status = 1", siteID).Count(&mounted)

	// 告警
	var activeAlerts int64
	h.DB.Model(&model.Alert{}).Where("status = 0").Count(&activeAlerts)

	// 当天工单
	var todayOrders int64
	today := time.Now().Format("2006-01-02")
	h.DB.Model(&model.WorkOrder{}).Where("created_at::date = ?", today).Count(&todayOrders)

	// 最近告警
	var recentAlerts []model.Alert
	h.DB.Where("status = 0").Order("id DESC").Limit(5).Find(&recentAlerts)

	// 供电链路概览
	var powerTotal, powerNormal, powerAlarm int64
	h.DB.Model(&model.PowerChain{}).Where("site_id = ?", siteID).Count(&powerTotal)
	h.DB.Model(&model.PowerChain{}).Where("site_id = ? AND status = 1", siteID).Count(&powerNormal)
	h.DB.Model(&model.PowerChain{}).Where("site_id = ? AND status IN (2,3)", siteID).Count(&powerAlarm)

	// 网络设备
	var netDevices, netOnline int64
	h.DB.Model(&model.NetworkDevice{}).Where("site_id = ?", siteID).Count(&netDevices)
	h.DB.Model(&model.NetworkDevice{}).Where("site_id = ? AND status = 1", siteID).Count(&netOnline)

	// 最新PUE
	var latestPUE model.PUERecord
	h.DB.Where("site_id = ?", siteID).Order("id DESC").First(&latestPUE)

	// 机柜容量（修正字段：code/rated_power_kw）
	type cab struct {
		ID       uint    `json:"id"`
		Name     string  `json:"name"`
		RoomName string  `json:"room_name"`
		UsedU    int     `json:"used_u"`
		TotalU   int     `json:"total_u"`
		PowerKW  float64 `json:"power_kw"`
		RatedKW  float64 `json:"rated_kw"`
	}
	var cabs []cab
	h.DB.Raw(`SELECT c.id, c.code as name, COALESCE(r.name,'') as room_name,
		0 as used_u, COALESCE(c.total_u,42) as total_u,
		COALESCE(pa.kw,0) as power_kw, COALESCE(c.rated_power_kw,0) as rated_kw
		FROM t_cabinet c LEFT JOIN t_room r ON c.room_id = r.id
		LEFT JOIN (SELECT cabinet_id, ROUND(SUM(power_w)/1000.0, 2) as kw FROM t_asset WHERE cabinet_id > 0 GROUP BY cabinet_id) pa ON pa.cabinet_id = c.id
		WHERE c.site_id = ? ORDER BY r.name, c.code LIMIT 20`, siteID).Scan(&cabs)
	// used_u 由在架资产高度累加（每台设备 1U 计）
	for i := range cabs {
		var used int64
		h.DB.Raw(`SELECT COUNT(*) FROM t_asset WHERE site_id = ? AND cabinet_id = ? AND status = 1`, siteID, cabs[i].ID).Scan(&used)
		cabs[i].UsedU = int(used)
	}

	// 监控摘要
	var monTotal, monNormal, monAlert int64
	h.DB.Model(&model.MonitoringPoint{}).Where("site_id = ?", siteID).Count(&monTotal)
	h.DB.Model(&model.MonitoringPoint{}).Where("site_id = ? AND status = 1", siteID).Count(&monNormal)
	h.DB.Model(&model.MonitoringPoint{}).Where("site_id = ? AND status = 2", siteID).Count(&monAlert)

	// ===== V1 大屏增强：功率趋势（最近12小时） =====
	type puePoint struct {
		T          string  `json:"t"`
		TotalPower float64 `json:"total_power"`
		ITPower    float64 `json:"it_power"`
		PUE        float64 `json:"pue"`
	}
	var pue12h []model.PUERecord
	h.DB.Where("site_id = ? AND recorded_at >= ?", siteID, time.Now().Add(-12*time.Hour)).
		Order("recorded_at").Find(&pue12h)
	powerTrend := make([]puePoint, 0, len(pue12h))
	for _, r := range pue12h {
		powerTrend = append(powerTrend, puePoint{
			T:          r.RecordedAt.Format("15:04"),
			TotalPower: r.TotalPower, ITPower: r.ITPower, PUE: r.PUE,
		})
	}
	// 最新功率分项估算：IT=最近IT功率，制冷≈总-IT 的 62%，配电损耗≈25%，辅助≈13%
	var lastTotal, lastIT float64
	if len(pue12h) > 0 {
		lastTotal = pue12h[len(pue12h)-1].TotalPower
		lastIT = pue12h[len(pue12h)-1].ITPower
	}

	// ===== V1 大屏增强：温湿度矩阵（各房间测点实时值） =====
	type thCell struct {
		RoomID   uint    `json:"room_id"`
		RoomName string  `json:"room_name"`
		Point    string  `json:"point"`
		Value    float64 `json:"value"`
		Status   int     `json:"status"`
	}
	var thCells []thCell
	h.DB.Raw(`SELECT COALESCE(r.id,0) as room_id, COALESCE(r.name,'未分配') as room_name,
		p.name as point, COALESCE(p.current_value,0) as value, COALESCE(p.status,0) as status
		FROM t_monitoring_point p LEFT JOIN t_room r ON p.room_id = r.id
		WHERE p.site_id = ? AND p.type IN ('temp_humidity','hvac')
		ORDER BY r.name, p.code LIMIT 24`, siteID).Scan(&thCells)

	// ===== V1 大屏增强：告警分级分布 =====
	type sevStat struct {
		Severity int   `json:"severity"`
		Count    int64 `json:"count"`
	}
	var sevStats []sevStat
	h.DB.Raw(`SELECT severity, COUNT(*) as count FROM t_alert WHERE status = 0 GROUP BY severity`).Scan(&sevStats)
	sevMap := map[int]int64{1: 0, 2: 0, 3: 0}
	for _, s := range sevStats {
		sevMap[s.Severity] = s.Count
	}
	var todayAlerts int64
	h.DB.Model(&model.Alert{}).Where("triggered_at::date = ?", today).Count(&todayAlerts)

	// ===== V1 大屏增强：测点分类统计 =====
	type typeStat struct {
		PType string `json:"p_type"`
		Count int64  `json:"count"`
	}
	var typeStats []typeStat
	h.DB.Raw(`SELECT "type" as p_type, COUNT(*) as count FROM t_monitoring_point WHERE site_id = ? GROUP BY "type" ORDER BY count DESC`, siteID).Scan(&typeStats)

	// 运行天数（以最早资产创建时间为基准）
	var earliest time.Time
	h.DB.Raw(`SELECT COALESCE(MIN(created_at), NOW()) FROM t_site WHERE id = ?`, siteID).Scan(&earliest)

	// ===== V21-③ 功率四分解（IT/制冷/配电损耗/辅助）=====
	// IT=PUE记录最新值；非IT部分按行业经验比例 制冷62%/配电25%/辅助13% 拆分
	var nonIT, coolKW, distKW, auxKW float64
	if lastTotal > 0 {
		nonIT = lastTotal - lastIT
		if nonIT < 0 {
			nonIT = 0
		}
		coolKW = mathRound(nonIT*0.62, 2)
		distKW = mathRound(nonIT*0.25, 2)
		auxKW = mathRound(nonIT*0.13, 2)
	}
	powerBreakdown := gin.H{"it": mathRound(lastIT, 2), "cooling": coolKW, "electrical": distKW, "auxiliary": auxKW, "total": mathRound(lastTotal, 2)}

	// ===== V21-③ 容量雷达（市电/UPS/制冷/机柜 利用率%）=====
	type capRow struct {
		Rated float64 `json:"rated"`
		Load  float64 `json:"load"`
	}
	var utilityCap, upsCap capRow
	h.DB.Raw(`SELECT COALESCE(SUM(rated_kva),0)*0.9 as rated, COALESCE(SUM(load_kw),0) as load FROM t_power_chain WHERE site_id = ? AND kind = 'utility'`, siteID).Scan(&utilityCap)
	h.DB.Raw(`SELECT COALESCE(SUM(rated_kva),0)*0.9 as rated, COALESCE(SUM(load_kw),0) as load FROM t_power_chain WHERE site_id = ? AND kind = 'ups'`, siteID).Scan(&upsCap)
	var cabCap capRow
	h.DB.Raw(`SELECT
		(SELECT COALESCE(SUM(rated_power_kw),0) FROM t_cabinet WHERE site_id = ?) as rated,
		(SELECT COALESCE(SUM(power_w),0)/1000.0 FROM t_asset a JOIN t_cabinet c2 ON a.cabinet_id = c2.id WHERE c2.site_id = ? AND a.status = 1) as load`, siteID, siteID).Scan(&cabCap)
	// 制冷容量：hvac 测点数 × 单台额定制冷量(估算12.5kW/台) vs 非IT热负荷
	var hvacN int64
	h.DB.Model(&model.MonitoringPoint{}).Where(`site_id = ? AND "type" = 'hvac'`, siteID).Count(&hvacN)
	coolRated := float64(hvacN) * 12.5
	pctOf := func(load, rated float64) float64 {
		if rated <= 0 {
			return 0
		}
		return mathRound(load/rated*100, 1)
	}
	capacityRadar := gin.H{
		"utility": pctOf(utilityCap.Load, utilityCap.Rated),
		"ups":     pctOf(upsCap.Load, upsCap.Rated),
		"cooling": pctOf(coolKW, coolRated),
		"cabinet": pctOf(cabCap.Load, cabCap.Rated),
		"detail": gin.H{
			"utility_rated_kw": mathRound(utilityCap.Rated, 1), "utility_load_kw": mathRound(utilityCap.Load, 1),
			"ups_rated_kw": mathRound(upsCap.Rated, 1), "ups_load_kw": mathRound(upsCap.Load, 1),
			"cooling_rated_kw": coolRated, "cooling_load_kw": coolKW,
			"cabinet_rated_kw": mathRound(cabCap.Rated, 1), "cabinet_load_kw": mathRound(cabCap.Load, 1),
		},
	}

	// ===== V21-③ 项目铭牌（规划容量）=====
	var siteRow struct {
		Name    string  `json:"name"`
		Address string  `json:"address"`
		Level   int     `json:"level"`
		Manager string  `json:"manager"`
	}
	h.DB.Raw(`SELECT name, COALESCE(address,'') as address, COALESCE(level,0) as level, COALESCE(manager,'') as manager FROM t_site WHERE id = ?`, siteID).Scan(&siteRow)
	var areaSum, itRated float64
	var cabN int64
	h.DB.Raw(`SELECT COALESCE(SUM(area),0) FROM t_room WHERE site_id = ?`, siteID).Scan(&areaSum)
	type cabSum struct {
		Rated float64 `json:"rated"`
		N     int64   `json:"n"`
	}
	var cs cabSum
	h.DB.Raw(`SELECT COALESCE(SUM(rated_power_kw),0) as rated, COUNT(*) as n FROM t_cabinet WHERE site_id = ?`, siteID).Scan(&cs)
	itRated, cabN = cs.Rated, cs.N
	var uTotal, uUsed int64
	h.DB.Raw(`SELECT COALESCE(SUM(total_u),0) FROM t_cabinet WHERE site_id = ?`, siteID).Scan(&uTotal)
	h.DB.Raw(`SELECT COALESCE(SUM(a.height_u),0) FROM t_asset a JOIN t_cabinet c ON a.cabinet_id = c.id WHERE c.site_id = ? AND a.status = 1`, siteID).Scan(&uUsed)
	nameplate := gin.H{
		"site_name":     siteRow.Name,
		"address":       siteRow.Address,
		"level":         siteRow.Level,
		"manager":       siteRow.Manager,
		"utility_kva":   mathRound(utilityCap.Rated/0.9, 0), // 还原额定kVA
		"it_rated_kw":   mathRound(itRated, 0),
		"area_sqm":      mathRound(areaSum, 0),
		"cabinets":      cabN,
		"u_total":       uTotal,
		"u_used":        uUsed,
		"cooling_units": hvacN,
		"running_since": earliest.Format("2006-01-02"),
	}

	// V30-③ 运维预警角标: 近24h 趋势预警/低库存通知(轻量查询, 不重算回归)
	type opsWarn struct {
		Kind      string    `json:"kind"`
		Title     string    `json:"title"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}
	opsWarnings := []opsWarn{}
	h.DB.Raw(`SELECT kind, title, content, created_at FROM t_notification
		WHERE kind IN ('trend_forecast','spare_low') AND created_at > NOW() - INTERVAL '24 hours'
		ORDER BY created_at DESC LIMIT 8`).Scan(&opsWarnings)

	// G1(M6-F同步): 今日巡检进度 — 复用移动端 mobileTodayPlans 口径(全部站点/全员)
	todayInspections := h.mobileTodayPlans(0, 0)
	if todayInspections == nil {
		todayInspections = []gin.H{}
	}

	// G2(M6-F同步): 备件低库存 TOP5 — 复用移动端 mobileSpareWarn
	spareLow := h.mobileSpareWarn()

	response.Success(c, gin.H{
		"devices":        devices,
		"devices_mounted": mounted,
		"active_alerts":  activeAlerts,
		"today_orders":   todayOrders,
		"recent_alerts":  recentAlerts,
		"power_total":    powerTotal,
		"power_normal":   powerNormal,
		"power_alarm":    powerAlarm,
		"net_devices":    netDevices,
		"net_online":     netOnline,
		"latest_pue":     latestPUE.PUE,
		"cabinets":       cabs,
		"monitor_total":  monTotal,
		"monitor_normal": monNormal,
		"monitor_alert":  monAlert,
		// V1 新增
		"power_trend":    powerTrend,
		"power_latest":   gin.H{"total": lastTotal, "it": lastIT},
		"th_matrix":      thCells,
		"alert_severity": sevMap,
		"today_alerts":   todayAlerts,
		"point_types":    typeStats,
		"running_since":  earliest.Format("2006-01-02"),
		// V21-③
		"power_breakdown": powerBreakdown,
		"capacity_radar":  capacityRadar,
		"nameplate":       nameplate,
		// V30-③ 运维预警角标: 近24h趋势预警+低库存通知(轻量, 不重算回归)
		"ops_warnings":    opsWarnings,
		// G批: 与移动端 E/F 同源的数据块
		"today_inspections": todayInspections,
		"spare_low":         spareLow,
	})
}

// ==================== 报表统计 ====================

func (h *Handler) ReportSummary(c *gin.Context) {
	siteID := c.DefaultQuery("site_id", "1")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 || days > 365 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days)

	var totalAssets, inRack, maint int64
	h.DB.Model(&model.Asset{}).Where("site_id = ?", siteID).Count(&totalAssets)
	h.DB.Model(&model.Asset{}).Where("site_id = ? AND status = 2", siteID).Count(&inRack)
	h.DB.Model(&model.Asset{}).Where("site_id = ? AND status = 3", siteID).Count(&maint)

	var totalCabinets, usedCabinets int64
	h.DB.Model(&model.Cabinet{}).Where("site_id = ?", siteID).Count(&totalCabinets)
	if totalCabinets > 0 {
		h.DB.Raw(`SELECT COUNT(DISTINCT cabinet_id) FROM t_asset WHERE cabinet_id IN (SELECT id FROM t_cabinet WHERE site_id = ?)`, siteID).Scan(&usedCabinets)
	}

	var openOrders, closedOrders int64
	h.DB.Model(&model.WorkOrder{}).Where("site_id = ? AND status IN (1,2)", siteID).Count(&openOrders)
	h.DB.Model(&model.WorkOrder{}).Where("site_id = ? AND status >= 4", siteID).Count(&closedOrders)

	var totalChanges int64
	h.DB.Model(&model.ChangeRequest{}).Where("site_id = ?", siteID).Count(&totalChanges)

	var totalAlerts, resolvedAlerts int64
	h.DB.Model(&model.Alert{}).Where("site_id = ?", siteID).Count(&totalAlerts)
	h.DB.Model(&model.Alert{}).Where("site_id = ? AND status = 2", siteID).Count(&resolvedAlerts)

	// ---- 期间统计（近 days 天）----
	var periodOrders, periodChanges, periodAlerts, periodInspections, periodAbnormal int64
	h.DB.Model(&model.WorkOrder{}).Where("site_id = ? AND created_at >= ?", siteID, since).Count(&periodOrders)
	h.DB.Model(&model.ChangeRequest{}).Where("site_id = ? AND created_at >= ?", siteID, since).Count(&periodChanges)
	h.DB.Model(&model.Alert{}).Where("site_id = ? AND created_at >= ?", siteID, since).Count(&periodAlerts)
	h.DB.Model(&model.InspectionRecord{}).Where("site_id = ? AND inspected_at >= ?", siteID, since).Count(&periodInspections)
	h.DB.Model(&model.InspectionRecord{}).Where("site_id = ? AND inspected_at >= ? AND abnormal <> ''", siteID, since).Count(&periodAbnormal)

	// 期间已结工单 MTTR（创建→完成平均小时数）
	var mttrHours float64
	h.DB.Raw(`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (finished_at - created_at))/3600.0),0)
		FROM t_work_order WHERE site_id = ? AND status >= 4 AND finished_at IS NOT NULL AND created_at >= ?`, siteID, since).Scan(&mttrHours)

	// 工单类型分布（期间）
	type typeRow struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var woTypes []typeRow
	h.DB.Raw(`SELECT type, COUNT(*) as count FROM t_work_order WHERE site_id = ? AND created_at >= ? GROUP BY type ORDER BY count DESC`, siteID, since).Scan(&woTypes)

	// 告警严重度分布（期间）
	var alertSev []typeRow
	h.DB.Raw(`SELECT severity::text as type, COUNT(*) as count FROM t_alert WHERE site_id = ? AND created_at >= ? GROUP BY severity ORDER BY severity DESC`, siteID, since).Scan(&alertSev)

	// 每日告警/工单数（期间趋势，供报表折线图）
	type dayRow struct {
		Day    string `json:"day"`
		Alerts int64  `json:"alerts"`
		Orders int64  `json:"orders"`
	}
	var dailyTrend []dayRow
	h.DB.Raw(`SELECT d.day,
			COALESCE(a.cnt,0) as alerts, COALESCE(w.cnt,0) as orders
		FROM (SELECT TO_CHAR(generate_series(?::date, CURRENT_DATE, '1 day'),'YYYY-MM-DD') as day) d
		LEFT JOIN (SELECT TO_CHAR(created_at,'YYYY-MM-DD') as day, COUNT(*) as cnt FROM t_alert WHERE site_id = ? AND created_at >= ? GROUP BY 1) a ON a.day = d.day
		LEFT JOIN (SELECT TO_CHAR(created_at,'YYYY-MM-DD') as day, COUNT(*) as cnt FROM t_work_order WHERE site_id = ? AND created_at >= ? GROUP BY 1) w ON w.day = d.day
		ORDER BY d.day`, since, siteID, since, siteID, since).Scan(&dailyTrend)

	// 期间能耗汇总（PUE 均值/能耗/电费）
	var avgPUE, avgTotal float64
	h.DB.Raw(`SELECT COALESCE(AVG(pue),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ?`, siteID, since).Scan(&avgPUE)
	h.DB.Raw(`SELECT COALESCE(AVG(total_power),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ?`, siteID, since).Scan(&avgTotal)
	periodKWh := avgTotal * float64(days) * 24
	priceKWh := h.settingFloat("energy_price_kwh", 0.8)
	co2Factor := h.settingFloat("co2_factor", 0.5703)

	// PUE 趋势
	var pueTrend []model.PUERecord
	h.DB.Where("site_id = ?", siteID).Order("recorded_at").Limit(168).Find(&pueTrend)

	// 资产类型分布
	type cat struct {
		Category string `json:"category"`
		Count    int64  `json:"count"`
	}
	var cats []cat
	h.DB.Raw(`SELECT category, COUNT(*) as count FROM t_asset WHERE site_id = ? GROUP BY category ORDER BY count DESC`, siteID).Scan(&cats)

	response.Success(c, gin.H{
		"days":               days,
		"total_assets":       totalAssets,
		"in_rack":            inRack,
		"in_maint":           maint,
		"total_cabinets":     totalCabinets,
		"used_cabinets":      usedCabinets,
		"open_orders":        openOrders,
		"closed_orders":      closedOrders,
		"total_changes":      totalChanges,
		"total_alerts":       totalAlerts,
		"resolved_alerts":    resolvedAlerts,
		"period_orders":      periodOrders,
		"period_changes":     periodChanges,
		"period_alerts":      periodAlerts,
		"period_inspections": periodInspections,
		"period_abnormal":    periodAbnormal,
		"mttr_hours":         round2(mttrHours),
		"wo_types":           woTypes,
		"alert_severity":     alertSev,
		"daily_trend":        dailyTrend,
		"period_energy_kwh":  round2(periodKWh),
		"period_cost_yuan":   round2(periodKWh * priceKWh),
		"period_co2_kg":      round2(periodKWh * co2Factor),
		"period_avg_pue":     round3(avgPUE),
		"pue_trend":          pueTrend,
		"asset_cats":         cats,
	})
}

// ==================== 开放 API 文档 ====================

func (h *Handler) APIDocs(c *gin.Context) {
	endpoints := []gin.H{
		{"path": "/api/v1/health", "method": "GET", "desc": "健康检查与阶段标识"},
		{"path": "/api/v1/captcha", "method": "GET", "desc": "获取验证码"},
		{"path": "/api/v1/login", "method": "POST", "desc": "用户登录"},
		{"path": "/api/v1/sites", "method": "GET/POST", "desc": "站点管理"},
		{"path": "/api/v1/rooms", "method": "GET/POST", "desc": "房间管理"},
		{"path": "/api/v1/cabinets", "method": "GET/POST", "desc": "机柜管理"},
		{"path": "/api/v1/assets", "method": "GET/POST/PUT", "desc": "资产台账"},
		{"path": "/api/v1/maintenances", "method": "GET/POST/PUT", "desc": "维保管理"},
		{"path": "/api/v1/work-orders", "method": "GET/POST", "desc": "运维工单"},
		{"path": "/api/v1/inspection-plans", "method": "GET/POST", "desc": "巡检计划"},
		{"path": "/api/v1/changes", "method": "GET/POST", "desc": "变更管理"},
		{"path": "/api/v1/power-chains", "method": "GET/POST", "desc": "供电链路"},
		{"path": "/api/v1/notifications", "method": "GET", "desc": "通知中心"},
		{"path": "/api/v1/monitoring-points", "method": "GET/POST", "desc": "监控测点"},
		{"path": "/api/v1/push-reading", "method": "POST", "desc": "网关推送读数"},
		{"path": "/api/v1/alerts", "method": "GET/POST", "desc": "告警管理"},
		{"path": "/api/v1/alert-rules", "method": "GET/POST", "desc": "告警规则"},
		{"path": "/api/v1/network-devices", "method": "GET/POST", "desc": "网络设备"},
		{"path": "/api/v1/network-topology", "method": "GET", "desc": "网络拓扑"},
		{"path": "/api/v1/ip-pools", "method": "GET/POST", "desc": "IP地址池"},
		{"path": "/api/v1/vlans", "method": "GET/POST", "desc": "VLAN管理"},
		{"path": "/api/v1/config-backups", "method": "GET/POST", "desc": "配置备份"},
		{"path": "/api/v1/pue", "method": "GET/POST", "desc": "PUE能效"},
		{"path": "/api/v1/big-screen", "method": "GET", "desc": "大屏聚合数据"},
		{"path": "/api/v1/report", "method": "GET", "desc": "报表统计"},
		{"path": "/api/v1/api-docs", "method": "GET", "desc": "开放API文档（本页）"},
		{"path": "/api/v1/dashboard", "method": "GET", "desc": "仪表盘"},
		{"path": "/api/v1/users", "method": "GET/POST", "desc": "用户管理（全局管理员）"},
		{"path": "/api/v1/audits", "method": "GET", "desc": "审计日志"},
	}
	response.Success(c, gin.H{
		"title":     "DCIM 开放 API",
		"version":   "v1",
		"phase":     "P5",
		"endpoints": endpoints,
		"note":      "登录后使用 Bearer Token 认证；写操作需站点管理员以上权限",
	})
}

// ==================== 仪表盘 P5 统计 ====================

// DashboardP5Stats 仪表盘 P5 扩展统计
func (h *Handler) DashboardP5Stats(c *gin.Context) {
	var latestPUE model.PUERecord
	h.DB.Order("id DESC").First(&latestPUE)

	var reportAssets int64
	h.DB.Model(&model.Asset{}).Count(&reportAssets)

	var reportAlerts int64
	h.DB.Model(&model.Alert{}).Where("status = 0").Count(&reportAlerts)

	response.Success(c, gin.H{
		"pue":          fmt.Sprintf("%.2f", latestPUE.PUE),
		"p5":           true,
		"total_assets": reportAssets,
		"active_alerts": reportAlerts,
	})
}