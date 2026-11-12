package api

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// V61 运维月报自动生成
// 闭环: 采集(复用全模块既有数据) → 评估(月度快照聚合+能效合规+SLA+健康度) → 使用(报表中心「运维月报」页签查看/CSV导出/发布归档) → 报警(自动生成后通知+Webhook ops_report 事件)
// 每月1日 01:00 后自动生成上月全站报告(系统设置 auto_monthly_report 可关)；管理员可手动生成任意月份/站点、force 重新生成

// ---------- 工具 ----------

func v61MonthRange(month string) (time.Time, time.Time, error) {
	t, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("月份格式须为 yyyy-MM")
	}
	return t, t.AddDate(0, 1, 0), nil
}

// v61f 兼容 PG 各数值类型的 map 扫描值转换
func v61f(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case int:
		return float64(n)
	case []byte:
		f, _ := strconv.ParseFloat(string(n), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	case nil:
		return 0
	}
	return 0
}

func v61i(v interface{}) int64 { return int64(v61f(v)) }

func v61s(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

// ---------- 报告内容构建 ----------

// buildOpsReportContent 聚合指定月份/站点的全模块指标快照（所有查询容错：单项失败不阻断整报）
func (h *Handler) buildOpsReportContent(siteID uint, start, end time.Time) gin.H {
	siteSQL, siteArgs := "", []interface{}{}
	if siteID > 0 {
		siteSQL, siteArgs = " AND site_id = ?", []interface{}{siteID}
	}
	raw := func(sql string, args ...interface{}) []map[string]interface{} {
		var rows []map[string]interface{}
		if err := h.DB.Raw(sql, args...).Scan(&rows).Error; err != nil {
			log.Printf("V61 月报查询失败: %v | %s", err, sql)
			return []map[string]interface{}{}
		}
		if rows == nil {
			rows = []map[string]interface{}{}
		}
		return rows
	}
	scalar := func(sql string, args ...interface{}) float64 {
		var f float64
		if err := h.DB.Raw(sql, args...).Scan(&f).Error; err != nil {
			log.Printf("V61 月报标量查询失败: %v | %s", err, sql)
			return 0
		}
		return f
	}
	cnt := func(q *gorm.DB) int64 {
		var n int64
		if siteID > 0 {
			q = q.Where("site_id = ?", siteID)
		}
		q.Count(&n)
		return n
	}
	pct := func(part, total int64) float64 {
		if total <= 0 {
			return 0
		}
		return round2(float64(part) * 100 / float64(total))
	}
	now := time.Now()
	periodEnd := end
	if periodEnd.After(now) {
		periodEnd = now
	}
	hours := periodEnd.Sub(start).Hours()
	if hours < 0 {
		hours = 0
	}

	// ---------- 告警 ----------
	alertCreated := cnt(h.DB.Model(&model.Alert{}).Where("created_at >= ? AND created_at < ?", start, end))
	alertResolved := cnt(h.DB.Model(&model.Alert{}).Where("status = 2 AND resolved_at >= ? AND resolved_at < ?", start, end))
	alertActive := cnt(h.DB.Model(&model.Alert{}).Where("status IN (0,1)"))
	alertSev := raw(`SELECT severity::int as sev, COUNT(*)::int as cnt FROM t_alert
		WHERE created_at >= ? AND created_at < ?`+siteSQL+` GROUP BY severity ORDER BY severity`,
		append([]interface{}{start, end}, siteArgs...)...)
	sevMap := map[string]int64{"sev1": 0, "sev2": 0, "sev3": 0}
	for _, r := range alertSev {
		sevMap[fmt.Sprintf("sev%d", v61i(r["sev"]))] = v61i(r["cnt"])
	}
	alertMttr := scalar(`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (resolved_at - created_at))/3600),0)::float8
		FROM t_alert WHERE status = 2 AND resolved_at >= ? AND resolved_at < ?`+siteSQL,
		append([]interface{}{start, end}, siteArgs...)...)
	topPoints := raw(`SELECT COALESCE(p.name, '测点#' || a.point_id) as pname, COALESCE(p.code,'') as pcode, COUNT(*)::int as cnt
		FROM t_alert a LEFT JOIN t_monitoring_point p ON p.id = a.point_id
		WHERE a.point_id IS NOT NULL AND a.created_at >= ? AND a.created_at < ?`+
		func() string {
			if siteID > 0 {
				return " AND a.site_id = ?"
			}
			return ""
		}()+`
		GROUP BY a.point_id, p.name, p.code ORDER BY cnt DESC LIMIT 5`,
		append([]interface{}{start, end}, siteArgs...)...)
	stormCnt := cnt(h.DB.Model(&model.AlertStorm{}).Where("created_at >= ? AND created_at < ?", start, end))
	stormSuppressed := int64(scalar(`SELECT COALESCE(SUM(suppressed_cnt),0)::float8 FROM t_alert_storm
		WHERE created_at >= ? AND created_at < ?`+siteSQL, append([]interface{}{start, end}, siteArgs...)...))
	escalated := cnt(h.DB.Model(&model.Alert{}).Where("escalated = true AND created_at >= ? AND created_at < ?", start, end))

	alerts := gin.H{
		"created": alertCreated, "resolved": alertResolved, "active_now": alertActive,
		"sev1": sevMap["sev1"], "sev2": sevMap["sev2"], "sev3": sevMap["sev3"],
		"mttr_hours": round2(alertMttr), "escalated": escalated,
		"storms": stormCnt, "storm_suppressed": stormSuppressed,
		"top_points": topPoints,
	}

	// ---------- 工单 ----------
	woCreated := cnt(h.DB.Model(&model.WorkOrder{}).Where("created_at >= ? AND created_at < ?", start, end))
	woFinished := cnt(h.DB.Model(&model.WorkOrder{}).Where("status >= 3 AND finished_at >= ? AND finished_at < ?", start, end))
	woOpen := cnt(h.DB.Model(&model.WorkOrder{}).Where("status IN (1,2)"))
	woByType := raw(`SELECT "type" as t, COUNT(*)::int as cnt FROM t_work_order
		WHERE created_at >= ? AND created_at < ?`+siteSQL+` GROUP BY "type" ORDER BY cnt DESC`,
		append([]interface{}{start, end}, siteArgs...)...)
	woByPri := raw(`SELECT priority::int as pri, COUNT(*)::int as cnt FROM t_work_order
		WHERE created_at >= ? AND created_at < ?`+siteSQL+` GROUP BY priority ORDER BY priority`,
		append([]interface{}{start, end}, siteArgs...)...)
	woRespBreach := cnt(h.DB.Model(&model.WorkOrder{}).Where("created_at >= ? AND created_at < ? AND sla_respond_breach = true", start, end))
	woResBreach := cnt(h.DB.Model(&model.WorkOrder{}).Where("created_at >= ? AND created_at < ? AND sla_resolve_breach = true", start, end))
	woMttr := scalar(`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (finished_at - created_at))/3600),0)::float8
		FROM t_work_order WHERE status >= 3 AND finished_at >= ? AND finished_at < ?`+siteSQL,
		append([]interface{}{start, end}, siteArgs...)...)
	topAssignees := raw(`SELECT COALESCE(u.username,'未指派') as uname, COUNT(*)::int as cnt,
			COALESCE(AVG(EXTRACT(EPOCH FROM (w.finished_at - w.created_at))/3600),0)::float8 as avg_hours,
			COALESCE(SUM(CASE WHEN w.sla_respond_breach OR w.sla_resolve_breach THEN 1 ELSE 0 END),0)::int as breach
		FROM t_work_order w LEFT JOIN t_user u ON u.id = w.assignee_id
		WHERE w.status >= 3 AND w.finished_at >= ? AND w.finished_at < ? AND w.assignee_id > 0`+
		func() string {
			if siteID > 0 {
				return " AND w.site_id = ?"
			}
			return ""
		}()+`
		GROUP BY u.username ORDER BY cnt DESC LIMIT 5`,
		append([]interface{}{start, end}, siteArgs...)...)
	woAutoFromAlert := cnt(h.DB.Model(&model.WorkOrder{}).Where("type = 'fault' AND created_at >= ? AND created_at < ?", start, end))

	workOrders := gin.H{
		"created": woCreated, "finished": woFinished, "open_now": woOpen,
		"by_type": woByType, "by_priority": woByPri,
		"respond_breach": woRespBreach, "resolve_breach": woResBreach,
		"respond_rate": pct(woCreated-woRespBreach, woCreated),
		"resolve_rate": pct(woCreated-woResBreach, woCreated),
		"mttr_hours": round2(woMttr), "top_assignees": topAssignees,
		"fault_orders": woAutoFromAlert,
	}

	// ---------- 巡检 ----------
	inspPlans := cnt(h.DB.Model(&model.InspectionPlan{}).Where("status = 1"))
	inspExec := cnt(h.DB.Model(&model.InspectionRecord{}).Where("status IN (1,2) AND inspected_at >= ? AND inspected_at < ?", start, end))
	inspAbnormal := cnt(h.DB.Model(&model.InspectionRecord{}).Where("status = 2 AND inspected_at >= ? AND inspected_at < ?", start, end))
	inspOverdue := cnt(h.DB.Model(&model.InspectionRecord{}).Where("status = 3 AND created_at >= ? AND created_at < ?", start, end))
	inspToWo := cnt(h.DB.Model(&model.InspectionRecord{}).Where("work_order_id > 0 AND inspected_at >= ? AND inspected_at < ?", start, end))
	inspScan := cnt(h.DB.Model(&model.InspectionRecord{}).Where("source = 'scan' AND inspected_at >= ? AND inspected_at < ?", start, end))
	inspSigned := cnt(h.DB.Model(&model.InspectionRecord{}).Where("report_code <> '' AND reviewed_at >= ? AND reviewed_at < ?", start, end))

	inspections := gin.H{
		"plans_active": inspPlans, "executions": inspExec, "abnormal": inspAbnormal,
		"overdue": inspOverdue, "to_work_orders": inspToWo, "scan_count": inspScan, "signed_reports": inspSigned,
		"exec_rate": pct(inspExec, inspExec+inspOverdue),
	}

	// ---------- 变更 ----------
	chgCreated := cnt(h.DB.Model(&model.ChangeRequest{}).Where("created_at >= ? AND created_at < ?", start, end))
	chgByStatus := raw(`SELECT status::int as st, COUNT(*)::int as cnt FROM t_change_request
		WHERE created_at >= ? AND created_at < ?`+siteSQL+` GROUP BY status ORDER BY status`,
		append([]interface{}{start, end}, siteArgs...)...)
	chgFinished := cnt(h.DB.Model(&model.ChangeRequest{}).Where("status = 6 AND finished_at >= ? AND finished_at < ?", start, end))
	chgRollback := cnt(h.DB.Model(&model.ChangeRequest{}).Where("status = 7 AND updated_at >= ? AND updated_at < ?", start, end))
	chgHighRisk := cnt(h.DB.Model(&model.ChangeRequest{}).Where("risk = 3 AND created_at >= ? AND created_at < ?", start, end))

	changes := gin.H{
		"created": chgCreated, "by_status": chgByStatus, "finished": chgFinished,
		"rollback": chgRollback, "high_risk": chgHighRisk,
	}

	// ---------- 能耗与 PUE 合规 ----------
	pueAvg := scalar(`SELECT COALESCE(AVG(pue),0)::float8 FROM t_pue_record WHERE recorded_at >= ? AND recorded_at < ?`+siteSQL,
		append([]interface{}{start, end}, siteArgs...)...)
	pueMax := scalar(`SELECT COALESCE(MAX(pue),0)::float8 FROM t_pue_record WHERE recorded_at >= ? AND recorded_at < ?`+siteSQL,
		append([]interface{}{start, end}, siteArgs...)...)
	pueMin := scalar(`SELECT COALESCE(MIN(pue),0)::float8 FROM t_pue_record WHERE recorded_at >= ? AND recorded_at < ? AND pue > 0`+siteSQL,
		append([]interface{}{start, end}, siteArgs...)...)
	powAvg := scalar(`SELECT COALESCE(AVG(total_power),0)::float8 FROM t_pue_record WHERE recorded_at >= ? AND recorded_at < ?`+siteSQL,
		append([]interface{}{start, end}, siteArgs...)...)
	energyKWh := powAvg * hours
	price := h.settingFloat("energy_price_kwh", 0.8)
	co2f := h.settingFloat("co2_factor", 0.5703)

	// 分站点合规行（月均 PUE vs 省份政策红线）
	var siteList []model.Site
	sq := h.DB.Model(&model.Site{})
	if siteID > 0 {
		sq = sq.Where("id = ?", siteID)
	}
	sq.Limit(20).Find(&siteList)
	compliance := make([]gin.H, 0, len(siteList))
	for i := range siteList {
		s := siteList[i]
		mAvg := scalar(`SELECT COALESCE(AVG(pue),0)::float8 FROM t_pue_record WHERE site_id = ? AND recorded_at >= ? AND recorded_at < ?`, s.ID, start, end)
		row := gin.H{"site_id": s.ID, "site_name": s.Name, "province": s.Province, "avg_pue": round2(mAvg), "status": "nodata", "limit": 0.0, "margin": 0.0}
		policy, _ := h.matchSitePolicy(&s)
		if policy == nil {
			row["status"] = "unknown"
		} else if mAvg > 0 && policy.NewLargeLimit > 0 {
			row["limit"] = policy.NewLargeLimit
			row["margin"] = round2(policy.NewLargeLimit - mAvg)
			if mAvg > policy.NewLargeLimit {
				row["status"] = "over"
			} else if policy.NewLargeLimit-mAvg < 0.05 {
				row["status"] = "edge"
			} else {
				row["status"] = "ok"
			}
		}
		compliance = append(compliance, row)
	}

	// V73/B3-3 合规告警事件（本期 over/edge 主动告警留档，含差别电价敞口）
	pueEventCount := cnt(h.DB.Model(&model.Alert{}).Where("detail LIKE ? AND created_at >= ? AND created_at < ?", "%pue_compliance%", start, end))
	pueOverCount := cnt(h.DB.Model(&model.Alert{}).Where("detail LIKE ? AND detail LIKE ? AND created_at >= ? AND created_at < ?", "%pue_compliance%", "%\"status\":\"over\"%", start, end))
	pueEdgeCount := cnt(h.DB.Model(&model.Alert{}).Where("detail LIKE ? AND detail LIKE ? AND created_at >= ? AND created_at < ?", "%pue_compliance%", "%\"status\":\"edge\"%", start, end))
	pueEventRows := raw(`SELECT id, severity::int as sev, message, detail, created_at FROM t_alert
		WHERE detail LIKE ? AND created_at >= ? AND created_at < ?`+siteSQL+` ORDER BY created_at DESC LIMIT 20`,
		append([]interface{}{"%pue_compliance%", start, end}, siteArgs...)...)
	pueEvents := make([]gin.H, 0, len(pueEventRows))
	surchargeSum := 0.0
	for _, r := range pueEventRows {
		var ed v73AlertDetail
		if json.Unmarshal([]byte(v61s(r["detail"])), &ed) != nil {
			continue
		}
		ts := v61s(r["created_at"])
		if len(ts) >= 16 {
			ts = ts[:16]
		}
		surchargeSum += ed.SurchargeAnnual
		pueEvents = append(pueEvents, gin.H{
			"id": v61i(r["id"]), "severity": v61i(r["sev"]), "message": v61s(r["message"]),
			"site_name": ed.SiteName, "status": ed.Status, "pue": round2(ed.PUE),
			"limit": round2(ed.Limit), "margin": round2(ed.Margin),
			"surcharge_annual": round2(ed.SurchargeAnnual), "surcharge_hit": ed.SurchargeHit,
			"created_at": ts,
		})
	}

	energy := gin.H{
		"avg_pue": round2(pueAvg), "max_pue": round2(pueMax), "min_pue": round2(pueMin),
		"avg_power_kw": round2(powAvg), "energy_kwh": round2(energyKWh),
		"cost_yuan": round2(energyKWh * price), "co2_kg": round2(energyKWh * co2f),
		"price_kwh": price, "compliance": compliance,
		"compliance_event_count": pueEventCount, "compliance_over_count": pueOverCount,
		"compliance_edge_count": pueEdgeCount, "compliance_surcharge_annual": round2(surchargeSum),
		"compliance_events": pueEvents,
	}

	// ---------- 资产 ----------
	astTotal := cnt(h.DB.Model(&model.Asset{}))
	astInRack := cnt(h.DB.Model(&model.Asset{}).Where("status = 1"))
	astMaint := cnt(h.DB.Model(&model.Asset{}).Where("status = 3"))
	astNew := cnt(h.DB.Model(&model.Asset{}).Where("created_at >= ? AND created_at < ?", start, end))
	astScrapped := cnt(h.DB.Model(&model.Asset{}).Where("status = 4"))
	retireDone := cnt(h.DB.Model(&model.AssetRetirement{}).Where("status = 4 AND disposal_at >= ? AND disposal_at < ?", start, end))
	retireApplied := cnt(h.DB.Model(&model.AssetRetirement{}).Where("created_at >= ? AND created_at < ?", start, end))
	retireAmount := scalar(`SELECT COALESCE(SUM(disposal_amount),0)::float8 FROM t_asset_retirement
		WHERE status = 4 AND disposal_at >= ? AND disposal_at < ?`+siteSQL, append([]interface{}{start, end}, siteArgs...)...)
	healthGrades := raw(`SELECT grade, COUNT(*)::int as cnt FROM t_asset_health WHERE 1=1`+siteSQL+` GROUP BY grade`, siteArgs...)
	astActive := cnt(h.DB.Model(&model.Asset{}).Where("status IN (1,3)"))
	coveredArgs := append([]interface{}{now}, siteArgs...)
	coveredSQL := `SELECT COUNT(*)::float8 FROM t_asset a WHERE a.status IN (1,3)` +
		func() string {
			if siteID > 0 {
				return " AND a.site_id = ?"
			}
			return ""
		}() +
		` AND EXISTS (SELECT 1 FROM t_maintenance m WHERE m.asset_id = a.id AND m.kind = 'contract' AND m.status = 1 AND m.end_at > ?)`
	// 注意参数顺序: site 在前, now 在后 → 重新组装
	if siteID > 0 {
		coveredArgs = []interface{}{siteID, now}
	} else {
		coveredArgs = []interface{}{now}
	}
	astCovered := int64(scalar(coveredSQL, coveredArgs...))
	expiring := int64(scalar(`SELECT COUNT(*)::float8 FROM t_maintenance m JOIN t_asset a ON a.id = m.asset_id
		WHERE m.kind = 'contract' AND m.status = 1 AND m.end_at > ? AND m.end_at < ?`+
		func() string {
			if siteID > 0 {
				return " AND a.site_id = ?"
			}
			return ""
		}(),
		append([]interface{}{now, now.AddDate(0, 0, 30)}, siteArgs...)...))

	assets := gin.H{
		"total": astTotal, "in_rack": astInRack, "in_maint": astMaint, "scrapped": astScrapped,
		"new_in_month": astNew, "retire_applied": retireApplied, "retire_done": retireDone,
		"retire_amount": round2(retireAmount), "health_grades": healthGrades,
		"maint_covered": astCovered, "maint_coverage_pct": pct(astCovered, astActive),
		"maint_expiring_30d": expiring,
	}

	// ---------- 盘点 ----------
	stCreated := cnt(h.DB.Model(&model.StocktakeTask{}).Where("created_at >= ? AND created_at < ?", start, end))
	stDone := cnt(h.DB.Model(&model.StocktakeTask{}).Where("status = 2 AND finished_at >= ? AND finished_at < ?", start, end))
	stDiff := int64(scalar(`SELECT COALESCE(SUM(diff_count),0)::float8 FROM t_stocktake_task
		WHERE status = 2 AND finished_at >= ? AND finished_at < ?`+siteSQL, append([]interface{}{start, end}, siteArgs...)...))
	stocktake := gin.H{"created": stCreated, "finished": stDone, "diff_total": stDiff}

	// ---------- 网络 ----------
	netDev := cnt(h.DB.Model(&model.NetworkDevice{}))
	netLinkTotal := cnt(h.DB.Model(&model.NetworkLink{}))
	netLinkUp := cnt(h.DB.Model(&model.NetworkLink{}).Where("status = 1"))
	network := gin.H{"devices": netDev, "links_total": netLinkTotal, "links_up": netLinkUp, "links_up_pct": pct(netLinkUp, netLinkTotal)}

	// ---------- 备件与采购 ----------
	spKinds := cnt(h.DB.Model(&model.SparePart{}))
	spLow := cnt(h.DB.Model(&model.SparePart{}).Where("safety_stock > 0 AND stock <= safety_stock"))
	spMoveSQL := `SELECT COALESCE(SUM(CASE WHEN mv.type = 'in' THEN mv.qty ELSE 0 END),0)::float8 as qin,
			COALESCE(SUM(CASE WHEN mv.type = 'out' THEN mv.qty ELSE 0 END),0)::float8 as qout,
			COUNT(*)::int as cnt
		FROM t_spare_movement mv JOIN t_spare_part sp ON sp.id = mv.spare_id
		WHERE mv.created_at >= ? AND mv.created_at < ?`
	if siteID > 0 {
		spMoveSQL += " AND sp.site_id = ?"
	}
	spMoves := raw(spMoveSQL+" GROUP BY 1=1 LIMIT 1", append([]interface{}{start, end}, siteArgs...)...)
	spIn, spOut, spMoveCnt := int64(0), int64(0), int64(0)
	if len(spMoves) > 0 {
		spIn, spOut, spMoveCnt = v61i(spMoves[0]["qin"]), v61i(spMoves[0]["qout"]), v61i(spMoves[0]["cnt"])
	}
	prSQL := `SELECT COUNT(*)::int as cnt, COALESCE(SUM(CASE WHEN pr.status = 1 THEN 1 ELSE 0 END),0)::int as pending
		FROM t_purchase_request pr JOIN t_spare_part sp ON sp.id = pr.spare_id
		WHERE pr.created_at >= ? AND pr.created_at < ?`
	if siteID > 0 {
		prSQL += " AND sp.site_id = ?"
	}
	prRows := raw(prSQL+" GROUP BY 1=1 LIMIT 1", append([]interface{}{start, end}, siteArgs...)...)
	prCreated, prPending := int64(0), int64(0)
	if len(prRows) > 0 {
		prCreated, prPending = v61i(prRows[0]["cnt"]), v61i(prRows[0]["pending"])
	}
	spares := gin.H{
		"kinds": spKinds, "low_stock": spLow, "move_in": spIn, "move_out": spOut, "moves": spMoveCnt,
		"purchase_created": prCreated, "purchase_pending": prPending,
	}

	// ---------- 应急预案与演练 ----------
	epActive := cnt(h.DB.Model(&model.EmergencyPlan{}).Where("status = 1"))
	drills := cnt(h.DB.Model(&model.DrillRecord{}).Where("created_at >= ? AND created_at < ?", start, end))
	drillResults := raw(`SELECT result, COUNT(*)::int as cnt FROM t_drill_record
		WHERE created_at >= ? AND created_at < ?`+siteSQL+` GROUP BY result`,
		append([]interface{}{start, end}, siteArgs...)...)
	drillToWo := cnt(h.DB.Model(&model.DrillRecord{}).Where("wo_id > 0 AND created_at >= ? AND created_at < ?", start, end))
	overduePlans := int64(0)
	{
		var plans []model.EmergencyPlan
		pq := h.DB.Where("status = 1 AND drill_cycle_days > 0")
		if siteID > 0 {
			pq = pq.Where("site_id IN (0, ?)", siteID)
		}
		pq.Find(&plans)
		for i := range plans {
			if st, _ := drillState(&plans[i]); st == "overdue" {
				overduePlans++
			}
		}
	}
	emergency := gin.H{
		"plans_active": epActive, "drills": drills, "drill_results": drillResults,
		"drill_to_work_orders": drillToWo, "overdue_plans_now": overduePlans,
	}

	// ---------- 监控可用性（当前快照） ----------
	availRows := raw(`SELECT COUNT(*)::int as total,
			COALESCE(SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END),0)::int as normal,
			COALESCE(SUM(CASE WHEN status = 2 THEN 1 ELSE 0 END),0)::int as alarm,
			COALESCE(SUM(CASE WHEN status IN (0,3) THEN 1 ELSE 0 END),0)::int as offline
		FROM t_monitoring_point WHERE 1=1`+siteSQL, siteArgs...)
	availability := gin.H{"points_total": int64(0), "normal": int64(0), "alarm": int64(0), "offline": int64(0), "online_pct": 0.0}
	if len(availRows) > 0 {
		a := availRows[0]
		total, normal, alarm, offline := v61i(a["total"]), v61i(a["normal"]), v61i(a["alarm"]), v61i(a["offline"])
		availability = gin.H{"points_total": total, "normal": normal, "alarm": alarm, "offline": offline,
			"online_pct": pct(total-offline, total)}
	}

	// ---------- 每日趋势 ----------
	seriesEnd := end.AddDate(0, 0, -1)
	if seriesEnd.After(now) {
		seriesEnd = now
	}
	trendArgs := []interface{}{start, seriesEnd, start, end}
	if siteID > 0 {
		trendArgs = append(trendArgs, siteID)
	}
	trendArgs = append(trendArgs, start, end)
	if siteID > 0 {
		trendArgs = append(trendArgs, siteID)
	}
	dailyTrend := raw(`SELECT d.day, COALESCE(a.cnt,0)::int as alerts, COALESCE(w.cnt,0)::int as orders
		FROM (SELECT TO_CHAR(generate_series(?::date, ?::date, '1 day'),'YYYY-MM-DD') as day) d
		LEFT JOIN (SELECT TO_CHAR(created_at,'YYYY-MM-DD') as day, COUNT(*) as cnt FROM t_alert
			WHERE created_at >= ? AND created_at < ?`+siteSQL+` GROUP BY 1) a ON a.day = d.day
		LEFT JOIN (SELECT TO_CHAR(created_at,'YYYY-MM-DD') as day, COUNT(*) as cnt FROM t_work_order
			WHERE created_at >= ? AND created_at < ?`+siteSQL+` GROUP BY 1) w ON w.day = d.day
		ORDER BY d.day`, trendArgs...)

	return gin.H{
		"period": gin.H{
			"start": start.Format("2006-01-02"), "end": end.AddDate(0, 0, -1).Format("2006-01-02"),
			"generated_at": now.Format("2006-01-02 15:04:05"), "hours": round2(hours),
		},
		"alerts": alerts, "work_orders": workOrders, "inspections": inspections,
		"changes": changes, "energy": energy, "assets": assets, "stocktake": stocktake,
		"network": network, "spares": spares, "emergency": emergency,
		"availability": availability, "daily_trend": dailyTrend,
	}
}

// buildOpsReportSummary 一句话摘要（列表/通知展示）
func buildOpsReportSummary(content gin.H) string {
	get := func(sec, key string) interface{} {
		if m, ok := content[sec].(gin.H); ok {
			return m[key]
		}
		return nil
	}
	gi := func(sec, key string) int64 { return v61i(get(sec, key)) }
	gf := func(sec, key string) float64 { return v61f(get(sec, key)) }
	return fmt.Sprintf("告警 %d 条(严重 %d) · 工单 %d 单(SLA响应达标 %v%%) · 巡检 %d 次(逾期 %d) · 能耗 %s kWh · 平均PUE %v · 备件低库存 %d",
		gi("alerts", "created"), gi("alerts", "sev3"),
		gi("work_orders", "created"), gf("work_orders", "respond_rate"),
		gi("inspections", "executions"), gi("inspections", "overdue"),
		strconv.FormatFloat(gf("energy", "energy_kwh"), 'f', 0, 64), gf("energy", "avg_pue"),
		gi("spares", "low_stock"))
}

// ---------- 生成（手动 + 自动共用） ----------

// generateOpsReport 生成月报；已存在时返回旧报告与 existed=true（force 重建时由调用方先删或传 force）
func (h *Handler) generateOpsReport(month string, siteID uint, byUser uint, auto, force bool) (*model.OpsReport, bool, error) {
	start, end, err := v61MonthRange(month)
	if err != nil {
		return nil, false, err
	}
	if start.After(time.Now()) {
		return nil, false, fmt.Errorf("不能生成未来月份的报告")
	}
	var existing model.OpsReport
	found := h.DB.Where("month = ? AND site_id = ?", month, siteID).First(&existing).Error == nil
	if found && !force {
		return &existing, true, nil
	}
	siteName := "全站"
	if siteID > 0 {
		var s model.Site
		if err := h.DB.First(&s, siteID).Error; err == nil {
			siteName = s.Name
		}
	}
	content := h.buildOpsReportContent(siteID, start, end)
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, false, err
	}
	title := fmt.Sprintf("%s 运维月报 · %s", siteName, month)
	rec := model.OpsReport{
		Code:        fmt.Sprintf("MR-%s-S%d", month, siteID),
		Title:       title,
		SiteID:      siteID,
		Month:       month,
		PeriodStart: start,
		PeriodEnd:   end,
		Summary:     buildOpsReportSummary(content),
		Content:     string(contentJSON),
		Status:      1,
		GeneratedBy: byUser,
		AutoGen:     auto,
	}
	if found {
		// force 重建：保留原状态与发布痕迹以外的生成信息
		rec.ID = existing.ID
		rec.Status = existing.Status
		rec.PublishedAt = existing.PublishedAt
		if err := h.DB.Model(&model.OpsReport{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
			"title": rec.Title, "period_start": rec.PeriodStart, "period_end": rec.PeriodEnd,
			"summary": rec.Summary, "content": rec.Content, "generated_by": byUser,
			"auto_gen": auto, "updated_at": time.Now(),
		}).Error; err != nil {
			return nil, false, err
		}
		return &rec, false, nil
	}
	if err := h.DB.Create(&rec).Error; err != nil {
		return nil, false, err
	}
	return &rec, false, nil
}

