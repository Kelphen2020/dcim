package api

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// genFaultCaseCode FC-yyyyMMdd-NNN
func (h *Handler) genFaultCaseCode() string {
	today := time.Now().Format("20060102")
	var cnt int64
	h.DB.Table("t_fault_case").Where("code LIKE ?", "FC-"+today+"%").Count(&cnt)
	return fmt.Sprintf("FC-%s-%03d", today, cnt+1)
}

// genCode 补 FC 前缀分支（供其它调用点复用）
// 注：不修改原 genCode，案例库单独走 genFaultCaseCode

type faultCaseReq struct {
	SiteID      uint   `json:"site_id"`
	Title       string `json:"title"`
	Symptom     string `json:"symptom"`
	Cause       string `json:"cause"`
	Solution    string `json:"solution"`
	Category    string `json:"category"`
	PointType   string `json:"point_type"`
	AssetID     uint   `json:"asset_id"`
	Keywords    string `json:"keywords"`
	SourceWOID  uint   `json:"source_wo_id"`
	DowntimeMin int    `json:"downtime_min"`
}

// ListFaultCases GET /fault-cases?site_id=&category=&point_type=&asset_id=&keyword=&page=&size=
func (h *Handler) ListFaultCases(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.FaultCase{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ? OR site_id = 0", scope)
	}
	if v := c.Query("site_id"); v != "" && v != "0" {
		q = q.Where("site_id = ?", v)
	}
	if v := c.Query("category"); v != "" {
		q = q.Where("category = ?", v)
	}
	if v := c.Query("point_type"); v != "" {
		q = q.Where("point_type = ?", v)
	}
	if v := c.Query("asset_id"); v != "" && v != "0" {
		q = q.Where("asset_id = ?", v)
	}
	if v := c.Query("keyword"); v != "" {
		kw := "%" + v + "%"
		q = q.Where("title LIKE ? OR symptom LIKE ? OR cause LIKE ? OR solution LIKE ? OR keywords LIKE ?", kw, kw, kw, kw, kw)
	}
	var total int64
	q.Count(&total)
	var items []model.FaultCase
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	if items == nil {
		items = []model.FaultCase{}
	}
	response.Paginated(c, items, total, page, size)
}

// GetFaultCase GET /fault-cases/:id（阅读+1）
func (h *Handler) GetFaultCase(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var fc model.FaultCase
	if err := h.DB.First(&fc, id).Error; err != nil {
		response.NotFound(c, "案例不存在")
		return
	}
	h.DB.Model(&fc).UpdateColumn("usage_count", fc.UsageCount+1)
	fc.UsageCount++
	response.Success(c, fc)
}

func (h *Handler) CreateFaultCase(c *gin.Context) {
	var req faultCaseReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		response.BadRequest(c, "标题必填")
		return
	}
	if req.SiteID != 0 && !middleware.InSiteScope(c, req.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if req.Category == "" {
		req.Category = "other"
	}
	uidN := middleware.GetUserID(c)
	fc := model.FaultCase{
		Code: h.genFaultCaseCode(), SiteID: req.SiteID, Title: strings.TrimSpace(req.Title),
		Symptom: req.Symptom, Cause: req.Cause, Solution: req.Solution,
		Category: req.Category, PointType: req.PointType, AssetID: req.AssetID,
		Keywords: req.Keywords, SourceWOID: req.SourceWOID, DowntimeMin: req.DowntimeMin,
		CreatedBy: uidN,
	}
	if err := h.DB.Create(&fc).Error; err != nil {
		response.ServerError(c, "创建失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_fault_case", fc.Code, fc.Title)
	response.Created(c, fc)
}

func (h *Handler) UpdateFaultCase(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var fc model.FaultCase
	if err := h.DB.First(&fc, id).Error; err != nil {
		response.NotFound(c, "案例不存在")
		return
	}
	var req faultCaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数无效")
		return
	}
	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Title); v != "" {
		updates["title"] = v
	}
	for k, v := range map[string]string{
		"symptom": req.Symptom, "cause": req.Cause, "solution": req.Solution,
		"category": req.Category, "point_type": req.PointType, "keywords": req.Keywords,
	} {
		updates[k] = v
	}
	updates["asset_id"] = req.AssetID
	updates["downtime_min"] = req.DowntimeMin
	h.DB.Model(&fc).Updates(updates)
	middleware.WriteAudit(h.DB, c, "update_fault_case", fc.Code, req.Title)
	h.DB.First(&fc, id)
	response.Success(c, fc)
}

func (h *Handler) DeleteFaultCase(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var fc model.FaultCase
	if err := h.DB.First(&fc, id).Error; err != nil {
		response.NotFound(c, "案例不存在")
		return
	}
	h.DB.Delete(&fc)
	middleware.WriteAudit(h.DB, c, "delete_fault_case", fc.Code, fc.Title)
	response.Success(c, gin.H{"deleted": id})
}

