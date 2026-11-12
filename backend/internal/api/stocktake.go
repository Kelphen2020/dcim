package api

import (
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V13 资产盘点 ====================

type stocktakeReq struct {
	SiteID    uint   `json:"site_id"`
	Name      string `json:"name"`
	CabinetID uint   `json:"cabinet_id"`
	Remark    string `json:"remark"`
}

// ListStocktakes 盘点任务列表
func (h *Handler) ListStocktakes(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.StocktakeTask{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if sid := c.Query("site_id"); sid != "" {
		q = q.Where("site_id = ?", sid)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	var total int64
	q.Count(&total)
	var tasks []model.StocktakeTask
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&tasks)
	if tasks == nil {
		tasks = []model.StocktakeTask{}
	}
	response.Success(c, gin.H{"items": tasks, "total": total, "page": page, "size": size})
}

// CreateStocktake 创建盘点任务（草稿），快照账面资产生成明细
func (h *Handler) CreateStocktake(c *gin.Context) {
	var req stocktakeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}
	if req.SiteID == 0 {
		response.Error(c, 400, "须指定站点")
		return
	}
	if req.Name == "" {
		response.Error(c, 400, "须填写盘点名称")
		return
	}
	task := model.StocktakeTask{
		SiteID: req.SiteID, Code: h.genCode("ST"), Name: req.Name,
		CabinetID: req.CabinetID, Status: 0, Remark: req.Remark,
		CreatedBy: c.GetUint("user_id"),
	}
	if err := h.DB.Create(&task).Error; err != nil {
		response.Error(c, 500, "创建失败: "+err.Error())
		return
	}
	// 快照账面在库/在架资产（不含已报废）
	aq := h.DB.Model(&model.Asset{}).Where("site_id = ? AND status <> 4", req.SiteID)
	if req.CabinetID > 0 {
		aq = aq.Where("cabinet_id = ?", req.CabinetID)
	}
	var assets []model.Asset
	aq.Find(&assets)
	cabCode := map[uint]string{}
	{
		var cabs []model.Cabinet
		h.DB.Find(&cabs)
		for _, cb := range cabs {
			cabCode[cb.ID] = cb.Code
		}
	}
	items := make([]model.StocktakeItem, 0, len(assets))
	for _, a := range assets {
		items = append(items, model.StocktakeItem{
			TaskID: task.ID, AssetID: a.ID, AssetCode: a.Code, AssetName: a.Name,
			CabinetID: a.CabinetID, CabinetCode: cabCode[a.CabinetID],
			StartU: a.StartU, BookStatus: a.Status, Result: 0,
		})
	}
	if len(items) > 0 {
		h.DB.CreateInBatches(&items, 200)
	}
	h.DB.Model(&task).Update("total_count", len(items))
	task.TotalCount = len(items)
	go middleware.WriteAudit(h.DB, c, "create", "stocktake", "创建盘点任务 "+task.Code)
	response.Success(c, task)
}

// GetStocktake 任务详情（含明细，支持 result 过滤与分页）
func (h *Handler) GetStocktake(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var task model.StocktakeTask
	if err := h.DB.First(&task, id).Error; err != nil {
		response.Error(c, 404, "盘点任务不存在")
		return
	}
	q := h.DB.Model(&model.StocktakeItem{}).Where("task_id = ?", id)
	if r := c.Query("result"); r != "" {
		q = q.Where("result = ?", r)
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("asset_code ILIKE ? OR asset_name ILIKE ?", like, like)
	}
	var total int64
	q.Count(&total)
	page, size := h.paging(c)
	var items []model.StocktakeItem
	q.Order("id").Offset((page - 1) * size).Limit(size).Find(&items)
	if items == nil {
		items = []model.StocktakeItem{}
	}
	// 结果分布统计
	var stats []struct {
		Result int   `json:"result"`
		N      int64 `json:"n"`
	}
	h.DB.Model(&model.StocktakeItem{}).Select("result, COUNT(*) as n").
		Where("task_id = ?", id).Group("result").Scan(&stats)
	response.Success(c, gin.H{
		"task": task, "items": items, "total": total,
		"page": page, "size": size, "stats": stats,
	})
}

// StartStocktake 开始盘点（草稿→盘点中）
func (h *Handler) StartStocktake(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var task model.StocktakeTask
	if err := h.DB.First(&task, id).Error; err != nil {
		response.Error(c, 404, "盘点任务不存在")
		return
	}
	if task.Status != 0 {
		response.Error(c, 400, "仅草稿任务可开始盘点")
		return
	}
	now := time.Now()
	h.DB.Model(&task).Updates(map[string]interface{}{"status": 1, "started_at": now})
	go middleware.WriteAudit(h.DB, c, "update", "stocktake", "开始盘点 "+task.Code)
	response.Success(c, nil)
}

// CheckStocktakeItem 核对单条明细（批量也走此接口，body 传 items 数组）
type checkItemReq struct {
	ID              uint   `json:"id"`
	Result          int    `json:"result"` // 1相符 2盘亏 3盘盈 4位置不符
	ActualCabinetID uint   `json:"actual_cabinet_id"`
	ActualStartU    int    `json:"actual_start_u"`
	Remark          string `json:"remark"`
	// 盘盈录入
	AssetCode string `json:"asset_code"`
	AssetName string `json:"asset_name"`
}

type checkStocktakeReq struct {
	Items []checkItemReq `json:"items"`
}

// CheckStocktakeItems 批量核对
func (h *Handler) CheckStocktakeItems(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var task model.StocktakeTask
	if err := h.DB.First(&task, id).Error; err != nil {
		response.Error(c, 404, "盘点任务不存在")
		return
	}
	if task.Status != 1 {
		response.Error(c, 400, "任务不在盘点中状态")
		return
	}
	var req checkStocktakeReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		response.Error(c, 400, "参数错误: 须传 items 数组")
		return
	}
	now := time.Now()
	uid := c.GetUint("user_id")
	updated, created := 0, 0
	for _, it := range req.Items {
		if it.Result < 1 || it.Result > 4 {
			continue
		}
		if it.Result == 3 {
			// 盘盈：新建明细（asset_id=0）
			ni := model.StocktakeItem{
				TaskID: task.ID, AssetID: 0, AssetCode: it.AssetCode, AssetName: it.AssetName,
				ActualCabinetID: it.ActualCabinetID, ActualStartU: it.ActualStartU,
				Result: 3, Remark: it.Remark, CheckedBy: uid, CheckedAt: &now,
			}
			if ni.AssetName == "" {
				ni.AssetName = "未入账设备"
			}
			h.DB.Create(&ni)
			created++
			continue
		}
		if it.ID == 0 {
			continue
		}
		updates := map[string]interface{}{
			"result": it.Result, "checked_by": uid, "checked_at": now,
		}
		if it.Remark != "" {
			updates["remark"] = it.Remark
		}
		if it.Result == 4 {
			updates["actual_cabinet_id"] = it.ActualCabinetID
			updates["actual_start_u"] = it.ActualStartU
		}
		res := h.DB.Model(&model.StocktakeItem{}).
			Where("id = ? AND task_id = ?", it.ID, task.ID).Updates(updates)
		updated += int(res.RowsAffected)
	}
	h.refreshStocktakeCounters(&task)
	response.Success(c, gin.H{"updated": updated, "created": created})
}

