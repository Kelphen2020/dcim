package api

// V73 B3-3（G6）PUE 合规红线主动告警
// 闭环：小时扫描 → 政策库红线判定（over/edge）→ 持续时间达标 → 告警记录 + 通知 + Webhook + 审计
//       → 恢复自动结束 + 恢复通知；设置白名单 pue_over_notify / pue_over_sustain_hours
// 幂等：同站点仅 1 条活跃告警；同站点同状态通知 24h 唯一；关闭通知开关仍记录告警，仅不发通知

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// v73Snapshot 单站点合规快照（口径与 V49 EnergyCompliance 一致）
type v73Snapshot struct {
	SiteID     uint
	SiteCode   string
	SiteName   string
	MatchBy    string
	Policy     *model.PUEPolicy
	LatestPUE  float64
	Avg30      float64
	AnnualKWh  float64
	Limit      float64
	Status     string // ok/edge/over/nodata/unknown
	Margin     float64
	Extra      float64
	MatchedHit string
}

// v73AlertDetail 合规告警 detail JSON（解析用于幂等与恢复）
type v73AlertDetail struct {
	Kind            string  `json:"kind"`
	SiteID          uint    `json:"site_id"`
	SiteName        string  `json:"site_name"`
	Status          string  `json:"status"`
	PUE             float64 `json:"pue"`
	Limit           float64 `json:"limit"`
	Margin          float64 `json:"margin"`
	Avg30           float64 `json:"avg30"`
	SurchargeAnnual float64 `json:"surcharge_annual"`
	SurchargeHit    string  `json:"surcharge_hit"`
}

func (h *Handler) v73SustainHours() int {
	hours := 2
	if v, err := strconv.Atoi(h.healthSetting("pue_over_sustain_hours", "2")); err == nil && v >= 1 && v <= 24 {
		hours = v
	}
	return hours
}

func (h *Handler) v73NotifyEnabled() bool {
	return h.healthSetting("pue_over_notify", "1") != "0"
}

// v73ComplianceSnapshot 计算站点当前合规状态（与 V49 页面同口径）
func (h *Handler) v73ComplianceSnapshot(site *model.Site) v73Snapshot {
	snap := v73Snapshot{SiteID: site.ID, SiteCode: site.Code, SiteName: site.Name, Status: "unknown"}
	policy, matchBy := h.matchSitePolicy(site)
	snap.MatchBy = matchBy
	if policy == nil || policy.NewLargeLimit <= 0 {
		return snap
	}
	snap.Policy = policy
	snap.Limit = policy.NewLargeLimit

	var lr struct {
		PUE float64
	}
	h.DB.Raw(`SELECT COALESCE(pue,0) as pue FROM t_pue_record WHERE site_id = ? ORDER BY recorded_at DESC LIMIT 1`, site.ID).Scan(&lr)
	cutoff := time.Now().AddDate(0, 0, -30)
	var avg30, avgTotal float64
	h.DB.Raw(`SELECT COALESCE(AVG(pue),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ?`, site.ID, cutoff).Scan(&avg30)
	h.DB.Raw(`SELECT COALESCE(AVG(total_power),0) FROM t_pue_record WHERE site_id = ? AND recorded_at >= ?`, site.ID, cutoff).Scan(&avgTotal)
	snap.LatestPUE = roundV49(lr.PUE)
	snap.Avg30 = roundV49(avg30)

	annualKWh := avgTotal * 24 * 365
	pue := lr.PUE
	if pue <= 0 {
		pue = avg30
	}
	if annualKWh <= 0 {
		var itPower float64
		h.DB.Raw(`SELECT COALESCE(SUM(power_w),0)/1000.0 FROM t_asset WHERE site_id = ? AND power_w > 0 AND status = 1`, site.ID).Scan(&itPower)
		p := pue
		if p <= 0 {
			p = 1.4
		}
		annualKWh = itPower * p * 24 * 365
	}
	snap.AnnualKWh = roundV49(annualKWh)

	if pue <= 0 {
		snap.Status = "nodata"
		return snap
	}
	snap.Margin = roundV49(snap.Limit - pue)
	switch {
	case pue > snap.Limit:
		snap.Status = "over"
	case snap.Limit-pue < 0.05:
		snap.Status = "edge"
	default:
		snap.Status = "ok"
	}
	snap.Extra, snap.MatchedHit = tariffExposure(policy.SurchargeRules, pue, annualKWh)
	snap.Extra = roundV49(snap.Extra)
	return snap
}