// notifyOpsReport 月报生成/发布通知 + Webhook
func (h *Handler) notifyOpsReport(rec *model.OpsReport, action string) {
	verb := "已生成"
	if action == "publish" {
		verb = "已发布"
	}
	msg := fmt.Sprintf("%s%s：%s", rec.Title, verb, rec.Summary)
	h.DB.Create(&model.Notification{
		UserID: 0, Kind: "ops_report",
		Title:   "运维月报" + verb,
		Content: msg,
		RefType: "ops_report", RefID: rec.ID,
	})
	h.DispatchWebhook(model.EvOpsReport, "运维月报"+verb, msg, map[string]interface{}{
		"report_id": rec.ID, "code": rec.Code, "month": rec.Month, "site_id": rec.SiteID,
	})
}

// CheckAutoMonthlyReport 每月1日自动生成上月全站月报（幂等：已存在则跳过）；main.go 每小时调用
func (h *Handler) CheckAutoMonthlyReport() {
	if h.healthSetting("auto_monthly_report", "1") == "0" {
		return
	}
	now := time.Now()
	if now.Day() != 1 || now.Hour() < 1 {
		return
	}
	month := now.AddDate(0, 0, -1).Format("2006-01")
	var exists int64
	h.DB.Model(&model.OpsReport{}).Where("month = ? AND site_id = 0", month).Count(&exists)
	if exists > 0 {
		return
	}
	rec, _, err := h.generateOpsReport(month, 0, 0, true, false)
	if err != nil {
		log.Printf("V61 自动月报生成失败: %v", err)
		return
	}
	h.notifyOpsReport(rec, "generate")
	log.Printf("V61 自动月报已生成: %s", rec.Code)
}

