package main

// DCIM V1 大屏演示数据生成器（随机游走）
// 用途：真实测点接入前，为监控大屏提供逼真的演示数据
// 幂等：可重复执行（先清理带 demo_ 前缀的演示数据）

import (
	"fmt"
	gormlog "log"
	"math/rand"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"dcim/backend/internal/model"
)

func main() {
	user := env("DB_USER", "dcim")
	pass := env("DB_PASSWORD", "")
	if pass == "" {
		// V69 安全修复：不再内置默认口令（历史交付包曾因此泄漏 DB 密码）。fail-closed：必须 env 注入
		fmt.Println("DB_PASSWORD 未设置：demo-data 不再内置默认口令，请用环境变量注入后重试")
		os.Exit(1)
	}
	host := env("DB_HOST", "127.0.0.1")
	port := env("DB_PORT", "5432")
	name := env("DB_NAME", "dcim_platform")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, pass, name)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.New(gormlog.New(os.Stdout, "\r\n", gormlog.LstdFlags), gormlogger.Config{SlowThreshold: 500 * time.Millisecond, LogLevel: gormlogger.Error, IgnoreRecordNotFoundError: true, Colorful: false})})
	if err != nil {
		fmt.Println("DB连接失败:", err)
		os.Exit(1)
	}
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	// ===== -1. 全新库种子：站点/房间/机柜列（幂等，已存在则跳过） =====
	var siteCnt int64
	db.Model(&model.Site{}).Count(&siteCnt)
	var siteID uint = 1
	if siteCnt == 0 {
		s := model.Site{Code: "DEMO01", Name: "示例数据中心", Level: 1, Address: "北京市", Manager: "运维部", Phone: "010-88888888", Status: 1}
		db.Create(&s)
		siteID = s.ID
		fmt.Println("创建演示站点:", s.ID, s.Name)
	} else {
		var s0 model.Site
		db.Order("id").First(&s0)
		siteID = s0.ID
	}

	// ===== 0. 清理旧演示数据（按 code 前缀 demo_；资产先删防止引用悬空机柜） =====
	var oldPointIDs []uint
	db.Model(&model.MonitoringPoint{}).Where("code LIKE 'demo_%'").Pluck("id", &oldPointIDs)
	if len(oldPointIDs) > 0 {
		db.Where("point_id IN ?", oldPointIDs).Delete(&model.Reading{})
		db.Where("id IN ?", oldPointIDs).Delete(&model.MonitoringPoint{})
	}
	db.Where("code LIKE 'demo_%'").Delete(&model.Asset{})
	db.Where("code LIKE 'demo_%'").Delete(&model.Cabinet{})
	fmt.Println("清理旧演示数据完成, 旧测点数:", len(oldPointIDs))

	// ===== 1. 确保有 2 个房间 =====
	var room1, room2 model.Room
	db.Where("site_id = ?", siteID).Order("id").First(&room1)
	if room1.ID == 0 {
		room1 = model.Room{SiteID: siteID, Name: "A栋主机房", Floor: "1F", Area: 200, Grade: "A"}
		db.Create(&room1)
	}
	db.Where("site_id = ? AND id != ?", siteID, room1.ID).Order("id").First(&room2)
	if room2.ID == 0 {
		room2 = model.Room{SiteID: siteID, Name: "B 栋 201 机房", Floor: "2F", Area: 120, Grade: "B"}
		db.Create(&room2)
	}
	fmt.Println("房间:", room1.ID, room1.Name, "|", room2.ID, room2.Name)

	// V21: 补充 GB 50174 功能房间（电池室/配电室/空调机房/值班室），供楼层级 3D 平面下钻
	gbRooms := []struct {
		Name  string
		Floor string
		Area  float64
	}{
		{"电池室", room1.Floor, 120},
		{"高低压配电室", room1.Floor, 150},
		{"空调机房", room1.Floor, 90},
		{"值班监控室", room1.Floor, 60},
	}
	for _, gr := range gbRooms {
		var cnt int64
		db.Model(&model.Room{}).Where("site_id = ? AND name = ?", siteID, gr.Name).Count(&cnt)
		if cnt == 0 {
			db.Create(&model.Room{SiteID: siteID, Name: gr.Name, Floor: gr.Floor, Area: gr.Area, Grade: "A"})
		}
	}
	fmt.Println("GB 功能房间已补充(电池室/配电室/空调机房/值班室)")

	// ===== 2. 创建演示测点（温湿度/配电/UPS/空调） =====
	type ptDef struct {
		code, name, ptype, unit string
		roomID                  uint
		min, max, base          float64
	}
	defs := []ptDef{
		{"demo_th_a1", "A 机房冷通道温湿度-01", "temp_humidity", "°C", room1.ID, 18, 28, 22.5},
		{"demo_th_a2", "A 机房热通道温湿度-01", "temp_humidity", "°C", room1.ID, 25, 40, 33.0},
		{"demo_pd_a1", "A 机房列头柜-01", "power_dist", "kW", room1.ID, 0, 60, 28.5},
		{"demo_ups_a1", "A 机房 UPS-01", "ups", "kW", room1.ID, 0, 120, 62.0},
		{"demo_hvac_a1", "A 机房精密空调-01", "hvac", "°C", room1.ID, 16, 24, 19.5},
		{"demo_th_b1", "B 机房冷通道温湿度-01", "temp_humidity", "°C", room2.ID, 18, 28, 23.2},
		{"demo_th_b2", "B 机房热通道温湿度-01", "temp_humidity", "°C", room2.ID, 25, 40, 34.5},
		{"demo_pd_b1", "B 机房列头柜-01", "power_dist", "kW", room2.ID, 0, 60, 31.0},
		{"demo_ups_b1", "B 机房 UPS-01", "ups", "kW", room2.ID, 0, 120, 58.0},
		{"demo_hvac_b1", "B 机房精密空调-01", "hvac", "°C", room2.ID, 16, 24, 20.0},
	}
	var points []*model.MonitoringPoint
	for _, d := range defs {
		p := &model.MonitoringPoint{
			SiteID: siteID, Code: d.code, Name: d.name, Type: d.ptype,
			Protocol: "modbus", Unit: d.unit,
			RoomID:       &d.roomID,
			MinThreshold: &d.min, MaxThreshold: &d.max,
			PollInterval: 30, ScaleFactor: 1,
		}
		db.Create(p)
		points = append(points, p)
	}
	fmt.Println("创建演示测点:", len(points))

	// ===== 3. 生成过去 12 小时读数（5 分钟间隔，随机游走） =====
	now := time.Now().Truncate(time.Minute)
	const step = 5 * time.Minute
	const hours = 12
	total := 0
	for i, p := range points {
		base := defs[i].base
		val := base
		rows := make([]model.Reading, 0, hours*12)
		for t := now.Add(-hours * time.Hour); !t.After(now); t = t.Add(step) {
			// 随机游走 + 周期性波动
			phase := float64(t.Hour()) / 24 * 6.283
			drift := rnd.NormFloat64() * base * 0.015
			wave := base * 0.04 * float64(int(phase*10)%7-3) / 7
			val = base + wave + drift
			if val < defs[i].min*0.9 {
				val = defs[i].min * 0.9
			}
			if val > defs[i].max*0.95 {
				val = defs[i].max * 0.95
			}
			rows = append(rows, model.Reading{PointID: p.ID, Value: round2(val), Quality: 1, Timestamp: t})
		}
		db.CreateInBatches(rows, 200)
		total += len(rows)
		// 更新测点最新值
		last := rows[len(rows)-1].Value
		db.Model(p).Update("current_value", last).Update("status", 1)
	}
	fmt.Println("生成读数:", total)

	// ===== 4. 生成过去 30 天 PUE 记录（每小时 1 条） =====
	db.Where("site_id = ?", siteID).Delete(&model.PUERecord{})
	itBase := 55.0
	pueRows := make([]model.PUERecord, 0, 30*24)
	for t := now.Add(-30 * 24 * time.Hour); !t.After(now); t = t.Add(time.Hour) {
		// PUE 在 1.45~1.65 随机游走，白天略高
		hourFactor := 0.0
		if t.Hour() >= 9 && t.Hour() <= 18 {
			hourFactor = 0.05
		}
		pue := 1.52 + hourFactor + rnd.NormFloat64()*0.03
		if pue < 1.42 {
			pue = 1.42
		}
		if pue > 1.72 {
			pue = 1.72
		}
		it := itBase + rnd.NormFloat64()*2
		pueRows = append(pueRows, model.PUERecord{
			SiteID: siteID, TotalPower: round2(it * pue), ITPower: round2(it),
			PUE: round2(pue), RecordedAt: t,
		})
	}
	db.CreateInBatches(pueRows, 500)
	fmt.Println("生成PUE记录:", len(pueRows))

	// ===== 5. 扩容机柜到 12 个（带功率/承重/占用） =====
	var cabCount int64
	db.Model(&model.Cabinet{}).Where("site_id = ?", siteID).Count(&cabCount)
	if cabCount < 12 {
		need := 12 - cabCount
		rows := make([]model.Cabinet, 0, need)
		names := []string{"A01", "A02", "A03", "A04", "B01", "B02", "B03", "B04", "C01", "C02", "D01", "D02"}
		for i := int(cabCount); i < 12; i++ {
			rm := room1.ID
			if i%2 == 1 {
				rm = room2.ID
			}
			ratedKW := round2(4 + rnd.Float64()*4)
			rows = append(rows, model.Cabinet{
				SiteID: siteID, RoomID: rm, Code: "demo_" + names[i],
				TotalU: 42, RatedPowerKW: ratedKW,
				Status: 1,
			})
		}
		db.Create(&rows)
		fmt.Println("扩容机柜:", need)
	} else {
		fmt.Println("机柜已充足:", cabCount)
	}

	// ===== 6. 机柜列（3D机房冷热通道布局依赖 row_id） =====
	var rowCount int64
	db.Model(&model.Row{}).Where("site_id = ?", siteID).Count(&rowCount)
	if rowCount == 0 {
		rowDefs := []struct {
			name   string
			roomID uint
		}{{"A列", room1.ID}, {"B列", room1.ID}, {"C列", room2.ID}}
		for _, rd := range rowDefs {
			db.Create(&model.Row{SiteID: siteID, RoomID: rd.roomID, Name: rd.name})
		}
		var allRows []model.Row
		db.Where("site_id = ?", siteID).Order("id").Find(&allRows)
		var cabs []model.Cabinet
		db.Where("site_id = ? AND row_id = 0", siteID).Order("id").Find(&cabs)
		for i, cb := range cabs {
			if len(allRows) > 0 {
				db.Model(&model.Cabinet{}).Where("id = ?", cb.ID).Update("row_id", allRows[i%len(allRows)].ID)
			}
		}
		fmt.Println("创建机柜列:", len(rowDefs))
	}

	// ===== 7. 在架演示资产（容量热力/3D机房/影响分析依赖） =====
	var assetCnt int64
	db.Model(&model.Asset{}).Where("code LIKE 'demo_%'").Count(&assetCnt)
	if assetCnt == 0 {
		var cabs []model.Cabinet
		db.Where("site_id = ?", siteID).Order("id").Find(&cabs)
		type aDef struct {
			prefix, name, cat, brand, mdl string
			hU, wW                        int
			kg                            float64
		}
		aDefs := []aDef{
			{"SRV", "应用服务器", "server", "浪潮", "NF5280M6", 2, 520, 25},
			{"SRV", "数据库服务器", "server", "华为", "RH2288H V5", 2, 550, 30},
			{"STG", "存储阵列", "storage", "戴尔", "PowerVault ME5", 4, 800, 60},
			{"SW", "接入交换机", "network", "华为", "S5735-L48T", 1, 150, 8},
			{"PDU", "智能PDU", "power", "维谛", "rPDU 32A", 1, 0, 5},
		}
		seq := 0
		var assets []model.Asset
		for _, cb := range cabs {
			n := 2 + rnd.Intn(3) // 每柜 2~4 台
			startU := 1
			for j := 0; j < n && startU < 36; j++ {
				d := aDefs[rnd.Intn(len(aDefs))]
				seq++
				code := fmt.Sprintf("demo_%s-%03d", cb.Code, seq)
				ip := fmt.Sprintf("10.20.%d.%d", cb.ID%200+1, seq%250+2)
				sn := fmt.Sprintf("SN%06d", 100000+seq)
				assets = append(assets, model.Asset{
					Code: code, Name: fmt.Sprintf("%s-%s", cb.Code, d.name),
					Category: d.cat, Brand: d.brand, Model: d.mdl, SN: sn,
					SiteID: siteID, CabinetID: cb.ID, StartU: startU, HeightU: d.hU,
					PowerW: d.wW, WeightKG: d.kg, IP: ip,
					Owner: "运维部", Status: 1,
				})
				startU += d.hU + 1
			}
		}
		if len(assets) > 0 {
			db.CreateInBatches(&assets, 100)
		}
		fmt.Println("创建演示资产:", len(assets))
	}

	// ===== 8. 供电链路（市电→UPS→PDU，绑定演示测点/机柜） =====
	var pcCnt int64
	db.Model(&model.PowerChain{}).Where("site_id = ?", siteID).Count(&pcCnt)
	if pcCnt == 0 {
		utility := model.PowerChain{SiteID: siteID, Name: "市电引入-10kV", Kind: "utility", RatedKVA: 1000, Status: 1}
		db.Create(&utility)
		trans := model.PowerChain{SiteID: siteID, ParentID: utility.ID, Name: "干式变压器-800kVA", Kind: "transformer", RatedKVA: 800, Status: 1}
		db.Create(&trans)
		var upsPoint model.MonitoringPoint
		db.Where("type = 'ups' AND site_id = ?", siteID).Order("id").First(&upsPoint)
		ups := model.PowerChain{SiteID: siteID, ParentID: trans.ID, Name: "UPS-1#", Kind: "ups", RatedKVA: 200, LoadKW: 85, Status: 1, PointID: upsPoint.ID}
		db.Create(&ups)
		var pdPoint model.MonitoringPoint
		db.Where("type = 'power_dist' AND site_id = ?", siteID).Order("id").First(&pdPoint)
		pdu := model.PowerChain{SiteID: siteID, ParentID: ups.ID, Name: "A列PDU-01", Kind: "pdu", RatedKVA: 30, LoadKW: 12, Status: 1, PointID: pdPoint.ID}
		db.Create(&pdu)
		// 每柜一条支路节点（绑机柜），停电影响分析可覆盖全部机柜
		var allCabs []model.Cabinet
		db.Where("site_id = ?", siteID).Order("id").Find(&allCabs)
		for i, cb := range allCabs {
			br := model.PowerChain{
				SiteID: siteID, ParentID: ups.ID,
				Name: fmt.Sprintf("%s柜支路", cb.Code), Kind: "branch",
				RatedKVA: 8, LoadKW: round2(1.5 + rnd.Float64()*3), Status: 1,
				CabinetID: cb.ID,
			}
			if i == 0 {
				br.ParentID = pdu.ID
				br.Name = cb.Code + "列PDU支路"
			}
			db.Create(&br)
		}
		fmt.Println("创建供电链路: 市电→变压器→UPS→PDU +", len(allCabs), "条机柜支路")
	}

	// ===== 9. 网络设备+端口+链路 =====
	var ndCnt int64
	db.Model(&model.NetworkDevice{}).Where("site_id = ?", siteID).Count(&ndCnt)
	if ndCnt == 0 {
		core := model.NetworkDevice{SiteID: siteID, RoomID: &room1.ID, Code: "demo-CORE-SW01", Name: "核心交换机", Type: "switch", Vendor: "华为", Model: "CE6865", MgmtIP: "10.20.0.2", PortCount: 8, Status: 1}
		db.Create(&core)
		acc1 := model.NetworkDevice{SiteID: siteID, RoomID: &room1.ID, Code: "demo-ACC-SW01", Name: "接入交换机-A列", Type: "switch", Vendor: "华为", Model: "S5735", MgmtIP: "10.20.0.3", PortCount: 6, Status: 1}
		db.Create(&acc1)
		acc2 := model.NetworkDevice{SiteID: siteID, RoomID: &room2.ID, Code: "demo-ACC-SW02", Name: "接入交换机-B列", Type: "switch", Vendor: "锐捷", Model: "RG-S5760", MgmtIP: "10.20.0.4", PortCount: 6, Status: 1}
		db.Create(&acc2)
		fw := model.NetworkDevice{SiteID: siteID, RoomID: &room1.ID, Code: "demo-FW01", Name: "边界防火墙", Type: "firewall", Vendor: "深信服", Model: "AF-2000", MgmtIP: "10.20.0.1", PortCount: 4, Status: 1}
		db.Create(&fw)
		mkPorts := func(devID uint, n int) []model.NetworkPort {
			ps := make([]model.NetworkPort, 0, n)
			for i := 1; i <= n; i++ {
				ps = append(ps, model.NetworkPort{DeviceID: devID, Name: fmt.Sprintf("GE0/0/%d", i), Index: i, Type: "copper", Speed: "10G", Status: 1})
			}
			db.Create(&ps)
			return ps
		}
		corePs := mkPorts(core.ID, 8)
		acc1Ps := mkPorts(acc1.ID, 6)
		acc2Ps := mkPorts(acc2.ID, 6)
		fwPs := mkPorts(fw.ID, 4)
		db.Create(&[]model.NetworkLink{
			{SiteID: siteID, PortAID: fwPs[0].ID, PortBID: corePs[0].ID, LinkType: "fiber", Bandwidth: "10G", Status: 1, Label: "出口上联"},
			{SiteID: siteID, PortAID: corePs[1].ID, PortBID: acc1Ps[0].ID, LinkType: "fiber", Bandwidth: "10G", Status: 1, Label: "核心-A列"},
			{SiteID: siteID, PortAID: corePs[2].ID, PortBID: acc2Ps[0].ID, LinkType: "fiber", Bandwidth: "10G", Status: 1, Label: "核心-B列"},
		})
		fmt.Println("创建网络设备4台+端口24个+链路3条")
	}

	fmt.Println("DEMO_DATA_OK")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