// v73Sustained 最近连续采样点是否持续越线（至少 2 个点，且连续段起点在 hours+1 小时内）
// 说明：只看“自最近一次合规采样之后”的连续越线段，避免恢复后旧的低值样本长期污染窗口
func (h *Handler) v73Sustained(siteID uint, limit float64, status string, hours int) bool {
	threshold := limit
	if status == "edge" {
		threshold = limit - 0.05
	}
	var rows []struct {
		PUE        float64
		RecordedAt time.Time
	}
	if err := h.DB.Raw(`SELECT pue, recorded_at FROM t_pue_record WHERE site_id = ? AND pue > 0 ORDER BY recorded_at DESC LIMIT 50`, siteID).Scan(&rows).Error; err != nil {
		return false
	}
	need := hours
	if need < 2 {
		need = 2
	}
	streak := 0
	var oldest time.Time
	for _, r := range rows {
		if r.PUE <= threshold {
			break
		}
		streak++
		oldest = r.RecordedAt
		if streak >= need {
			break
		}
	}
	if streak < need {
		return false
	}
	if !oldest.IsZero() && time.Since(oldest) > time.Duration(hours+1)*time.Hour {
		return false
	}
	return true
}

// v73NotifiedRecently 同站点同状态通知 24h 幂等
func (h *Handler) v73NotifiedRecently(siteID uint, status string) bool {
	label := "超标"
	if status == "edge" {
		label = "临界"
	}
	var n int64
	h.DB.Model(&model.Notification{}).
		Where("kind = ? AND ref_type = ? AND ref_id = ? AND created_at > ? AND content LIKE ?",
			"pue_over", "site", siteID, time.Now().Add(-24*time.Hour), "%"+label+"%").
		Count(&n)
	return n > 0
}

func (h *Handler) v73NotifyText(snap v73Snapshot) (title, content string) {
	tag := "超标"
	if snap.Status == "edge" {
		tag = "临界"
	}
	title = fmt.Sprintf("[PUE%s] %s", tag, snap.SiteName)
	rule := "未触发差别电价加价规则"
	if snap.MatchedHit != "" {
		rule = snap.MatchedHit
	}
	if snap.Status == "over" {
		content = fmt.Sprintf("站点 %s（%s）当前 PUE %.2f，高于%s政策红线 %.2f（超出 %.2f）。近30天均值 %.2f；按当地差别电价规则（%s）测算，年化电费敞口约 ¥%.0f。建议核查冷通道封闭、送回风组织与冷机/水泵运行策略，整改后到能耗总览复核。",
			snap.SiteName, snap.SiteCode, snap.LatestPUE, snap.Policy.Province, snap.Limit, -snap.Margin, snap.Avg30, rule, snap.Extra)
	} else {
		content = fmt.Sprintf("站点 %s（%s）当前 PUE %.2f，距离%s政策红线 %.2f 仅余 %.2f，已进入临界区。近30天均值 %.2f。建议提前排查制冷效率与高密机柜负载，避免越线后触发差别电价。",
			snap.SiteName, snap.SiteCode, snap.LatestPUE, snap.Policy.Province, snap.Limit, snap.Margin, snap.Avg30)
	}
	return
}

func (h *Handler) v73Emit(snap v73Snapshot, sev int) bool {
	title, content := h.v73NotifyText(snap)
	if !h.v73NotifyEnabled() || h.v73NotifiedRecently(snap.SiteID, snap.Status) {
		return false
	}
	h.DB.Create(&model.Notification{
		UserID: 0, Kind: "pue_over", Title: title, Content: content,
		RefType: "site", RefID: snap.SiteID,
	})
	h.DispatchWebhook(model.EvPUECompliance, title, content, map[string]interface{}{
		"site_id": snap.SiteID, "site_code": snap.SiteCode, "site_name": snap.SiteName,
		"status": snap.Status, "pue": snap.LatestPUE, "avg30": snap.Avg30,
		"limit": snap.Limit, "margin": snap.Margin,
		"surcharge_annual": snap.Extra, "surcharge_hit": snap.MatchedHit, "severity": sev,
	})
	return true
}

