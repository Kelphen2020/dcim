package api

// V64 告警质量治理与规则有效性分析
// 闭环定位：采集=告警五态流转留痕(acknowledged_at/resolved_at)+风暴抑制标记+规则表(既有数据,零新采集通道);
//           评估=GET /alerts/quality-analysis 聚合引擎(疑似误报/抑制/确认/转单/重复/MTTA/MTTR/噪声分级)
//                + ScanAlertQuality 周期评估高噪声规则;
//           使用=告警规则页「质量治理」页签(KPI/噪声TOP/重复测点/逐日趋势/调优建议/CSV导出);
//           报警=高噪声规则通知 kind=alert_quality(ref_type=alert_rule 可穿透) + Webhook 事件 alert_quality(7天幂等)。
// 判定口径：已结束(status=3) 且 从未被确认(acknowledged_at IS NULL) 且 存续 < 10 分钟 → 疑似误报(抖动即自动恢复)。

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

const v64FalseWindowMin = 10.0 // 疑似误报判定窗口(分钟)
const v64ScanMinTriggers = 5   // 周期评估最少触发次数
const v64ScanFalseRate = 60.0  // 周期评估误报率红线(%)

// v64IsSuspectedFalse 疑似误报判定（未确认即短时自动恢复）
func v64IsSuspectedFalse(a *model.Alert) bool {
	return a.Status == 3 && a.AcknowledgedAt == nil && a.ResolvedAt != nil &&
		a.ResolvedAt.Sub(a.CreatedAt).Minutes() < v64FalseWindowMin
}

type v64RuleAgg struct {
	RuleID         uint    `json:"rule_id"`
	RuleName       string  `json:"rule_name"`
	Enabled        bool    `json:"enabled"`
	Total          int     `json:"total"`
	SuspectedFalse int     `json:"suspected_false"`
	Suppressed     int     `json:"suppressed"`
	Acked          int     `json:"acked"`
	WoConverted    int     `json:"wo_converted"`
	Escalated      int     `json:"escalated"`
	FalseRate      float64 `json:"false_rate"`
	AckRate        float64 `json:"ack_rate"`
	AvgAckMin      float64 `json:"avg_ack_min"`
	NoiseScore     int     `json:"noise_score"`
	Suggestion     string  `json:"suggestion"`
	ackSum         float64
	ackN           int
}

type v64PointAgg struct {
	PointID        uint       `json:"point_id"`
	PointName      string     `json:"point_name"`
	PointCode      string     `json:"point_code"`
	Count          int        `json:"count"`
	Days           int        `json:"days"`
	SuspectedFalse int        `json:"suspected_false"`
	Suppressed     int        `json:"suppressed"`
	SeverityMax    int        `json:"severity_max"`
	LastAt         *time.Time `json:"last_at"`
	daySet         map[string]bool
}

