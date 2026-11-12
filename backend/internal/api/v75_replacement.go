package api

// V75 B4-3：健康度/退役 → 替换建议 → 采购申请草稿
// 触发：健康分 <60 或退役申请已批准（status=2）
// 复用 V71/V30 采购管道：为被替换资产生成 RPL-A{assetID} 替换件台账，再建采购申请（待审批）
// 幂等：同资产同时间段仅 1 条待审批；30 天内不重复生成（无论上次审批结果）
// 人工确认：GET 只出建议，POST 才落采购申请；审批仍走既有采购审批流

import (
	"fmt"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

const v75ReasonPrefix = "[资产替换] "

type v75Candidate struct {
	ID          uint    `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Brand       string  `json:"brand"`
	Model       string  `json:"model"`
	PowerW      int     `json:"power_w"`
	HeightU     int     `json:"height_u"`
	Stock       int     `json:"stock"`
	Price       float64 `json:"price"`
	HealthScore *int    `json:"health_score"`
	HealthGrade string  `json:"health_grade"`
	Kind        string  `json:"kind"` // asset=同分类在架 spare=在库备件
}

func (h *Handler) v75Health(assetID uint) *model.AssetHealth {
	var ah model.AssetHealth
	if h.DB.Where("asset_id = ?", assetID).First(&ah).Error == nil {
		return &ah
	}
	return nil
}

func (h *Handler) v75ApprovedRetirement(assetID uint) *model.AssetRetirement {
	var rt model.AssetRetirement
	if h.DB.Where("asset_id = ? AND status = 2", assetID).Order("id DESC").First(&rt).Error == nil {
		return &rt
	}
	return nil
}

func (h *Handler) v75ExistingPR(assetCode string) *model.PurchaseRequest {
	var pr model.PurchaseRequest
	if h.DB.Where("reason LIKE ? AND status = 1", v75ReasonPrefix+assetCode+"%").Order("id DESC").First(&pr).Error == nil {
		return &pr
	}
	return nil
}

// ReplacementSuggestion GET /assets/:id/replacement-suggestion
func (h *Handler) ReplacementSuggestion(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, id).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	if !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权查看该站点资产")
		return
	}
	ah := h.v75Health(a.ID)
	rt := h.v75ApprovedRetirement(a.ID)
	healthRisk := ah != nil && ah.Score > 0 && ah.Score < 60
	retireOK := rt != nil
	recommended := healthRisk || retireOK

	var reasons []string
	if healthRisk {
		reasons = append(reasons, fmt.Sprintf("健康分 %d 低于 60（%s）", ah.Score, ah.Grade))
	}
	if retireOK {
		reasons = append(reasons, "退役申请已批准，待处置")
	}
	if len(reasons) == 0 {
		if ah != nil {
			reasons = append(reasons, fmt.Sprintf("当前健康分 %d，未触发替换阈值", ah.Score))
		} else {
			reasons = append(reasons, "尚无健康评估，未触发替换阈值")
		}
	}

	// 同分类在架资产（候选参照）
	var assets []model.Asset
	h.DB.Where("category = ? AND id <> ? AND status = 1 AND site_id = ?", a.Category, a.ID, a.SiteID).
		Order("id DESC").Limit(5).Find(&assets)
	ids := make([]uint, 0, len(assets))
	for _, x := range assets {
		ids = append(ids, x.ID)
	}
	healthMap := map[uint]model.AssetHealth{}
	if len(ids) > 0 {
		var ahs []model.AssetHealth
		h.DB.Where("asset_id IN ?", ids).Find(&ahs)
		for _, x := range ahs {
			healthMap[x.AssetID] = x
		}
	}
	candidates := make([]v75Candidate, 0, len(assets))
	for _, x := range assets {
		c := v75Candidate{ID: x.ID, Code: x.Code, Name: x.Name, Brand: x.Brand, Model: x.Model,
			PowerW: x.PowerW, HeightU: x.HeightU, Kind: "asset"}
		if hh, exists := healthMap[x.ID]; exists {
			score := hh.Score
			c.HealthScore = &score
			c.HealthGrade = hh.Grade
		}
		candidates = append(candidates, c)
	}
	var spares []model.SparePart
	h.DB.Where("category = ? AND stock > 0", a.Category).Order("stock DESC").Limit(5).Find(&spares)
	spareCands := make([]v75Candidate, 0, len(spares))
	for _, s := range spares {
		spareCands = append(spareCands, v75Candidate{ID: s.ID, Code: s.Code, Name: s.Name, Model: s.Spec,
			Stock: s.Stock, Price: s.Price, Kind: "spare"})
	}

	var avgPrice float64
	h.DB.Model(&model.SparePart{}).Where("category = ? AND price > 0", a.Category).
		Select("COALESCE(AVG(price),0)").Scan(&avgPrice)
	supplier := a.Vendor
	suggested := "设备已达替换条件，建议采购同规格设备替换"
	if healthRisk && retireOK {
		suggested = "设备健康度低于 60 且退役已批准，建议采购同规格设备替换后处置旧机"
	} else if retireOK {
		suggested = "退役已批准，建议采购同规格设备替换，避免业务能力缺口"
	}

	var existing *model.PurchaseRequest
	pending := h.v75ExistingPR(a.Code)
	if pending != nil {
		existing = pending
	}
	// 30 天内任意状态的历史申请
	var recent model.PurchaseRequest
	recentID := uint(0)
	recentCode := ""
	if h.DB.Where("reason LIKE ? AND created_at >= ?", v75ReasonPrefix+a.Code+"%", time.Now().AddDate(0, 0, -30)).
		Order("id DESC").First(&recent).Error == nil {
		recentID, recentCode = recent.ID, recent.Code
	}

	response.Success(c, gin.H{
		"asset": gin.H{"id": a.ID, "code": a.Code, "name": a.Name, "category": a.Category,
			"brand": a.Brand, "model": a.Model, "site_id": a.SiteID, "vendor": a.Vendor},
		"health": func() interface{} {
			if ah == nil {
				return nil
			}
			return gin.H{"score": ah.Score, "grade": ah.Grade, "computed_at": ah.ComputedAt}
		}(),
		"retirement": func() interface{} {
			if rt == nil {
				return nil
			}
			return gin.H{"id": rt.ID, "code": rt.Code, "reason": rt.Reason, "approved_at": rt.ApprovedAt}
		}(),
		"trigger": gin.H{"health_risk": healthRisk, "retire_approved": retireOK,
			"recommended": recommended, "reason": strings.Join(reasons, "；")},
		"budget_yuan": round2(avgPrice), "supplier": supplier,
		"suggested_reason": suggested,
		"candidates":       candidates, "spares": spareCands,
		"existing": func() interface{} {
			if existing == nil {
				return nil
			}
			return gin.H{"id": existing.ID, "code": existing.Code, "status": existing.Status}
		}(),
		"recent": func() interface{} {
			if recentID == 0 {
				return nil
			}
			return gin.H{"id": recentID, "code": recentCode}
		}(),
	})
}

// CreateReplacementRequest POST /assets/:id/replacement-request {qty,supplier,budget_yuan,reason}
func (h *Handler) CreateReplacementRequest(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var a model.Asset
	if err := h.DB.First(&a, id).Error; err != nil {
		response.NotFound(c, "资产不存在")
		return
	}
	if !middleware.InSiteScope(c, a.SiteID) {
		response.Forbidden(c, "无权操作该站点资产")
		return
	}
	ah := h.v75Health(a.ID)
	rt := h.v75ApprovedRetirement(a.ID)
	if !(ah != nil && ah.Score > 0 && ah.Score < 60) && rt == nil {
		response.BadRequest(c, "未达到替换触发条件（健康分 <60 或退役已批准）")
		return
	}
	var req struct {
		Qty        int     `json:"qty"`
		Supplier   string  `json:"supplier"`
		BudgetYuan float64 `json:"budget_yuan"`
		Reason     string  `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	if req.Qty <= 0 {
		req.Qty = 1
	}
	if req.Qty > 99 {
		response.BadRequest(c, "数量过大")
		return
	}
	if pending := h.v75ExistingPR(a.Code); pending != nil {
		response.BadRequest(c, fmt.Sprintf("该资产已有待审批的替换采购申请 %s", pending.Code))
		return
	}
	var recent model.PurchaseRequest
	if h.DB.Where("reason LIKE ? AND created_at >= ?", v75ReasonPrefix+a.Code+"%", time.Now().AddDate(0, 0, -30)).
		First(&recent).Error == nil {
		response.BadRequest(c, fmt.Sprintf("该资产 30 天内已生成过替换申请 %s，请勿重复提交", recent.Code))
		return
	}
	if req.Supplier == "" {
		req.Supplier = a.Vendor
	}
	if strings.TrimSpace(req.Reason) == "" {
		if rt != nil {
			req.Reason = "退役已批准，采购同规格设备替换，保障业务连续性"
		} else {
			req.Reason = fmt.Sprintf("健康分 %d 低于 60，采购同规格设备替换", ah.Score)
		}
	}
	// 复用采购管道：为被替换资产建/复用替换件台账（RPL-A{id}），不污染真实备件库存
	spareCode := fmt.Sprintf("RPL-A%d", a.ID)
	var sp model.SparePart
	if err := h.DB.Where("code = ?", spareCode).First(&sp).Error; err != nil {
		sp = model.SparePart{
			Code: spareCode, Name: a.Name + "（替换采购）", Category: a.Category,
			Spec: strings.TrimSpace(a.Brand + " " + a.Model), Unit: "台", SiteID: a.SiteID,
			Stock: 0, SafetyStock: 0, Price: req.BudgetYuan / float64(req.Qty),
			Supplier: req.Supplier, Remark: "V75 资产替换建议自动创建，仅供采购申请关联",
		}
		if err := h.DB.Create(&sp).Error; err != nil {
			response.ServerError(c, "创建替换件台账失败")
			return
		}
	} else {
		h.DB.Model(&sp).Updates(map[string]interface{}{
			"name": a.Name + "（替换采购）", "spec": strings.TrimSpace(a.Brand + " " + a.Model),
			"supplier": req.Supplier, "price": req.BudgetYuan / float64(req.Qty),
		})
	}
	reason := fmt.Sprintf("%s%s %s：%s", v75ReasonPrefix, a.Code, a.Name, strings.TrimSpace(req.Reason))
	pr := model.PurchaseRequest{
		Code: h.genCode("PR"), SpareID: sp.ID, Qty: req.Qty, Reason: reason,
		Supplier: req.Supplier, BudgetYuan: req.BudgetYuan, Status: 1,
		ApplicantID: uint(middleware.GetUserID(c)), Applicant: usernameOf(c),
	}
	if err := h.DB.Create(&pr).Error; err != nil {
		response.ServerError(c, "创建采购申请失败")
		return
	}
	content := fmt.Sprintf("%s 申请为资产 %s（%s）采购替换设备 ×%d，预算 ¥%.2f。事由：%s",
		pr.Applicant, a.Name, a.Code, pr.Qty, pr.BudgetYuan, req.Reason)
	if ah != nil && ah.Score > 0 && ah.Score < 60 {
		content += fmt.Sprintf("（健康分 %d）", ah.Score)
	}
	if rt != nil {
		content += "（退役已批准）"
	}
	h.DB.Create(&model.Notification{
		Kind: "purchase_request", Title: "资产替换采购待审批: " + a.Name,
		Content: content, RefType: "purchase_request", RefID: pr.ID,
	})
	middleware.WriteAudit(h.DB, c, "create_asset_replacement_request", a.Code,
		fmt.Sprintf("%s；PR=%s；预算=%.2f；健康分=%d", req.Reason, pr.Code, pr.BudgetYuan, func() int {
			if ah == nil {
				return 0
			}
			return ah.Score
		}()))
	response.Created(c, gin.H{"id": pr.ID, "code": pr.Code, "spare_id": sp.ID, "spare_code": sp.Code,
		"qty": pr.Qty, "budget_yuan": pr.BudgetYuan, "status": pr.Status})
}