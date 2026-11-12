package api

// V65 B1-2 Dashboard「风险与洞察」聚合区（断点 G7：新洞察未回流总览）
// 闭环定位：五指标全部直查既有表、复用既有口径（v50 健康度 / v49 PUE合规 / v56 容量预测 /
// v64 告警质量 / v63 SLA），不新算法、不新表、不新采集通道；前端 Dashboard 五卡聚合区卡卡穿透。

import (
	"math"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// DashboardInsights GET /dashboard/insights
func (h *Handler) DashboardInsights(c *gin.Context) {
	scope := middleware.GetSiteScope(c)
	inScope := func(siteID uint) bool {
		if scope == nil {
			return true
		}
		for _, s := range scope {
			if s == siteID {
				return true
			}
		}
		return false
	}
	round1 := func(v float64) float64 { return math.Round(v*10) / 10 }

	// ① 设备健康度（口径同 v50 GetAssetHealthSummary）
	health := gin.H{"total": 0, "avg_score": 0, "grades": map[string]int{"healthy": 0, "attention": 0, "sub_healthy": 0, "risk": 0},
		"open_work_orders": 0, "top_risks": []gin.H{}, "detail_link": "/assets?health_grade=risk"}
	{
		var rows []model.AssetHealth
		q := h.DB.Model(&model.AssetHealth{})
		if scope != nil {
			q = q.Where("site_id IN ?", scope)
		}
		q.Find(&rows)
		grades := map[string]int{"healthy": 0, "attention": 0, "sub_healthy": 0, "risk": 0}
		sum, openWO := 0, 0
		for _, r := range rows {
			grades[r.Grade]++
			sum += r.Score
			if r.OpenWorkOrderID > 0 {
				openWO++
			}
		}
		avg := 0
		if len(rows) > 0 {
			avg = int(math.Round(float64(sum) / float64(len(rows))))
		}
		type riskRow struct {
			AssetID uint   `json:"asset_id"`
			Name    string `json:"name"`
			Score   int    `json:"score"`
			Grade   string `json:"grade"`
		}
		var risks []riskRow
		rq := h.DB.Raw(`SELECT ah.asset_id, a.name, ah.score, ah.grade
			FROM t_asset_health ah JOIN t_asset a ON a.id = ah.asset_id
			WHERE ah.grade IN ('sub_healthy','risk') ORDER BY ah.score ASC LIMIT 5`)
		rq.Scan(&risks)
		top := make([]gin.H, 0, len(risks))
		for _, r := range risks {
			top = append(top, gin.H{"asset_id": r.AssetID, "name": r.Name, "score": r.Score, "grade": r.Grade})
		}
		health["total"] = len(rows)
		health["avg_score"] = avg
		health["grades"] = grades
		health["open_work_orders"] = openWO
		health["top_risks"] = top
	}

	// ② PUE 合规（口径同 v49 EnergyCompliance：最新PUE(缺省30天均值) vs 政策红线）
	pue := gin.H{"sites": 0, "ok": 0, "edge": 0, "over": 0, "unknown": 0, "nodata": 0,
		"items": []gin.H{}, "detail_link": "/energy?tab=compliance"}
	{
		var sites []model.Site
		h.DB.Find(&sites)
		cutoff := time.Now().AddDate(0, 0, -30)
		items := make([]gin.H, 0)
		cnt := map[string]int{"ok": 0, "edge": 0, "over": 0, "unknown": 0, "nodata": 0}
		n := 0
		for i := range sites {
			s := &sites[i]
			if !inScope(s.ID) {
				continue
			}
			n++
			var latest float64
			h.DB.Raw(`SELECT COALESCE(pue,0) FROM t_pue_record WHERE site_id = ? ORDER BY recorded_at DESC LIMIT 1`, s.ID).Scan(&latest)
			var avg30 float64
			h.DB.Raw(`SELECT COALESCE(AVG(pue),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ?`, s.ID, cutoff).Scan(&avg30)
			p := latest
			if p <= 0 {
				p = avg30
			}
			status := "nodata"
			limit, margin := 0.0, 0.0
			policy, _ := h.matchSitePolicy(s)
			if policy == nil {
				status = "unknown"
			} else if p <= 0 {
				status = "nodata"
			} else {
				limit = policy.NewLargeLimit
				status = "ok"
				if limit > 0 {
					margin = roundV49(limit - p)
					if p > limit {
						status = "over"
					} else if margin < 0.05 {
						status = "edge"
					}
				}
			}
			cnt[status]++
			items = append(items, gin.H{"site_id": s.ID, "site_name": s.Name, "status": status,
				"latest_pue": roundV49(latest), "avg_pue_30d": roundV49(avg30),
				"limit": roundV49(limit), "margin": margin})
		}
		pue["sites"] = n
		pue["ok"] = cnt["ok"]
		pue["edge"] = cnt["edge"]
		pue["over"] = cnt["over"]
		pue["unknown"] = cnt["unknown"]
		pue["nodata"] = cnt["nodata"]
		pue["items"] = items
	}

	// ③ 容量预测（口径同 v56 ListCapacityForecast：grade 汇总 + 告急站点距饱和天数）
	capacity := gin.H{"total": 0, "ok": 0, "warn": 0, "risk": 0, "stable": 0,
		"items": []gin.H{}, "detail_link": "/capacity?tab=forecast"}
	{
		var recs []model.CapacityForecast
		q := h.DB.Model(&model.CapacityForecast{})
		if scope != nil {
			q = q.Where("site_id IN ?", scope)
		}
		q.Find(&recs)
		siteName := map[uint]string{}
		var ss []model.Site
		h.DB.Select("id", "name").Find(&ss)
		for _, s := range ss {
			siteName[s.ID] = s.Name
		}
		sum := map[string]int{"ok": 0, "warn": 0, "risk": 0, "stable": 0}
		items := make([]gin.H, 0)
		for _, r := range recs {
			sum[r.Grade]++
			days := -1
			fullAt := r.UFullAt
			if fullAt == "" || (r.PowerFullAt != "" && r.PowerFullAt < fullAt) {
				fullAt = r.PowerFullAt
			}
			if fullAt != "" {
				if t, err := time.Parse("2006-01-02", fullAt); err == nil {
					days = int(time.Until(t).Hours() / 24)
				}
			}
			if r.Grade == "risk" || r.Grade == "warn" {
				items = append(items, gin.H{"site_id": r.SiteID, "site_name": siteName[r.SiteID],
					"grade": r.Grade, "days_to_full": days, "low_confidence": r.LowConfidence})
			}
		}
		capacity["total"] = len(recs)
		capacity["ok"] = sum["ok"]
		capacity["warn"] = sum["warn"]
		capacity["risk"] = sum["risk"]
		capacity["stable"] = sum["stable"]
		capacity["items"] = items
	}

	// ④ 告警质量（口径同 v64：近7天，疑似误报=已结束未确认且存续<10min；噪声率=(误报+抑制)/总数）
	alertQ := gin.H{"days": 7, "total": 0, "suspected_false": 0, "suppressed": 0,
		"noise_rate": 0.0, "grade": "ok", "detail_link": "/alert-rules?tab=quality"}
	{
		since := time.Now().AddDate(0, 0, -7)
		q := h.DB.Model(&model.Alert{}).Where("created_at >= ?", since)
		if scope != nil {
			q = q.Where("site_id IN ?", scope)
		}
		var rows []model.Alert
		q.Find(&rows)
		suspected, suppressed := 0, 0
		for i := range rows {
			a := &rows[i]
			if v64IsSuspectedFalse(a) {
				suspected++
			}
			if a.Suppressed {
				suppressed++
			}
		}
		noise, grade := 0.0, "ok"
		if len(rows) > 0 {
			noise = round1(float64(suspected+suppressed) * 100 / float64(len(rows)))
			if noise >= 40 {
				grade = "bad"
			} else if noise >= 20 {
				grade = "warn"
			}
		}
		alertQ["total"] = len(rows)
		alertQ["suspected_false"] = suspected
		alertQ["suppressed"] = suppressed
		alertQ["noise_rate"] = noise
		alertQ["grade"] = grade
	}

	// ⑤ SLA（口径同 v63：近30天工单违约标记计数）
	sla := gin.H{"days": 30, "total": 0, "finished": 0, "respond_breach": 0, "resolve_breach": 0,
		"breached": 0, "breach_rate": 0.0, "detail_link": "/work-orders?tab=sla"}
	{
		since := time.Now().AddDate(0, 0, -30)
		q := h.DB.Model(&model.WorkOrder{}).Where("created_at >= ?", since)
		if scope != nil {
			q = q.Where("site_id IN ?", scope)
		}
		var rows []model.WorkOrder
		q.Find(&rows)
		respB, resB, breached, finished := 0, 0, 0, 0
		for i := range rows {
			w := &rows[i]
			if w.SlaRespondBreach {
				respB++
			}
			if w.SlaResolveBreach {
				resB++
			}
			if w.SlaRespondBreach || w.SlaResolveBreach {
				breached++
			}
			if w.Status == 3 || w.Status == 4 {
				finished++
			}
		}
		rate := 0.0
		if len(rows) > 0 {
			rate = round1(float64(breached) * 100 / float64(len(rows)))
		}
		sla["total"] = len(rows)
		sla["finished"] = finished
		sla["respond_breach"] = respB
		sla["resolve_breach"] = resB
		sla["breached"] = breached
		sla["breach_rate"] = rate
	}

	response.Success(c, gin.H{
		"health": health, "pue": pue, "capacity": capacity,
		"alert_quality": alertQ, "sla": sla,
		"generated_at": time.Now(),
	})
}
