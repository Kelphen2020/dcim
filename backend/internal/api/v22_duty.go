package api

import (
	"strconv"
	"strings"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== V22-③ 值班排班 ====================
// DutyShift: date+shift(day/night/full)+user+site(0=全部)
// 当班人查询: GET /duty/current → 今天此刻当班的人(供告警/工单自动指派)
// 交接清单: GET /duty/handover → 未闭环告警+进行中工单+今日巡检

// ListDutyShifts GET /duty-shifts?site_id=&month=2026-09&start=&end=
func (h *Handler) ListDutyShifts(c *gin.Context) {
	q := h.DB.Model(&model.DutyShift{})
	if v := c.Query("site_id"); v != "" && v != "0" {
		q = q.Where("site_id = ? OR site_id = 0", v)
	}
	if v := c.Query("month"); v != "" {
		if t, err := time.Parse("2006-01", v); err == nil {
			q = q.Where("duty_date >= ? AND duty_date < ?", t, t.AddDate(0, 1, 0))
		}
	}
	if v := c.Query("start"); v != "" {
		q = q.Where("duty_date >= ?", v)
	}
	if v := c.Query("end"); v != "" {
		q = q.Where("duty_date <= ?", v)
	}
	var items []model.DutyShift
	q.Order("duty_date, shift").Limit(500).Find(&items)
	if items == nil {
		items = []model.DutyShift{}
	}
	// 富化用户名
	userNames := map[uint]string{}
	var users []model.User
	h.DB.Select("id, display_name, username").Find(&users)
	for _, u := range users {
		if u.DisplayName != "" {
			userNames[u.ID] = u.DisplayName
		} else {
			userNames[u.ID] = u.Username
		}
	}
	type row struct {
		model.DutyShift
		UserName string `json:"user_name"`
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		rows = append(rows, row{it, userNames[it.UserID]})
	}
	response.Success(c, gin.H{"items": rows, "total": len(rows)})
}

type dutyReq struct {
	SiteID   uint   `json:"site_id"`
	UserID   uint   `json:"user_id"`
	DutyDate string `json:"duty_date"` // yyyy-MM-dd
	Shift    string `json:"shift"`     // day/night/full
	Remark   string `json:"remark"`
}

// CreateDutyShift POST /duty-shifts（管理员；支持批量: duty_date 逗号分隔多天）
func (h *Handler) CreateDutyShift(c *gin.Context) {
	var req dutyReq
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 || strings.TrimSpace(req.DutyDate) == "" {
		response.BadRequest(c, "值班人与日期必填")
		return
	}
	var u model.User
	if h.DB.First(&u, req.UserID).Error != nil {
		response.BadRequest(c, "值班人不存在")
		return
	}
	shift := req.Shift
	if shift != "day" && shift != "night" && shift != "full" {
		shift = "full"
	}
	created := 0
	for _, ds := range strings.Split(req.DutyDate, ",") {
		ds = strings.TrimSpace(ds)
		if ds == "" {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02", ds, time.Local)
		if err != nil {
			continue
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		// 幂等: 同日同班同人去重
		var cnt int64
		h.DB.Model(&model.DutyShift{}).Where("user_id = ? AND duty_date = ? AND shift = ?", req.UserID, t, shift).Count(&cnt)
		if cnt > 0 {
			continue
		}
		h.DB.Create(&model.DutyShift{SiteID: req.SiteID, UserID: req.UserID, DutyDate: t, Shift: shift, Remark: req.Remark})
		created++
	}
	if created == 0 {
		response.BadRequest(c, "未创建任何排班（日期无效或已存在）")
		return
	}
	middleware.WriteAudit(h.DB, c, "create_duty_shift", req.DutyDate, "user="+strconv.Itoa(int(req.UserID)))
	response.Created(c, gin.H{"created": created})
}

func (h *Handler) DeleteDutyShift(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var ds model.DutyShift
	if err := h.DB.First(&ds, id).Error; err != nil {
		response.NotFound(c, "排班不存在")
		return
	}
	h.DB.Delete(&ds)
	middleware.WriteAudit(h.DB, c, "delete_duty_shift", ds.DutyDate.Format("2006-01-02"), strconv.Itoa(int(ds.UserID)))
	response.Success(c, gin.H{"deleted": id})
}

// currentShift 当前时刻班次: 8-20 白班 / 其余夜班
func currentShift(now time.Time) string {
	hh := now.Hour()
	if hh >= 8 && hh < 20 {
		return "day"
	}
	return "night"
}

// OnDutyUser 查询当前当班人(后端内部复用): 精确站点+班次 > 全站点+班次 > 精确+full > 全站+full
func (h *Handler) OnDutyUser(siteID uint) *model.DutyShift {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	shift := currentShift(now)
	for _, q := range [][2]interface{}{
		{"site_id = ? AND duty_date = ? AND shift = ?", []interface{}{siteID, today, shift}},
		{"site_id = 0 AND duty_date = ? AND shift = ?", []interface{}{today, shift}},
		{"site_id = ? AND duty_date = ? AND shift = 'full'", []interface{}{siteID, today}},
		{"site_id = 0 AND duty_date = ? AND shift = 'full'", []interface{}{today}},
	} {
		var ds model.DutyShift
		if err := h.DB.Where(q[0].(string), q[1].([]interface{})...).First(&ds).Error; err == nil {
			return &ds
		}
	}
	return nil
}

// CurrentDuty GET /duty/current?site_id= → 当前当班人（登录可读）
func (h *Handler) CurrentDuty(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.DefaultQuery("site_id", "1"))
	ds := h.OnDutyUser(uint(siteID))
	out := gin.H{"on_duty": false, "shift": currentShift(time.Now())}
	if ds != nil {
		var u model.User
		h.DB.First(&u, ds.UserID)
		out = gin.H{
			"on_duty": true, "shift": ds.Shift, "user_id": u.ID,
			"user_name": u.DisplayName, "username": u.Username, "phone": u.Phone,
			"duty_date": ds.DutyDate.Format("2006-01-02"), "remark": ds.Remark,
		}
	}
	response.Success(c, out)
}

// DutyHandover GET /duty/handover?site_id= 交接清单:
// 未闭环告警 + 进行中工单(含SLA状态) + 今日巡检 + 活跃风暴
func (h *Handler) DutyHandover(c *gin.Context) {
	siteID, _ := strconv.Atoi(c.DefaultQuery("site_id", "0"))

	// 未闭环告警
	alQ := h.DB.Model(&model.Alert{}).Where("status < 3")
	if siteID > 0 {
		alQ = alQ.Where("site_id = ?", siteID)
	}
	var alerts []model.Alert
	alQ.Order("severity DESC, triggered_at DESC").Limit(50).Find(&alerts)
	if alerts == nil {
		alerts = []model.Alert{}
	}
	// 进行中工单
	woQ := h.DB.Model(&model.WorkOrder{}).Where("status IN (1,2)")
	if siteID > 0 {
		woQ = woQ.Where("site_id = ?", siteID)
	}
	var orders []model.WorkOrder
	woQ.Order("priority DESC, id DESC").Limit(50).Find(&orders)
	if orders == nil {
		orders = []model.WorkOrder{}
	}
	// 今日巡检
	today := time.Now().Truncate(24 * time.Hour)
	recQ := h.DB.Model(&model.InspectionRecord{}).Where("inspected_at >= ?", today)
	if siteID > 0 {
		recQ = recQ.Where("site_id = ?", siteID)
	}
	var recs []model.InspectionRecord
	recQ.Order("id DESC").Limit(20).Find(&recs)
	if recs == nil {
		recs = []model.InspectionRecord{}
	}
	// 活跃风暴
	var storms []model.AlertStorm
	stQ := h.DB.Where("status = 0")
	if siteID > 0 {
		stQ = stQ.Where("site_id = ?", siteID)
	}
	stQ.Order("id DESC").Limit(10).Find(&storms)
	if storms == nil {
		storms = []model.AlertStorm{}
	}
	response.Success(c, gin.H{
		"open_alerts": alerts, "open_alert_count": len(alerts),
		"open_orders": orders, "open_order_count": len(orders),
		"today_inspections": recs,
		"active_storms": storms,
		"generated_at":  time.Now().Format("2006-01-02 15:04:05"),
	})
}

// ==================== 风暴查询接口 ====================

// ListAlertStorms GET /alert-storms?status=0
func (h *Handler) ListAlertStorms(c *gin.Context) {
	q := h.DB.Model(&model.AlertStorm{})
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("site_id"); v != "" && v != "0" {
		q = q.Where("site_id = ?", v)
	}
	var items []model.AlertStorm
	q.Order("id DESC").Limit(100).Find(&items)
	if items == nil {
		items = []model.AlertStorm{}
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

// StormDetail GET /alert-storms/:id → 风暴信息+成员告警列表
func (h *Handler) StormDetail(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var st model.AlertStorm
	if err := h.DB.First(&st, id).Error; err != nil {
		response.NotFound(c, "风暴不存在")
		return
	}
	var alerts []model.Alert
	h.DB.Where("storm_id = ?", id).Order("id").Limit(200).Find(&alerts)
	if alerts == nil {
		alerts = []model.Alert{}
	}
	response.Success(c, gin.H{"storm": st, "alerts": alerts, "count": len(alerts)})
}