// AlertQualityAnalysis GET /alerts/quality-analysis?days=7|30|90&site_id=
func (h *Handler) AlertQualityAnalysis(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days != 7 && days != 90 {
		days = 30
	}
	siteID := c.Query("site_id")
	now := time.Now()
	since := now.AddDate(0, 0, -days)

	q := h.DB.Model(&model.Alert{}).Where("created_at >= ?", since)
	if siteID != "" && siteID != "0" {
		q = q.Where("site_id = ?", siteID)
	}
	var rows []model.Alert
	q.Find(&rows)

	// 规则名/测点名映射
	ruleName := map[uint]string{}
	ruleEnabled := map[uint]bool{}
	var rules []model.AlertRule
	h.DB.Find(&rules)
	for _, r := range rules {
		ruleName[r.ID] = r.Name
		ruleEnabled[r.ID] = r.Enabled
	}
	pointName := map[uint]string{}
	pointCode := map[uint]string{}
	var points []model.MonitoringPoint
	h.DB.Find(&points)
	for _, p := range points {
		pointName[p.ID] = p.Name
		pointCode[p.ID] = p.Code
	}

	pct := func(ok, all int) float64 {
		if all == 0 {
			return 0
		}
		return mathRound(float64(ok)*100/float64(all), 1)
	}

	total := len(rows)
	open, resolved, acked := 0, 0, 0
	suspected, suppressedCnt, woConv, escalatedCnt, blockedCnt := 0, 0, 0, 0, 0
	var mttaSum, mttrSum float64
	var mttaN, mttrN int
	stormSet := map[uint]bool{}
	byRule := map[uint]*v64RuleAgg{}
	byPoint := map[uint]*v64PointAgg{}
	bySev := map[int]map[string]int{}

	trend := make([]gin.H, days)
	dayIdx := map[string]int{}
	for i := 0; i < days; i++ {
		d := since.AddDate(0, 0, i+1).Format("2006-01-02")
		dayIdx[d] = i
		trend[i] = gin.H{"date": d, "total": 0, "suspected": 0, "suppressed": 0, "wo": 0}
	}

	for i := range rows {
		a := &rows[i]
		isFalse := v64IsSuspectedFalse(a)
		if a.Status == 3 {
			resolved++
		} else {
			open++
		}
		if a.AcknowledgedAt != nil {
			acked++
			m := a.AcknowledgedAt.Sub(a.CreatedAt).Minutes()
			if m >= 0 {
				mttaSum += m
				mttaN++
			}
		}
		if a.Status == 3 && a.ResolvedAt != nil {
			m := a.ResolvedAt.Sub(a.CreatedAt).Minutes()
			if m >= 0 {
				mttrSum += m
				mttrN++
			}
		}
		if isFalse {
			suspected++
		}
		if a.Suppressed {
			suppressedCnt++
		}
		if a.WorkOrderID != nil && *a.WorkOrderID > 0 {
			woConv++
		}
		if a.Escalated {
			escalatedCnt++
		}
		if a.Blocked {
			blockedCnt++
		}
		if a.StormID > 0 {
			stormSet[a.StormID] = true
		}

		// 规则维度（rule_id 空 = 内置过高线告警，无规则记录）
		rid := uint(0)
		if a.RuleID != nil {
			rid = *a.RuleID
		}
		ra := byRule[rid]
		if ra == nil {
			ra = &v64RuleAgg{RuleID: rid}
			if rid == 0 {
				ra.RuleName = "内置过高线（无规则）"
				ra.Enabled = true
			} else {
				ra.RuleName = ruleName[rid]
				if ra.RuleName == "" {
					ra.RuleName = fmt.Sprintf("规则#%d（已删除）", rid)
				}
				ra.Enabled = ruleEnabled[rid]
			}
			byRule[rid] = ra
		}
		ra.Total++
		if isFalse {
			ra.SuspectedFalse++
		}
		if a.Suppressed {
			ra.Suppressed++
		}
		if a.AcknowledgedAt != nil {
			ra.Acked++
			m := a.AcknowledgedAt.Sub(a.CreatedAt).Minutes()
			if m >= 0 {
				ra.ackSum += m
				ra.ackN++
			}
		}
		if a.WorkOrderID != nil && *a.WorkOrderID > 0 {
			ra.WoConverted++
		}
		if a.Escalated {
			ra.Escalated++
		}

		// 测点维度
		pid := uint(0)
		if a.PointID != nil {
			pid = *a.PointID
		}
		pa := byPoint[pid]
		if pa == nil {
			pa = &v64PointAgg{PointID: pid, PointName: pointName[pid], PointCode: pointCode[pid], daySet: map[string]bool{}}
			if pa.PointName == "" {
				pa.PointName = fmt.Sprintf("测点#%d（已删除）", pid)
			}
			byPoint[pid] = pa
		}
		pa.Count++
		pa.daySet[a.CreatedAt.Format("2006-01-02")] = true
		if isFalse {
			pa.SuspectedFalse++
		}
		if a.Suppressed {
			pa.Suppressed++
		}
		if a.Severity > pa.SeverityMax {
			pa.SeverityMax = a.Severity
		}
		t := a.CreatedAt
		if pa.LastAt == nil || t.After(*pa.LastAt) {
			pa.LastAt = &t
		}

		// 严重度维度
		sv := bySev[a.Severity]
		if sv == nil {
			sv = map[string]int{}
			bySev[a.Severity] = sv
		}
		sv["total"]++
		if isFalse {
			sv["suspected_false"]++
		}
		if a.Status == 3 {
			sv["resolved"]++
		}
		if a.WorkOrderID != nil && *a.WorkOrderID > 0 {
			sv["wo_converted"]++
		}

		// 趋势桶（按创建日）
		dk := a.CreatedAt.Format("2006-01-02")
		if idx, ok := dayIdx[dk]; ok {
			trend[idx]["total"] = trend[idx]["total"].(int) + 1
			if isFalse {
				trend[idx]["suspected"] = trend[idx]["suspected"].(int) + 1
			}
			if a.Suppressed {
				trend[idx]["suppressed"] = trend[idx]["suppressed"].(int) + 1
			}
			if a.WorkOrderID != nil && *a.WorkOrderID > 0 {
				trend[idx]["wo"] = trend[idx]["wo"].(int) + 1
			}
		}
	}

	// 重复告警：同测点窗口内 >=3 次
	repeatPoints, repeatAlerts := 0, 0
	for _, pa := range byPoint {
		pa.Days = len(pa.daySet)
		if pa.Count >= 3 {
			repeatPoints++
			repeatAlerts += pa.Count
		}
	}

	noiseRate := pct(suspected+suppressedCnt, total)
	grade := "ok"
	if total > 0 {
		if noiseRate >= 40 {
			grade = "bad"
		} else if noiseRate >= 20 {
			grade = "warn"
		}
	}

	kpi := gin.H{
		"total":          total,
		"open":           open,
		"resolved":       resolved,
		"acked":          acked,
		"ack_rate":       pct(acked, total),
		"suspected_false": suspected,
		"false_rate":     pct(suspected, total),
		"suppressed":     suppressedCnt,
		"suppress_rate":  pct(suppressedCnt, total),
		"storms":         len(stormSet),
		"wo_converted":   woConv,
		"wo_rate":        pct(woConv, total),
		"escalated":      escalatedCnt,
		"blocked":        blockedCnt,
		"mtta_min":       mathRound(mttaSum/float64(maxInt(mttaN, 1)), 1),
		"mttr_min":       mathRound(mttrSum/float64(maxInt(mttrN, 1)), 1),
		"repeat_points":  repeatPoints,
		"repeat_alerts":  repeatAlerts,
		"repeat_rate":    pct(repeatAlerts, total),
		"noise_rate":     noiseRate,
		"grade":          grade,
	}
	if mttaN == 0 {
		kpi["mtta_min"] = 0
	}
	if mttrN == 0 {
		kpi["mttr_min"] = 0
	}

	// 规则维度输出：噪声分 = 疑似误报×2 + 被抑制×1，附中文调优建议
	ruleOut := make([]*v64RuleAgg, 0, len(byRule))
	for _, ra := range byRule {
		ra.FalseRate = pct(ra.SuspectedFalse, ra.Total)
		ra.AckRate = pct(ra.Acked, ra.Total)
		if ra.ackN > 0 {
			ra.AvgAckMin = mathRound(ra.ackSum/float64(ra.ackN), 1)
		}
		ra.NoiseScore = ra.SuspectedFalse*2 + ra.Suppressed
		switch {
		case ra.Total >= v64ScanMinTriggers && ra.FalseRate >= v64ScanFalseRate:
			ra.Suggestion = "误报率高：建议上调阈值、增大告警延迟(alarm_delay)或核查测点接线/抖动"
		case ra.Total >= v64ScanMinTriggers && ra.Suppressed*2 >= ra.Total:
			ra.Suggestion = "多为风暴伴随告警：建议优先治理根因对象，本规则可降低严重度"
		case ra.Total >= v64ScanMinTriggers && ra.AckRate <= 20 && ra.WoConverted == 0:
			ra.Suggestion = "长期无人确认也未转工单：建议评估保留必要性，或降级为提示"
		case ra.Total >= v64ScanMinTriggers && ra.FalseRate >= 30:
			ra.Suggestion = "存在一定误报：建议结合趋势分析核对阈值裕度"
		}
		ruleOut = append(ruleOut, ra)
	}
	sort.Slice(ruleOut, func(i, j int) bool {
		if ruleOut[i].NoiseScore != ruleOut[j].NoiseScore {
			return ruleOut[i].NoiseScore > ruleOut[j].NoiseScore
		}
		return ruleOut[i].Total > ruleOut[j].Total
	})
	if len(ruleOut) > 20 {
		ruleOut = ruleOut[:20]
	}

	// 测点维度输出：重复告警 TOP（>=2 次才进榜）
	pointOut := make([]*v64PointAgg, 0, len(byPoint))
	for _, pa := range byPoint {
		if pa.Count >= 2 {
			pointOut = append(pointOut, pa)
		}
	}
	sort.Slice(pointOut, func(i, j int) bool {
		if pointOut[i].Count != pointOut[j].Count {
			return pointOut[i].Count > pointOut[j].Count
		}
		return pointOut[i].SuspectedFalse > pointOut[j].SuspectedFalse
	})
	if len(pointOut) > 10 {
		pointOut = pointOut[:10]
	}

	sevLabel := map[int]string{1: "提示", 2: "警告", 3: "严重"}
	sevOut := make([]gin.H, 0, 3)
	for s := 1; s <= 3; s++ {
		if sv, ok := bySev[s]; ok {
			sevOut = append(sevOut, gin.H{
				"severity": s, "label": sevLabel[s], "total": sv["total"],
				"suspected_false": sv["suspected_false"], "resolved": sv["resolved"],
				"wo_converted": sv["wo_converted"],
			})
		}
	}

	response.Success(c, gin.H{
		"days":         days,
		"kpi":          kpi,
		"by_rule":      ruleOut,
		"by_point":     pointOut,
		"by_severity":  sevOut,
		"trend":        trend,
		"criteria": gin.H{
			"false_window_min":  v64FalseWindowMin,
			"scan_min_triggers": v64ScanMinTriggers,
			"scan_false_rate":   v64ScanFalseRate,
			"explain":           "疑似误报=已结束且从未被确认、存续不足10分钟（抖动即自动恢复）；噪声率=(疑似误报+被抑制)/总数",
		},
		"generated_at": now,
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ScanAlertQuality 周期评估：近7天高误报规则 → 通知 + Webhook（同规则7天幂等）
func (h *Handler) ScanAlertQuality() map[string]interface{} {
	stats := map[string]interface{}{"evaluated": 0, "notified": 0}
	if h.healthSetting("alert_quality_notify", "1") == "0" {
		return stats
	}
	since := time.Now().AddDate(0, 0, -7)
	var rows []model.Alert
	h.DB.Where("created_at >= ?", since).Find(&rows)

	type agg struct{ total, falseN int }
	byRule := map[uint]*agg{}
	for i := range rows {
		a := &rows[i]
		if a.RuleID == nil || *a.RuleID == 0 {
			continue
		}
		rid := *a.RuleID
		g := byRule[rid]
		if g == nil {
			g = &agg{}
			byRule[rid] = g
		}
		g.total++
		if v64IsSuspectedFalse(a) {
			g.falseN++
		}
	}
	for rid, g := range byRule {
		stats["evaluated"] = stats["evaluated"].(int) + 1
		if g.total < v64ScanMinTriggers {
			continue
		}
		rate := float64(g.falseN) * 100 / float64(g.total)
		if rate < v64ScanFalseRate {
			continue
		}
		var dup int64
		h.DB.Model(&model.Notification{}).
			Where("kind = ? AND ref_id = ? AND created_at > ?", "alert_quality", rid, since).
			Count(&dup)
		if dup > 0 {
			continue
		}
		var rule model.AlertRule
		if err := h.DB.First(&rule, rid).Error; err != nil {
			continue
		}
		msg := fmt.Sprintf("规则「%s」近7天触发 %d 次，其中 %d 次疑似误报（误报率 %.0f%%）。建议上调阈值或增大告警延迟，详见 告警规则→质量治理", rule.Name, g.total, g.falseN, rate)
		h.DB.Create(&model.Notification{
			UserID: 0, Kind: "alert_quality",
			Title: "告警规则噪声预警", Content: msg,
			RefType: "alert_rule", RefID: rid,
		})
		h.DispatchWebhook(model.EvAlertQuality, "[噪声] 规则 "+rule.Name+" 误报率偏高", msg, map[string]interface{}{
			"rule_id": rid, "rule_name": rule.Name, "total": g.total,
			"suspected_false": g.falseN, "false_rate": mathRound(rate, 1),
		})
		stats["notified"] = stats["notified"].(int) + 1
	}
	return stats
}

// AlertQualityScan POST /alerts/quality-scan 手动触发评估（管理员）
func (h *Handler) AlertQualityScan(c *gin.Context) {
	stats := h.ScanAlertQuality()
	response.Success(c, stats)
}
