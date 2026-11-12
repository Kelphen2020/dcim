package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V18-1 SLA 工单时效管理 ====================
// 按优先级配置响应/解决时限（小时），存 t_system_setting：
//   sla_respond_hours = "1,2,4,8"   （下标0=优先级1低 ... 下标3=优先级4紧急）
//   sla_resolve_hours = "24,48,72,96"
// 创建工单时计算 sla_respond_due / sla_resolve_due；
// 每分钟扫描：临近80%时限预警（一次），超时限违约标记（一次）+ 通知 + Webhook 外发。

func (h *Handler) slaHours(key string, fallback []float64) []float64 {
	var s model.SystemSetting
	if err := h.DB.First(&s, "key = ?", key).Error; err != nil || strings.TrimSpace(s.Value) == "" {
		return fallback
	}
	parts := strings.Split(s.Value, ",")
	out := make([]float64, 4)
	copy(out, fallback)
	for i := 0; i < 4 && i < len(parts); i++ {
		if v, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64); err == nil && v > 0 {
			out[i] = v
		}
	}
	return out
}

var (
	slaRespondFallback = []float64{8, 4, 2, 1}   // 低→紧急 响应时限(小时)
	slaResolveFallback = []float64{96, 72, 48, 24} // 低→紧急 解决时限(小时)
)

// applySLA 为新建/改优先级的工单计算时限
func (h *Handler) applySLA(wo *model.WorkOrder) {
	respond := h.slaHours("sla_respond_hours", slaRespondFallback)
	resolve := h.slaHours("sla_resolve_hours", slaResolveFallback)
	idx := wo.Priority - 1
	if idx < 0 || idx > 3 {
		idx = 1
	}
	base := wo.CreatedAt
	if base.IsZero() {
		base = time.Now()
	}
	rd := base.Add(time.Duration(respond[idx] * float64(time.Hour)))
	sd := base.Add(time.Duration(resolve[idx] * float64(time.Hour)))
	wo.SlaRespondDue = &rd
	wo.SlaResolveDue = &sd
}

