package api

import (
	"fmt"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// V69/B2-4: 网络设备从资产创建（强关联）+ 资产退役联动

// netdevRetireLinkage 资产退役批准后调用：关联网络设备自动置离线并留痕，返回受影响设备名
func (h *Handler) netdevRetireLinkage(c *gin.Context, assetID uint, assetCode string) []string {
	var devs []model.NetworkDevice
	h.DB.Where("asset_id = ?", assetID).Find(&devs)
	names := []string{}
	for i := range devs {
		if devs[i].Status != 0 {
			h.DB.Model(&devs[i]).Update("status", 0)
			go middleware.WriteAudit(h.DB, c, "update", "network_device",
				fmt.Sprintf("资产退役联动：设备 %s(ID=%d) 自动置离线，资产=%s", devs[i].Name, devs[i].ID, assetCode))
		}
		names = append(names, devs[i].Name)
	}
	return names
}

// NetdevAssetOptions GET /network-devices/asset-options?site_id=&keyword=
// 「从资产创建」候选：在架/在库网络设备类资产（category IN switch/router/firewall/ap 或全部可选），返回精简字段
func (h *Handler) NetdevAssetOptions(c *gin.Context) {
	q := h.DB.Model(&model.Asset{}).Where("status IN ?", []int{1, 2})
	if sid := c.Query("site_id"); sid != "" && sid != "0" {
		q = q.Where("site_id = ?", sid)
	}
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("name ILIKE ? OR code ILIKE ? OR ip ILIKE ?", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
	}
	// 已被网络设备绑定的资产排除（一资产一设备，避免重复建）
	var bound []uint
	h.DB.Model(&model.NetworkDevice{}).Where("asset_id IS NOT NULL").Pluck("asset_id", &bound)
	if len(bound) > 0 {
		q = q.Where("id NOT IN ?", bound)
	}
	var list []model.Asset
	q.Select("id, code, name, category, brand, model, sn, ip, site_id, cabinet_id, room_id, status").
		Order("site_id, category, code").Limit(200).Find(&list)
	if list == nil {
		list = []model.Asset{}
	}
	response.Success(c, gin.H{"items": list, "total": len(list)})
}
