package api

// B3-2 (G4) 低库存备件每日扫描自动生成采购申请
// 闭环: 低库存(stock<=safety_stock) → 自动生成待审批采购申请(建议+确认模式: 仍需人工审批) → 通知管理员 → 留痕审计
// 幂等键: 同备件已有待审批(status=1)申请不重复生成; 自动生成的申请 24h 内不重复(被驳回后每日最多再生成一次)
// 设置白名单开关: spare_auto_purchase(1/0, 默认1)

import (
	"fmt"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ScanSpareAutoPurchase 扫描低库存备件并自动生成采购申请，返回生成数
func (h *Handler) ScanSpareAutoPurchase() int {
	if h.healthSetting("spare_auto_purchase", "1") == "0" {
		return 0
	}
	var lows []model.SparePart
	h.DB.Where("stock <= safety_stock AND safety_stock > 0").Find(&lows)
	created := 0
	since := time.Now().Add(-24 * time.Hour)
	for _, sp := range lows {
		var pending int64
		h.DB.Model(&model.PurchaseRequest{}).Where("spare_id = ? AND status = 1", sp.ID).Count(&pending)
		if pending > 0 {
			continue // 幂等: 已有待审批申请
		}
		var recent int64
		h.DB.Model(&model.PurchaseRequest{}).
			Where("spare_id = ? AND created_at > ? AND reason LIKE ?", sp.ID, since, "[自动补货]%").Count(&recent)
		if recent > 0 {
			continue // 每日节奏: 24h 内已自动生成过(如被驳回)不立刻重开
		}
		qty := sp.SafetyStock*2 - sp.Stock // 目标库存 = 2×安全水位
		if qty < 1 {
			qty = 1
		}
		reason := fmt.Sprintf("[自动补货] 库存 %d%s 低于安全水位 %d，系统每日扫描自动生成，目标补至 %d",
			sp.Stock, sp.Unit, sp.SafetyStock, sp.SafetyStock*2)
		pr := model.PurchaseRequest{
			Code: h.genCode("PR"), SpareID: sp.ID, Qty: qty, Reason: reason,
			Supplier: sp.Supplier, BudgetYuan: float64(qty) * sp.Price,
			Status: 1, ApplicantID: 0, Applicant: "系统自动",
		}
		if err := h.DB.Create(&pr).Error; err != nil {
			continue
		}
		created++
		h.DB.Create(&model.Notification{
			Kind: "purchase_request", Title: "低库存自动采购申请待审批: " + sp.Name,
			Content: fmt.Sprintf("备件 %s(%s) 库存 %d%s 低于安全水位 %d，系统已自动生成采购申请 %s（%d%s，预算 ¥%.2f），请及时审批。",
				sp.Name, sp.Code, sp.Stock, sp.Unit, sp.SafetyStock, pr.Code, qty, sp.Unit, pr.BudgetYuan),
			RefType: "purchase_request", RefID: pr.ID,
		})
		h.DB.Create(&model.Audit{
			Username: "system", Action: "auto_purchase_request", Target: pr.Code,
			IP: "127.0.0.1", Detail: fmt.Sprintf("低库存扫描自动生成: 备件 %s ×%d", sp.Name, qty),
		})
	}
	return created
}

// TriggerSpareAutoPurchase POST /spare-auto-purchase-scan 手动触发(admin)
func (h *Handler) TriggerSpareAutoPurchase(c *gin.Context) {
	n := h.ScanSpareAutoPurchase()
	middleware.WriteAudit(h.DB, c, "trigger_spare_auto_purchase", "spare_parts", fmt.Sprintf("手动触发低库存采购扫描，生成 %d 条", n))
	response.Success(c, gin.H{"created": n})
}
