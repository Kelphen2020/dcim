package api

// V53 AI 运维助手：
//   POST /ai/assistant  自然语言问答 —— 实时数据快照 + 意图识别 + 大模型回答（百炼 OpenAI 兼容接口），
//                       大模型不可用时自动降级为规则式本地回答（degraded=true），保证功能闭环不依赖外部服务。
//   POST /ai/report     周期报告生成（week/month，确定性口径复用报表数据，Markdown 输出可下载）。
//   GET  /ai/status     AI 能力状态（前端徽标/提示用）。
// 凭据只走环境变量（DCIM_AI_API_KEY / DCIM_AI_BASE_URL / DCIM_AI_MODEL，回退 CUSTOM_ALIYUN_*），配置文件不落密钥。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// aiSnapshot 行类型（包级命名，buildAISnapshot 与 aiLocalAnswer 共享，避免匿名类型断言 panic）
type aiAlertRow struct {
	ID          uint   `json:"id"`
	Severity    int    `json:"severity"`
	Message     string `json:"message"`
	PointName   string `json:"point_name"`
	TriggeredAt string `json:"triggered_at"`
}

type aiRiskRow struct {
	AssetID   uint   `json:"asset_id"`
	AssetName string `json:"asset_name"`
	Score     int    `json:"score"`
	Grade     string `json:"grade"`
}

type aiDutyRow struct {
	UserName string `json:"user_name"`
	Shift    string `json:"shift"`
}

// ==================== 数据快照 ====================

