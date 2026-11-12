package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ============ V59 应急预案与演练管理 ============
// 闭环: 内置预案种子 → 告警触发匹配推荐(AlertWall/告警详情) → 定期演练要求(drill_cycle_days)
//       → 演练登记留痕(军队/等保检查硬性要求) → 发现问题转整改工单 → 逾期未演练自动扫描通知责任人

// ---------- 种子 ----------

type seedPlan struct {
	Title       string
	Scenario    string
	TriggerCond string
	Steps       string
	Priority    int
	Keywords    string
	PointTypes  string
	Cycle       int
}

var builtinPlans = []seedPlan{
	{
		Title: "市电中断应急处置", Scenario: "power", Priority: 1, Cycle: 90,
		TriggerCond: "市电输入电压异常/停电告警，或电力监测测点显示市电失压",
		Steps: "1. 确认 UPS 已自动切换电池供电，查看剩余后备时间\n2. 确认柴油发电机组状态，按规程启动发电（10 分钟内）\n3. 核实各列头柜输出正常，重点保障核心网络设备与存储\n4. 联系供电部门了解停电原因与预计恢复时间\n5. 市电恢复后确认双路切换正常，机组转热备\n6. 记录全过程时间线，事后 24h 内出报告",
		Keywords: "市电,停电,失压,输入电压", PointTypes: "power_dist,ups",
	},
	{
		Title: "UPS 故障应急处置", Scenario: "ups", Priority: 1, Cycle: 90,
		TriggerCond: "UPS 告警（过载/过温/电池异常/旁路切换）",
		Steps: "1. 确认负载是否已转旁路或电池，评估当前风险等级\n2. 检查 UPS 面板告警代码并记录\n3. 若单模块故障，确认冗余模块承载情况（N+1 是否满足）\n4. 联系维保厂商（维保信息见资产台账），启动应急维修\n5. 必要时按变更流程执行负载转移\n6. 故障排除后确认回切正常并复位告警",
		Keywords: "UPS,不间断电源,电池,旁路", PointTypes: "ups",
	},
	{
		Title: "精密空调故障与温升处置", Scenario: "hvac", Priority: 1, Cycle: 90,
		TriggerCond: "冷通道温度超过告警阈值，或空调机组故障/停机告警",
		Steps: "1. 查看温度场分布，确认热点位置与温升速率\n2. 检查故障空调机组（压缩机/水系统/风机），尝试复位\n3. 启动备用空调或移动应急制冷，优先保障高热密度机柜\n4. 温度超过 35℃ 且持续上升时，按预案有序关停非核心设备\n5. 通知维保厂商紧急到场\n6. 温度回稳后逐步恢复设备并复盘",
		Keywords: "空调,温度,温升,制冷,热点", PointTypes: "temp_humidity,hvac",
	},
	{
		Title: "漏水应急处置", Scenario: "water", Priority: 1, Cycle: 180,
		Steps: "1. 立即定位漏水点（漏水绳告警区域）\n2. 切断附近水源阀门/相关水管段\n3. 检查漏水区域下方与附近机柜、桥架是否进水，必要时切断受影响回路电源\n4. 组织排水与吸水防护，铺设防水布保护设备\n5. 查明原因（管路接头/空调冷凝水/外部渗入）并修复\n6. 确认干燥后恢复供电，绝缘检测合格方可上电",
		TriggerCond: "漏水绳/水浸传感器告警",
		Keywords:  "漏水,水浸,渗水", PointTypes: "water_leak",
	},
	{
		Title: "火情应急处置", Scenario: "fire", Priority: 1, Cycle: 180,
		TriggerCond: "烟感/温感消防告警",
		Steps: "1. 立即现场确认火情（勿盲目启动气体灭火）\n2. 真实火情：按消防规程启动灭火系统，人员撤离并报警 119\n3. 联动切断非消防电源、关闭空调新风（防烟气扩散）\n4. 误报：查明探测器原因并复位，记录误报分析\n5. 灭火后评估设备受损情况，未受损设备按流程恢复\n6. 配合消防部门调查，出事故报告",
		Keywords: "烟感,火警,消防,烟雾", PointTypes: "smoke",
	},
	{
		Title: "核心网络中断应急处置", Scenario: "network", Priority: 2, Cycle: 180,
		TriggerCond: "核心交换机/上联链路中断告警，业务侧大面积失联",
		Steps: "1. 确认中断范围（单设备/单链路/全站）\n2. 检查设备状态灯与 console 日志，判断软硬件故障\n3. 链路故障切换至备用链路/备用设备\n4. 设备故障启用备机或备件更换（备件台账核对库存）\n5. 恢复后验证各业务系统连通性\n6. 记录中断时长与影响范围，沉淀故障案例",
		Keywords: "网络中断,交换机,链路,失联", PointTypes: "network",
	},
}