// ScanSLABreaches 扫描 SLA 预警/违约（幂等：warned/breach 标记只触发一次）
// 返回本轮新增预警数、违约数
func (h *Handler) ScanSLABreaches() (warned, breached int) {
	var orders []model.WorkOrder
	// 只扫未闭环工单（1待受理 2处理中）
	h.DB.Where("status IN (1,2)").Find(&orders)
	now := time.Now()
	for i := range orders {
		wo := &orders[i]
		updates := map[string]interface{}{}
		// 响应时限：仅对「待受理」生效（受理后响应义务完成）
		if wo.Status == 1 && wo.SlaRespondDue != nil {
			due := *wo.SlaRespondDue
			remaining := due.Sub(now)
			total := due.Sub(wo.CreatedAt)
			if remaining <= 0 && !wo.SlaRespondBreach {
				updates["sla_respond_breach"] = true
				wo.SlaRespondBreach = true
				breached++
				msg := fmt.Sprintf("工单 %s「%s」（优先级%s）超过响应时限 %s 仍未受理", wo.Code, wo.Title, priName(wo.Priority), fmtDur(total))
				h.DB.Create(&model.Notification{UserID: 0, Kind: "sla_breach", Title: "[SLA违约] " + wo.Code + " 响应超时", Content: msg, RefType: "work_order", RefID: wo.ID})
				h.DB.Create(&model.WorkOrderLog{OrderID: wo.ID, UserID: 0, Action: "progress", Content: "⚠ SLA 响应超时：" + msg})
				h.DispatchWebhook(model.EvSlaBreach, "SLA响应违约", msg, map[string]interface{}{"wo_id": wo.ID, "wo_code": wo.Code, "priority": wo.Priority, "kind": "respond"})
			} else if remaining > 0 && total > 0 && remaining < time.Duration(float64(total)*0.2) && !wo.SlaRespondWarned && !wo.SlaRespondBreach {
				updates["sla_respond_warned"] = true
				wo.SlaRespondWarned = true
				warned++
				msg := fmt.Sprintf("工单 %s「%s」响应时限剩余不足 %s，请尽快受理", wo.Code, wo.Title, fmtDur(remaining))
				h.DB.Create(&model.Notification{UserID: 0, Kind: "sla_warn", Title: "[SLA预警] " + wo.Code + " 即将响应超时", Content: msg, RefType: "work_order", RefID: wo.ID})
				h.DispatchWebhook(model.EvSlaWarn, "SLA响应预警", msg, map[string]interface{}{"wo_id": wo.ID, "wo_code": wo.Code, "kind": "respond"})
			}
		}
		// 解决时限：待受理+处理中都算
		if wo.SlaResolveDue != nil {
			due := *wo.SlaResolveDue
			remaining := due.Sub(now)
			total := due.Sub(wo.CreatedAt)
			if remaining <= 0 && !wo.SlaResolveBreach {
				updates["sla_resolve_breach"] = true
				wo.SlaResolveBreach = true
				breached++
				msg := fmt.Sprintf("工单 %s「%s」（优先级%s）超过解决时限 %s 仍未完成", wo.Code, wo.Title, priName(wo.Priority), fmtDur(total))
				h.DB.Create(&model.Notification{UserID: 0, Kind: "sla_breach", Title: "[SLA违约] " + wo.Code + " 解决超时", Content: msg, RefType: "work_order", RefID: wo.ID})
				h.DB.Create(&model.WorkOrderLog{OrderID: wo.ID, UserID: 0, Action: "progress", Content: "⚠ SLA 解决超时：" + msg})
				h.DispatchWebhook(model.EvSlaBreach, "SLA解决违约", msg, map[string]interface{}{"wo_id": wo.ID, "wo_code": wo.Code, "priority": wo.Priority, "kind": "resolve"})
			} else if remaining > 0 && total > 0 && remaining < time.Duration(float64(total)*0.2) && !wo.SlaResolveWarned && !wo.SlaResolveBreach {
				updates["sla_resolve_warned"] = true
				wo.SlaResolveWarned = true
				warned++
				msg := fmt.Sprintf("工单 %s「%s」解决时限剩余不足 %s，请关注处理进度", wo.Code, wo.Title, fmtDur(remaining))
				h.DB.Create(&model.Notification{UserID: 0, Kind: "sla_warn", Title: "[SLA预警] " + wo.Code + " 即将解决超时", Content: msg, RefType: "work_order", RefID: wo.ID})
				h.DispatchWebhook(model.EvSlaWarn, "SLA解决预警", msg, map[string]interface{}{"wo_id": wo.ID, "wo_code": wo.Code, "kind": "resolve"})
			}
		}
		if len(updates) > 0 {
			h.DB.Model(&model.WorkOrder{}).Where("id = ?", wo.ID).Updates(updates)
		}
	}
	if warned+breached > 0 {
		log.Printf("SLA 扫描：预警 %d 条，违约 %d 条", warned, breached)
	}
	return
}

// GetSLAConfig 读取 SLA 时限配置（登录可读）
func (h *Handler) GetSLAConfig(c *gin.Context) {
	respond := h.slaHours("sla_respond_hours", slaRespondFallback)
	resolve := h.slaHours("sla_resolve_hours", slaResolveFallback)
	response.Success(c, gin.H{
		"respond_hours": respond, "resolve_hours": resolve,
		"labels": []string{"低", "中", "高", "紧急"},
	})
}