// ---------- Handlers ----------

// ListOpsReports GET /ops-reports?site_id=&year=&keyword=&page=&size=（不返回 content 大字段）
func (h *Handler) ListOpsReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", c.DefaultQuery("page_size", "20")))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	q := h.DB.Model(&model.OpsReport{})
	if sid := c.Query("site_id"); sid != "" {
		if n, err := strconv.ParseUint(sid, 10, 64); err == nil {
			q = q.Where("site_id = ?", n)
		}
	}
	if y := c.Query("year"); y != "" {
		q = q.Where("month LIKE ?", y+"-%")
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("title LIKE ? OR code LIKE ? OR summary LIKE ?", like, like, like)
	}
	var total int64
	q.Count(&total)
	var items []model.OpsReport
	q.Select("id, code, title, site_id, month, period_start, period_end, summary, status, generated_by, auto_gen, published_at, created_at, updated_at").
		Order("month DESC, site_id ASC, id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	if items == nil {
		items = []model.OpsReport{}
	}
	// 站点名与生成人附加
	siteNames := map[uint]string{0: "全站"}
	{
		var sites []model.Site
		h.DB.Find(&sites)
		for _, s := range sites {
			siteNames[s.ID] = s.Name
		}
	}
	userNames := map[uint]string{0: "系统自动"}
	{
		var users []model.User
		h.DB.Find(&users)
		for _, u := range users {
			userNames[u.ID] = u.Username
		}
	}
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"id": it.ID, "code": it.Code, "title": it.Title, "site_id": it.SiteID,
			"site_name": siteNames[it.SiteID], "month": it.Month,
			"period_start": it.PeriodStart, "period_end": it.PeriodEnd,
			"summary": it.Summary, "status": it.Status, "auto_gen": it.AutoGen,
			"generated_by": it.GeneratedBy, "generated_by_name": userNames[it.GeneratedBy],
			"published_at": it.PublishedAt, "created_at": it.CreatedAt, "updated_at": it.UpdatedAt,
		})
	}
	response.Paginated(c, out, total, page, size)
}