// SeedEmergencyPlans 幂等种子：按 title 逐条补缺（容忍部分种子状态）
func (h *Handler) SeedEmergencyPlans() int {
	n := 0
	for _, sp := range builtinPlans {
		var cnt int64
		h.DB.Model(&model.EmergencyPlan{}).Where("title = ? AND builtin = ?", sp.Title, true).Count(&cnt)
		if cnt > 0 {
			continue
		}
		p := model.EmergencyPlan{
			Code: h.genCode("EP"), Title: sp.Title, Scenario: sp.Scenario,
			TriggerCond: sp.TriggerCond, Steps: sp.Steps, Priority: sp.Priority,
			MatchKeywords: sp.Keywords, MatchPointTypes: sp.PointTypes,
			DrillCycleDays: sp.Cycle, Status: 1, Builtin: true,
		}
		if h.DB.Create(&p).Error == nil {
			n++
		}
	}
	return n
}

// v59UserNames 用户 ID → 显示名映射
func (h *Handler) v59UserNames() map[uint]string {
	var users []model.User
	h.DB.Select("id, display_name, username").Find(&users)
	m := make(map[uint]string, len(users))
	for _, u := range users {
		if u.DisplayName != "" {
			m[u.ID] = u.DisplayName
		} else {
			m[u.ID] = u.Username
		}
	}
	return m
}

// ---------- 演练状态计算 ----------

// drillState 返回 (状态, 距下次应演练天数)：ok 正常 / due_soon 14天内到期 / overdue 已逾期 / none 无周期要求
func drillState(p *model.EmergencyPlan) (string, int) {
	if p.DrillCycleDays <= 0 || p.Status != 1 {
		return "none", 0
	}
	base := p.CreatedAt
	if p.LastDrillAt != nil {
		base = *p.LastDrillAt
	}
	next := base.AddDate(0, 0, p.DrillCycleDays)
	daysLeft := int(time.Until(next).Hours() / 24)
	if daysLeft < 0 {
		return "overdue", daysLeft
	}
	if daysLeft <= 14 {
		return "due_soon", daysLeft
	}
	return "ok", daysLeft
}

// ---------- 列表 / 详情 ----------

// ListEmergencyPlans GET /emergency-plans?site_id=&scenario=&status=&overdue=1&keyword=
func (h *Handler) ListEmergencyPlans(c *gin.Context) {
	q := h.DB.Model(&model.EmergencyPlan{})
	if v := c.Query("site_id"); v != "" && v != "0" {
		q = q.Where("site_id = ? OR site_id = 0", v)
	}
	if v := c.Query("scenario"); v != "" {
		q = q.Where("scenario = ?", v)
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("keyword"); v != "" {
		like := "%" + v + "%"
		q = q.Where("title LIKE ? OR match_keywords LIKE ?", like, like)
	}
	var plans []model.EmergencyPlan
	q.Order("priority ASC, id ASC").Find(&plans)

	userNames := h.v59UserNames()
	type row struct {
		model.EmergencyPlan
		OwnerName  string `json:"owner_name"`
		DrillState string `json:"drill_state"`
		DaysLeft   int    `json:"days_left"`
	}
	out := make([]row, 0, len(plans))
	overdueCnt, dueCnt := 0, 0
	for i := range plans {
		st, dl := drillState(&plans[i])
		if st == "overdue" {
			overdueCnt++
		}
		if st == "due_soon" {
			dueCnt++
		}
		out = append(out, row{EmergencyPlan: plans[i], OwnerName: userNames[plans[i].OwnerID], DrillState: st, DaysLeft: dl})
	}
	if c.Query("overdue") == "1" {
		filtered := out[:0]
		for _, r := range out {
			if r.DrillState == "overdue" || r.DrillState == "due_soon" {
				filtered = append(filtered, r)
			}
		}
		out = filtered
	}
	response.Success(c, gin.H{
		"list": out, "total": len(out),
		"summary": gin.H{"total": len(plans), "overdue": overdueCnt, "due_soon": dueCnt,
			"enabled": func() int { n := 0; for _, p := range plans { if p.Status == 1 { n++ } }; return n }()},
	})
}

// GetEmergencyPlan GET /emergency-plans/:id 详情+演练历史
func (h *Handler) GetEmergencyPlan(c *gin.Context) {
	var p model.EmergencyPlan
	if err := h.DB.First(&p, c.Param("id")).Error; err != nil {
		response.NotFound(c, "预案不存在")
		return
	}
	var drills []model.DrillRecord
	h.DB.Where("plan_id = ?", p.ID).Order("drill_date DESC").Limit(50).Find(&drills)
	userNames := h.v59UserNames()
	st, dl := drillState(&p)
	siteName := ""
	if p.SiteID > 0 {
		var s model.Site
		if h.DB.Select("name").First(&s, p.SiteID).Error == nil {
			siteName = s.Name
		}
	} else {
		siteName = "全站通用"
	}
	response.Success(c, gin.H{
		"plan": p, "site_name": siteName, "owner_name": userNames[p.OwnerID],
		"drill_state": st, "days_left": dl, "drills": drills,
	})
}

// ---------- 写操作 ----------

// CreateEmergencyPlan POST /emergency-plans（ops+）
func (h *Handler) CreateEmergencyPlan(c *gin.Context) {
	var p model.EmergencyPlan
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Scenario) == "" {
		response.BadRequest(c, "标题与场景必填")
		return
	}
	if p.Priority < 1 || p.Priority > 3 {
		p.Priority = 2
	}
	p.Code = h.genCode("EP")
	p.Builtin = false
	p.Status = 1
	p.DrillCount = 0
	p.LastDrillAt = nil
	uid := middleware.GetUserID(c)
	if p.OwnerID == 0 {
		p.OwnerID = uid
	}
	if err := h.DB.Create(&p).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	go middleware.WriteAudit(h.DB, c, "create", "emergency_plan", p.Code+" "+p.Title)
	response.Success(c, p)
}