// SetSLAConfig 保存 SLA 时限配置（管理员）body: {respond_hours:[..4], resolve_hours:[..4]}
func (h *Handler) SetSLAConfig(c *gin.Context) {
	var req struct {
		RespondHours []float64 `json:"respond_hours"`
		ResolveHours []float64 `json:"resolve_hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	valid := func(arr []float64) (string, bool) {
		if len(arr) != 4 {
			return "", false
		}
		parts := make([]string, 4)
		for i, v := range arr {
			if v <= 0 || v > 24*365 {
				return "", false
			}
			parts[i] = strconv.FormatFloat(v, 'f', -1, 64)
		}
		return strings.Join(parts, ","), true
	}
	rStr, ok1 := valid(req.RespondHours)
	sStr, ok2 := valid(req.ResolveHours)
	if !ok1 || !ok2 {
		response.BadRequest(c, "时限须为4个正数（按优先级 低/中/高/紧急）")
		return
	}
	for _, kv := range [][2]string{{"sla_respond_hours", rStr}, {"sla_resolve_hours", sStr}} {
		h.DB.Save(&model.SystemSetting{Key: kv[0], Value: kv[1], UpdatedAt: time.Now()})
	}
	middleware.WriteAudit(h.DB, c, "sla_config.update", "respond="+rStr, "resolve="+sStr)
	response.Success(c, gin.H{"respond_hours": req.RespondHours, "resolve_hours": req.ResolveHours})
}

// SLAStats 达标率统计：GET /work-orders/sla-stats?days=30&site_id=
// 口径：期内创建的工单；响应达标 = 已受理且 accepted_at<=respond_due，或未受理但已违约=false 且未超时
//       （简单可审计口径：违约标记即不达标）
func (h *Handler) SLAStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 || days > 365 {
		days = 30
	}
	siteID := c.Query("site_id")
	since := time.Now().AddDate(0, 0, -days)
	q := h.DB.Model(&model.WorkOrder{}).Where("created_at >= ?", since)
	if siteID != "" && siteID != "0" {
		q = q.Where("site_id = ?", siteID)
	}
	var rows []model.WorkOrder
	q.Find(&rows)
	total := len(rows)
	respBreach, resBreach, respOk, resOk, openBreached := 0, 0, 0, 0, 0
	now := time.Now()
	byPri := map[int]map[string]int{} // pri -> {total, resp_breach, resolve_breach}
	for _, w := range rows {
		b := byPri[w.Priority]
		if b == nil {
			b = map[string]int{}
			byPri[w.Priority] = b
		}
		b["total"]++
		if w.SlaRespondBreach {
			respBreach++
			b["resp_breach"]++
		} else {
			respOk++
		}
		if w.SlaResolveBreach {
			resBreach++
			b["resolve_breach"]++
		} else if w.Status >= 3 {
			resOk++ // 已闭环且未违约 = 解决达标
		} else if w.SlaResolveDue != nil && now.After(*w.SlaResolveDue) {
			openBreached++ // 尚未被扫描器标记但已过期（统计口径含实时判断）
		}
	}
	pct := func(ok, all int) float64 {
		if all == 0 {
			return 0
		}
		return mathRound(float64(ok)*100/float64(all), 1)
	}
	priOut := make([]gin.H, 0, 4)
	for p := 1; p <= 4; p++ {
		if b, ok := byPri[p]; ok {
			priOut = append(priOut, gin.H{
				"priority": p, "label": priName(p), "total": b["total"],
				"respond_breach": b["resp_breach"], "resolve_breach": b["resolve_breach"],
				"respond_rate": pct(b["total"]-b["resp_breach"], b["total"]),
			})
		}
	}
	response.Success(c, gin.H{
		"days":               days,
		"total":              total,
		"respond_ok":         respOk,
		"respond_breach":     respBreach,
		"respond_rate":       pct(respOk, total),
		"resolve_ok":         resOk,
		"resolve_breach":     resBreach,
		"open_overdue":       openBreached,
		"resolve_rate":       pct(resOk, total),
		"by_priority":        priOut,
	})
}

// TriggerSLAScan 手动触发 SLA 扫描（管理员，测试/补偿用）
func (h *Handler) TriggerSLAScan(c *gin.Context) {
	warned, breached := h.ScanSLABreaches()
	response.Success(c, gin.H{"warned": warned, "breached": breached})
}

func priName(p int) string {
	return map[int]string{1: "低", 2: "中", 3: "高", 4: "紧急"}[p]
}

func fmtDur(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h >= 24 {
		return fmt.Sprintf("%d天%d小时", h/24, h%24)
	}
	if h > 0 {
		return fmt.Sprintf("%d小时%d分", h, m)
	}
	return fmt.Sprintf("%d分钟", m)
}

func mathRound(v float64, prec int) float64 {
	p := math_Pow10(prec)
	return float64(int64(v*p+0.5)) / p
}

func math_Pow10(n int) float64 {
	r := 1.0
	for i := 0; i < n; i++ {
		r *= 10
	}
	return r
}

// ==================== V18-2 Webhook 外发通知通道 ====================
// 通道 CRUD + 测试发送 + 事件分发。事件发生时按通道 events 匹配（*=全部），
// 按 kind 组装载荷（企业微信/飞书/钉钉 markdown 消息 or 通用 JSON POST）。

type channelReq struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	WebhookURL string `json:"webhook_url"`
	Events     string `json:"events"`
	Enabled    *bool  `json:"enabled"`
	Remark     string `json:"remark"`
}

func (h *Handler) ListNotifyChannels(c *gin.Context) {
	var items []model.NotifyChannel
	h.DB.Order("id DESC").Find(&items)
	if items == nil {
		items = []model.NotifyChannel{}
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) CreateNotifyChannel(c *gin.Context) {
	var req channelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.WebhookURL = strings.TrimSpace(req.WebhookURL)
	if req.Name == "" || req.WebhookURL == "" {
		response.BadRequest(c, "名称与 Webhook 地址必填")
		return
	}
	if !strings.HasPrefix(req.WebhookURL, "http://") && !strings.HasPrefix(req.WebhookURL, "https://") {
		response.BadRequest(c, "Webhook 地址须以 http(s):// 开头")
		return
	}
	if req.Kind == "" {
		req.Kind = "webhook"
	}
	if !validChannelKind(req.Kind) {
		response.BadRequest(c, "通道类型无效")
		return
	}
	if req.Events == "" {
		req.Events = "*"
	}
	ch := model.NotifyChannel{
		Name: req.Name, Kind: req.Kind, WebhookURL: req.WebhookURL,
		Events: req.Events, Enabled: true, Remark: req.Remark,
	}
	if req.Enabled != nil {
		ch.Enabled = *req.Enabled
	}
	if err := h.DB.Create(&ch).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_notify_channel", ch.Name, ch.Kind)
	response.Created(c, ch)
}

func (h *Handler) UpdateNotifyChannel(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var ch model.NotifyChannel
	if err := h.DB.First(&ch, id).Error; err != nil {
		response.NotFound(c, "通道不存在")
		return
	}
	var req channelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Name); v != "" {
		updates["name"] = v
	}
	if v := strings.TrimSpace(req.WebhookURL); v != "" {
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			response.BadRequest(c, "Webhook 地址须以 http(s):// 开头")
			return
		}
		updates["webhook_url"] = v
	}
	if v := strings.TrimSpace(req.Kind); v != "" {
		if !validChannelKind(v) {
			response.BadRequest(c, "通道类型无效")
			return
		}
		updates["kind"] = v
	}
	if v := strings.TrimSpace(req.Events); v != "" {
		updates["events"] = v
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if v := strings.TrimSpace(req.Remark); v != "" {
		updates["remark"] = v
	}
	if len(updates) > 0 {
		h.DB.Model(&ch).Updates(updates)
	}
	middleware.WriteAudit(h.DB, c, "update_notify_channel", ch.Name, fmt.Sprint(id))
	h.DB.First(&ch, id)
	response.Success(c, ch)
}

func (h *Handler) DeleteNotifyChannel(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var ch model.NotifyChannel
	if err := h.DB.First(&ch, id).Error; err != nil {
		response.NotFound(c, "通道不存在")
		return
	}
	h.DB.Delete(&ch)
	middleware.WriteAudit(h.DB, c, "delete_notify_channel", ch.Name, fmt.Sprint(id))
	response.Success(c, gin.H{"deleted": id})
}

func validChannelKind(k string) bool {
	switch k {
	case "wecom", "feishu", "dingtalk", "webhook":
		return true
	}
	return false
}

// TestNotifyChannel 发送测试消息验证连通性
func (h *Handler) TestNotifyChannel(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var ch model.NotifyChannel
	if err := h.DB.First(&ch, id).Error; err != nil {
		response.NotFound(c, "通道不存在")
		return
	}
	err := h.sendToChannel(&ch, "channel_test", "DCIM 连通性测试", "这是一条来自 DCIM 平台的测试消息，收到即代表 Webhook 通道配置正确。", nil)
	if err != nil {
		response.Success(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// DispatchWebhook 事件分发：查所有启用通道，events 匹配的异步发送（带 5s 超时）
func (h *Handler) DispatchWebhook(event, title, content string, extra map[string]interface{}) {
	var channels []model.NotifyChannel
	h.DB.Where("enabled = ?", true).Find(&channels)
	for i := range channels {
		ch := channels[i]
		if !channelMatches(ch.Events, event) {
			continue
		}
		go func(ch model.NotifyChannel) {
			if err := h.sendToChannel(&ch, event, title, content, extra); err != nil {
				log.Printf("Webhook[%s] 发送失败: %v", ch.Name, err)
			}
		}(ch)
	}
}

func channelMatches(events, event string) bool {
	events = strings.TrimSpace(events)
	if events == "" || events == "*" {
		return true
	}
	for _, e := range strings.Split(events, ",") {
		if strings.TrimSpace(e) == event {
			return true
		}
	}
	return false
}

// sendToChannel 按通道类型组装载荷并 POST；更新 last_status/last_sent_at
func (h *Handler) sendToChannel(ch *model.NotifyChannel, event, title, content string, extra map[string]interface{}) error {
	var payload []byte
	switch ch.Kind {
	case "wecom": // 企业微信群机器人 markdown
		payload, _ = json.Marshal(map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": fmt.Sprintf("**%s**\n>%s\n>事件: %s\n>时间: %s", title, content, event, time.Now().Format("2006-01-02 15:04:05")),
			},
		})
	case "feishu": // 飞书群机器人：M3 升级为交互卡片（分级配色+移动端详情跳转按钮）
		var baseS model.SystemSetting
		base := ""
		if err := h.DB.First(&baseS, "key = ?", "mobile_base_url").Error; err == nil {
			base = strings.TrimSpace(baseS.Value)
		}
		if base == "" {
			base = "http://localhost/m"
		}
		template := "blue"
		if strings.Contains(title, "严重") || event == "alarm" || event == "sla_breach" {
			template = "red"
		} else if strings.Contains(title, "警告") || event == "sla_warn" || event == "maintenance_due" {
			template = "orange"
		} else if event == "alarm_resolve" {
			template = "green"
		}
		// 详情深链：extra 键 alert_id / wo_id（与各 DispatchWebhook 调用点一致）
		link := ""
		if v, ok := extra["alert_id"]; ok {
			if id, err := strconv.Atoi(fmt.Sprint(v)); err == nil && id > 0 {
				link = fmt.Sprintf("%s/#/pages/alert/detail?id=%d", base, id)
			}
		} else if v, ok := extra["wo_id"]; ok {
			if id, err := strconv.Atoi(fmt.Sprint(v)); err == nil && id > 0 {
				link = fmt.Sprintf("%s/#/pages/wo/detail?id=%d", base, id)
			}
		}
		elements := []map[string]interface{}{
			{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": content}},
			{"tag": "note", "elements": []map[string]string{{"tag": "plain_text", "content": fmt.Sprintf("事件: %s · %s", event, time.Now().Format("2006-01-02 15:04:05"))}}},
		}
		if link != "" {
			elements = append(elements, map[string]interface{}{
				"tag": "action",
				"actions": []map[string]interface{}{{
					"tag":  "button",
					"text": map[string]string{"tag": "plain_text", "content": "查看详情"},
					"type": "primary",
					"url":  link,
				}},
			})
		}
		payload, _ = json.Marshal(map[string]interface{}{
			"msg_type": "interactive",
			"card": map[string]interface{}{
				"config":   map[string]bool{"wide_screen_mode": true},
				"header":   map[string]interface{}{"template": template, "title": map[string]string{"tag": "plain_text", "content": "【DCIM】" + title}},
				"elements": elements,
			},
		})
	case "dingtalk": // 钉钉群机器人 markdown
		payload, _ = json.Marshal(map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": title,
				"text":  fmt.Sprintf("### %s\n%s\n\n- 事件: %s\n- 时间: %s", title, content, event, time.Now().Format("2006-01-02 15:04:05")),
			},
		})
	default: // 通用 webhook：结构化 JSON
		body := map[string]interface{}{
			"event": event, "title": title, "content": content,
			"source": "dcim", "timestamp": time.Now().Format(time.RFC3339),
		}
		for k, v := range extra {
			body[k] = v
		}
		payload, _ = json.Marshal(body)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(ch.WebhookURL, "application/json", bytes.NewReader(payload))
	now := time.Now()
	status := "ok"
	if err != nil {
		status = "fail:" + truncateStr(err.Error(), 150)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			status = fmt.Sprintf("fail:http_%d", resp.StatusCode)
		}
	}
	h.DB.Model(&model.NotifyChannel{}).Where("id = ?", ch.ID).Updates(map[string]interface{}{
		"last_status": status, "last_sent_at": now,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	return nil
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
