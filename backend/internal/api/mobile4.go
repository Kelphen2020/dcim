package api

// mobile4.go — M6 移动端功能完善
// 附件表（工单/巡检照片真闭环）+ overview 设备统计块 + 附件查询接口
//
// GET /api/v1/mobile/attachments?ref_type=work_order&ref_id=1  附件列表
// /mobile/upload 升级：落盘同时写 t_attachment 记录（见 mobile.go MobileUpload 尾部调用 saveAttachment）

import (
	"fmt"
	"time"

	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)
// Attachment 通用附件记录（工单照片/巡检照片等）
type Attachment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RefType   string    `gorm:"size:30;index:idx_attach_ref;not null" json:"ref_type"` // work_order / inspection
	RefID     uint      `gorm:"index:idx_attach_ref;not null" json:"ref_id"`
	URL       string    `gorm:"size:300;not null" json:"url"`
	Name      string    `gorm:"size:200" json:"name"`
	Size      int64     `json:"size"`
	UserID    uint      `gorm:"index" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (Attachment) TableName() string { return "t_attachment" }

// saveAttachment 上传成功后持久化附件记录（ref 为空也记录，便于追溯）
func (h *Handler) saveAttachment(c *gin.Context, url, name string, size int64) {
	refType := c.PostForm("ref_type")
	var refID uint
	fmt.Sscanf(c.PostForm("ref_id"), "%d", &refID)
	uid := c.GetUint("user_id")
	att := Attachment{RefType: refType, RefID: refID, URL: url, Name: name, Size: size, UserID: uid}
	h.DB.Create(&att) // 附件记录失败不阻断上传本身
}

// ListAttachments GET /mobile/attachments?ref_type=&ref_id=
func (h *Handler) ListAttachments(c *gin.Context) {
	q := h.DB.Model(&Attachment{})
	if rt := c.Query("ref_type"); rt != "" {
		q = q.Where("ref_type = ?", rt)
	}
	if rid := c.Query("ref_id"); rid != "" {
		q = q.Where("ref_id = ?", rid)
	}
	var items []Attachment
	q.Order("id ASC").Limit(100).Find(&items)
	if items == nil {
		items = []Attachment{}
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

// mobileAssetStats 设备统计块（overview 用）
func (h *Handler) mobileAssetStats(siteID uint) gin.H {
	type kv struct {
		K string `json:"k"`
		N int64  `json:"n"`
	}
	scope := func(q *gorm.DB) *gorm.DB {
		if siteID > 0 {
			return q.Where("site_id = ?", siteID)
		}
		return q
	}
	var total int64
	scope(h.DB.Model(&model.Asset{})).Count(&total)

	var byStatus []kv
	scope(h.DB.Model(&model.Asset{})).Select("status::text as k, COUNT(*) as n").Group("status").Scan(&byStatus)
	statusMap := map[string]int64{"1": 0, "2": 0, "3": 0, "4": 0}
	for _, r := range byStatus {
		statusMap[r.K] = r.N
	}

	var byCat []kv
	scope(h.DB.Model(&model.Asset{})).Select("category as k, COUNT(*) as n").Group("category").Order("n DESC").Scan(&byCat)
	cats := make([]gin.H, 0, len(byCat))
	for _, r := range byCat {
		cats = append(cats, gin.H{"category": r.K, "count": r.N})
	}

	var cabTotal, cabUsed int64
	scope(h.DB.Model(&model.Cabinet{})).Count(&cabTotal)
	// 有在架资产的机柜数
	cuq := h.DB.Model(&model.Asset{}).Where("status = 1 AND cabinet_id > 0").Distinct("cabinet_id")
	if siteID > 0 {
		cuq = cuq.Where("site_id = ?", siteID)
	}
	cuq.Count(&cabUsed)

	return gin.H{
		"total":         total,
		"rack_on":       statusMap["1"],
		"rack_off":      statusMap["2"],
		"repair":        statusMap["3"],
		"scrapped":      statusMap["4"],
		"by_category":   cats,
		"cabinet_total": cabTotal,
		"cabinet_used":  cabUsed,
	}
}

// mobileTodayPlans M6-E2: 今日巡检计划清单（含执行状态）
func (h *Handler) mobileTodayPlans(siteID uint, uid uint) []gin.H {
	now := time.Now()
	today0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	type planRow struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		Freq     string `json:"freq"`
		SiteID   uint   `json:"site_id"`
		Items    string `json:"items"`
		Assignee uint   `json:"assignee_id"`
	}
	var plans []planRow
	pq := h.DB.Table("t_inspection_plan").
		Select("id, name, freq, site_id, items, assignee_id").
		Where("status = 1")
	if siteID > 0 {
		pq = pq.Where("site_id = ? OR site_id = 0", siteID)
	}
	pq.Limit(20).Scan(&plans)

	// 今日已执行的 plan_id 集合
	var doneIDs []uint
	h.DB.Table("t_inspection_record").
		Where("inspected_at >= ?", today0).
		Distinct().Pluck("plan_id", &doneIDs)
	doneSet := map[uint]bool{}
	for _, id := range doneIDs {
		doneSet[id] = true
	}

	out := make([]gin.H, 0, len(plans))
	for _, p := range plans {
		mine := p.Assignee == 0 || p.Assignee == uid
		if !mine {
			continue
		}
		out = append(out, gin.H{
			"id": p.ID, "name": p.Name, "freq": p.Freq,
			"done": doneSet[p.ID],
		})
	}
	return out
}

// mobileSpareWarn M6-E4: 备件低库存预警摘要
func (h *Handler) mobileSpareWarn() gin.H {
	type spRow struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Code        string `json:"code"`
		Stock       int    `json:"stock"`
		SafetyStock int    `gorm:"column:safety_stock" json:"safety_stock"`
		Category    string `json:"category"`
	}
	var low []spRow
	h.DB.Table("t_spare_part").
		Select("id, name, code, stock, safety_stock, category").
		Where("stock <= safety_stock AND safety_stock > 0").
		Order("(safety_stock - stock) DESC").Limit(5).Scan(&low)

	var totalLow int64
	h.DB.Table("t_spare_part").
		Where("stock <= safety_stock AND safety_stock > 0").Count(&totalLow)

	items := make([]gin.H, 0, len(low))
	for _, s := range low {
		items = append(items, gin.H{
			"id": s.ID, "name": s.Name, "code": s.Code,
			"stock": s.Stock, "safety_stock": s.SafetyStock, "category": s.Category,
		})
	}
	return gin.H{"low_count": totalLow, "top": items}
}