// UpdateEmergencyPlan PUT /emergency-plans/:id（admin）局部更新用键探测保留未传字段
func (h *Handler) UpdateEmergencyPlan(c *gin.Context) {
	var p model.EmergencyPlan
	if err := h.DB.First(&p, c.Param("id")).Error; err != nil {
		response.NotFound(c, "预案不存在")
		return
	}
	raw, _ := io.ReadAll(c.Request.Body)
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	updates := map[string]interface{}{}
	strFields := map[string]string{"title": "title", "scenario": "scenario", "trigger_cond": "trigger_cond",
		"steps": "steps", "match_keywords": "match_keywords", "match_point_types": "match_point_types"}
	for jk, col := range strFields {
		if v, ok := m[jk]; ok {
			updates[col] = fmt.Sprint(v)
		}
	}
	intFields := map[string]string{"site_id": "site_id", "owner_id": "owner_id", "priority": "priority",
		"drill_cycle_days": "drill_cycle_days", "status": "status"}
	for jk, col := range intFields {
		if v, ok := m[jk]; ok {
			switch n := v.(type) {
			case float64:
				updates[col] = int(n)
			case string:
				if iv, err := strconv.Atoi(n); err == nil {
					updates[col] = iv
				}
			}
		}
	}
	if len(updates) == 0 {
		response.BadRequest(c, "没有可更新字段")
		return
	}
	if err := h.DB.Model(&p).Updates(updates).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	go middleware.WriteAudit(h.DB, c, "update", "emergency_plan", p.Code)
	response.Success(c, p)
}

// DeleteEmergencyPlan DELETE /emergency-plans/:id（admin）
// 保护：内置预案与有演练记录的预案不可删（只能停用）——演练留痕是检查要求，不许销毁
func (h *Handler) DeleteEmergencyPlan(c *gin.Context) {
	var p model.EmergencyPlan
	if err := h.DB.First(&p, c.Param("id")).Error; err != nil {
		response.NotFound(c, "预案不存在")
		return
	}
	if p.Builtin {
		response.BadRequest(c, "内置预案不可删除，可将其停用")
		return
	}
	var drillCnt int64
	h.DB.Model(&model.DrillRecord{}).Where("plan_id = ?", p.ID).Count(&drillCnt)
	if drillCnt > 0 {
		response.BadRequest(c, fmt.Sprintf("该预案已有 %d 条演练记录（留痕不可销毁），请改为停用", drillCnt))
		return
	}
	h.DB.Delete(&p)
	go middleware.WriteAudit(h.DB, c, "delete", "emergency_plan", p.Code+" "+p.Title)
	response.Success(c, gin.H{"deleted": p.ID})
}

// ---------- 告警匹配推荐 ----------

