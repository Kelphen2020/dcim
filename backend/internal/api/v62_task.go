package api

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// V62 巡检任务自动生成与漏检治理
// 闭环: 计划 → 周期任务自动生成(责任人=计划责任人,缺省当班人兜底) → 巡检提交自动核销(Web/扫码) → 周期结束未执行判漏检(通知+Webhook,幂等) → 豁免/改派留痕 → 治理统计(完成率/漏检TOP/趋势/按人)
// 设置: inspection_task_auto(自动生成开关,默认1) / inspection_missed_notify(漏检通知开关,默认1)

// v62Period 按计划周期计算 t 所属周期的 key 与起止
func v62Period(freq string, t time.Time) (string, time.Time, time.Time, bool) {
	mid := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	switch freq {
	case "daily":
		return mid.Format("2006-01-02"), mid, mid.AddDate(0, 0, 1), true
	case "weekly":
		y, w := t.ISOWeek()
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		start := mid.AddDate(0, 0, -(wd - 1))
		return fmt.Sprintf("%d-W%02d", y, w), start, start.AddDate(0, 0, 7), true
	case "monthly":
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
		return t.Format("2006-01"), start, start.AddDate(0, 1, 0), true
	case "quarterly":
		q := (int(t.Month())-1)/3 + 1
		start := time.Date(t.Year(), time.Month((q-1)*3+1), 1, 0, 0, 0, 0, time.Local)
		return fmt.Sprintf("%d-Q%d", t.Year(), q), start, start.AddDate(0, 3, 0), true
	}
	return "", time.Time{}, time.Time{}, false
}

// V62TaskCycle 定时循环入口：任务生成（可关）→ 核销 → 漏检扫描
func (h *Handler) V62TaskCycle() {
	now := time.Now()
	if h.healthSetting("inspection_task_auto", "1") != "0" {
		h.GenerateInspectionTasks(now, 0)
	}
	h.ScanMissedInspectionTasks(now)
}

// v62LinkTask 若周期内已有真实巡检记录(status 1/2)，把待巡检任务核销为已完成
func (h *Handler) v62LinkTask(task *model.InspectionTask) bool {
	if task.Status != 0 {
		return false
	}
	var rec model.InspectionRecord
	err := h.DB.Where("plan_id = ? AND inspected_at >= ? AND inspected_at < ? AND status IN (1,2)",
		task.PlanID, task.PeriodStart, task.PeriodEnd).Order("inspected_at ASC").First(&rec).Error
	if err != nil {
		return false
	}
	done := rec.InspectedAt
	h.DB.Model(task).Updates(map[string]interface{}{"status": 1, "record_id": rec.ID, "done_at": done})
	task.Status = 1
	task.RecordID = rec.ID
	return true
}

// GenerateInspectionTasks 为启用计划生成指定时刻所在周期的任务（幂等：plan_id+period_key 唯一）；planID=0 表示全部计划
func (h *Handler) GenerateInspectionTasks(now time.Time, planID uint) map[string]int {
	stats := map[string]int{"generated": 0, "linked": 0, "plans": 0}
	var plans []model.InspectionPlan
	db := h.DB.Where("status = 1")
	if planID != 0 {
		db = db.Where("id = ?", planID)
	}
	db.Find(&plans)
	for _, p := range plans {
		key, start, end, ok := v62Period(p.Freq, now)
		if !ok {
			continue // once 等不生成任务
		}
		stats["plans"]++
		var task model.InspectionTask
		err := h.DB.Where("plan_id = ? AND period_key = ?", p.ID, key).First(&task).Error
		if err == nil {
			if h.v62LinkTask(&task) {
				stats["linked"]++
			}
			continue
		}
		assignee := p.AssigneeID
		if assignee == 0 {
			if ds := h.OnDutyUser(p.SiteID); ds != nil {
				assignee = ds.UserID
			}
		}
		task = model.InspectionTask{
			PlanID: p.ID, PeriodKey: key, SiteID: p.SiteID, RoomID: p.RoomID,
			Freq: p.Freq, PeriodStart: start, PeriodEnd: end,
			AssigneeID: assignee, Status: 0,
		}
		if h.DB.Create(&task).Error != nil {
			continue
		}
		stats["generated"]++
		if h.v62LinkTask(&task) {
			stats["linked"]++
		}
	}
	if stats["generated"] > 0 {
		log.Printf("V62 巡检任务生成: %v", stats)
	}
	return stats
}

