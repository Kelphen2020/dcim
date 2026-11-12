package api

import (
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// Room3DData 3D机房一次性聚合数据：
// 房间(含面积/楼层) + 机柜列(冷热通道) + 机柜(利用率/功耗) + 测点(实时值) + 供电节点 + 在架资产(U位)
func (h *Handler) Room3DData(c *gin.Context) {
	siteID := c.DefaultQuery("site_id", "1")

	var rooms []model.Room
	h.DB.Where("site_id = ?", siteID).Order("id").Find(&rooms)

	var rows []model.Row
	h.DB.Where("site_id = ?", siteID).Order("id").Find(&rows)

	var cabinets []model.Cabinet
	h.DB.Where("site_id = ?", siteID).Order("room_id, code").Find(&cabinets)

	// 机柜附加：已用U/功耗/承重
	type cabExtra struct {
		ID         uint    `json:"id"`
		UsedU      int     `json:"used_u"`
		PowerKW    float64 `json:"power_kw"`
		WeightKG   float64 `json:"weight_kg"`
		AssetCount int     `json:"asset_count"`
	}
	var extras []cabExtra
	h.DB.Raw(`SELECT c.id,
			COALESCE(SUM(a.height_u),0)::int as used_u,
			COALESCE(SUM(a.power_w),0)/1000.0 as power_kw,
			COALESCE(SUM(a.weight_kg),0) as weight_kg,
			COUNT(a.id)::int as asset_count
		FROM t_cabinet c
		LEFT JOIN t_asset a ON a.cabinet_id = c.id AND a.status = 1
		WHERE c.site_id = ? GROUP BY c.id`, siteID).Scan(&extras)

	// 在架资产（U位渲染 + 点击弹窗详情）
	type assetLite struct {
		ID         uint    `json:"id"`
		CabinetID  uint    `json:"cabinet_id"`
		Code       string  `json:"code"`
		Name       string  `json:"name"`
		Category   string  `json:"category"`
		Brand      string  `json:"brand"`
		Model      string  `json:"model"`
		SN         string  `json:"sn"`
		IP         string  `json:"ip"`
		StartU     int     `json:"start_u"`
		HeightU    int     `json:"height_u"`
		PowerW     int     `json:"power_w"`
		WeightKG   float64 `json:"weight_kg"`
		StatusText string  `json:"status_text"`
	}
	var assets []assetLite
	h.DB.Raw(`SELECT id, cabinet_id, COALESCE(code,'') as code, name, COALESCE(category,'other') as category,
			COALESCE(brand,'') as brand, COALESCE(model,'') as model, COALESCE(sn,'') as sn, COALESCE(ip,'') as ip,
			start_u, height_u, COALESCE(power_w,0) as power_w, COALESCE(weight_kg,0) as weight_kg,
			CASE status WHEN 1 THEN '运行中' WHEN 3 THEN '维修中' ELSE '在架' END as status_text
		FROM t_asset WHERE site_id = ? AND status = 1 AND cabinet_id > 0
		ORDER BY cabinet_id, start_u`, siteID).Scan(&assets)

	// V34: 辅助房间资产（非机架设备：配电柜/变压器/电池组/冷机/操作台等，room_id>0 且未上架）
	// 富化：在保状态 + 活跃告警数（绑定测点）
	type roomAsset struct {
		ID          uint    `json:"id"`
		RoomID      uint    `json:"room_id"`
		Code        string  `json:"code"`
		Name        string  `json:"name"`
		Category    string  `json:"category"`
		Brand       string  `json:"brand"`
		Model       string  `json:"model"`
		SN          string  `json:"sn"`
		IP          string  `json:"ip"`
		PowerW      int     `json:"power_w"`
		WeightKG    float64 `json:"weight_kg"`
		Vendor      string  `json:"vendor"`
		Status      int     `json:"status"`
		StatusText  string  `json:"status_text"`
		Remark      string  `json:"remark"`
		InWarranty  bool    `json:"in_warranty"`  // 有效维保合同覆盖
		WarrantyEnd string  `json:"warranty_end"` // 最近合同到期日
		Alarms      int     `json:"alarms"`       // 关联测点活跃告警数
		InWarrantyN int     `json:"-"`            // Raw Scan 接收列, 转 bool 用
	}
	var roomAssets []roomAsset
	h.DB.Raw(`SELECT a.id, a.room_id, COALESCE(a.code,'') as code, a.name, COALESCE(a.category,'other') as category,
			COALESCE(a.brand,'') as brand, COALESCE(a.model,'') as model, COALESCE(a.sn,'') as sn, COALESCE(a.ip,'') as ip,
			COALESCE(a.power_w,0) as power_w, COALESCE(a.weight_kg,0) as weight_kg, COALESCE(a.vendor,'') as vendor,
			COALESCE(a.status,1) as status,
			CASE a.status WHEN 1 THEN '运行中' WHEN 2 THEN '在库' WHEN 3 THEN '维修中' ELSE '已报废' END as status_text,
			COALESCE(a.remark,'') as remark,
			COALESCE((SELECT COUNT(*) FROM t_maintenance m WHERE m.asset_id = a.id AND m.kind = 'contract'
				AND m.status = 1 AND m.end_at > NOW()), 0)::int as in_warranty_n,
			COALESCE((SELECT to_char(MAX(m2.end_at),'YYYY-MM-DD') FROM t_maintenance m2 WHERE m2.asset_id = a.id
				AND m2.kind = 'contract' AND m2.status = 1 AND m2.end_at > NOW()), '') as warranty_end
		FROM t_asset a WHERE a.site_id = ? AND a.room_id > 0 AND a.status IN (1,2,3)
		ORDER BY a.room_id, a.id`, siteID).Scan(&roomAssets)
	// in_warranty_n 列由 GORM Scan 映射到 InWarrantyN(json:"-"), 转 bool
	for i := range roomAssets {
		roomAssets[i].InWarranty = roomAssets[i].InWarrantyN > 0
	}

	var points []model.MonitoringPoint
	h.DB.Where("site_id = ?", siteID).Order("type, id").Find(&points)

	type powerNode struct {
		ID       uint    `json:"id"`
		ParentID uint    `json:"parent_id"`
		Name     string  `json:"name"`
		Kind     string  `json:"kind"`
		Model    string  `json:"model"`
		RatedKVA float64 `json:"rated_kva"`
		LoadKW   float64 `json:"load_kw"`
		Status   int     `json:"status"`
	}
	var powerNodes []powerNode
	h.DB.Raw(`SELECT id, parent_id, name, kind, COALESCE(model,'') as model,
			COALESCE(rated_kva,0) as rated_kva, COALESCE(load_kw,0) as load_kw, COALESCE(status,1) as status
		FROM t_power_chain WHERE site_id = ? ORDER BY id`, siteID).Scan(&powerNodes)

	response.Success(c, gin.H{
		"rooms":       rooms,
		"rows":        rows,
		"cabinets":    cabinets,
		"cabinet_ext": extras,
		"assets":      assets,
		"room_assets": roomAssets,
		"points":      points,
		"power_nodes": powerNodes,
	})
}