// scoreCase 案例与告警/测点/关键词的匹配得分
func scoreCase(fc model.FaultCase, pointType, category string, text string) int {
	score := 0
	text = strings.ToLower(text)
	if pointType != "" && fc.PointType == pointType {
		score += 40
	}
	if category != "" && fc.Category == category {
		score += 25
	}
	// 关键词命中
	for _, kw := range strings.Split(strings.ToLower(fc.Keywords), ",") {
		kw = strings.TrimSpace(kw)
		if kw != "" && strings.Contains(text, kw) {
			score += 15
		}
	}
	// 标题/现象词命中（粗粒度：按 2 字滑窗）
	fields := strings.ToLower(fc.Title + " " + fc.Symptom)
	for _, w := range tokenize(text) {
		if strings.Contains(fields, w) {
			score += 5
		}
	}
	score += fc.UsageCount // 热度加权
	return score
}

// tokenize 简易中文/英文分词（英文按空格，中文按 2-gram）
func tokenize(s string) []string {
	s = strings.ToLower(s)
	out := []string{}
	// 英文词
	for _, w := range strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if len(w) >= 2 {
			out = append(out, w)
		}
	}
	// 中文 2-gram
	rs := []rune(s)
	for i := 0; i+1 < len(rs); i++ {
		if rs[i] >= 0x4e00 && rs[i] <= 0x9fff && rs[i+1] >= 0x4e00 && rs[i+1] <= 0x9fff {
			out = append(out, string(rs[i:i+2]))
		}
	}
	return out
}

// RecommendFaultCases GET /fault-cases/recommend?point_id= | &alert_id= | &text=&point_type=&category=
// 返回按得分排序的相似历史案例（供告警弹窗/工单页/测点页推荐）
func (h *Handler) RecommendFaultCases(c *gin.Context) {
	var pointType, category, text string
	var siteID uint
	if v := c.Query("point_id"); v != "" && v != "0" {
		var p model.MonitoringPoint
		if h.DB.First(&p, v).Error == nil {
			pointType, siteID, text = p.Type, p.SiteID, p.Name+" "+p.Description
		}
	}
	if v := c.Query("alert_id"); v != "" && v != "0" {
		var al model.Alert
		if h.DB.First(&al, v).Error == nil {
			text = al.Message
			siteID = al.SiteID
			if al.PointID != nil {
				var p model.MonitoringPoint
				if h.DB.Select("type").First(&p, *al.PointID).Error == nil {
					pointType = p.Type
				}
			}
		}
	}
	if v := c.Query("text"); v != "" {
		text = v
	}
	if v := c.Query("point_type"); v != "" {
		pointType = v
	}
	if v := c.Query("category"); v != "" {
		category = v
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	var cases []model.FaultCase
	q := h.DB
	if siteID != 0 {
		q = q.Where("site_id = ? OR site_id = 0", siteID)
	}
	q.Order("id DESC").Limit(500).Find(&cases)
	type scored struct {
		model.FaultCase
		Score int `json:"score"`
	}
	res := []scored{}
	for _, fc := range cases {
		sc := scoreCase(fc, pointType, category, text)
		if sc > 0 {
			res = append(res, scored{fc, sc})
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Score > res[j].Score })
	if len(res) > limit {
		res = res[:limit]
	}
	if res == nil {
		res = []scored{}
	}
	response.Success(c, gin.H{"items": res, "total": len(res)})
}

// PromoteFromWorkOrder POST /fault-cases/from-work-order/:id  工单完成→一键沉淀为案例
func (h *Handler) PromoteFromWorkOrder(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var wo model.WorkOrder
	if err := h.DB.First(&wo, id).Error; err != nil {
		response.NotFound(c, "工单不存在")
		return
	}
	// 推断类别/测点类型
	category, pointType := "other", ""
	if strings.Contains(wo.Type, "alarm") || wo.Type == "alarm" {
		category = "power"
	}
	var assetName, keywords string
	if wo.AssetID != 0 {
		var a model.Asset
		if h.DB.First(&a, wo.AssetID).Error == nil {
			assetName = a.Name
			category = assetCategory(a.Category)
			keywords = a.Name + "," + a.Code
		}
	}
	downtime := 0
	if wo.AcceptedAt != nil && wo.FinishedAt != nil {
		downtime = int(wo.FinishedAt.Sub(*wo.AcceptedAt).Minutes())
	}
	uidN := middleware.GetUserID(c)
	fc := model.FaultCase{
		Code: h.genFaultCaseCode(), SiteID: wo.SiteID,
		Title:       wo.Title,
		Symptom:     wo.Description,
		Cause:       "", // 待人工补充
		Solution:    wo.Resolution,
		Category:    category,
		PointType:   pointType,
		AssetID:     wo.AssetID,
		Keywords:    keywords,
		SourceWOID:  wo.ID,
		DowntimeMin: downtime,
		CreatedBy:   uidN,
	}
	_ = assetName
	if err := h.DB.Create(&fc).Error; err != nil {
		response.ServerError(c, "沉淀失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "promote_fault_case", fc.Code, wo.Code)
	response.Created(c, fc)
}

func assetCategory(cat string) string {
	m := map[string]string{
		"server": "server", "network": "network", "storage": "storage",
		"power": "power", "cooling": "hvac", "security": "security",
	}
	if v, ok := m[cat]; ok {
		return v
	}
	return "other"
}
