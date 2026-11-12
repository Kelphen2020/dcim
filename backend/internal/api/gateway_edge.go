package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V47: XM-E16 边缘网关接入 ====================

// GatewayAuth 网关身份认证：X-Gateway-Code + X-Gateway-Secret 双头校验（Gateway 表）。
// 用于 push-reading(s)、测点下发、命令拉取、心跳等机器通道，与用户 JWT 完全隔离。
func (h *Handler) GatewayAuth(c *gin.Context) (*model.Gateway, bool) {
	code := c.GetHeader("X-Gateway-Code")
	secret := c.GetHeader("X-Gateway-Secret")
	if code == "" || secret == "" {
		response.Error(c, http.StatusUnauthorized, "缺少网关认证头 X-Gateway-Code/X-Gateway-Secret")
		return nil, false
	}
	var gw model.Gateway
	if err := h.DB.Where("code = ? AND secret = ?", code, secret).First(&gw).Error; err != nil {
		response.Error(c, http.StatusUnauthorized, "网关认证失败：编码或密钥错误")
		return nil, false
	}
	if gw.Status != 1 {
		response.Error(c, http.StatusForbidden, "网关已停用")
		return nil, false
	}
	return &gw, true
}

// GatewayPoints GET /gateway/points （网关认证）
// 下发该网关站点下所有需要边缘采集的测点：modbus(rtu)/di/mqtt/manual。
// dcim-edge 启动与每 5 分钟增量拉取，按 poll_interval 轮询采集。
func (h *Handler) GatewayPoints(c *gin.Context) {
	gw, ok := h.GatewayAuth(c)
	if !ok {
		return
	}
	var points []model.MonitoringPoint
	q := h.DB.Where("site_id = ?", gw.SiteID)
	// 增量：since 之后更新过的（含新建/修改/删除软信号——删除由全量模式兜底）
	if since := c.Query("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			q = q.Where("updated_at > ?", t)
		}
	}
	if err := q.Order("id asc").Limit(2000).Find(&points).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	type pointDTO struct {
		ID             uint   `json:"id"`
		Code           string `json:"code"`
		Name           string `json:"name"`
		Type           string `json:"type"`
		Protocol       string `json:"protocol"`
		DataType       string `json:"data_type"`
		Unit           string `json:"unit"`
		PollInterval   int    `json:"poll_interval"`
		ScaleFactor    float64 `json:"scale_factor"`
		ProtocolConfig string `json:"protocol_config"`
		UpdatedAt      string `json:"updated_at"`
	}
	out := make([]pointDTO, 0, len(points))
	for _, p := range points {
		// 只下发边缘可执行的协议；modbus 仅 rtu（tcp 直连由平台侧采集，不占网关串口）
		if p.Protocol == "snmp" {
			continue
		}
		if p.Protocol == "modbus" {
			var cfg map[string]interface{}
			_ = json.Unmarshal([]byte(p.ProtocolConfig), &cfg)
			if ct, _ := cfg["conn_type"].(string); ct != "" && ct != "rtu" {
				continue
			}
		}
		out = append(out, pointDTO{p.ID, p.Code, p.Name, p.Type, p.Protocol, p.DataType, p.Unit,
			p.PollInterval, p.ScaleFactor, p.ProtocolConfig, p.UpdatedAt.Format(time.RFC3339)})
	}
	response.Success(c, gin.H{"points": out, "server_time": time.Now().Format(time.RFC3339)})
}

// GatewayHeartbeat2 POST /gateway/heartbeat2 （网关认证走 header，body 带运行指标）
// 相比旧 heartbeat（body 内 code/secret），新通道用统一 header 认证，并回传 DI/DO 位图与采集统计。
func (h *Handler) GatewayHeartbeat2(c *gin.Context) {
	gw, ok := h.GatewayAuth(c)
	if !ok {
		return
	}
	var body struct {
		Version    string `json:"version"`
		DiState    *int   `json:"di_state"`  // /dev/di 位图（-1=不可用）
		DoState    *int   `json:"do_state"`  // /dev/do 位图
		EdgePoints int    `json:"edge_points"`
		PushFail   int    `json:"push_fail"`   // 缓存队列积压数
		UptimeSec  int64  `json:"uptime_sec"`
		Serials    string `json:"serials"`     // 串口占用摘要，如 "com1,com3"
	}
	_ = c.ShouldBindJSON(&body)
	now := time.Now()
	updates := map[string]interface{}{"online": true, "last_seen_at": now, "ip": c.ClientIP()}
	if body.Version != "" {
		updates["version"] = body.Version
	}
	extra := map[string]interface{}{}
	if body.DiState != nil {
		extra["di_state"] = *body.DiState
	}
	if body.DoState != nil {
		extra["do_state"] = *body.DoState
	}
	extra["edge_points"] = body.EdgePoints
	extra["push_fail"] = body.PushFail
	extra["uptime_sec"] = body.UptimeSec
	extra["serials"] = body.Serials
	if b, err := json.Marshal(extra); err == nil {
		updates["runtime_state"] = string(b)
	}
	h.DB.Model(gw).Updates(updates)
	response.Success(c, gin.H{"hb_interval": 15, "server_time": now.Format(time.RFC3339)})
}