// ApplyStocktake 差异调账：完成任务并把差异写回资产台账
func (h *Handler) ApplyStocktake(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var task model.StocktakeTask
	if err := h.DB.First(&task, id).Error; err != nil {
		response.Error(c, 404, "盘点任务不存在")
		return
	}
	if task.Status != 1 {
		response.Error(c, 400, "仅盘点中任务可调账完成")
		return
	}
	var body struct {
		Adjust bool `json:"adjust"` // true=写回台账 false=仅完成
	}
	_ = c.ShouldBindJSON(&body)

	applied := gin.H{"marked_missing": 0, "relocated": 0, "created_assets": 0}
	if body.Adjust {
		// 盘亏 → 资产状态改 2(在库下架待定)并备注；位置不符 → 更新机柜/U位；盘盈 → 新建资产(在库)
		var loss []model.StocktakeItem
		h.DB.Where("task_id = ? AND result = 2 AND asset_id > 0", task.ID).Find(&loss)
		for _, it := range loss {
			res := h.DB.Model(&model.Asset{}).Where("id = ?", it.AssetID).Updates(map[string]interface{}{
				"status": 2, "cabinet_id": 0, "start_u": 0,
				"remark": "盘点 " + task.Code + " 盘亏下架待查",
			})
			applied["marked_missing"] = applied["marked_missing"].(int) + int(res.RowsAffected)
		}
		var moved []model.StocktakeItem
		h.DB.Where("task_id = ? AND result = 4 AND asset_id > 0", task.ID).Find(&moved)
		for _, it := range moved {
			if it.ActualCabinetID == 0 {
				continue
			}
			res := h.DB.Model(&model.Asset{}).Where("id = ?", it.AssetID).Updates(map[string]interface{}{
				"cabinet_id": it.ActualCabinetID, "start_u": it.ActualStartU, "status": 1,
			})
			applied["relocated"] = applied["relocated"].(int) + int(res.RowsAffected)
			// 同步明细账面位置
			h.DB.Model(&model.StocktakeItem{}).Where("id = ?", it.ID).Updates(map[string]interface{}{
				"cabinet_id": it.ActualCabinetID, "start_u": it.ActualStartU,
			})
		}
		var surplus []model.StocktakeItem
		h.DB.Where("task_id = ? AND result = 3 AND asset_id = 0", task.ID).Find(&surplus)
		for _, it := range surplus {
			code := it.AssetCode
			if code == "" {
				code = h.genCode("AST")
			} else {
				var dup int64
				h.DB.Model(&model.Asset{}).Where("code = ?", code).Count(&dup)
				if dup > 0 {
					continue
				}
			}
			a := model.Asset{
				Code: code, Name: it.AssetName, Category: "other",
				SiteID: task.SiteID, CabinetID: it.ActualCabinetID, StartU: it.ActualStartU,
				HeightU: 1, Status: 2,
				Remark: "盘点 " + task.Code + " 盘盈入账",
			}
			if err := h.DB.Create(&a).Error; err == nil {
				h.DB.Model(&model.StocktakeItem{}).Where("id = ?", it.ID).
					Update("asset_id", a.ID)
				applied["created_assets"] = applied["created_assets"].(int) + 1
			}
		}
	}
	now := time.Now()
	h.DB.Model(&task).Updates(map[string]interface{}{"status": 2, "finished_at": now})
	h.refreshStocktakeCounters(&task)
	h.DB.Create(&model.Notification{
		UserID: 0, Kind: "stocktake",
		Title:   "盘点完成",
		Content: "盘点任务「" + task.Name + "」(" + task.Code + ") 已完成，差异 " + strconv.Itoa(task.DiffCount) + " 项",
	})
	go middleware.WriteAudit(h.DB, c, "update", "stocktake", "盘点调账完成 "+task.Code)
	response.Success(c, applied)
}