// LinkInspectionTask 巡检记录提交后的核销钩子（SubmitInspection / ScanSubmit 调用）
func (h *Handler) LinkInspectionTask(rec *model.InspectionRecord) {
	if rec == nil || rec.PlanID == 0 {
		return
	}
	var plan model.InspectionPlan
	if h.DB.First(&plan, rec.PlanID).Error != nil {
		return
	}
	key, _, _, ok := v62Period(plan.Freq, rec.InspectedAt)
	if !ok {
		return
	}
	var task model.InspectionTask
	if h.DB.Where("plan_id = ? AND period_key = ?", rec.PlanID, key).First(&task).Error != nil {
		return
	}
	if task.Status != 0 {
		return
	}
	done := rec.InspectedAt
	if err := h.DB.Model(&task).Updates(map[string]interface{}{"status": 1, "record_id": rec.ID, "done_at": done}).Error; err != nil {
		log.Printf("V62 任务核销失败 task=%d: %v", task.ID, err)
	}
}

// ScanMissedInspectionTasks 周期已结束仍未执行的任务判定为漏检（幂等：状态迁移 0→2）
func (h *Handler) ScanMissedInspectionTasks(now time.Time) int {
	var tasks []model.InspectionTask
	h.DB.Where("status = 0 AND period_end <= ?", now).Find(&tasks)
	missed := 0
	notifyOn := h.healthSetting("inspection_missed_notify", "1") != "0"
	for i := range tasks {
		t := &tasks[i]
		updates := map[string]interface{}{"status": 2}
		var plan model.InspectionPlan
		h.DB.First(&plan, t.PlanID)
		if notifyOn && !t.MissNotified {
			msg := fmt.Sprintf("巡检计划「%s」周期 %s 已结束仍未执行，判定为漏检，请说明原因或安排补检", plan.Name, t.PeriodKey)
			h.DB.Create(&model.Notification{
				UserID: t.AssigneeID, Kind: "inspection_missed",
				Title: "巡检漏检提醒", Content: msg,
				RefType: "inspection_task", RefID: t.ID,
			})
			h.DispatchWebhook(model.EvInspectionMissed, "[漏检] "+plan.Name+" "+t.PeriodKey, msg, map[string]interface{}{
				"task_id": t.ID, "plan_id": t.PlanID, "plan_name": plan.Name, "period_key": t.PeriodKey,
			})
			updates["miss_notified"] = true
		}
		if err := h.DB.Model(t).Updates(updates).Error; err == nil {
			missed++
		}
	}
	if missed > 0 {
		log.Printf("V62 漏检扫描: %d 条", missed)
	}
	return missed
}

// ---------- HTTP ----------

type taskPlus struct {
	model.InspectionTask
	PlanName     string `json:"plan_name"`
	SiteName     string `json:"site_name"`
	AssigneeName string `json:"assignee_name"`
	RecordCode   string `json:"record_code"`
}