// ScanPUECompliance 合规扫描（siteID=0 全站；手动触发可限定单站）
func (h *Handler) ScanPUECompliance(siteID uint) map[string]interface{} {
	stats := map[string]interface{}{
		"evaluated": 0, "alerts": 0, "notified": 0, "updated": 0, "recovered": 0, "skipped": 0,
	}
	sustain := h.v73SustainHours()
	now := time.Now()

	var sites []model.Site
	q := h.DB.Order("id")
	if siteID > 0 {
		q = q.Where("id = ?", siteID)
	}
	q.Find(&sites)

	var actives []model.Alert
	h.DB.Where("rule_id IS NULL AND status < 3 AND detail LIKE ?", "%pue_compliance%").
		Order("id").Find(&actives)
	activeBySite := map[uint]*model.Alert{}
	activeDetail := map[uint]v73AlertDetail{}
	for i := range actives {
		var d v73AlertDetail
		if json.Unmarshal([]byte(actives[i].Detail), &d) == nil && d.Kind == "pue_compliance" && d.SiteID > 0 {
			activeBySite[d.SiteID] = &actives[i]
			activeDetail[d.SiteID] = d
		}
	}

	visited := map[uint]bool{}
	for i := range sites {
		s := &sites[i]
		if s.Status == 0 {
			continue
		}
		visited[s.ID] = true
		stats["evaluated"] = stats["evaluated"].(int) + 1
		snap := h.v73ComplianceSnapshot(s)
		want := snap.Status == "over" || snap.Status == "edge"
		active, hasActive := activeBySite[s.ID]

		if hasActive {
			old := activeDetail[s.ID]
			if !want {
				h.DB.Model(&model.Alert{}).Where("id = ?", active.ID).
					Updates(map[string]interface{}{"status": 3, "resolved_at": now})
				h.DB.Create(&model.AlertActionLog{
					AlertID: active.ID, Action: "resolve",
					Reason: "PUE 已恢复合规或政策不再适用，系统自动结束", Operator: 0,
				})
				if h.v73NotifyEnabled() {
					title := "[PUE恢复] " + s.Name
					content := ""
					if snap.Status == "unknown" || snap.Status == "nodata" {
						content = fmt.Sprintf("站点 %s 已无有效 PUE 数据或政策匹配，原合规告警自动结束，请核实采集与站点省份配置。", s.Name)
					} else {
						content = fmt.Sprintf("站点 %s 当前 PUE %.2f，已回到%s政策红线 %.2f 以内，合规告警自动结束。", s.Name, snap.LatestPUE, snap.Policy.Province, snap.Limit)
					}
					h.DB.Create(&model.Notification{UserID: 0, Kind: "pue_over", Title: title, Content: content, RefType: "site", RefID: s.ID})
					h.DispatchWebhook(model.EvPUECompliance, title, content, map[string]interface{}{
						"site_id": s.ID, "site_code": s.Code, "site_name": s.Name,
						"status": snap.Status, "pue": snap.LatestPUE, "recovered": true,
					})
				}
				h.DB.Create(&model.Audit{
					Username: "system", Action: "pue_compliance_recover", Target: s.Code,
					IP: "127.0.0.1", Detail: fmt.Sprintf("PUE 合规告警自动结束：%s → %s", old.Status, snap.Status),
				})
				stats["recovered"] = stats["recovered"].(int) + 1
				continue
			}
			// 状态升级（edge→over）或降级：更新同一条活跃告警，保持单站点唯一
			sev := 2
			if snap.Status == "over" {
				sev = 3
			}
			if old.Status != snap.Status || active.Severity != sev {
				title, _ := h.v73NotifyText(snap)
				detail, _ := json.Marshal(v73AlertDetail{
					Kind: "pue_compliance", SiteID: s.ID, SiteName: s.Name, Status: snap.Status,
					PUE: snap.LatestPUE, Limit: snap.Limit, Margin: snap.Margin, Avg30: snap.Avg30,
					SurchargeAnnual: snap.Extra, SurchargeHit: snap.MatchedHit,
				})
				h.DB.Model(&model.Alert{}).Where("id = ?", active.ID).Updates(map[string]interface{}{
					"severity": sev, "message": title, "detail": string(detail),
				})
				h.DB.Create(&model.AlertActionLog{
					AlertID: active.ID, Action: "update",
					Reason: fmt.Sprintf("合规状态变化：%s → %s，系统更新告警级别", old.Status, snap.Status), Operator: 0,
				})
				if snap.Status == "over" && old.Status == "edge" {
					if h.v73Emit(snap, sev) {
						stats["notified"] = stats["notified"].(int) + 1
					}
				}
				h.DB.Create(&model.Audit{
					Username: "system", Action: "pue_compliance_update", Target: s.Code,
					IP: "127.0.0.1", Detail: fmt.Sprintf("合规状态 %s → %s，告警 #%d 已更新", old.Status, snap.Status, active.ID),
				})
				stats["updated"] = stats["updated"].(int) + 1
			}
			continue
		}

		if !want {
			continue
		}
		if !h.v73Sustained(s.ID, snap.Limit, snap.Status, sustain) {
			stats["skipped"] = stats["skipped"].(int) + 1
			continue
		}
		sev := 2
		if snap.Status == "over" {
			sev = 3
		}
		title, _ := h.v73NotifyText(snap)
		detail, _ := json.Marshal(v73AlertDetail{
			Kind: "pue_compliance", SiteID: s.ID, SiteName: s.Name, Status: snap.Status,
			PUE: snap.LatestPUE, Limit: snap.Limit, Margin: snap.Margin, Avg30: snap.Avg30,
			SurchargeAnnual: snap.Extra, SurchargeHit: snap.MatchedHit,
		})
		al := model.Alert{
			SiteID: s.ID, Severity: sev, Message: title, Detail: string(detail),
			Status: 0, TriggeredAt: now, CreatedAt: now,
		}
		if err := h.DB.Create(&al).Error; err != nil {
			continue
		}
		stats["alerts"] = stats["alerts"].(int) + 1
		if h.v73Emit(snap, sev) {
			stats["notified"] = stats["notified"].(int) + 1
		}
		h.DB.Create(&model.Audit{
			Username: "system", Action: "pue_compliance_alert", Target: s.Code,
			IP: "127.0.0.1", Detail: fmt.Sprintf("PUE %s：当前 %.2f / 红线 %.2f，年化敞口 ¥%.0f，告警 #%d（持续 %dh）",
				snap.Status, snap.LatestPUE, snap.Limit, snap.Extra, al.ID, sustain),
		})
	}

	// 站点已删除/停用的孤儿告警自动结束（仅全站扫描时处理，单站手动扫描不得误结束其他站点）
	if siteID == 0 {
		for sid, al := range activeBySite {
			if visited[sid] {
				continue
			}
			h.DB.Model(&model.Alert{}).Where("id = ?", al.ID).Updates(map[string]interface{}{"status": 3, "resolved_at": now})
			h.DB.Create(&model.AlertActionLog{AlertID: al.ID, Action: "resolve", Reason: "站点停用或不存在，系统自动结束", Operator: 0})
			stats["recovered"] = stats["recovered"].(int) + 1
		}
	}
	return stats
}

