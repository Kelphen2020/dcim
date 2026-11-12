package api

// mobile3.go — M3 移动端管理摘要聚合
// GET /api/v1/mobile/summary  管理摘要驾驶舱一屏数据
//   (能耗/PUE、SLA 达标、维保覆盖、告警趋势、变更待审、盘点进行中)

import (
	"strconv"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// MobileSummary GET /mobile/summary?site_id=0&days=30
// 只读聚合，管理者视角一屏；复用既有统计口径（与 PC 报表页一致）
func (h *Handler) MobileSummary(c *gin.Context) {
	siteID := uint(0)
	if s := c.Query("site_id"); s != "" && s != "0" {
		v, _ := strconv.Atoi(s)
		siteID = uint(v)
	}
	days := 30
	if d, err := strconv.Atoi(c.DefaultQuery("days", "30")); err == nil && d > 0 && d <= 90 {
		days = d
	}
	now := time.Now()
	since := now.AddDate(0, 0, -days)

	// ---- 能耗/PUE ----
	var pueRow struct {
		PUE        float64   `json:"pue"`
		TotalPower float64   `json:"total_power"`
		ITPower    float64   `json:"it_power"`
		RecordedAt time.Time `json:"recorded_at"`
	}
	pq := h.DB.Model(&model.PUERecord{})
	if siteID > 0 {
		pq = pq.Where("site_id = ?", siteID)
	}
	pq.Order("recorded_at DESC").Limit(1).Scan(&pueRow)
	// 期间能耗近似：条数×平均总功率（与 dashboard energy_7d_kwh 同口径）
	var eRow struct {
		N   int64   `json:"n"`
		Avg float64 `json:"avg_kw"`
	}
	eq := h.DB.Model(&model.PUERecord{}).Where("recorded_at >= ?", since)
	if siteID > 0 {
		eq = eq.Where("site_id = ?", siteID)
	}
	eq.Select("COUNT(*) as n, COALESCE(AVG(total_power),0) as avg_kw").Scan(&eRow)
	energyKwh := float64(eRow.N) * eRow.Avg
	// PUE 日趋势（近 days 天，按日聚合）
	type pueDay struct {
		Day string  `json:"day"`
		PUE float64 `json:"pue"`
	}
	var pueTrend []pueDay
	ptq := h.DB.Model(&model.PUERecord{}).
		Select("to_char(date_trunc('day', recorded_at),'MM-DD') as day, ROUND(AVG(pue)::numeric,3) as pue").
		Where("recorded_at >= ?", since)
	if siteID > 0 {
		ptq = ptq.Where("site_id = ?", siteID)
	}
	ptq.Group("day").Order("day").Scan(&pueTrend)
	if pueTrend == nil {
		pueTrend = []pueDay{}
	}

	// ---- SLA（近 days 天工单）----
	var woRows []model.WorkOrder
	wq := h.DB.Model(&model.WorkOrder{}).Where("created_at >= ?", since)
	if siteID > 0 {
		wq = wq.Where("site_id = ?", siteID)
	}
	wq.Find(&woRows)
	woTotal := len(woRows)
	respBreach, resBreach := 0, 0
	for _, w := range woRows {
		if w.SlaRespondBreach {
			respBreach++
		}
		if w.SlaResolveBreach {
			resBreach++
		}
	}
	pct := func(ok, all int) float64 {
		if all == 0 {
			return 0
		}
		return float64(int(float64(ok)*1000/float64(all))) / 10
	}

	// ---- 维保覆盖（口径同 V17 coverage: kind=contract & status=1 & end_at>now，资产非报废）----
	aq := h.DB.Model(&model.Asset{}).Where("status <> 4")
	if siteID > 0 {
		aq = aq.Where("site_id = ?", siteID)
	}
	var assetTotal int64
	aq.Count(&assetTotal)
	var assets []model.Asset
	aq.Find(&assets)
	coveredMap := map[uint]bool{}
	{
		var ms []model.Maintenance
		h.DB.Where("kind = 'contract' AND end_at IS NOT NULL AND end_at > ?", now).Find(&ms)
		for _, m := range ms {
			coveredMap[m.AssetID] = true
		}
	}
	coveredN := 0
	for _, a := range assets {
		if coveredMap[a.ID] {
			coveredN++
		}
	}

	// ---- 告警趋势（近 days 天按日计数）----
	type alertDay struct {
		Day string `json:"day"`
		N   int64  `json:"n"`
	}
	var alertTrend []alertDay
	atq := h.DB.Model(&model.Alert{}).
		Select("to_char(date_trunc('day', triggered_at),'MM-DD') as day, COUNT(*) as n").
		Where("triggered_at >= ?", since)
	if siteID > 0 {
		atq = atq.Where("site_id = ?", siteID)
	}
	atq.Group("day").Order("day").Scan(&alertTrend)
	if alertTrend == nil {
		alertTrend = []alertDay{}
	}
	var alertActive, alertWeek int64
	aq2 := h.DB.Model(&model.Alert{}).Where("status < 3 AND suppressed = ?", false)
	if siteID > 0 {
		aq2 = aq2.Where("site_id = ?", siteID)
	}
	aq2.Count(&alertActive)
	aq3 := h.DB.Model(&model.Alert{}).Where("triggered_at >= ?", now.AddDate(0, 0, -7))
	if siteID > 0 {
		aq3 = aq3.Where("site_id = ?", siteID)
	}
	aq3.Count(&alertWeek)

	// ---- 变更待审批 / 盘点进行中 ----
	var chgPending int64
	cq := h.DB.Model(&model.ChangeRequest{}).Where("status = 2")
	if siteID > 0 {
		cq = cq.Where("site_id = ?", siteID)
	}
	cq.Count(&chgPending)
	var chgWindow int64 // 已批准待执行且窗口临近(7天内)
	cq2 := h.DB.Model(&model.ChangeRequest{}).
		Where("status = 3 AND window_start IS NOT NULL AND window_start <= ?", now.AddDate(0, 0, 7))
	if siteID > 0 {
		cq2 = cq2.Where("site_id = ?", siteID)
	}
	cq2.Count(&chgWindow)
	var stRunning int64
	h.DB.Model(&model.StocktakeTask{}).Where("status = 1").Count(&stRunning)

	// ---- 资产规模 ----
	var assetOnline, cabinetTotal int64
	ao := h.DB.Model(&model.Asset{}).Where("status = 1")
	if siteID > 0 {
		ao = ao.Where("site_id = ?", siteID)
	}
	ao.Count(&assetOnline)
	cb := h.DB.Model(&model.Cabinet{})
	if siteID > 0 {
		cb = cb.Where("site_id = ?", siteID)
	}
	cb.Count(&cabinetTotal)

	response.Success(c, gin.H{
		"days": days,
		"energy": gin.H{
			"latest_pue": pueRow.PUE, "total_power_kw": pueRow.TotalPower,
			"it_power_kw": pueRow.ITPower, "period_kwh": int64(energyKwh),
			"pue_trend": pueTrend, "recorded_at": pueRow.RecordedAt,
		},
		"sla": gin.H{
			"total": woTotal, "respond_breach": respBreach, "resolve_breach": resBreach,
			"respond_rate": pct(woTotal-respBreach, woTotal),
		},
		"maintenance": gin.H{
			"asset_total": assetTotal, "covered": coveredN,
			"coverage_pct": pct(coveredN, int(assetTotal)),
		},
		"alerts": gin.H{
			"active": alertActive, "week_new": alertWeek, "trend": alertTrend,
		},
		"changes":   gin.H{"pending_approval": chgPending, "window_7d": chgWindow},
		"stocktake": gin.H{"running": stRunning},
		"assets":    gin.H{"online": assetOnline, "cabinets": cabinetTotal},
		"generated_at": now.Format(time.RFC3339),
	})
}
