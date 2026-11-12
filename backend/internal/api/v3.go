package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== 趋势分析 ====================

// PointTree 测点树：站点→房间→机柜→测点（支持多选对比）
func (h *Handler) PointTree(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.Query("site_id"))
	if siteID == 0 {
		response.Error(c, http.StatusBadRequest, "site_id 必填")
		return
	}
	var rooms []model.Room
	h.DB.Where("site_id = ?", siteID).Order("id").Find(&rooms)
	var points []model.MonitoringPoint
	h.DB.Where("site_id = ?", siteID).Order("room_id, cabinet_id, id").Find(&points)
	var cabinets []model.Cabinet
	h.DB.Where("site_id = ?", siteID).Order("code").Find(&cabinets)

	type pointOut struct {
		model.MonitoringPoint
	}
	type cabNode struct {
		ID     uint       `json:"id"`
		Code   string     `json:"code"`
		Name   string     `json:"name"`
		Points []pointOut `json:"points"`
	}
	type roomNode struct {
		ID       uint       `json:"id"`
		Name     string     `json:"name"`
		Cabinets []cabNode  `json:"cabinets"`
		Points   []pointOut `json:"points"` // 未挂机柜的测点
	}
	cabByID := map[uint]model.Cabinet{}
	for _, cb := range cabinets {
		cabByID[cb.ID] = cb
	}
	out := []roomNode{}
	roomIdx := map[uint]int{}
	for _, r := range rooms {
		out = append(out, roomNode{ID: r.ID, Name: r.Name, Cabinets: []cabNode{}, Points: []pointOut{}})
		roomIdx[r.ID] = len(out) - 1
	}
	// 虚拟房间：测点无 room_id
	for _, p := range points {
		var idx int
		if p.RoomID != nil {
			if i, ok := roomIdx[*p.RoomID]; ok {
				idx = i
			} else {
				out = append(out, roomNode{ID: 0, Name: "未分区", Cabinets: []cabNode{}, Points: []pointOut{}})
				roomIdx[0] = len(out) - 1
				idx = roomIdx[0]
			}
		} else {
			if _, ok := roomIdx[0]; !ok {
				out = append(out, roomNode{ID: 0, Name: "未分区", Cabinets: []cabNode{}, Points: []pointOut{}})
				roomIdx[0] = len(out) - 1
			}
			idx = roomIdx[0]
		}
		if p.CabinetID != nil {
			if _, ok := cabByID[*p.CabinetID]; ok {
				found := false
				for ci := range out[idx].Cabinets {
					if out[idx].Cabinets[ci].ID == *p.CabinetID {
						out[idx].Cabinets[ci].Points = append(out[idx].Cabinets[ci].Points, pointOut{p})
						found = true
						break
					}
				}
				if !found {
					cb := cabByID[*p.CabinetID]
					out[idx].Cabinets = append(out[idx].Cabinets, cabNode{ID: cb.ID, Code: cb.Code, Name: cb.Code, Points: []pointOut{pointOut{p}}})
				}
				continue
			}
		}
		out[idx].Points = append(out[idx].Points, pointOut{p})
	}
	response.Success(c, gin.H{"rooms": out})
}

var rangePresets = map[string]time.Duration{
	"1h": time.Hour, "24h": 24 * time.Hour, "1w": 7 * 24 * time.Hour,
	"1m": 30 * 24 * time.Hour, "3m": 90 * 24 * time.Hour,
}

// TrendSeries 多测点趋势序列（自动降采样至约 600 点/测点）
func (h *Handler) TrendSeries(c *gin.Context) {
	idsStr := c.Query("point_ids")
	if idsStr == "" {
		response.Error(c, http.StatusBadRequest, "point_ids 必填")
		return
	}
	var ids []uint
	for _, s := range strings.Split(idsStr, ",") {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && v > 0 {
			ids = append(ids, uint(v))
		}
	}
	if len(ids) == 0 || len(ids) > 20 {
		response.Error(c, http.StatusBadRequest, "测点数量须在 1~20 之间")
		return
	}
	now := time.Now()
	var start, end time.Time
	if s := c.Query("start"); s != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
			start = t
		}
	}
	if e := c.Query("end"); e != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", e, time.Local); err == nil {
			end = t
		}
	}
	if end.IsZero() {
		end = now
	}
	if start.IsZero() {
		dur, ok := rangePresets[c.DefaultQuery("range", "24h")]
		if !ok {
			dur = 24 * time.Hour
		}
		start = end.Add(-dur)
	}

	// 元信息
	var points []model.MonitoringPoint
	h.DB.Where("id IN ?", ids).Find(&points)
	pm := map[uint]model.MonitoringPoint{}
	for _, p := range points {
		pm[p.ID] = p
	}

	type seriesPoint struct {
		T string  `json:"t"`
		V float64 `json:"v"`
	}
	type seriesOut struct {
		PointID uint          `json:"point_id"`
		Name    string        `json:"name"`
		Unit    string        `json:"unit"`
		Type    string        `json:"type"`
		Stats   gin.H         `json:"stats"`
		Data    []seriesPoint `json:"data"`
	}
	out := []seriesOut{}
	for _, pid := range ids {
		var rows []model.Reading
		h.DB.Where("point_id = ? AND timestamp >= ? AND timestamp <= ? AND quality = 1", pid, start, end).
			Order("timestamp").Limit(60000).Find(&rows)
		if len(rows) == 0 {
			out = append(out, seriesOut{PointID: pid, Name: pm[pid].Name, Unit: pm[pid].Unit, Type: pm[pid].Type, Data: []seriesPoint{}})
			continue
		}
		// 降采样：等距抽取至 ~600 点
		stride := 1
		if len(rows) > 600 {
			stride = len(rows) / 600
		}
		data := make([]seriesPoint, 0, len(rows)/stride+1)
		minV, maxV, sum := rows[0].Value, rows[0].Value, 0.0
		for i := 0; i < len(rows); i++ {
			r := rows[i]
			if r.Value < minV {
				minV = r.Value
			}
			if r.Value > maxV {
				maxV = r.Value
			}
			sum += r.Value
			if i%stride == 0 || i == len(rows)-1 {
				data = append(data, seriesPoint{T: r.Timestamp.Format("2006-01-02 15:04:05"), V: r.Value})
			}
		}
		avg := sum / float64(len(rows))
		out = append(out, seriesOut{
			PointID: pid, Name: pm[pid].Name, Unit: pm[pid].Unit, Type: pm[pid].Type,
			Stats: gin.H{"min": round2(minV), "max": round2(maxV), "avg": round2(avg), "count": len(rows)},
			Data:  data,
		})
	}
	response.Success(c, gin.H{
		"start":  start.Format("2006-01-02 15:04:05"),
		"end":    end.Format("2006-01-02 15:04:05"),
		"series": out,
	})
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// ==================== 趋势模板 ====================