// TriggerPUEScan POST /energy/pue-scan?site_id= （管理员手动触发）
func (h *Handler) TriggerPUEScan(c *gin.Context) {
	var sid uint
	if v := c.Query("site_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			sid = uint(n)
		}
	}
	stats := h.ScanPUECompliance(sid)
	middleware.WriteAudit(h.DB, c, "trigger_pue_compliance_scan", "energy", fmt.Sprintf("手动触发 PUE 合规扫描 site=%d，新建 %v 条，恢复 %v 条", sid, stats["alerts"], stats["recovered"]))
	response.Success(c, stats)
}

// v73NotifyInfo 供能耗页展示当前告警与通知状态
func (h *Handler) v73NotifyInfo(siteID uint) gin.H {
	info := gin.H{
		"enabled": h.v73NotifyEnabled(), "sustain_hours": h.v73SustainHours(),
		"active": nil, "recent_count": 0, "last_at": "", "last_notified_at": "",
	}
	var active model.Alert
	if err := h.DB.Where("rule_id IS NULL AND status < 3 AND site_id = ? AND detail LIKE ?", siteID, "%pue_compliance%").
		Order("id desc").First(&active).Error; err == nil {
		var d v73AlertDetail
		if json.Unmarshal([]byte(active.Detail), &d) == nil {
			info["active"] = gin.H{
				"id": active.ID, "message": active.Message, "severity": active.Severity,
				"status": d.Status, "pue": d.PUE, "limit": d.Limit, "margin": d.Margin,
				"surcharge_annual": d.SurchargeAnnual, "triggered_at": active.TriggeredAt.Format("2006-01-02 15:04"),
			}
		}
	}
	var cnt int64
	h.DB.Model(&model.Alert{}).Where("site_id = ? AND detail LIKE ? AND created_at >= ?", siteID, "%pue_compliance%", time.Now().AddDate(0, 0, -30)).Count(&cnt)
	info["recent_count"] = cnt
	var last model.Alert
	if err := h.DB.Where("site_id = ? AND detail LIKE ?", siteID, "%pue_compliance%").Order("id desc").First(&last).Error; err == nil {
		info["last_at"] = last.CreatedAt.Format("2006-01-02 15:04")
	}
	var n model.Notification
	if err := h.DB.Where("kind = ? AND ref_type = ? AND ref_id = ?", "pue_over", "site", siteID).Order("id desc").First(&n).Error; err == nil {
		info["last_notified_at"] = n.CreatedAt.Format("2006-01-02 15:04")
	}
	return info
}