package api

import (
	"net/http"
	"time"

	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// allowedSettingKeys 允许通过 SetSystemSetting 写入的配置项白名单
var allowedSettingKeys = map[string]bool{
	"default_theme":        true, // 默认主题
	"site_title":           true, // 站点标题
	"login_banner":         true, // 登录页公告
	"password_min_length":  true, // 密码最小长度
	"session_note":         true, // 会话/安全说明
	"maintenance_contact":  true, // 运维联系电话
	"dashboard_refresh":    true, // 仪表盘刷新间隔(秒)
	"company_name":         true, // 单位名（关于信息）
	"system_version":       true, // 版本号
	"energy_price_kwh":     true, // 电价(元/kWh)，能耗费用估算
	"co2_factor":           true, // 电网排放因子(kgCO2/kWh)
	"sla_respond_hours":    true, // V18 SLA 响应时限(小时,逗号4值)
	"sla_resolve_hours":    true, // V18 SLA 解决时限(小时,逗号4值)
	"scan_checkin_fresh_min": true, // V19 扫码签到新鲜度窗口(分钟)
	"mobile_base_url":        true, // M3 移动端 H5 基址（飞书卡片详情深链用）
	"health_grade_thresholds": true, // V50 健康等级阈值（逗号3值，默认 85,70,55）
	"health_auto_work_order":  true, // V50 风险级自动开预测性维护工单（1/0，默认1）
	"power_redundancy_auto_work_order": true, // V54 风险级自动开供电冗余整改工单（1/0，默认1）
	"capacity_forecast_auto_work_order": true, // V56 风险级自动开容量扩容规划工单（1/0，默认1）
	"drill_overdue_notify":              true, // V59 应急预案演练逾期通知（1/0，默认1）
	"auto_monthly_report":               true, // V61 每月1日自动生成上月运维月报（1/0，默认1）
	"inspection_task_auto":              true, // V62 巡检任务自动生成（1/0，默认1）
	"inspection_missed_notify":          true, // V62 巡检漏检通知（1/0，默认1）
	"alert_quality_notify":              true, // V64 高噪声告警规则预警通知（1/0，默认1）
	"netdev_offline_min":                true, // V67/B2-2 网络设备心跳超时分钟数（默认10）
	"spare_auto_purchase":               true, // B3-2/G4 低库存备件每日扫描自动生成采购申请（1/0，默认1）
	"pue_over_notify":                   true, // V73/B3-3/G6 PUE 合规越线通知+Webhook（1/0，默认1；关闭仍记录告警）
	"pue_over_sustain_hours":            true, // V73/B3-3 PUE 越线持续时间（小时，默认2）
}

// publicSettingKeys 登录前可读取的键
var publicSettingKeys = []string{"default_theme", "site_title", "login_banner"}

// GetPublicSettings 公共配置（登录前可读，用于应用站点默认主题/标题）
func (h *Handler) GetPublicSettings(c *gin.Context) {
	out := gin.H{"default_theme": "", "site_title": "", "login_banner": ""}
	for _, k := range publicSettingKeys {
		var s model.SystemSetting
		if err := h.DB.First(&s, "key = ?", k).Error; err == nil {
			out[k] = s.Value
		}
	}
	response.Success(c, out)
}

// ListSystemSettings 读取全部系统配置（管理员）
func (h *Handler) ListSystemSettings(c *gin.Context) {
	var items []model.SystemSetting
	h.DB.Order("key").Find(&items)
	out := make(map[string]string, len(items))
	for _, it := range items {
		if allowedSettingKeys[it.Key] {
			out[it.Key] = it.Value
		}
	}
	response.Success(c, out)
}

type settingReq struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
}

// SetSystemSetting 写入系统配置（管理员）
func (h *Handler) SetSystemSetting(c *gin.Context) {
	var req settingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	allowed := allowedSettingKeys
	if !allowed[req.Key] {
		response.BadRequest(c, "不允许的配置项")
		return
	}
	s := model.SystemSetting{Key: req.Key, Value: req.Value, UpdatedAt: time.Now()}
	if err := h.DB.Save(&s).Error; err != nil {
		response.ServerError(c, "保存失败")
		return
	}
	middleware.WriteAudit(h.DB, c, "setting.update", req.Key, req.Value)
	response.Success(c, s)
}

var _ = http.StatusOK