// ListInspectionTasks GET /inspection-tasks
func (h *Handler) ListInspectionTasks(c *gin.Context) {
	page, size := h.paging(c)
	q := h.DB.Model(&model.InspectionTask{})
	if scope := middleware.GetSiteScope(c); scope != nil {
		q = q.Where("site_id IN ?", scope)
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("plan_id"); v != "" && v != "0" {
		q = q.Where("plan_id = ?", v)
	}
	if v := c.Query("site_id"); v != "" && v != "0" {
		q = q.Where("site_id = ?", v)
	}
	if v := c.Query("assignee_id"); v != "" && v != "0" {
		q = q.Where("assignee_id = ?", v)
	}
	if v := c.Query("period_key"); v != "" {
		q = q.Where("period_key = ?", v)
	}
	if v := c.Query("date_from"); v != "" {
		q = q.Where("period_start >= ?", v)
	}
	if v := c.Query("date_to"); v != "" {
		q = q.Where("period_start < ?", v+" 23:59:59")
	}
	var total int64
	q.Count(&total)
	var items []model.InspectionTask
	q.Order("period_start DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&items)
	if items == nil {
		items = []model.InspectionTask{}
	}
	// 批量富化
	planSet, siteSet, userSet, recSet := map[uint]bool{}, map[uint]bool{}, map[uint]bool{}, map[uint]bool{}
	for _, t := range items {
		planSet[t.PlanID] = true
		siteSet[t.SiteID] = true
		userSet[t.AssigneeID] = true
		if t.RecordID != 0 {
			recSet[t.RecordID] = true
		}
	}
	planNames, siteNames, userNames, recCodes := map[uint]string{}, map[uint]string{}, map[uint]string{}, map[uint]string{}
	if len(planSet) > 0 {
		ids := make([]uint, 0, len(planSet))
		for id := range planSet {
			ids = append(ids, id)
		}
		var plans []model.InspectionPlan
		h.DB.Select("id, name").Where("id IN ?", ids).Find(&plans)
		for _, p := range plans {
			planNames[p.ID] = p.Name
		}
	}
	if len(siteSet) > 0 {
		ids := make([]uint, 0, len(siteSet))
		for id := range siteSet {
			ids = append(ids, id)
		}
		var sites []model.Site
		h.DB.Select("id, name").Where("id IN ?", ids).Find(&sites)
		for _, s := range sites {
			siteNames[s.ID] = s.Name
		}
	}
	if len(userSet) > 0 {
		ids := make([]uint, 0, len(userSet))
		for id := range userSet {
			ids = append(ids, id)
		}
		var users []model.User
		h.DB.Select("id, display_name, username").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			name := u.DisplayName
			if name == "" {
				name = u.Username
			}
			userNames[u.ID] = name
		}
	}
	if len(recSet) > 0 {
		ids := make([]uint, 0, len(recSet))
		for id := range recSet {
			ids = append(ids, id)
		}
		var recs []model.InspectionRecord
		h.DB.Select("id, report_code").Where("id IN ?", ids).Find(&recs)
		for _, r := range recs {
			recCodes[r.ID] = r.ReportCode
		}
	}
	out := make([]taskPlus, 0, len(items))
	for _, t := range items {
		out = append(out, taskPlus{
			InspectionTask: t,
			PlanName:       planNames[t.PlanID],
			SiteName:       siteNames[t.SiteID],
			AssigneeName:   userNames[t.AssigneeID],
			RecordCode:     recCodes[t.RecordID],
		})
	}
	response.Paginated(c, out, total, page, size)
}

// InspectionTaskStats GET /inspection-tasks/stats 漏检治理统计
func (h *Handler) InspectionTaskStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days != 7 && days != 30 && days != 90 {
		days = 30
	}
	now := time.Now()
	since := now.AddDate(0, 0, -days)
	q := func() *gorm.DB {
		db := h.DB.Model(&model.InspectionTask{}).Where("period_start >= ?", since)
		if scope := middleware.GetSiteScope(c); scope != nil {
			db = db.Where("site_id IN ?", scope)
		}
		if v := c.Query("site_id"); v != "" && v != "0" {
			db = db.Where("site_id = ?", v)
		}
		return db
	}
	var cntDone, cntMissed, cntExempt, cntPending, cntTotal int64
	q().Where("status = 1").Count(&cntDone)
	q().Where("status = 2").Count(&cntMissed)
	q().Where("status = 3").Count(&cntExempt)
	q().Where("status = 0").Count(&cntPending)
	q().Count(&cntTotal)
	rate := 0.0
	if cntDone+cntMissed > 0 {
		rate = float64(cntDone) / float64(cntDone+cntMissed) * 100
	}
	// 当前周期（覆盖此刻）的任务盘面
	var curPending, curDone, curMissed int64
	cq := func() *gorm.DB {
		db := h.DB.Model(&model.InspectionTask{}).Where("period_start <= ? AND period_end > ?", now, now)
		if scope := middleware.GetSiteScope(c); scope != nil {
			db = db.Where("site_id IN ?", scope)
		}
		return db
	}
	cq().Where("status = 0").Count(&curPending)
	cq().Where("status = 1").Count(&curDone)
	cq().Where("status = 2").Count(&curMissed)
	// 每日趋势
	type trendRow struct {
		D      string `json:"d"`
		Total  int64  `json:"total"`
		Done   int64  `json:"done"`
		Missed int64  `json:"missed"`
	}
	var trend []trendRow
	h.DB.Raw(`SELECT to_char(period_start,'YYYY-MM-DD') AS d, COUNT(*) AS total,
		SUM(CASE WHEN status=1 THEN 1 ELSE 0 END) AS done,
		SUM(CASE WHEN status=2 THEN 1 ELSE 0 END) AS missed
		FROM t_inspection_task WHERE period_start >= ? GROUP BY d ORDER BY d`, since).Scan(&trend)
	if trend == nil {
		trend = []trendRow{}
	}
	// 漏检 TOP 计划
	type topRow struct {
		PlanID   uint   `json:"plan_id"`
		PlanName string `json:"plan_name"`
		Cnt      int64  `json:"cnt"`
	}
	var tops []topRow
	h.DB.Raw(`SELECT t.plan_id, p.name AS plan_name, COUNT(*) AS cnt
		FROM t_inspection_task t LEFT JOIN t_inspection_plan p ON p.id = t.plan_id
		WHERE t.status = 2 AND t.period_start >= ? GROUP BY t.plan_id, p.name ORDER BY cnt DESC LIMIT 5`, since).Scan(&tops)
	if tops == nil {
		tops = []topRow{}
	}
	// 按责任人完成率
	type assigneeRow struct {
		AssigneeID uint   `json:"assignee_id"`
		Name       string `json:"name"`
		Done       int64  `json:"done"`
		Missed     int64  `json:"missed"`
		Rate       float64 `json:"rate"`
	}
	type aggrRow struct {
		AssigneeID uint  `json:"assignee_id"`
		Done       int64 `json:"done"`
		Missed     int64 `json:"missed"`
	}
	var aggr []aggrRow
	h.DB.Raw(`SELECT assignee_id, SUM(CASE WHEN status=1 THEN 1 ELSE 0 END) AS done,
		SUM(CASE WHEN status=2 THEN 1 ELSE 0 END) AS missed
		FROM t_inspection_task WHERE period_start >= ? AND assignee_id > 0
		GROUP BY assignee_id HAVING SUM(CASE WHEN status IN (1,2) THEN 1 ELSE 0 END) > 0
		ORDER BY missed DESC, done DESC LIMIT 10`, since).Scan(&aggr)
	byAssignee := make([]assigneeRow, 0, len(aggr))
	for _, a := range aggr {
		var u model.User
		name := ""
		if h.DB.Select("display_name, username").First(&u, a.AssigneeID).Error == nil {
			name = u.DisplayName
			if name == "" {
				name = u.Username
			}
		}
		r := 0.0
		if a.Done+a.Missed > 0 {
			r = float64(a.Done) / float64(a.Done+a.Missed) * 100
		}
		byAssignee = append(byAssignee, assigneeRow{AssigneeID: a.AssigneeID, Name: name, Done: a.Done, Missed: a.Missed, Rate: float64(int(r*10)) / 10})
	}
	response.Success(c, gin.H{
		"window_days": days,
		"totals": gin.H{
			"generated": cntTotal, "done": cntDone, "missed": cntMissed,
			"exempt": cntExempt, "pending": cntPending,
			"done_rate": float64(int(rate*10)) / 10,
		},
		"current":     gin.H{"pending": curPending, "done": curDone, "missed": curMissed},
		"trend":       trend,
		"top_missed":  tops,
		"by_assignee": byAssignee,
	})
}