// MatchEmergencyPlans GET /emergency-plans/match?alert_id=&point_type=&text=&site_id=
// 复用 RecommendFaultCases 的入参模式；打分 = 测点类型命中×3 + 关键词命中×2 + 站点匹配×1
func (h *Handler) MatchEmergencyPlans(c *gin.Context) {
	var pointType, text string
	var siteID uint
	if v := c.Query("alert_id"); v != "" && v != "0" {
		var al model.Alert
		if h.DB.First(&al, v).Error == nil {
			text = al.Message
			siteID = al.SiteID
			if al.PointID != nil {
				var p model.MonitoringPoint
				if h.DB.Select("type").First(&p, *al.PointID).Error == nil {
					pointType = p.Type
				}
			}
		}
	}
	if v := c.Query("point_type"); v != "" {
		pointType = v
	}
	if v := c.Query("text"); v != "" {
		text = v
	}
	if v := c.Query("site_id"); v != "" && v != "0" {
		if n, err := strconv.Atoi(v); err == nil {
			siteID = uint(n)
		}
	}
	var plans []model.EmergencyPlan
	h.DB.Where("status = 1").Order("priority ASC, id ASC").Find(&plans)
	type scored struct {
		model.EmergencyPlan
		Score  int      `json:"score"`
		Hits   []string `json:"hits"`
		Reason string   `json:"reason"`
	}
	var out []scored
	for _, p := range plans {
		sc := 0
		var hits []string
		if pointType != "" {
			for _, t := range strings.Split(p.MatchPointTypes, ",") {
				if strings.TrimSpace(t) == pointType {
					sc += 3
					hits = append(hits, "测点类型 "+pointType)
					break
				}
			}
		}
		if text != "" {
			for _, kw := range strings.Split(p.MatchKeywords, ",") {
				kw = strings.TrimSpace(kw)
				if kw != "" && strings.Contains(text, kw) {
					sc += 2
					hits = append(hits, "关键词 "+kw)
				}
			}
		}
		if siteID > 0 && p.SiteID == siteID {
			sc++
			hits = append(hits, "站点专属")
		}
		if sc > 0 {
			out = append(out, scored{EmergencyPlan: p, Score: sc, Hits: hits, Reason: strings.Join(hits, "、")})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Priority < out[j].Priority
	})
	if len(out) > 5 {
		out = out[:5]
	}
	response.Success(c, gin.H{"list": out, "total": len(out)})
}

// ---------- 演练登记 ----------

