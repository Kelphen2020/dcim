package api

// 演示数据模拟器：真实网关/测点接入前，为动环测点持续生成逼真读数，
// 使大屏、趋势分析、告警链路全为"活数据"。生产接入真实网关后可关闭。

import (
	"log"
	"math"
	"math/rand"
	"time"

	"dcim/backend/internal/model"

	"gorm.io/gorm"
)

// SimulatorConfig 模拟器配置
type SimulatorConfig struct {
	Enabled     bool
	IntervalSec int
}

// StartSimulator 启动后台模拟采集（阻塞在 goroutine 中调用）
func StartSimulator(db *gorm.DB, cfg SimulatorConfig) {
	if !cfg.Enabled {
		return
	}
	interval := cfg.IntervalSec
	if interval < 10 {
		interval = 30
	}
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	log.Printf("演示数据模拟器已启动: 每 %ds 采集一轮", interval)

	// 启动回填：补齐最近 12 小时缺失的 PUE 记录，使大屏趋势图一进入即完整
	backfillPUE(db, rnd, 12)

	// 每轮：对所有在线测点生成新读数（随机游走），并按小时写入 PUE 记录
	lastPueHour := time.Time{}
	for range time.Tick(time.Duration(interval) * time.Second) {
		var points []model.MonitoringPoint
		db.Where("status IN (1,2)").Find(&points)
		now := time.Now()
		for i := range points {
			p := &points[i]
			base := 0.0
			if p.CurrentValue != nil {
				base = *p.CurrentValue
			} else {
				base = defaultBase(p.Type)
			}
			// 随机游走 ±1.5%，叠加日周期
			drift := rnd.NormFloat64() * math.Abs(base) * 0.015
			hourF := math.Sin(float64(now.Hour()) / 24 * 2 * math.Pi)
			wave := math.Abs(base) * 0.02 * hourF
			val := base + drift + wave
			// 阈值范围内收敛，偶尔越限触发真实告警链路
			if p.MaxThreshold != nil && val > *p.MaxThreshold*1.02 {
				if rnd.Float64() > 0.15 {
					val = *p.MaxThreshold * (0.95 + rnd.Float64()*0.05)
				}
			}
			if p.MinThreshold != nil && val < *p.MinThreshold*0.98 {
				if rnd.Float64() > 0.15 {
					val = *p.MinThreshold * (0.99 + rnd.Float64()*0.03)
				}
			}
			if val < 0 {
				val = 0
			}
			val = math.Round(val*100) / 100
			db.Create(&model.Reading{PointID: p.ID, Value: val, Quality: 1, Timestamp: now})
			db.Model(p).Update("current_value", val).Update("updated_at", now)
		}

		// 每小时一条 PUE 记录（总功率/IT功率随之波动）
		if now.Hour() != lastPueHour.Hour() || lastPueHour.IsZero() {
			lastPueHour = now
			var itPower float64
			db.Raw(`SELECT COALESCE(SUM(power_w),0)/1000.0 FROM t_asset WHERE power_w > 0`).Scan(&itPower)
			if itPower <= 0 {
				itPower = 50
			}
			// IT 负载加日间波动
			hf := math.Sin(float64(now.Hour()) / 24 * 2 * math.Pi)
			itNow := itPower * (1 + 0.06*hf + rnd.NormFloat64()*0.01)
			pue := 1.52 + 0.05*hf + rnd.NormFloat64()*0.02
			if pue < 1.42 {
				pue = 1.42
			}
			if pue > 1.72 {
				pue = 1.72
			}
			pue = math.Round(pue*1000) / 1000
			total := itNow * pue
			db.Create(&model.PUERecord{
				SiteID: 1, TotalPower: math.Round(total*100) / 100,
				ITPower: math.Round(itNow*100) / 100, PUE: pue, RecordedAt: now,
			})
		}
	}
}

func defaultBase(ptype string) float64 {
	switch ptype {
	case "temp_humidity":
		return 23
	case "hvac":
		return 19.5
	case "power_dist":
		return 28
	case "ups":
		return 60
	case "water_leak", "smoke":
		return 0
	case "cdu":
		return 25 // 供液温度典型值；流量/压力类由阈值收敛
	case "leak_rope":
		return 0
	}
	return 20
}

// backfillPUE 回填最近 hours 小时的 PUE 记录（仅补缺失的小时），
// 使大屏/能耗趋势在模拟器启动后即有完整曲线，而非只有最新 1 个点。
func backfillPUE(db *gorm.DB, rnd *rand.Rand, hours int) {
	var itPower float64
	db.Raw(`SELECT COALESCE(SUM(power_w),0)/1000.0 FROM t_asset WHERE power_w > 0`).Scan(&itPower)
	if itPower <= 0 {
		itPower = 50
	}
	now := time.Now().Truncate(time.Hour)
	created := 0
	for i := hours; i >= 1; i-- {
		h := now.Add(-time.Duration(i) * time.Hour)
		var cnt int64
		db.Model(&model.PUERecord{}).Where("site_id = 1 AND recorded_at >= ? AND recorded_at < ?", h, h.Add(time.Hour)).Count(&cnt)
		if cnt > 0 {
			continue // 该小时已有真实/历史数据，不覆盖
		}
		hf := math.Sin(float64(h.Hour()) / 24 * 2 * math.Pi)
		itNow := itPower * (1 + 0.06*hf + rnd.NormFloat64()*0.01)
		pue := 1.52 + 0.05*hf + rnd.NormFloat64()*0.02
		if pue < 1.42 {
			pue = 1.42
		}
		if pue > 1.72 {
			pue = 1.72
		}
		pue = math.Round(pue*1000) / 1000
		db.Create(&model.PUERecord{
			SiteID: 1, TotalPower: math.Round(itNow*pue*100) / 100,
			ITPower: math.Round(itNow*100) / 100, PUE: pue, RecordedAt: h,
		})
		created++
	}
	if created > 0 {
		log.Printf("模拟器回填 PUE 记录 %d 条（最近 %d 小时）", created, hours)
	}
}
