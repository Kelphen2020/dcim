package api

import (
	"math"
	"strconv"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V5 能耗总览 ====================

// settingFloat 读取系统配置数值（缺省 fallback）
func (h *Handler) settingFloat(key string, fallback float64) float64 {
	var s model.SystemSetting
	if err := h.DB.First(&s, "key = ?", key).Error; err == nil {
		if v, err := strconv.ParseFloat(s.Value, 64); err == nil && v > 0 {
			return v
		}
	}
	return fallback
}

// EnergyOverview 能耗总览：KPI + 房间分布 + 机柜排行 + PUE 趋势 + 节能预测 + 环比
func (h *Handler) EnergyOverview(c *gin.Context) {
	siteID, _ := strconv.ParseUint(c.DefaultQuery("site_id", "1"), 10, 64)
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days <= 0 || days > 90 {
		days = 7
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	prevCutoff := time.Now().AddDate(0, 0, -2*days)

	// 可配置系数：电价(元/kWh)、电网排放因子(kgCO2/kWh)
	priceKWh := h.settingFloat("energy_price_kwh", 0.8)
	co2Factor := h.settingFloat("co2_factor", 0.5703)

	// ---- KPI：当前 IT 总功率（在架+在库资产）与额定容量 ----
	var itPower, ratedPower float64
	h.DB.Raw(`SELECT COALESCE(SUM(power_w),0)/1000.0 FROM t_asset WHERE site_id = ? AND power_w > 0`, siteID).Scan(&itPower)
	h.DB.Raw(`SELECT COALESCE(SUM(rated_power_kw),0) FROM t_cabinet WHERE site_id = ?`, siteID).Scan(&ratedPower)

	// ---- 最新 PUE 与期间均值 ----
	type latestRow struct {
		PUE         float64
		TotalPower  float64
		ITPower     float64
	}
	var lr latestRow
	h.DB.Raw(`SELECT COALESCE(pue,0) as pue, COALESCE(total_power,0) as total_power, COALESCE(it_power,0) as it_power FROM t_pue_record WHERE site_id = ? ORDER BY recorded_at DESC LIMIT 1`, siteID).Scan(&lr)
	latestPUE, latestTotal, latestIT := lr.PUE, lr.TotalPower, lr.ITPower
	var avgPUE float64
	h.DB.Raw(`SELECT COALESCE(AVG(pue),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ?`, siteID, cutoff).Scan(&avgPUE)

	// 期间能耗估算：平均总功率 kW × 时长 = kWh
	var periodAvgTotal float64
	h.DB.Raw(`SELECT COALESCE(AVG(total_power),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ?`, siteID, cutoff).Scan(&periodAvgTotal)
	energyKWh := periodAvgTotal * float64(days) * 24

	// 上一周期（环比基准）
	var prevAvgTotal, prevAvgPUE float64
	h.DB.Raw(`SELECT COALESCE(AVG(total_power),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ? AND recorded_at < ?`, siteID, prevCutoff, cutoff).Scan(&prevAvgTotal)
	h.DB.Raw(`SELECT COALESCE(AVG(pue),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ? AND recorded_at < ?`, siteID, prevCutoff, cutoff).Scan(&prevAvgPUE)
	prevEnergyKWh := prevAvgTotal * float64(days) * 24
	energyDeltaPct, pueDeltaPct := 0.0, 0.0
	if prevEnergyKWh > 0 {
		energyDeltaPct = (energyKWh - prevEnergyKWh) / prevEnergyKWh * 100
	}
	if prevAvgPUE > 0 {
		pueDeltaPct = (avgPUE - prevAvgPUE) / prevAvgPUE * 100
	}

	// 碳排放估算（可配置电网排放因子）+ 电费估算（可配置电价）
	co2Kg := energyKWh * co2Factor
	costYuan := energyKWh * priceKWh

	// 容量利用率：IT 功率 / 额定容量
	capUsage := 0.0
	if ratedPower > 0 {
		capUsage = itPower / ratedPower * 100
	}

	// ---- 房间分布 ----
	type roomRow struct {
		RoomID    uint    `json:"room_id"`
		RoomName  string  `json:"room_name"`
		Cabinets  int     `json:"cabinets"`
		ITPowerKW float64 `json:"it_power_kw"`
		RatedKW   float64 `json:"rated_kw"`
	}
	var rooms []roomRow
	h.DB.Raw(`SELECT r.id as room_id, r.name as room_name,
			COUNT(DISTINCT c.id) as cabinets,
			COALESCE(SUM(cab_kw.kw),0) as it_power_kw,
			COALESCE(SUM(c.rated_power_kw),0) as rated_kw
		FROM t_room r
		LEFT JOIN t_cabinet c ON c.room_id = r.id AND c.site_id = r.site_id
		LEFT JOIN (SELECT cabinet_id, ROUND(SUM(power_w)/1000.0,2) as kw FROM t_asset WHERE cabinet_id > 0 AND power_w > 0 GROUP BY cabinet_id) cab_kw ON cab_kw.cabinet_id = c.id
		WHERE r.site_id = ? GROUP BY r.id, r.name ORDER BY r.name`, siteID).Scan(&rooms)

	// ---- 机柜功耗排行 TOP10 ----
	type cabRow struct {
		ID        uint    `json:"id"`
		Name      string  `json:"name"`
		RoomName  string  `json:"room_name"`
		PowerKW   float64 `json:"power_kw"`
		RatedKW   float64 `json:"rated_kw"`
		LoadPct   float64 `json:"load_pct"`
	}
	var topCabs []cabRow
	h.DB.Raw(`SELECT c.id, c.code as name, COALESCE(r.name,'') as room_name,
			COALESCE(pa.kw,0) as power_kw, COALESCE(c.rated_power_kw,0) as rated_kw,
			CASE WHEN COALESCE(c.rated_power_kw,0) > 0 THEN ROUND(COALESCE(pa.kw,0)/c.rated_power_kw*100,1) ELSE 0 END as load_pct
		FROM t_cabinet c LEFT JOIN t_room r ON c.room_id = r.id
		LEFT JOIN (SELECT cabinet_id, ROUND(SUM(power_w)/1000.0,2) as kw FROM t_asset WHERE cabinet_id > 0 AND power_w > 0 GROUP BY cabinet_id) pa ON pa.cabinet_id = c.id
		WHERE c.site_id = ? ORDER BY power_kw DESC LIMIT 10`, siteID).Scan(&topCabs)

	// ---- PUE 趋势（按天聚合）----
	type puePoint struct {
		Day        string  `json:"day"`
		AvgPUE     float64 `json:"avg_pue"`
		MaxPUE     float64 `json:"max_pue"`
		MinPUE     float64 `json:"min_pue"`
		AvgITPower float64 `json:"avg_it_power"`
		AvgTotal   float64 `json:"avg_total"`
	}
	var pueTrend []puePoint
	h.DB.Raw(`SELECT TO_CHAR(recorded_at,'YYYY-MM-DD') as day,
			ROUND(AVG(pue)::numeric,3) as avg_pue, ROUND(MAX(pue)::numeric,3) as max_pue, ROUND(MIN(pue)::numeric,3) as min_pue,
			ROUND(AVG(it_power)::numeric,2) as avg_it_power, ROUND(AVG(total_power)::numeric,2) as avg_total
		FROM t_pue_record WHERE site_id = ? AND recorded_at >= ? GROUP BY day ORDER BY day`, siteID, cutoff).Scan(&pueTrend)

	// ---- PUE 历史曲线（原始点，最近 72 点）----
	type pueRaw struct {
		At  string  `json:"at"`
		PUE float64 `json:"pue"`
	}
	var pueRawPoints []pueRaw
	h.DB.Raw(`SELECT TO_CHAR(recorded_at,'MM-DD HH24:MI') as at, ROUND(pue::numeric,3) as pue
		FROM t_pue_record WHERE site_id = ? ORDER BY recorded_at DESC LIMIT 72`, siteID).Scan(&pueRawPoints)
	for i, j := 0, len(pueRawPoints)-1; i < j; i, j = i+1, j-1 {
		pueRawPoints[i], pueRawPoints[j] = pueRawPoints[j], pueRawPoints[i]
	}

	// ---- 节能预测：PUE 降 0.1 节省多少 ----
	savingKWh, savingCo2 := 0.0, 0.0
	if latestIT > 0 {
		savingKWh = latestIT * 0.1 * float64(days) * 24
		savingCo2 = savingKWh * co2Factor
	}

	// ---- 负载率分布（机柜功率利用率分桶）----
	type bucketRow struct {
		Bucket string `json:"bucket"`
		Count  int    `json:"count"`
	}
	var buckets []bucketRow
	h.DB.Raw(`SELECT CASE
			WHEN rated_power_kw <= 0 OR COALESCE(pa.kw,0) = 0 THEN '空闲'
			WHEN COALESCE(pa.kw,0)/rated_power_kw < 0.4 THEN '轻载 <40%'
			WHEN COALESCE(pa.kw,0)/rated_power_kw < 0.7 THEN '中载 40-70%'
			ELSE '重载 >70%'
		END as bucket, COUNT(*) as count
		FROM t_cabinet c
		LEFT JOIN (SELECT cabinet_id, SUM(power_w)/1000.0 as kw FROM t_asset WHERE cabinet_id > 0 AND power_w > 0 GROUP BY cabinet_id) pa ON pa.cabinet_id = c.id
		WHERE c.site_id = ? GROUP BY bucket ORDER BY count DESC`, siteID).Scan(&buckets)

	response.Success(c, gin.H{
		"kpi": gin.H{
			"it_power_kw":      round2(itPower),
			"rated_power_kw":   round2(ratedPower),
			"latest_pue":       round3(latestPUE),
			"latest_total_kw":  round2(latestTotal),
			"latest_it_kw":     round2(latestIT),
			"avg_pue":          round3(avgPUE),
			"energy_kwh":       round2(energyKWh),
			"prev_energy_kwh":  round2(prevEnergyKWh),
			"energy_delta_pct": round2(energyDeltaPct),
			"pue_delta_pct":    round2(pueDeltaPct),
			"co2_kg":           round2(co2Kg),
			"cost_yuan":        round2(costYuan),
			"price_kwh":        priceKWh,
			"co2_factor":       co2Factor,
			"cap_usage_pct":    round2(capUsage),
			"days":             days,
		},
		"rooms":       rooms,
		"top_cabinets": topCabs,
		"pue_trend":   pueTrend,
		"pue_raw":     pueRawPoints,
		"buckets":     buckets,
		"prediction": gin.H{
			"target_pue":     round3(latestPUE - 0.1),
			"saving_kwh":     round2(savingKWh),
			"saving_co2_kg":  round2(savingCo2),
			"period_hours":   days * 24,
		},
	})
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