// ListTrendTemplates 模板列表
func (h *Handler) ListTrendTemplates(c *gin.Context) {
	var items []model.TrendTemplate
	h.DB.Order("id DESC").Find(&items)
	response.Success(c, gin.H{"items": items})
}

// CreateTrendTemplate 保存模板
func (h *Handler) CreateTrendTemplate(c *gin.Context) {
	var req model.TrendTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Name == "" || req.PointIDs == "" {
		response.Error(c, http.StatusBadRequest, "名称/测点不能为空")
		return
	}
	req.ID = 0
	req.CreatedBy = c.GetUint("user_id")
	if err := h.DB.Create(&req).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "保存失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_trend_template", req.Name, "测点="+req.PointIDs)
	response.Success(c, req)
}

// DeleteTrendTemplate 删除模板
func (h *Handler) DeleteTrendTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var t model.TrendTemplate
	if err := h.DB.First(&t, id).Error; err == nil {
		middleware.WriteAudit(h.DB, c, "delete_trend_template", t.Name, "")
		h.DB.Delete(&model.TrendTemplate{}, id)
	}
	response.Success(c, nil)
}

// ==================== 告警五态流转 ====================

var alertActionNames = map[string]string{
	"suspend": "挂起", "resume": "恢复待处理", "acknowledge": "确认", "resolve": "结束",
	"watch": "标记关注", "unwatch": "取消关注", "block": "屏蔽", "unblock": "取消屏蔽",
}

// AlertAction 告警操作（五态流转 + 关注/屏蔽），每次写留痕+审计
func (h *Handler) AlertAction(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Action string `json:"action" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	name, ok := alertActionNames[req.Action]
	if !ok {
		response.Error(c, http.StatusBadRequest, "不支持的操作: "+req.Action)
		return
	}
	var a model.Alert
	if err := h.DB.First(&a, id).Error; err != nil {
		response.Error(c, http.StatusNotFound, "告警不存在")
		return
	}
	userID := c.GetUint("user_id")
	now := time.Now()

	switch req.Action {
	case "suspend":
		if a.Status != 0 {
			response.Error(c, http.StatusBadRequest, "仅待处理告警可挂起")
			return
		}
		h.DB.Model(&a).Update("status", 1)
	case "resume":
		if a.Status != 1 {
			response.Error(c, http.StatusBadRequest, "仅已挂起告警可恢复")
			return
		}
		h.DB.Model(&a).Update("status", 0)
	case "acknowledge":
		if a.Status != 0 && a.Status != 1 {
			response.Error(c, http.StatusBadRequest, "仅待处理/已挂起告警可确认")
			return
		}
		h.DB.Model(&a).Updates(map[string]interface{}{"status": 2, "acknowledged_by": userID, "acknowledged_at": now})
	case "resolve":
		if a.Status == 3 {
			response.Error(c, http.StatusBadRequest, "告警已结束")
			return
		}
		h.DB.Model(&a).Updates(map[string]interface{}{"status": 3, "resolved_at": now})
		h.restorePointStatus(id)
	case "watch", "unwatch":
		h.DB.Model(&a).Update("watched", req.Action == "watch")
	case "block", "unblock":
		h.DB.Model(&a).Update("blocked", req.Action == "block")
	}

	h.DB.Create(&model.AlertActionLog{AlertID: uint(id), Action: req.Action, Reason: req.Reason, Operator: userID})
	middleware.WriteAudit(h.DB, c, "alert_"+req.Action, strconv.Itoa(id), name+"；原因="+req.Reason)
	response.Success(c, gin.H{"id": id, "action": req.Action})
}

// AlertLogs 告警操作留痕列表
func (h *Handler) AlertLogs(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var logs []model.AlertActionLog
	h.DB.Where("alert_id = ?", id).Order("id DESC").Find(&logs)
	response.Success(c, gin.H{"items": logs})
}