// buildAISnapshot 汇聚当前运行态核心数据（口径与仪表盘/健康度/能耗页一致），
// 既作为大模型回答的事实依据，也作为降级模式的回答素材。
func (h *Handler) buildAISnapshot(scope []uint) map[string]interface{} {
	cnt := func(m interface{}, where string, args ...interface{}) int64 {
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
	today := time.Now().Format("2006-01-02")

	snap := map[string]interface{}{
		"generated_at": time.Now().Format("2006-01-02 15:04:05"),
		"infra": map[string]interface{}{
			"sites":    cnt(&model.Site{}, ""),
			"rooms":    cnt(&model.Room{}, ""),
			"cabinets": cnt(&model.Cabinet{}, ""),
		},
		"assets": map[string]interface{}{
			"total":    cnt(&model.Asset{}, ""),
			"mounted":  cnt(&model.Asset{}, "status = 1"),
			"repair":   cnt(&model.Asset{}, "status = 3"),
			"scrapped": cnt(&model.Asset{}, "status = 4"),
		},
		"points": map[string]interface{}{
			"total":   cnt(&model.MonitoringPoint{}, ""),
			"normal":  cnt(&model.MonitoringPoint{}, "status = 1"),
			"alert":   cnt(&model.MonitoringPoint{}, "status IN ?", []int{2, 3}),
			"offline": cnt(&model.MonitoringPoint{}, "status = 0"),
		},
		"alerts": map[string]interface{}{
			"active": cnt(&model.Alert{}, "status = 0"),
			"sev3":   cnt(&model.Alert{}, "status = 0 AND severity = 3"),
			"sev2":   cnt(&model.Alert{}, "status = 0 AND severity = 2"),
			"sev1":   cnt(&model.Alert{}, "status = 0 AND severity = 1"),
			"today":  cnt(&model.Alert{}, "created_at::date = ?", today),
		},
		"orders": map[string]interface{}{
			"open":         cnt(&model.WorkOrder{}, "status IN ?", []int{1, 2}),
			"sla_breach":   cnt(&model.WorkOrder{}, "status IN ? AND sla_breach = ?", []int{1, 2}, true),
			"today_create": cnt(&model.WorkOrder{}, "created_at::date = ?", today),
		},
		"changes": map[string]interface{}{
			"pending": cnt(&model.ChangeRequest{}, "status = 2"),
			"running": cnt(&model.ChangeRequest{}, "status = 5"),
		},
	}

	// 活跃告警 TOP6（带测点名）
	var alerts []aiAlertRow
	aq := h.DB.Table("t_alert a").
		Select("a.id, a.severity, a.message, COALESCE(p.name,'') as point_name, to_char(a.triggered_at,'MM-DD HH24:MI') as triggered_at").
		Joins("LEFT JOIN t_monitoring_point p ON p.id = a.point_id").
		Where("a.status = 0").Order("a.severity DESC, a.id DESC").Limit(6)
	if scope != nil {
		aq = aq.Where("a.site_id IN ?", scope)
	}
	aq.Scan(&alerts)
	if alerts == nil {
		alerts = []aiAlertRow{}
	}
	snap["top_alerts"] = alerts

	// 健康度分布 + 风险设备 TOP5
	grades := map[string]int64{"healthy": 0, "attention": 0, "sub_healthy": 0, "risk": 0}
	type gRow struct {
		Grade string
		N     int64
	}
	var grows []gRow
	gq := h.DB.Model(&model.AssetHealth{}).Select("grade, COUNT(*) as n").Group("grade")
	if scope != nil {
		gq = gq.Where("site_id IN ?", scope)
	}
	gq.Scan(&grows)
	for _, g := range grows {
		grades[g.Grade] = g.N
	}
	var risks []aiRiskRow
	rq := h.DB.Table("t_asset_health hh").
		Select("hh.asset_id, COALESCE(a.name,'') as asset_name, hh.score, hh.grade").
		Joins("LEFT JOIN t_asset a ON a.id = hh.asset_id").
		Where("hh.grade IN ?", []string{"risk", "sub_healthy"}).
		Order("hh.score ASC").Limit(5)
	if scope != nil {
		rq = rq.Where("hh.site_id IN ?", scope)
	}
	rq.Scan(&risks)
	if risks == nil {
		risks = []aiRiskRow{}
	}
	snap["health"] = map[string]interface{}{"grades": grades, "risk_assets": risks}

	// 最新 PUE + 7 天均值
	var pueRow struct {
		Pue        float64 `json:"pue"`
		TotalPower float64 `json:"total_power"`
		ItPower    float64 `json:"it_power"`
	}
	pq := h.DB.Model(&model.PUERecord{}).Select("COALESCE(pue,0) as pue, COALESCE(total_power,0) as total_power, COALESCE(it_power,0) as it_power").Order("recorded_at DESC").Limit(1)
	if scope != nil {
		pq = pq.Where("site_id IN ?", scope)
	}
	pq.Scan(&pueRow)
	var pueAvg float64
	paq := h.DB.Model(&model.PUERecord{}).Select("COALESCE(AVG(pue),0)").Where("recorded_at > ?", time.Now().AddDate(0, 0, -7))
	if scope != nil {
		paq = paq.Where("site_id IN ?", scope)
	}
	paq.Scan(&pueAvg)
	snap["energy"] = map[string]interface{}{
		"latest_pue":     pueRow.Pue,
		"pue_7d_avg":     float64(int(pueAvg*1000)) / 1000,
		"total_power_kw": pueRow.TotalPower,
		"it_power_kw":    pueRow.ItPower,
	}

	// 维保 30 天内到期
	snap["maintenance"] = map[string]interface{}{
		"expiring_30d": cnt(&model.Maintenance{}, "kind = 'contract' AND status = 1 AND end_at IS NOT NULL AND end_at > NOW() AND end_at < ?", time.Now().AddDate(0, 0, 30)),
	}

	// 今日巡检执行情况
	type inspRow struct {
		PlanID uint
		N      int64
	}
	var donePlans []inspRow
	h.DB.Model(&model.InspectionRecord{}).
		Select("plan_id, COUNT(*) as n").
		Where("created_at::date = ?", today).Group("plan_id").Scan(&donePlans)
	snap["inspection"] = map[string]interface{}{
		"today_records":   cnt(&model.InspectionRecord{}, "created_at::date = ?", today),
		"today_plans_hit": len(donePlans),
	}

	// 活跃告警风暴
	snap["storms"] = map[string]interface{}{
		"active": cnt(&model.AlertStorm{}, "status = 0"),
	}

	// 今日值班
	var duties []aiDutyRow
	h.DB.Table("t_duty_shift d").
		Select("COALESCE(u.username,'') as user_name, d.shift").
		Joins("LEFT JOIN t_user u ON u.id = d.user_id").
		Where("d.duty_date::date = ?", today).Limit(5).Scan(&duties)
	if duties == nil {
		duties = []aiDutyRow{}
	}
	snap["duty_today"] = duties

	return snap
}

// ==================== 意图识别 + 深链 ====================

type aiIntent struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

var aiIntentRules = []struct {
	Key      string
	Label    string
	URL      string
	Keywords []string
}{
	{"alert", "告警盯屏", "/alert-wall", []string{"告警", "报警", "警报", "异常", "故障"}},
	{"monitor", "动环监控", "/monitor", []string{"测点", "监控", "温度", "湿度", "漏水", "漏液", "烟感", "液冷", "cdu"}},
	{"health", "风险设备", "/assets?health_grade=risk", []string{"健康", "风险", "劣化", "预测", "老化"}},
	{"asset", "资产台账", "/assets", []string{"资产", "设备台账", "服务器", "在架"}},
	{"order", "运维工单", "/work-orders", []string{"工单", "派单", "sla", "超时"}},
	{"energy", "能耗总览", "/energy", []string{"能耗", "pue", "电费", "用电", "功率", "碳"}},
	{"capacity", "机柜容量", "/capacity", []string{"容量", "机柜", "u位", "上架", "承重"}},
	{"inspect", "巡检管理", "/inspection", []string{"巡检", "检查项", "签到"}},
	{"change", "变更管理", "/changes", []string{"变更", "审批", "割接"}},
	{"maint", "维保管理", "/maintenances", []string{"维保", "保修", "续保", "脱保"}},
	{"duty", "值班与交接", "/duty-roster", []string{"值班", "交接", "排班", "当班"}},
	{"storm", "告警风暴", "/alert-wall", []string{"风暴"}},
	{"report", "报表中心", "/reports", []string{"报表", "报告", "周报", "月报", "统计"}},
	{"screen", "可视化大屏", "/big-screen", []string{"大屏", "驾驶舱", "投屏"}},
	{"room3d", "3D机房", "/room-3d", []string{"3d", "机房可视化", "三维"}},
	{"power", "供电链路", "/power-chain", []string{"供电", "ups", "配电", "市电", "停电"}},
	{"network", "网络拓扑", "/network-topology", []string{"网络", "交换机", "拓扑", "链路", "vlan", "ip"}},
	{"spare", "备件台账", "/spare-parts", []string{"备件", "库存", "领用"}},
}

func aiDetectIntents(q string) []aiIntent {
	lq := strings.ToLower(q)
	var hits []aiIntent
	for _, r := range aiIntentRules {
		for _, kw := range r.Keywords {
			if strings.Contains(lq, kw) {
				hits = append(hits, aiIntent{Key: r.Key, Label: r.Label, URL: r.URL})
				break
			}
		}
	}
	if len(hits) == 0 {
		hits = append(hits, aiIntent{Key: "dashboard", Label: "运行总览", URL: "/dashboard"})
	}
	if len(hits) > 4 {
		hits = hits[:4]
	}
	return hits
}

// ==================== 本地降级回答 ====================

func aiLocalAnswer(intents []aiIntent, snap map[string]interface{}) string {
	alerts := snap["alerts"].(map[string]interface{})
	points := snap["points"].(map[string]interface{})
	orders := snap["orders"].(map[string]interface{})
	energy := snap["energy"].(map[string]interface{})
	health := snap["health"].(map[string]interface{})
	grades := health["grades"].(map[string]int64)
	risks := health["risk_assets"].([]aiRiskRow)
	top := snap["top_alerts"].([]aiAlertRow)

	gradeName := map[string]string{"healthy": "健康", "attention": "关注", "sub_healthy": "亚健康", "risk": "风险"}
	sevName := map[int]string{3: "严重", 2: "警告", 1: "提示"}
	var b strings.Builder

	for _, it := range intents {
		switch it.Key {
		case "alert", "storm":
			b.WriteString(fmt.Sprintf("当前活跃告警 %d 条（严重 %d / 警告 %d / 提示 %d），今日新增 %d 条。",
				alerts["active"], alerts["sev3"], alerts["sev2"], alerts["sev1"], alerts["today"]))
			if len(top) > 0 {
				b.WriteString(" 最需关注：")
				for i, a := range top {
					if i >= 3 {
						break
					}
					b.WriteString(fmt.Sprintf("%d)[%s]%s", i+1, sevName[a.Severity], a.Message))
					if a.PointName != "" {
						b.WriteString(fmt.Sprintf("（测点：%s，%s）", a.PointName, a.TriggeredAt))
					}
					b.WriteString(" ")
				}
			}
			b.WriteString(fmt.Sprintf(" 活跃告警风暴 %v 起。", snap["storms"].(map[string]interface{})["active"]))
		case "monitor":
			b.WriteString(fmt.Sprintf("测点共 %v 个：正常 %v / 告警 %v / 离线 %v。",
				points["total"], points["normal"], points["alert"], points["offline"]))
		case "health":
			b.WriteString(fmt.Sprintf("设备健康度分布：健康 %d / 关注 %d / 亚健康 %d / 风险 %d。",
				grades["healthy"], grades["attention"], grades["sub_healthy"], grades["risk"]))
			if len(risks) > 0 {
				b.WriteString(" 风险最高设备：")
				for i, r := range risks {
					if i >= 3 {
						break
					}
					b.WriteString(fmt.Sprintf("%s(%d分,%s) ", r.AssetName, r.Score, gradeName[r.Grade]))
				}
			}
		case "order":
			b.WriteString(fmt.Sprintf("进行中工单 %v 张（今日新建 %v），SLA 违约 %v 张。",
				orders["open"], orders["today_create"], orders["sla_breach"]))
		case "energy":
			b.WriteString(fmt.Sprintf("最新 PUE %.3f（近7天均值 %.3f），总功率 %.1f kW，其中 IT %.1f kW。",
				energy["latest_pue"], energy["pue_7d_avg"], energy["total_power_kw"], energy["it_power_kw"]))
		case "maint":
			b.WriteString(fmt.Sprintf("30 天内到期维保合同 %v 份，请及时续签。",
				snap["maintenance"].(map[string]interface{})["expiring_30d"]))
		case "inspect":
			insp := snap["inspection"].(map[string]interface{})
			b.WriteString(fmt.Sprintf("今日巡检记录 %v 条，覆盖计划 %v 个。", insp["today_records"], insp["today_plans_hit"]))
		case "duty":
			duties := snap["duty_today"].([]aiDutyRow)
			if len(duties) == 0 {
				b.WriteString("今日暂无排班记录，请到值班页安排。")
			} else {
				b.WriteString("今日值班：")
				for _, d := range duties {
					b.WriteString(fmt.Sprintf("%s(%s) ", d.UserName, d.Shift))
				}
			}
		case "asset":
			a := snap["assets"].(map[string]interface{})
			b.WriteString(fmt.Sprintf("资产共 %v 台：在架 %v / 维修 %v / 报废 %v。", a["total"], a["mounted"], a["repair"], a["scrapped"]))
		case "change":
			ch := snap["changes"].(map[string]interface{})
			b.WriteString(fmt.Sprintf("待审批变更 %v 项，执行中 %v 项。", ch["pending"], ch["running"]))
		case "report", "screen", "room3d", "capacity", "power", "network", "spare":
			b.WriteString(fmt.Sprintf("已为你定位到「%s」，点击下方链接查看明细数据。", it.Label))
		default:
			in := snap["infra"].(map[string]interface{})
			b.WriteString(fmt.Sprintf("系统运行快照：%v 站点 / %v 房间 / %v 机柜；活跃告警 %v 条，进行中工单 %v 张，最新 PUE %.3f。",
				in["sites"], in["rooms"], in["cabinets"], alerts["active"], orders["open"], energy["latest_pue"]))
		}
		b.WriteString("\n\n")
	}
	b.WriteString("（本地模式回答：AI 大模型暂不可用，以上数据直接取自平台实时统计，口径与各功能页一致。）")
	return b.String()
}

// ==================== LLM 调用 ====================

func (h *Handler) aiCallLLM(question string, snapText string, links []aiIntent) (string, error) {
	aiKey, aiBase, aiModel := h.getAI() // V58: 锁保护读取（DB 配置可热更新）
	if aiKey == "" || aiBase == "" {
		return "", fmt.Errorf("AI 未配置")
	}
	var linkLines []string
	for _, l := range links {
		linkLines = append(linkLines, fmt.Sprintf("[%s](%s)", l.Label, l.URL))
	}
	sys := "你是数据中心基础设施管理(DCIM)平台的AI运维助手。用户问题与平台实时数据快照(JSON)一起给出。" +
		"要求：1)只依据数据快照回答，禁止编造数字；2)用简体中文，简洁分点，突出需要人工处理的风险项；" +
		"3)在回答末尾附上最相关的功能页链接(只能从给定链接表中选，Markdown格式)；" +
		"4)数据快照为空或字段缺失时如实说明。可用链接表：" + strings.Join(linkLines, " ")
	body, _ := json.Marshal(map[string]interface{}{
		"model":       aiModel,
		"temperature": 0.3,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": "问题：" + question + "\n\n实时数据快照：\n" + snapText},
		},
	})
	req, err := http.NewRequest("POST", strings.TrimRight(aiBase, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bear"+"er "+aiKey)
	cli := &http.Client{Timeout: 25 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("LLM HTTP %d: %s", resp.StatusCode, string(raw[:min(len(raw), 200)]))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("LLM 返回空内容")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ==================== HTTP Handlers ====================

// AIStatus GET /ai/status
func (h *Handler) AIStatus(c *gin.Context) {
	aiKey, aiBase, aiModel := h.getAI() // V58
	response.Success(c, gin.H{
		"enabled":  aiKey != "" && aiBase != "",
		"has_key":  aiKey != "",
		"model":    aiModel,
		"base_set": aiBase != "",
	})
}

// AIAssistant POST /ai/assistant {question, force_local}
func (h *Handler) AIAssistant(c *gin.Context) {
	var req struct {
		Question   string `json:"question"`
		ForceLocal bool   `json:"force_local"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Question) == "" {
		response.BadRequest(c, "请输入问题")
		return
	}
	q := strings.TrimSpace(req.Question)
	if len([]rune(q)) > 500 {
		response.BadRequest(c, "问题过长（最多500字）")
		return
	}
	scope := middleware.GetSiteScope(c)
	snap := h.buildAISnapshot(scope)
	intents := aiDetectIntents(q)
	snapJSON, _ := json.Marshal(snap)

	degraded := false
	reason := ""
	answer := ""
	aiKey0, aiBase0, aiModel0 := h.getAI() // V58
	if req.ForceLocal || aiKey0 == "" || aiBase0 == "" {
		degraded = true
		if req.ForceLocal {
			reason = "用户指定本地模式"
		} else {
			reason = "AI 大模型未配置（缺少密钥或接口地址）"
		}
		answer = aiLocalAnswer(intents, snap)
	} else {
		ans, err := h.aiCallLLM(q, string(snapJSON), intents)
		if err != nil {
			degraded = true
			reason = "AI 服务调用失败，已切换本地模式：" + err.Error()
			answer = aiLocalAnswer(intents, snap)
		} else {
			answer = ans
		}
	}

	go middleware.WriteAudit(h.DB, c, "query", "ai_assistant", "问题: "+q)
	response.Success(c, gin.H{
		"answer":   answer,
		"links":    intents,
		"degraded": degraded,
		"reason":   reason,
		"model":    aiModel0,
		"snapshot": snap,
	})
}

// AIReport POST /ai/report {period: week|month}
// 确定性口径生成 Markdown 运行报告（不依赖大模型，永远可用）
func (h *Handler) AIReport(c *gin.Context) {
	var req struct {
		Period string `json:"period"`
	}
	_ = c.ShouldBindJSON(&req)
	days := 7
	if req.Period == "month" {
		days = 30
	} else {
		req.Period = "week"
	}
	scope := middleware.GetSiteScope(c)
	since := time.Now().AddDate(0, 0, -days)
	cnt := func(m interface{}, where string, args ...interface{}) int64 {
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

	newAlerts := cnt(&model.Alert{}, "created_at > ?", since)
	resolved := cnt(&model.Alert{}, "resolved_at IS NOT NULL AND resolved_at > ?", since)
	var sev3 int64
	s3q := h.DB.Model(&model.Alert{}).Where("created_at > ? AND severity = 3", since)
	if scope != nil {
		s3q = s3q.Where("site_id IN ?", scope)
	}
	s3q.Count(&sev3)
	woCreated := cnt(&model.WorkOrder{}, "created_at > ?", since)
	var woDone int64
	wdq := h.DB.Model(&model.WorkOrder{}).Where("finished_at IS NOT NULL AND finished_at IS NOT NULL AND finished_at > ?", since)
	if scope != nil {
		wdq = wdq.Where("site_id IN ?", scope)
	}
	// finished_at 可能不存在，用 updated_at+status 兜底
	if h.DB.Migrator().HasColumn(&model.WorkOrder{}, "finished_at") {
		wdq.Count(&woDone)
	} else {
		woDone = cnt(&model.WorkOrder{}, "status >= 3 AND updated_at > ?", since)
	}
	inspDone := cnt(&model.InspectionRecord{}, "created_at > ?", since)
	changesDone := cnt(&model.ChangeRequest{}, "status = 6 AND updated_at > ?", since)

	var pueAvg, pueMax float64
	pq := h.DB.Model(&model.PUERecord{}).Select("COALESCE(AVG(pue),0), COALESCE(MAX(pue),0)").Where("recorded_at > ?", since)
	if scope != nil {
		pq = pq.Where("site_id IN ?", scope)
	}
	type pRow struct {
		Avg float64
		Max float64
	}
	var pr pRow
	pq.Scan(&pr)
	pueAvg, pueMax = pr.Avg, pr.Max

	// 能耗积分近似（口径同能耗页：行数×均值）
	var eRow struct {
		N   int64
		Avg float64
	}
	eq := h.DB.Model(&model.PUERecord{}).Select("COUNT(*) as n, COALESCE(AVG(total_power),0) as avg").Where("recorded_at > ?", since)
	if scope != nil {
		eq = eq.Where("site_id IN ?", scope)
	}
	eq.Scan(&eRow)
	kwh := float64(eRow.N) * eRow.Avg * 0.5 // 采样按 30min 一条近似

	periodName := "周"
	if days == 30 {
		periodName = "月"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# DCIM 运行%s报（%s ~ %s）\n\n", periodName,
		since.Format("2006-01-02"), time.Now().Format("2006-01-02 15:04")))
	b.WriteString("## 一、告警与事件\n")
	b.WriteString(fmt.Sprintf("- 新增告警 %d 条，其中严重级 %d 条；已恢复 %d 条\n", newAlerts, sev3, resolved))
	b.WriteString(fmt.Sprintf("- 活跃告警风暴 %d 起（当前）\n\n", cnt(&model.AlertStorm{}, "status = 0")))
	b.WriteString("## 二、运维工单\n")
	b.WriteString(fmt.Sprintf("- 新建工单 %d 张，完成 %d 张；当前进行中 %d 张，SLA 违约 %d 张\n\n",
		woCreated, woDone, cnt(&model.WorkOrder{}, "status IN ?", []int{1, 2}), cnt(&model.WorkOrder{}, "status IN ? AND sla_breach = ?", []int{1, 2}, true)))
	b.WriteString("## 三、巡检与变更\n")
	b.WriteString(fmt.Sprintf("- 巡检执行 %d 次；完成变更 %d 项，待审批 %d 项\n\n", inspDone, changesDone, cnt(&model.ChangeRequest{}, "status = 2")))
	b.WriteString("## 四、能效\n")
	b.WriteString(fmt.Sprintf("- 期间平均 PUE %.3f，峰值 %.3f；估算总用电 %.0f kWh，碳排放约 %.0f kgCO₂\n\n",
		pueAvg, pueMax, kwh, kwh*0.5703))
	b.WriteString("## 五、设备健康\n")
	type hgRow struct {
		Grade string
		N     int64
	}
	var hgs []hgRow
	hgq := h.DB.Model(&model.AssetHealth{}).Select("grade, COUNT(*) as n").Group("grade")
	if scope != nil {
		hgq = hgq.Where("site_id IN ?", scope)
	}
	hgq.Scan(&hgs)
	gname := map[string]string{"healthy": "健康", "attention": "关注", "sub_healthy": "亚健康", "risk": "风险"}
	b.WriteString("- 健康度分布：")
	for _, g := range hgs {
		b.WriteString(fmt.Sprintf("%s %d 台；", gname[g.Grade], g.N))
	}
	b.WriteString(fmt.Sprintf("\n- 30 天内到期维保合同 %d 份\n\n", cnt(&model.Maintenance{}, "kind = 'contract' AND status = 1 AND end_at IS NOT NULL AND end_at > NOW() AND end_at < ?", time.Now().AddDate(0, 0, 30))))
	b.WriteString("## 六、风险与建议\n")
	riskN := cnt(&model.AssetHealth{}, "grade IN ?", []string{"risk", "sub_healthy"})
	if riskN > 0 {
		b.WriteString(fmt.Sprintf("- %d 台设备处于亚健康/风险等级，建议在资产台账按健康度筛选并安排预测性维护工单\n", riskN))
	}
	if sev3 > 0 {
		b.WriteString(fmt.Sprintf("- 期间出现 %d 条严重告警，建议在告警盯屏复盘处置过程并沉淀故障案例\n", sev3))
	}
	b.WriteString("- 数据口径与平台报表中心一致，详情可在对应功能页穿透查询\n")

	go middleware.WriteAudit(h.DB, c, "query", "ai_report", "period="+req.Period)
	response.Success(c, gin.H{
		"period":   req.Period,
		"markdown": b.String(),
		"filename": fmt.Sprintf("DCIM运行%s报-%s.md", periodName, time.Now().Format("20060102")),
	})
}
