package model

import "time"

// PUEPolicy 各省数据中心 PUE 政策规则（V49 能效合规）
type PUEPolicy struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Province       string    `gorm:"size:32;uniqueIndex;not null" json:"province"`      // 省市名（北京/安徽(芜湖)等）
	Region         string    `gorm:"size:16" json:"region"`                             // 区域：东部枢纽/西部枢纽/中部承接/东北拓展/港澳台
	NewLargeLimit  float64   `json:"new_large_limit"`                                   // 新建大型/超大型 PUE 限额（0=未设）
	NewSmallLimit  float64   `json:"new_small_limit"`                                   // 新建中小型限额
	ExistingTarget string    `gorm:"size:255" json:"existing_target"`                   // 存量改造目标
	SurchargeRules string    `gorm:"size:255" json:"surcharge_rules"`                   // 差别电价规则 "1.35-1.7:0.2;>1.7:0.5"
	Subsidy        string    `gorm:"size:255" json:"subsidy"`                           // 补贴/激励
	Features       string    `gorm:"size:255" json:"features"`                          // 特色要求
	Source         string    `gorm:"size:120" json:"source"`                            // 数据来源
	AsOf           string    `gorm:"size:10" json:"as_of"`                              // 数据截止日期
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (PUEPolicy) TableName() string { return "t_pue_policy" }

// SiteProvince 站点省份字段在 Site 模型上（V49 增加），见 models.go