// ==================== DO 联动命令通道（平台入队 → 边缘拉取执行 → 回报结果） ====================

// GatewayCommand POST /gateways/:id/command （admin JWT 认证）
// body: {"type":"do_set","payload":{"mask":1,"mode":"set"}}  mode: set=按位或闭合 clear=按位与断开 write=整字节覆写
// 也支持 {"type":"do_set","payload":{"channel":1,"action":"on"}} 单路开关（前端用）
func (h *Handler) GatewayCommand(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var gw model.Gateway
	if err := h.DB.First(&gw, id).Error; err != nil {
		response.Error(c, http.StatusNotFound, "网关不存在")
		return
	}
	var req struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Type == "" {
		response.Error(c, http.StatusBadRequest, "参数错误：需要 type 与 payload")
		return
	}
	if req.Type != "do_set" {
		response.Error(c, http.StatusBadRequest, "暂只支持 do_set 命令（DO 继电器控制）")
		return
	}
	// 校验 payload 合法性
	var p struct {
		Mask    *int   `json:"mask"`
		Channel *int   `json:"channel"`
		Action  string `json:"action"`
		Mode    string `json:"mode"`
	}
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		response.Error(c, http.StatusBadRequest, "payload 不是合法 JSON")
		return
	}
	if p.Mask == nil && p.Channel == nil {
		response.Error(c, http.StatusBadRequest, "payload 需要 mask(位图) 或 channel(1-4)+action(on/off)")
		return
	}
	if p.Channel != nil && (*p.Channel < 1 || *p.Channel > 4) {
		response.Error(c, http.StatusBadRequest, "channel 必须在 1-4 之间（XM-E16 共 4 路 DO）")
		return
	}
	if p.Mask != nil && (*p.Mask < 0 || *p.Mask > 15) {
		response.Error(c, http.StatusBadRequest, "mask 必须在 0-15 之间（4 路 DO 位图）")
		return
	}
	caller, _ := middleware.GetCaller(c)
	cmd := model.GatewayCommand{
		GatewayID: gw.ID, Type: req.Type, Payload: string(req.Payload),
		Status: "pending", CreatedBy: caller,
	}
	if err := h.DB.Create(&cmd).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "入队失败")
		return
	}
	go middleware.WriteAudit(h.DB, c, "gateway_command", fmt.Sprintf("gw%d", gw.ID), string(req.Payload))
	response.Success(c, gin.H{"command_id": cmd.ID})
}

// GatewayCommandPull GET /gateway/commands/pull （网关认证）
// 返回该网关 pending 命令并标记 dispatched（at-most-once 派发；超时未回报由平台重派）
func (h *Handler) GatewayCommandPull(c *gin.Context) {
	gw, ok := h.GatewayAuth(c)
	if !ok {
		return
	}
	var cmds []model.GatewayCommand
	h.DB.Where("gateway_id = ? AND status = ?", gw.ID, "pending").Order("id asc").Limit(10).Find(&cmds)
	ids := make([]uint, 0, len(cmds))
	for _, cm := range cmds {
		ids = append(ids, cm.ID)
	}
	if len(ids) > 0 {
		h.DB.Model(&model.GatewayCommand{}).Where("id IN ?", ids).
			Updates(map[string]interface{}{"status": "dispatched", "dispatched_at": time.Now()})
	}
	response.Success(c, gin.H{"commands": cmds})
}

// GatewayCommandResult POST /gateway/commands/:cid/result （网关认证）
// body: {"status":"done"|"failed","result":"DO 位图已写 0x01，回读 0x01"}
func (h *Handler) GatewayCommandResult(c *gin.Context) {
	gw, ok := h.GatewayAuth(c)
	if !ok {
		return
	}
	cid, _ := strconv.Atoi(c.Param("cid"))
	var cmd model.GatewayCommand
	if err := h.DB.Where("id = ? AND gateway_id = ?", cid, gw.ID).First(&cmd).Error; err != nil {
		response.Error(c, http.StatusNotFound, "命令不存在")
		return
	}
	var body struct {
		Status string `json:"status"`
		Result string `json:"result"`
	}
	_ = c.ShouldBindJSON(&body)
	st := body.Status
	if st != "done" && st != "failed" {
		st = "failed"
	}
	now := time.Now()
	h.DB.Model(&cmd).Updates(map[string]interface{}{"status": st, "result": body.Result, "done_at": &now})
	response.Success(c, gin.H{"ok": true})
}

// GatewayCommands 见 dashboard.go（V47 起返回真实命令日志）

// GatewayRedispatch 定时任务：dispatched 超过 60s 未回报的命令回到 pending（在 main.go 注册）
func (h *Handler) GatewayRedispatch() {
	cutoff := time.Now().Add(-60 * time.Second)
	h.DB.Model(&model.GatewayCommand{}).
		Where("status = ? AND dispatched_at < ?", "dispatched", cutoff).
		Updates(map[string]interface{}{"status": "pending"})
}