// CreateDrillRecord POST /emergency-plans/:id/drill 登记演练 → 更新预案最近演练
func (h *Handler) CreateDrillRecord(c *gin.Context) {
	var plan model.EmergencyPlan
	if err := h.DB.First(&plan, c.Param("id")).Error; err != nil {
		response.NotFound(c, "预案不存在")
		return
	}
	var req struct {
		Title        string `json:"title"`
		DrillDate    string `json:"drill_date"` // yyyy-MM-dd，空=今天
		Participants string `json:"participants"`
		DurationMin  int    `json:"duration_min"`
		Result       string `json:"result"` // success/partial/failed
		Issues       string `json:"issues"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}
	if req.Result != "success" && req.Result != "partial" && req.Result != "failed" {
		response.BadRequest(c, "演练结果须为 success/partial/failed")
		return
	}
	drillDate := time.Now()
	if req.DrillDate != "" {
		if t, err := time.ParseInLocation("2006-01-02", req.DrillDate, time.Local); err == nil {
			drillDate = t.Add(10*time.Hour + 30*time.Minute)
		}
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = plan.Title + " 演练"
	}
	rec := model.DrillRecord{
		PlanID: plan.ID, SiteID: plan.SiteID, Title: title, DrillDate: drillDate,
		Participants: req.Participants, DurationMin: req.DurationMin,
		Result: req.Result, Issues: req.Issues, RecorderID: middleware.GetUserID(c),
	}
	if err := h.DB.Create(&rec).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "登记失败: "+err.Error())
		return
	}
	// 演练日期晚于当前记录才前移 LastDrillAt（支持补录历史演练）
	if plan.LastDrillAt == nil || drillDate.After(*plan.LastDrillAt) {
		h.DB.Model(&plan).Update("last_drill_at", drillDate)
	}
	h.DB.Model(&model.EmergencyPlan{}).Where("id = ?", plan.ID).
		UpdateColumn("drill_count", gorm.Expr("COALESCE(drill_count,0)+1"))
	go middleware.WriteAudit(h.DB, c, "create", "drill_record", fmt.Sprintf("%s %s result=%s", plan.Code, title, req.Result))
	response.Success(c, rec)
}

// DrillWorkOrder POST /drill-records/:id/work-order 演练问题 → 整改工单（幂等：已有未关工单不重复开）
func (h *Handler) DrillWorkOrder(c *gin.Context) {
	var rec model.DrillRecord
	if err := h.DB.First(&rec, c.Param("id")).Error; err != nil {
		response.NotFound(c, "演练记录不存在")
		return
	}
	if strings.TrimSpace(rec.Issues) == "" {
		response.BadRequest(c, "该演练记录没有问题描述，无需整改工单")
		return
	}
	if rec.WoID > 0 {
		var cnt int64
		h.DB.Model(&model.WorkOrder{}).Where("id = ?", rec.WoID).Count(&cnt)
		if cnt > 0 {
			response.Success(c, gin.H{"work_order_id": rec.WoID, "reused": true})
			return
		}
	}
	var plan model.EmergencyPlan
	h.DB.First(&plan, rec.PlanID)
	var openCnt int64
	h.DB.Model(&model.WorkOrder{}).Where("title LIKE ? AND status <= 2", "【演练整改】%"+plan.Title+"%").Count(&openCnt)
	if openCnt > 0 {
		response.BadRequest(c, "该预案已有未关闭的整改工单")
		return
	}
	desc := fmt.Sprintf("预案：%s（%s）\n演练：%s（%s）\n演练结果：%s\n\n发现问题：\n%s\n\n本工单由演练记录一键生成，请整改后回填说明。",
		plan.Title, plan.Code, rec.Title, rec.DrillDate.Format("2006-01-02"), drillResultLabel(rec.Result), rec.Issues)
	wo := model.WorkOrder{
		SiteID: plan.SiteID, Code: h.genCode("WO"),
		Title:       fmt.Sprintf("【演练整改】%s", plan.Title),
		Type:        "task", Priority: plan.Priority, Status: 1,
		Description: desc,
		RequesterID: middleware.GetUserID(c),
		CreatedAt:   time.Now(),
	}
	if err := h.DB.Create(&wo).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "开工单失败: "+err.Error())
		return
	}
	h.DB.Model(&rec).Update("wo_id", wo.ID)
	go middleware.WriteAudit(h.DB, c, "create", "work_order", "【演练整改】"+plan.Title)
	response.Success(c, gin.H{"work_order_id": wo.ID, "code": wo.Code})
}

func drillResultLabel(r string) string {
	switch r {
	case "success":
		return "成功"
	case "partial":
		return "部分达成"
	case "failed":
		return "未达成"
	}
	return r
}

// ---------- 逾期未演练扫描 ----------

// ScanOverdueDrills 每日扫描：启用且有演练周期的预案，超过周期未演练 → 通知责任人（幂等：同预案 7 天窗口一次）
func (h *Handler) ScanOverdueDrills() int {
	if h.healthSetting("drill_overdue_notify", "1") == "0" {
		return 0
	}
	var plans []model.EmergencyPlan
	h.DB.Where("status = 1 AND drill_cycle_days > 0").Find(&plans)
	notified := 0
	window := time.Now().AddDate(0, 0, -7)
	for i := range plans {
		st, days := drillState(&plans[i])
		if st != "overdue" {
			continue
		}
		// 幂等：7 天内已通知过该预案则跳过
		var dup int64
		h.DB.Model(&model.Notification{}).
			Where("kind = ? AND ref_type = ? AND ref_id = ? AND created_at >= ?",
				"drill_overdue", "emergency_plan", plans[i].ID, window).Count(&dup)
		if dup > 0 {
			continue
		}
		title := fmt.Sprintf("应急预案演练逾期：%s", plans[i].Title)
		content := fmt.Sprintf("预案 %s 已逾期 %d 天未演练（要求周期 %d 天）。请尽快组织演练并在系统登记，演练记录是机房检查的硬性材料。",
			plans[i].Code, -days, plans[i].DrillCycleDays)
		targets := []uint{plans[i].OwnerID}
		if plans[i].OwnerID == 0 {
			targets = []uint{0} // 广播
		}
		for _, uid := range targets {
			h.DB.Create(&model.Notification{
				UserID: uid, Kind: "drill_overdue", Title: title, Content: content,
				RefType: "emergency_plan", RefID: plans[i].ID,
			})
		}
		notified++
	}
	return notified
}

// ScanOverdueDrillsNow POST /emergency-plans/scan-overdue（admin 手动触发逾期检查）
func (h *Handler) ScanOverdueDrillsNow(c *gin.Context) {
	n := h.ScanOverdueDrills()
	response.Success(c, gin.H{"notified": n})
}