// GenerateInspectionTasksAPI POST /inspection-tasks/generate 手动生成（admin）
func (h *Handler) GenerateInspectionTasksAPI(c *gin.Context) {
	now := time.Now()
	var req struct {
		Date   string `json:"date"`
		PlanID uint   `json:"plan_id"` // 可选：仅为指定计划生成（回补/测试用）
	}
	_ = c.ShouldBindJSON(&req) // body 可空
	if req.Date != "" {
		t, err := time.ParseInLocation("2006-01-02", req.Date, time.Local)
		if err != nil {
			response.BadRequest(c, "date 格式须为 yyyy-MM-dd")
			return
		}
		now = t.Add(12 * time.Hour) // 取当日中午，避免时区边界
	}
	stats := h.GenerateInspectionTasks(now, req.PlanID)
	go middleware.WriteAudit(h.DB, c, "generate", "inspection_task", fmt.Sprintf("date=%v plan_id=%v stats=%v", req.Date, req.PlanID, stats))
	response.Success(c, stats)
}

// ScanMissedTasksAPI POST /inspection-tasks/scan-missed 手动触发漏检扫描（admin）
func (h *Handler) ScanMissedTasksAPI(c *gin.Context) {
	n := h.ScanMissedInspectionTasks(time.Now())
	go middleware.WriteAudit(h.DB, c, "scan", "inspection_task", fmt.Sprintf("missed=%d", n))
	response.Success(c, gin.H{"missed": n})
}

