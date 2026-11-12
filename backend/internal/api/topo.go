package api

import (
	"strconv"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// GetTopoDiagram 获取拓扑图布局（按站点+类型；无记录返回空）
func (h *Handler) GetTopoDiagram(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.Query("site_id"))
	kind := c.Query("kind")
	if kind != "power_sld" && kind != "net_topo" {
		response.BadRequest(c, "kind 必须为 power_sld 或 net_topo")
		return
	}
	var d model.TopoDiagram
	err := h.DB.Where("site_id = ? AND kind = ?", siteID, kind).First(&d).Error
	if err != nil {
		response.Success(c, gin.H{"data": ""})
		return
	}
	response.Success(c, gin.H{"data": d.Data, "updated_at": d.UpdatedAt})
}

// SaveTopoDiagram 保存拓扑图布局（UPSERT）
func (h *Handler) SaveTopoDiagram(c *gin.Context) {
	var req struct {
		SiteID uint   `json:"site_id" binding:"required"`
		Kind   string `json:"kind" binding:"required"`
		Data   string `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Kind != "power_sld" && req.Kind != "net_topo" {
		response.BadRequest(c, "kind 必须为 power_sld 或 net_topo")
		return
	}
	if len(req.Data) > 500_000 {
		response.BadRequest(c, "布局数据过大")
		return
	}
	uid, _ := c.Get("user_id")
	var d model.TopoDiagram
	err := h.DB.Where("site_id = ? AND kind = ?", req.SiteID, req.Kind).First(&d).Error
	if err != nil {
		d = model.TopoDiagram{SiteID: req.SiteID, Kind: req.Kind}
	}
	d.Data = req.Data
	if u, ok := uid.(uint); ok {
		d.UpdatedBy = u
	}
	if err := h.DB.Save(&d).Error; err != nil {
		response.ServerError(c, "保存失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"id": d.ID})
}