// GetOpsReport GET /ops-reports/:id 详情（content 解析为 JSON 对象返回）
func (h *Handler) GetOpsReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "ID 无效")
		return
	}
	var rec model.OpsReport
	if err := h.DB.First(&rec, id).Error; err != nil {
		response.NotFound(c, "报告不存在")
		return
	}
	var content map[string]interface{}
	if err := json.Unmarshal([]byte(rec.Content), &content); err != nil {
		content = map[string]interface{}{}
	}
	siteName := "全站"
	if rec.SiteID > 0 {
		var s model.Site
		if err := h.DB.First(&s, rec.SiteID).Error; err == nil {
			siteName = s.Name
		}
	}
	genName := "系统自动"
	if rec.GeneratedBy > 0 {
		var u model.User
		if err := h.DB.First(&u, rec.GeneratedBy).Error; err == nil {
			genName = u.Username
		}
	}
	response.Success(c, gin.H{
		"id": rec.ID, "code": rec.Code, "title": rec.Title,
		"site_id": rec.SiteID, "site_name": siteName, "month": rec.Month,
		"period_start": rec.PeriodStart, "period_end": rec.PeriodEnd,
		"summary": rec.Summary, "status": rec.Status, "auto_gen": rec.AutoGen,
		"generated_by": rec.GeneratedBy, "generated_by_name": genName,
		"published_at": rec.PublishedAt, "created_at": rec.CreatedAt, "updated_at": rec.UpdatedAt,
		"content": content,
	})
}

