package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ============ V54 供电冗余与单点故障分析 ============
// 闭环：采集(链路拓扑+测点实时负载) → 评估(单点检测+负载率+机柜供电路径→评分/等级)
//       → 使用(链路页冗余视图+Dashboard穿透) → 报警(恶化通知+Webhook+风险级自动整改工单,恢复通知)

// powerIssue 供电冗余问题项（每条带中文证据与整改建议）
type powerIssue struct {
	Code      string `json:"code"`      // single_utility / orphan_root / node_overload / cabinet_single_ups / cabinet_single_feed
	Severity  string `json:"severity"`  // high / medium / low
	Label     string `json:"label"`     // 问题类型中文名
	Target    string `json:"target"`    // 对象名（节点名/机柜编码）
	NodeID    uint   `json:"node_id,omitempty"`
	CabinetID uint   `json:"cabinet_id,omitempty"`
	Evidence  string `json:"evidence"`
	Advice    string `json:"advice"`
	Deduct    int    `json:"deduct"`
}

// powerNodeRow 节点负载行（冗余详情用）
type powerNodeRow struct {
	ID         uint    `json:"id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	ParentID   uint    `json:"parent_id"`
	RatedKVA   float64 `json:"rated_kva"`
	LoadKW     float64 `json:"load_kw"`
	LoadPct    float64 `json:"load_pct"` // eff/(rated*0.9)*100，与 V14 口径一致
	Source     string  `json:"source"`   // point / manual
	CabinetID  uint    `json:"cabinet_id"`
	Status     int     `json:"status"`
}

func powerGradeLabel(g string) string {
	switch g {
	case model.PowerGradeOK:
		return "冗余充足"
	case model.PowerGradeWarn:
		return "冗余不足"
	default:
		return "单点风险"
	}
}

func powerIsBad(g string) bool { return g == model.PowerGradeWarn || g == model.PowerGradeRisk }

// powerEvaluateSite 评估单站点供电冗余：返回评分、问题列表、节点负载行、受电机柜数
func (h *Handler) powerEvaluateSite(site *model.Site, nodes []model.PowerChain) (int, string, []powerIssue, []powerNodeRow, int) {
	issues := []powerIssue{}
	byID := map[uint]*model.PowerChain{}
	children := map[uint][]*model.PowerChain{}
	for i := range nodes {
		byID[nodes[i].ID] = &nodes[i]
		children[nodes[i].ParentID] = append(children[nodes[i].ParentID], &nodes[i])
	}

	// 测点实时负载（point_id 绑定优先，与 V14 EffectiveKW 同口径）
	pointIDs := []uint{}
	for i := range nodes {
		if nodes[i].PointID > 0 {
			pointIDs = append(pointIDs, nodes[i].PointID)
		}
	}
	pointVal := map[uint]float64{}
	if len(pointIDs) > 0 {
		var ps []model.MonitoringPoint
		h.DB.Select("id", "current_value").Where("id IN ?", pointIDs).Find(&ps)
		for _, p := range ps {
			if p.CurrentValue != nil {
				pointVal[p.ID] = *p.CurrentValue
			}
		}
	}

	// ---- 节点负载行 + 过载检测 ----
	rows := make([]powerNodeRow, 0, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		eff, src := n.LoadKW, "manual"
		if v, ok := pointVal[n.PointID]; ok && n.PointID > 0 {
			eff, src = v, "point"
		}
		row := powerNodeRow{ID: n.ID, Name: n.Name, Kind: n.Kind, ParentID: n.ParentID,
			RatedKVA: n.RatedKVA, LoadKW: eff, Source: src, CabinetID: n.CabinetID, Status: n.Status}
		if n.RatedKVA > 0 {
			row.LoadPct = eff / (n.RatedKVA * 0.9) * 100
			if row.LoadPct > 100 {
				row.LoadPct = 100
			}
			if row.LoadPct > 80 {
				issues = append(issues, powerIssue{
					Code: "node_overload", Severity: "high", Label: "节点过载", Target: n.Name, NodeID: n.ID,
					Evidence: fmt.Sprintf("负载 %.1fkW / 额定 %.0fkVA（功率因数0.9折算 %.0fkW），负载率 %.0f%%，超过 80%% 安全线", eff, n.RatedKVA, n.RatedKVA*0.9, row.LoadPct),
					Advice:   "转移部分负载至其他回路或扩容，负载率长期超 80% 将失去故障时切换余量", Deduct: 15})
			} else if row.LoadPct > 60 {
				issues = append(issues, powerIssue{
					Code: "node_overload", Severity: "medium", Label: "负载偏高", Target: n.Name, NodeID: n.ID,
					Evidence: fmt.Sprintf("负载率 %.0f%%（%.1fkW / %.0fkVA），超过 60%% 关注线", row.LoadPct, eff, n.RatedKVA),
					Advice:   "关注负载增长趋势，提前规划扩容", Deduct: 6})
			}
		}
		rows = append(rows, row)
	}

	// ---- 单路市电检测（根节点中的市电/总进线数量）----
	utilityRoots := 0
	orphanRoots := []*model.PowerChain{}
	for i := range nodes {
		n := &nodes[i]
		if n.ParentID != 0 {
			if byID[n.ParentID] == nil {
				orphanRoots = append(orphanRoots, n) // 父节点不存在=挂空
			}
			continue
		}
		switch n.Kind {
		case "utility", "incoming":
			utilityRoots++
		case "transformer":
			// 变压器作根可视作独立进线来源
			utilityRoots++
		default:
			orphanRoots = append(orphanRoots, n) // UPS/PDU 直接作根=无上游电源
		}
	}
	if utilityRoots < 2 {
		sev, deduct := "medium", 15
		if site.Level >= 3 {
			sev, deduct = "high", 30 // T3/T4 级机房要求双重电源
		}
		issues = append(issues, powerIssue{
			Code: "single_utility", Severity: sev, Label: "单路市电", Target: site.Name,
			Evidence: fmt.Sprintf("全站仅 %d 路市电/总进线引入", utilityRoots),
			Advice:   "GB50174 要求 T3 及以上机房双重电源供电，建议增容第二路市电或配置柴油发电机+ATS 切换", Deduct: deduct})
	}
	for _, n := range orphanRoots {
		issues = append(issues, powerIssue{
			Code: "orphan_root", Severity: "medium", Label: "节点无上游电源", Target: n.Name, NodeID: n.ID,
			Evidence: fmt.Sprintf("节点「%s」（%s）没有上级供电节点，处于挂空状态", n.Name, n.Kind),
			Advice:   "在供电链路中挂接到正确的上游节点，或确认为废弃节点后删除", Deduct: 8})
	}

	// ---- 机柜供电路径分析（A/B 双路 + UPS 冗余）----
	cabLeaves := map[uint][]*model.PowerChain{}   // cabinet_id → 绑定该柜的末端节点
	cabUps := map[uint]map[uint]bool{}            // cabinet_id → 上溯可达的 UPS 集合
	cabName := map[uint]string{}
	for i := range nodes {
		n := &nodes[i]
		if n.CabinetID == 0 {
			continue
		}
		cabLeaves[n.CabinetID] = append(cabLeaves[n.CabinetID], n)
		if cabUps[n.CabinetID] == nil {
			cabUps[n.CabinetID] = map[uint]bool{}
		}
		cur := n
		for d := 0; d < 50 && cur != nil; d++ {
			if cur.Kind == "ups" {
				cabUps[n.CabinetID][cur.ID] = true
			}
			if cur.ParentID == 0 {
				break
			}
			cur = byID[cur.ParentID]
		}
	}
	if len(cabLeaves) > 0 {
		cabIDs := make([]uint, 0, len(cabLeaves))
		for cid := range cabLeaves {
			cabIDs = append(cabIDs, cid)
		}
		var cabs []model.Cabinet
		h.DB.Select("id", "code").Where("id IN ?", cabIDs).Find(&cabs)
		for _, cb := range cabs {
			cabName[cb.ID] = cb.Code
		}
		sort.Slice(cabIDs, func(a, b int) bool { return cabIDs[a] < cabIDs[b] })
		singleUpsDeducted, singleFeedDeducted := 0, 0
		for _, cid := range cabIDs {
			name := cabName[cid]
			if name == "" {
				name = fmt.Sprintf("机柜#%d", cid)
			}
			if len(cabUps[cid]) == 1 {
				if singleUpsDeducted < 30 {
					issues = append(issues, powerIssue{
						Code: "cabinet_single_ups", Severity: "medium", Label: "机柜单 UPS 供电", Target: name, CabinetID: cid,
						Evidence: fmt.Sprintf("机柜 %s 的所有供电回路仅上溯到 1 台 UPS，该 UPS 故障即整柜断电", name),
						Advice:   "重要机柜建议 A/B 双路分别接入不同 UPS（2N 或 N+1 架构）", Deduct: 10})
					singleUpsDeducted += 10
				}
			}
			if len(cabLeaves[cid]) < 2 {
				if singleFeedDeducted < 20 {
					issues = append(issues, powerIssue{
						Code: "cabinet_single_feed", Severity: "low", Label: "机柜单路供电", Target: name, CabinetID: cid,
						Evidence: fmt.Sprintf("机柜 %s 仅 %d 路 PDU/支路供电，无 A/B 路冗余", name, len(cabLeaves[cid])),
						Advice:   "双电源设备机柜建议补第二路供电（不同 PDU/不同相位）", Deduct: 4})
					singleFeedDeducted += 4
				}
			}
		}
	}

	// ---- 评分与等级 ----
	score := 100
	hasHigh, hasMedium := false, false
	for _, is := range issues {
		score -= is.Deduct
		switch is.Severity {
		case "high":
			hasHigh = true
		case "medium":
			hasMedium = true
		}
	}
	if score < 0 {
		score = 0
	}
	grade := model.PowerGradeOK
	if hasHigh || score < 70 {
		grade = model.PowerGradeRisk
	} else if hasMedium || score < 90 {
		grade = model.PowerGradeWarn
	}
	return score, grade, issues, rows, len(cabLeaves)
}

// ScanPowerRedundancy 全站点供电冗余评估（定时器+手动共用），返回统计
func (h *Handler) ScanPowerRedundancy() map[string]interface{} {
	autoWO := h.healthSetting("power_redundancy_auto_work_order", "1") == "1"
	stats := map[string]interface{}{"evaluated": 0, "ok": 0, "warn": 0, "risk": 0, "notified": 0, "recovered": 0, "work_orders": 0}

	var sites []model.Site
	h.DB.Where("status = 1").Find(&sites)
	for i := range sites {
		s := sites[i]
		var nodes []model.PowerChain
		h.DB.Where("site_id = ?", s.ID).Order("id ASC").Find(&nodes)
		if len(nodes) == 0 {
			continue // 无供电链路数据的站点不评估
		}
		score, grade, issues, _, cabCount := h.powerEvaluateSite(&s, nodes)
		ij, _ := json.Marshal(issues)

		var rec model.PowerRedundancy
		err := h.DB.Where("site_id = ?", s.ID).First(&rec).Error
		existed := err == nil
		oldGrade := ""
		if existed {
			oldGrade = rec.Grade
			// 已开工单闭环则清空关联
			if rec.OpenWorkOrderID > 0 {
				var wo model.WorkOrder
				if h.DB.Select("id", "status").First(&wo, rec.OpenWorkOrderID).Error == nil && wo.Status > 2 {
					rec.OpenWorkOrderID = 0
				}
			}
		} else {
			rec = model.PowerRedundancy{SiteID: s.ID}
		}
		rec.Score, rec.Grade, rec.Issues = score, grade, string(ij)
		rec.NodeCount, rec.CabinetCount = len(nodes), cabCount
		rec.ComputedAt = time.Now()

		// 等级变迁 → 报警/恢复（LastNotifiedGrade 幂等，与 V50 同模式）
		if grade != oldGrade {
			if powerIsBad(grade) && rec.LastNotifiedGrade != grade {
				highN, medN := 0, 0
				for _, is := range issues {
					if is.Severity == "high" {
						highN++
					} else if is.Severity == "medium" {
						medN++
					}
				}
				title := fmt.Sprintf("供电冗余预警：%s（%s）评分 %d", s.Name, powerGradeLabel(grade), score)
				content := h.powerRedundancyContent(&s, score, grade, issues, highN, medN)
				h.DB.Create(&model.Notification{
					UserID: 0, Kind: "power_redundancy", Title: title, Content: content,
					RefType: "site", RefID: s.ID,
				})
				h.DispatchWebhook(model.EvPowerRedundancy, title, content, map[string]interface{}{
					"site_id": s.ID, "site_name": s.Name, "score": score, "grade": grade, "issue_count": len(issues),
				})
				stats["notified"] = stats["notified"].(int) + 1
				if autoWO && grade == model.PowerGradeRisk && rec.OpenWorkOrderID == 0 {
					if woID := h.createRedundancyWorkOrder(&s, score, issues, true); woID > 0 {
						rec.OpenWorkOrderID = woID
						stats["work_orders"] = stats["work_orders"].(int) + 1
					}
				}
				rec.LastNotifiedGrade = grade
			} else if existed && powerIsBad(oldGrade) && !powerIsBad(grade) && powerIsBad(rec.LastNotifiedGrade) {
				title := fmt.Sprintf("供电冗余恢复：%s 评分回升至 %d", s.Name, score)
				content := fmt.Sprintf("站点「%s」供电冗余问题已整改，评分由 %s 区间回升至 %d 分（%s），整改闭环完成。",
					s.Name, powerGradeLabel(oldGrade), score, powerGradeLabel(grade))
				h.DB.Create(&model.Notification{
					UserID: 0, Kind: "power_redundancy", Title: title, Content: content,
					RefType: "site", RefID: s.ID,
				})
				h.DispatchWebhook(model.EvPowerRedundancy, title, content, map[string]interface{}{
					"site_id": s.ID, "site_name": s.Name, "score": score, "grade": grade, "recovered": true,
				})
				stats["recovered"] = stats["recovered"].(int) + 1
				rec.LastNotifiedGrade = grade
			}
		}

		if existed {
			h.DB.Model(&model.PowerRedundancy{}).Where("id = ?", rec.ID).Updates(map[string]interface{}{
				"score": rec.Score, "grade": rec.Grade, "issues": rec.Issues,
				"node_count": rec.NodeCount, "cabinet_count": rec.CabinetCount,
				"open_work_order_id": rec.OpenWorkOrderID, "last_notified_grade": rec.LastNotifiedGrade,
				"computed_at": rec.ComputedAt,
			})
		} else {
			h.DB.Create(&rec)
		}
		stats["evaluated"] = stats["evaluated"].(int) + 1
		stats[grade] = stats[grade].(int) + 1
	}
	return stats
}

func (h *Handler) powerRedundancyContent(s *model.Site, score int, grade string, issues []powerIssue, highN, medN int) string {
	c := fmt.Sprintf("站点「%s」供电冗余评分 %d/100（%s），发现 %d 项问题（高危 %d / 中危 %d）：\n",
		s.Name, score, powerGradeLabel(grade), len(issues), highN, medN)
	n := 0
	for _, is := range issues {
		if n >= 5 {
			c += fmt.Sprintf("……等共 %d 项，详见 供电链路→冗余分析\n", len(issues))
			break
		}
		c += fmt.Sprintf("- [%s] %s：%s\n", is.Label, is.Target, is.Evidence)
		n++
	}
	return c
}

// createRedundancyWorkOrder 供电冗余整改工单（幂等：同站点已有未闭环整改工单则跳过）
func (h *Handler) createRedundancyWorkOrder(s *model.Site, score int, issues []powerIssue, auto bool) uint {
	var openCnt int64
	h.DB.Model(&model.WorkOrder{}).Where("site_id = ? AND title LIKE ? AND status <= 2", s.ID, "【供电冗余整改】%").Count(&openCnt)
	if openCnt > 0 {
		return 0
	}
	desc := fmt.Sprintf("供电冗余评分 %d/100，发现 %d 项问题：\n", score, len(issues))
	for _, is := range issues {
		desc += fmt.Sprintf("- [%s] %s：%s（建议：%s）\n", is.Label, is.Target, is.Evidence, is.Advice)
	}
	if auto {
		desc += "\n本工单由供电冗余评估引擎自动生成。"
	}
	now := time.Now()
	wo := model.WorkOrder{
		SiteID: s.ID, Code: h.genCode("WO"),
		Title:       fmt.Sprintf("【供电冗余整改】%s 评分 %d", s.Name, score),
		Type:        "task", Priority: 2, Status: 1,
		Description: desc,
		CreatedAt:   now, UpdatedAt: now,
	}
	assignNote := ""
	if duty := h.OnDutyUser(s.ID); duty != nil {
		wo.AssigneeID = duty.UserID
		var u model.User
		if h.DB.Select("display_name", "username").First(&u, duty.UserID).Error == nil {
			nm := u.DisplayName
			if nm == "" {
				nm = u.Username
			}
			assignNote = "，自动指派当班人 " + nm
		}
	}
	h.applySLA(&wo)
	if err := h.DB.Create(&wo).Error; err != nil {
		return 0
	}
	src := "手动创建"
	if auto {
		src = "供电冗余评估引擎自动创建"
	}
	h.DB.Create(&model.WorkOrderLog{OrderID: wo.ID, UserID: 0, Action: "create", Content: src + assignNote})
	h.DispatchWebhook(model.EvWorkOrder, "供电冗余整改工单 "+wo.Code, wo.Title+assignNote, map[string]interface{}{
		"wo_id": wo.ID, "wo_code": wo.Code, "priority": wo.Priority, "site_id": s.ID, "source": "power_redundancy",
	})
	return wo.ID
}

// ============ HTTP Handlers ============

// ListPowerRedundancy GET /power/redundancy 站点冗余总览
func (h *Handler) ListPowerRedundancy(c *gin.Context) {
	var recs []model.PowerRedundancy
	q := h.DB.Model(&model.PowerRedundancy{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	q.Find(&recs)
	siteName := map[uint]string{}
	{
		var ss []model.Site
		h.DB.Select("id", "name", "level").Find(&ss)
		for _, s := range ss {
			siteName[s.ID] = s.Name
		}
	}
	items := make([]gin.H, 0, len(recs))
	sum := map[string]int{"ok": 0, "warn": 0, "risk": 0}
	for _, r := range recs {
		var issues []powerIssue
		_ = json.Unmarshal([]byte(r.Issues), &issues)
		highN := 0
		for _, is := range issues {
			if is.Severity == "high" {
				highN++
			}
		}
		sum[r.Grade]++
		items = append(items, gin.H{
			"site_id": r.SiteID, "site_name": siteName[r.SiteID],
			"score": r.Score, "grade": r.Grade, "issue_count": len(issues), "high_count": highN,
			"node_count": r.NodeCount, "cabinet_count": r.CabinetCount,
			"open_work_order_id": r.OpenWorkOrderID, "computed_at": r.ComputedAt,
		})
	}
	sort.Slice(items, func(a, b int) bool {
		return items[a]["score"].(int) < items[b]["score"].(int)
	})
	response.Success(c, gin.H{"items": items, "summary": sum})
}

// parseSiteIDParam 解析 :site_id 路由参数（parseIDParam 只认 :id）
func parseSiteIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("site_id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "站点 ID 无效")
		return 0, false
	}
	return uint(id), true
}

// GetPowerRedundancySite GET /power/redundancy/:site_id 站点冗余详情（实时重算+存储记录）
func (h *Handler) GetPowerRedundancySite(c *gin.Context) {
	siteID, ok := parseSiteIDParam(c)
	if !ok {
		return
	}
	if scope := middleware.GetSiteScope(c); scope != nil {
		allowed := false
		for _, s := range scope {
			if s == siteID {
				allowed = true
			}
		}
		if !allowed {
			response.Forbidden(c, "无权查看该站点")
			return
		}
	}
	var site model.Site
	if err := h.DB.First(&site, siteID).Error; err != nil {
		response.NotFound(c, "站点不存在")
		return
	}
	var nodes []model.PowerChain
	h.DB.Where("site_id = ?", siteID).Order("id ASC").Find(&nodes)
	score, grade, issues, rows, cabCount := h.powerEvaluateSite(&site, nodes)

	var rec model.PowerRedundancy
	h.DB.Where("site_id = ?", siteID).First(&rec)
	response.Success(c, gin.H{
		"site": gin.H{"id": site.ID, "name": site.Name, "level": site.Level},
		"score": score, "grade": grade, "issues": issues, "nodes": rows, "cabinet_count": cabCount,
		"open_work_order_id": rec.OpenWorkOrderID, "computed_at": rec.ComputedAt,
	})
}

// EvaluatePowerRedundancy POST /power/redundancy/evaluate 手动触发全量评估（admin）
func (h *Handler) EvaluatePowerRedundancy(c *gin.Context) {
	stats := h.ScanPowerRedundancy()
	go middleware.WriteAudit(h.DB, c, "evaluate", "power_redundancy",
		fmt.Sprintf("手动供电冗余评估: evaluated=%v risk=%v notified=%v work_orders=%v",
			stats["evaluated"], stats["risk"], stats["notified"], stats["work_orders"]))
	response.Success(c, stats)
}

// CreateRedundancyWorkOrderAPI POST /power/redundancy/:site_id/work-order 手动开整改工单（ops）
func (h *Handler) CreateRedundancyWorkOrderAPI(c *gin.Context) {
	siteID, ok := parseSiteIDParam(c)
	if !ok {
		return
	}
	var site model.Site
	if err := h.DB.First(&site, siteID).Error; err != nil {
		response.NotFound(c, "站点不存在")
		return
	}
	var rec model.PowerRedundancy
	score := 100
	issues := []powerIssue{}
	if h.DB.Where("site_id = ?", siteID).First(&rec).Error == nil {
		score = rec.Score
		_ = json.Unmarshal([]byte(rec.Issues), &issues)
		if rec.OpenWorkOrderID > 0 {
			var wo model.WorkOrder
			if h.DB.Select("id", "status").First(&wo, rec.OpenWorkOrderID).Error == nil && wo.Status <= 2 {
				response.BadRequest(c, fmt.Sprintf("该站点已有未闭环整改工单 %s", wo.Code))
				return
			}
		}
	}
	woID := h.createRedundancyWorkOrder(&site, score, issues, false)
	if woID == 0 {
		response.BadRequest(c, "已存在未闭环的供电冗余整改工单")
		return
	}
	h.DB.Model(&model.PowerRedundancy{}).Where("site_id = ?", siteID).Update("open_work_order_id", woID)
	go middleware.WriteAudit(h.DB, c, "create", "work_order", fmt.Sprintf("手动创建供电冗余整改工单 site=%d wo=%d", siteID, woID))
	response.Success(c, gin.H{"work_order_id": woID})
}