// ExemptInspectionTask PUT /inspection-tasks/:id/exempt 豁免（值班员以上，须填原因）
func (h *Handler) ExemptInspectionTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "ID 无效")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Reason == "" {
		response.BadRequest(c, "豁免原因必填")
		return
	}
	var task model.InspectionTask
	if h.DB.First(&task, id).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	if !middleware.InSiteScope(c, task.SiteID) {
		response.Forbidden(c, "无权操作该站点")
		return
	}
	if task.Status != 0 && task.Status != 2 {
		response.BadRequest(c, "仅待巡检或漏检状态的任务可豁免")
		return
	}
	if err := h.DB.Model(&task).Updates(map[string]interface{}{"status": 3, "exempt_reason": req.Reason}).Error; err != nil {
		response.Error(c, 500, "豁免失败: "+err.Error())
		return
	}
	var plan model.InspectionPlan
	h.DB.First(&plan, task.PlanID)
	go middleware.WriteAudit(h.DB, c, "exempt", "inspection_task", fmt.Sprintf("%s %s 原因:%s", plan.Name, task.PeriodKey, req.Reason))
	task.Status = 3
	task.ExemptReason = req.Reason
	response.Success(c, task)
}

// AssignInspectionTask PUT /inspection-tasks/:id/assign 改派（admin）
func (h *Handler) AssignInspectionTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "ID 无效")
		return
	}
	var req struct {
		AssigneeID uint `json:"assignee_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AssigneeID == 0 {
		response.BadRequest(c, "assignee_id 必填")
		return
	}
	var task model.InspectionTask
	if h.DB.First(&task, id).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	if task.Status != 0 {
		response.BadRequest(c, "仅待巡检任务可改派")
		return
	}
	var u model.User
	if h.DB.First(&u, req.AssigneeID).Error != nil {
		response.NotFound(c, "被指派人不存在")
		return
	}
	if err := h.DB.Model(&task).Update("assignee_id", req.AssigneeID).Error; err != nil {
		response.Error(c, 500, "改派失败: "+err.Error())
		return
	}
	var plan model.InspectionPlan
	h.DB.First(&plan, task.PlanID)
	name := u.DisplayName
	if name == "" {
		name = u.Username
	}
	h.DB.Create(&model.Notification{
		UserID: req.AssigneeID, Kind: "inspection",
		Title:   "巡检任务指派",
		Content: fmt.Sprintf("巡检任务「%s」(%s) 已指派给你，请在周期内完成", plan.Name, task.PeriodKey),
		RefType: "inspection_task", RefID: task.ID,
	})
	go middleware.WriteAudit(h.DB, c, "assign", "inspection_task", fmt.Sprintf("task=%d → %s", task.ID, name))
	task.AssigneeID = req.AssigneeID
	response.Success(c, task)
}
