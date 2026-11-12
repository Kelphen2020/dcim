package model

import "time"

// V61 运维月报自动生成
// 闭环: 每月1日自动生成上月报告(或管理员手动生成任意月份) → 快照聚合全模块指标 → 通知+Webhook → 报表中心「运维月报」页签查看/导出CSV/发布归档
// 业务背景: 甲方汇报/等保检查需要按月归档的运维报告材料, 手工汇总费时且口径不一

// OpsReport 运维月报快照
type OpsReport struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:50;uniqueIndex;not null" json:"code"` // MR-yyyyMM-Sn
	Title       string    `gorm:"size:200" json:"title"`
	SiteID      uint      `gorm:"uniqueIndex:uk_ops_report_month_site;index" json:"site_id"` // 0=全站
	Month       string    `gorm:"size:7;uniqueIndex:uk_ops_report_month_site;not null" json:"month"` // yyyy-MM
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Summary     string    `gorm:"size:500" json:"summary"` // 一句话摘要（列表展示）
	Content     string    `gorm:"type:text" json:"-"`      // JSON 快照（列表不返回，详情解析后返回）
	Status      int       `gorm:"default:1;index" json:"status"` // 1草稿 2已发布
	GeneratedBy uint      `json:"generated_by"`            // 0=系统自动
	AutoGen     bool      `gorm:"default:false" json:"auto_gen"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (OpsReport) TableName() string { return "t_ops_report" }

// EvOpsReport Webhook 事件：月报生成/发布
const EvOpsReport = "ops_report"
