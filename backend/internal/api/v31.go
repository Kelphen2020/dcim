package api

import (
	"fmt"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V31-① 备件 BOM 关联 ====================

type bomRow struct {
	ID         uint    `json:"id"`
	SpareID    uint    `json:"spare_id"`
	AssetCat   string  `json:"asset_cat"`
	AssetModel string  `json:"asset_model"`
	Qty        int     `json:"qty"`
	Remark     string  `json:"remark"`
	SpareName  string  `json:"spare_name"`
	SpareCode  string  `json:"spare_code"`
	SpareUnit  string  `json:"spare_unit"`
	SpareStock int     `json:"spare_stock"`
	SparePrice float64 `json:"spare_price"`
	Supplier   string  `json:"supplier"`
}

// ListSpareBOMs GET /spare-boms?spare_id=&asset_cat=
func (h *Handler) ListSpareBOMs(c *gin.Context) {
	q := h.DB.Table("t_spare_bom b").
		Select(`b.id, b.spare_id, b.asset_cat, b.asset_model, b.qty, b.remark,
			s.name as spare_name, s.code as spare_code, s.unit as spare_unit,
			s.stock as spare_stock, s.price as spare_price, s.supplier`).
		Joins("JOIN t_spare_part s ON s.id = b.spare_id")
	if sid := c.Query("spare_id"); sid != "" {
		q = q.Where("b.spare_id = ?", sid)
	}
	if cat := c.Query("asset_cat"); cat != "" {
		q = q.Where("b.asset_cat = ?", cat)
	}
	list := []bomRow{}
	q.Order("b.id DESC").Limit(500).Scan(&list)
	response.Success(c, gin.H{"list": list})
}

// CreateSpareBOM POST /spare-boms
func (h *Handler) CreateSpareBOM(c *gin.Context) {
	var b model.SpareBOM
	if err := c.ShouldBindJSON(&b); err != nil || b.SpareID == 0 || b.AssetCat == "" {
		response.BadRequest(c, "spare_id/asset_cat 必填")
		return
	}
	var sp model.SparePart
	if h.DB.First(&sp, b.SpareID).Error != nil {
		response.NotFound(c, "备件不存在")
		return
	}
	if b.Qty <= 0 {
		b.Qty = 1
	}
	if err := h.DB.Create(&b).Error; err != nil {
		response.BadRequest(c, "该备件×分类×型号组合已存在")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_spare_bom", sp.Code, fmt.Sprintf("%s→%s/%s", sp.Name, b.AssetCat, b.AssetModel))
	response.Created(c, b)
}

// DeleteSpareBOM DELETE /spare-boms/:id
func (h *Handler) DeleteSpareBOM(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	h.DB.Delete(&model.SpareBOM{}, id)
	response.Success(c, gin.H{"deleted": id})
}

// RecommendSpares GET /spare-boms/recommend?asset_id=
// 按资产分类+型号推荐备件: 型号精确匹配优先, 分类通用次之; 附库存/缺口
func (h *Handler) RecommendSpares(c *gin.Context) {
	assetID, ok := parseIDParam(c)
	if !ok {
		return
	}
	var a model.Asset
	if h.DB.First(&a, assetID).Error != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	type recRow struct {
		BID        uint    `json:"bom_id"`
		SpareID    uint    `json:"spare_id"`
		SpareName  string  `json:"spare_name"`
		SpareCode  string  `json:"spare_code"`
		SpareUnit  string  `json:"spare_unit"`
		Stock      int     `json:"stock"`
		Safety     int     `gorm:"column:safety_stock" json:"safety_stock"`
		Price      float64 `json:"price"`
		Supplier   string  `json:"supplier"`
		Qty        int     `json:"qty"`
		Match      string  `json:"match"` // model 型号精确 / category 分类通用
		Remark     string  `json:"remark"`
	}
	recs := []recRow{}
	// 型号精确匹配
	if a.Model != "" {
		h.DB.Raw(`SELECT b.id as bid, b.spare_id, b.qty, b.remark, b.asset_model,
				s.name as spare_name, s.code as spare_code, s.unit as spare_unit,
				s.stock, s.safety_stock, s.price, s.supplier, 'model' as match
			FROM t_spare_bom b JOIN t_spare_part s ON s.id=b.spare_id
			WHERE b.asset_cat = ? AND b.asset_model = ?`, a.Category, a.Model).Scan(&recs)
	}
	// 分类通用(排除已按型号匹配的备件)
	generic := []recRow{}
	h.DB.Raw(`SELECT b.id as bid, b.spare_id, b.qty, b.remark,
			s.name as spare_name, s.code as spare_code, s.unit as spare_unit,
			s.stock, s.safety_stock, s.price, s.supplier, 'category' as match
		FROM t_spare_bom b JOIN t_spare_part s ON s.id=b.spare_id
		WHERE b.asset_cat = ? AND b.asset_model = ''`, a.Category).Scan(&generic)
	seen := map[uint]bool{}
	for _, r := range recs {
		seen[r.SpareID] = true
	}
	for _, g := range generic {
		if !seen[g.SpareID] {
			recs = append(recs, g)
		}
	}
	// 库存不足标记
	lowCount := 0
	for i := range recs {
		if recs[i].Safety > 0 && recs[i].Stock <= recs[i].Safety {
			lowCount++
		}
	}
	response.Success(c, gin.H{
		"asset_id": a.ID, "asset_name": a.Name, "asset_category": a.Category, "asset_model": a.Model,
		"list": recs, "low_stock_count": lowCount,
	})
}