// GenerateOpsReport POST /ops-reports/generate {month:"yyyy-MM", site_id:0, force:false}（admin）
func (h *Handler) GenerateOpsReport(c *gin.Context) {
	var req struct {
		Month  string `json:"month"`
		SiteID uint   `json:"site_id"`
		Force  bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Month == "" {
		response.BadRequest(c, "请指定月份（yyyy-MM）")
		return
	}
	if req.SiteID > 0 {
		var s model.Site
		if err := h.DB.First(&s, req.SiteID).Error; err != nil {
			response.BadRequest(c, "站点不存在")
			return
		}
	}
	uid := c.GetUint("user_id")
	rec, existed, err := h.generateOpsReport(req.Month, req.SiteID, uid, false, req.Force)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	detail := fmt.Sprintf("month=%s site_id=%d force=%v code=%s", req.Month, req.SiteID, req.Force, rec.Code)
	go middleware.WriteAudit(h.DB, c, "create", "ops_report", detail)
	msg := "月报已生成"
	if existed {
		msg = "该月份报告已存在（如需重算请勾选强制重新生成）"
	} else if req.Force {
		msg = "月报已重新生成"
	}
	response.Success(c, gin.H{"id": rec.ID, "code": rec.Code, "existed": existed, "msg": msg})
}

// PublishOpsReport PUT /ops-reports/:id/publish（admin）
func (h *Handler) PublishOpsReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "ID 无效")
		return
	}
	var rec model.OpsReport
	if err := h.DB.First(&rec, id).Error; err != nil {
		response.NotFound(c, "报告不存在")
		return
	}
	if rec.Status == 2 {
		response.BadRequest(c, "报告已是发布状态")
		return
	}
	nowT := time.Now()
	if err := h.DB.Model(&rec).Updates(map[string]interface{}{"status": 2, "published_at": nowT}).Error; err != nil {
		response.ServerError(c, "发布失败")
		return
	}
	rec.Status = 2
	rec.PublishedAt = &nowT
	go middleware.WriteAudit(h.DB, c, "publish", "ops_report", rec.Code)
	h.notifyOpsReport(&rec, "publish")
	response.Success(c, gin.H{"id": rec.ID, "status": 2})
}

// DeleteOpsReport DELETE /ops-reports/:id（admin）
func (h *Handler) DeleteOpsReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "ID 无效")
		return
	}
	var rec model.OpsReport
	if err := h.DB.First(&rec, id).Error; err != nil {
		response.NotFound(c, "报告不存在")
		return
	}
	if err := h.DB.Delete(&rec).Error; err != nil {
		response.ServerError(c, "删除失败")
		return
	}
	go middleware.WriteAudit(h.DB, c, "delete", "ops_report", rec.Code)
	response.Success(c, gin.H{"id": id})
}