// DeleteStocktake 删除盘点任务（仅草稿）
func (h *Handler) DeleteStocktake(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var task model.StocktakeTask
	if err := h.DB.First(&task, id).Error; err != nil {
		response.Error(c, 404, "盘点任务不存在")
		return
	}
	if task.Status != 0 {
		response.Error(c, 400, "仅草稿任务可删除")
		return
	}
	h.DB.Where("task_id = ?", id).Delete(&model.StocktakeItem{})
	h.DB.Delete(&task)
	go middleware.WriteAudit(h.DB, c, "delete", "stocktake", "删除盘点任务 "+task.Code)
	response.Success(c, nil)
}

// refreshStocktakeCounters 重算任务计数
func (h *Handler) refreshStocktakeCounters(task *model.StocktakeTask) {
	var checked, diff int64
	h.DB.Model(&model.StocktakeItem{}).Where("task_id = ? AND result > 0", task.ID).Count(&checked)
	h.DB.Model(&model.StocktakeItem{}).Where("task_id = ? AND result IN (2,3,4)", task.ID).Count(&diff)
	h.DB.Model(task).Updates(map[string]interface{}{
		"checked_count": checked, "diff_count": diff,
	})
	task.CheckedCount = int(checked)
	task.DiffCount = int(diff)
}
