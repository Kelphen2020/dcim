package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V16 基础功能完善 ====================

// ResetGatewaySecret 重置网关接入密钥（返回新密钥，仅显示一次）
func (h *Handler) ResetGatewaySecret(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var gw model.Gateway
	if err := h.DB.First(&gw, id).Error; err != nil {
		response.NotFound(c, "网关不存在")
		return
	}
	if !middleware.InSiteScope(c, gw.SiteID) {
		response.Forbidden(c, "无权操作该网关")
		return
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		response.ServerError(c, "生成密钥失败")
		return
	}
	secret := hex.EncodeToString(buf)
	h.DB.Model(&gw).Update("secret", secret)
	middleware.WriteAudit(h.DB, c, "reset_gateway_secret", gw.Code, gw.Name)
	response.Success(c, gin.H{"gateway_id": gw.ID, "code": gw.Code, "secret": secret})
}

// GatewayStats 网关运行统计（测点数/最新推送/在线率）
func (h *Handler) GatewayStats(c *gin.Context) {
	var gws []model.Gateway
	q := h.DB.Model(&model.Gateway{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	q.Find(&gws)

	var totalPts, online int64
	h.DB.Model(&model.MonitoringPoint{}).Count(&totalPts)
	h.DB.Model(&model.Gateway{}).Where("online = ?", true).Count(&online)

	// 最近读数时间（全库）
	var lastReading struct {
		Latest *time.Time
		N24h   int64
	}
	h.DB.Model(&model.Reading{}).Select("MAX(timestamp) as latest").Scan(&lastReading)
	h.DB.Model(&model.Reading{}).Where("timestamp > ?", time.Now().Add(-24*time.Hour)).Count(&lastReading.N24h)

	response.Success(c, gin.H{
		"gateway_total":   len(gws),
		"gateway_online":  online,
		"point_total":     totalPts,
		"latest_reading":  lastReading.Latest,
		"readings_24h":    lastReading.N24h,
		"simulate_on":     h.simulateOn(),
	})
}

// simulateOn 读配置判断模拟器状态（供前端提示数据来源）
func (h *Handler) simulateOn() bool {
	var cnt int64
	// 模拟器开启时最近5分钟内必有自动读数(30s一轮)；用近似判断
	h.DB.Model(&model.Reading{}).Where("timestamp > ?", time.Now().Add(-90*time.Second)).Count(&cnt)
	return cnt > 0
}

// BatchAssets V16 资产批量状态流转
// 状态机: 1在架 2在库 3维修 4报废
// 允许流转: 在架→维修/报废(须先下架, 拒绝); 在库→维修/报废; 维修→在库/报废; 报废→(终态,拒绝)
func (h *Handler) BatchAssets(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids"`
		Action string `json:"action"` // to_repair / to_store / to_scrap / restore
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 || len(req.IDs) > 500 {
		response.Error(c, 400, "参数错误: ids 不能为空且最多 500")
		return
	}
	target := map[string]int{"to_repair": 3, "to_store": 2, "to_scrap": 4, "restore": 2}
	newStatus, valid := target[req.Action]
	if !valid {
		response.Error(c, 400, "action 须为 to_repair/to_store/to_scrap/restore 之一")
		return
	}
	var assets []model.Asset
	h.DB.Where("id IN ?", req.IDs).Find(&assets)
	if len(assets) == 0 {
		response.Error(c, 404, "未找到资产")
		return
	}
	done, rejected := 0, []string{}
	for _, a := range assets {
		// 状态机校验
		if a.Status == 1 {
			rejected = append(rejected, a.Code+"(在架资产须先下架)")
			continue
		}
		if a.Status == 4 {
			rejected = append(rejected, a.Code+"(已报废为终态)")
			continue
		}
		if a.Status == newStatus {
			rejected = append(rejected, a.Code+"(已是目标状态)")
			continue
		}
		updates := map[string]interface{}{"status": newStatus}
		if req.Note != "" {
			updates["remark"] = req.Note
		}
		h.DB.Model(&model.Asset{}).Where("id = ?", a.ID).Updates(updates)
		middleware.WriteAudit(h.DB, c, "batch_asset_status", a.Code,
			fmt.Sprintf("%d→%d %s", a.Status, newStatus, req.Note))
		done++
	}
	response.Success(c, gin.H{"done": done, "rejected": rejected})
}

// TopologyStats V16 基础设施层级统计（房间测点/告警数、机柜占用，供 Topology 页角标与占用条）
func (h *Handler) TopologyStats(c *gin.Context) {
	type roomStat struct {
		RoomID     uint  `json:"room_id"`
		PointCnt   int64 `json:"point_cnt"`
		AlertCnt   int64 `json:"alert_cnt"`
		CabinetCnt int64 `json:"cabinet_cnt"`
	}
	type cabStat struct {
		CabinetID uint    `json:"cabinet_id"`
		UsedU     int     `json:"used_u"`
		TotalU    int     `json:"total_u"`
		AssetCnt  int64   `json:"asset_cnt"`
		UsedKW    float64 `json:"used_kw"`
		RatedKW   float64 `json:"rated_kw"`
	}
	var rooms []model.Room
	rq := h.DB.Model(&model.Room{})
	if sid := c.Query("site_id"); sid != "" && sid != "0" {
		rq = rq.Where("site_id = ?", sid)
	}
	rq.Find(&rooms)
	roomStats := make([]roomStat, 0, len(rooms))
	for _, r := range rooms {
		st := roomStat{RoomID: r.ID}
		h.DB.Model(&model.Cabinet{}).Where("room_id = ?", r.ID).Count(&st.CabinetCnt)
		h.DB.Model(&model.MonitoringPoint{}).Where("room_id = ?", r.ID).Count(&st.PointCnt)
		h.DB.Model(&model.Alert{}).
			Where("status < 3 AND point_id IN (SELECT id FROM t_monitoring_point WHERE room_id = ?)", r.ID).
			Count(&st.AlertCnt)
		roomStats = append(roomStats, st)
	}

	var cabs []model.Cabinet
	cq := h.DB.Model(&model.Cabinet{})
	if sid := c.Query("site_id"); sid != "" && sid != "0" {
		cq = cq.Where("site_id = ?", sid)
	}
	if rid := c.Query("room_id"); rid != "" && rid != "0" {
		cq = cq.Where("room_id = ?", rid)
	}
	cq.Find(&cabs)
	cabStats := make([]cabStat, 0, len(cabs))
	for _, cb := range cabs {
		cs := cabStat{CabinetID: cb.ID, TotalU: cb.TotalU, RatedKW: cb.RatedPowerKW}
		var row struct {
			U int
			W int64
			N int64
		}
		h.DB.Model(&model.Asset{}).
			Select("COALESCE(SUM(height_u),0) as u, COALESCE(SUM(power_w),0) as w, COUNT(*) as n").
			Where("cabinet_id = ? AND status = 1", cb.ID).Scan(&row)
		cs.UsedU = row.U
		cs.UsedKW = float64(row.W) / 1000.0
		cs.AssetCnt = row.N
		cabStats = append(cabStats, cs)
	}
	response.Success(c, gin.H{"rooms": roomStats, "cabinets": cabStats})
}
