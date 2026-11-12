package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"dcim/backend/internal/api"
	"dcim/backend/internal/config"
	"dcim/backend/internal/middleware"
	"dcim/backend/internal/model"
	"dcim/backend/pkg/auth"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// loadAIEnvFile V53: 读取可选的本地 AI 凭据文件（configs/ai.env 或 /opt/dcim/ai.env，
// KEY=*** 每行一条，chmod 600，不进交付包）。已有同名环境变量时不覆盖。
func loadAIEnvFile() {
	for _, p := range []string{"configs/ai.env", "/opt/dcim/ai.env"} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if i := strings.Index(line, "="); i > 0 {
				k := strings.TrimSpace(line[:i])
				v := strings.TrimSpace(line[i+1:])
				if os.Getenv(k) == "" {
					os.Setenv(k, v)
				}
			}
		}
		log.Printf("[V53] 已加载 AI 凭据文件 %s", p)
		return
	}
}

func main() {
	cfgPath := flag.String("c", "configs/dev.yaml", "配置文件路径")
	flag.Parse()
	loadAIEnvFile()
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// JWT 密钥
	auth.SetSecret(cfg.JWT.Secret)
	expire := cfg.JWT.ExpireHours
	if expire <= 0 {
		expire = 24
	}

	// 数据库
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{SlowThreshold: 500 * time.Millisecond, LogLevel: logger.Error, IgnoreRecordNotFoundError: true, Colorful: false},
		),
	})
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	log.Printf("数据库连接成功: %s/%s", cfg.Database.Host, cfg.Database.DBName)

	// 建表
	if err := db.AutoMigrate(
		&model.User{}, &model.Audit{},
		&model.Site{}, &model.Room{}, &model.Row{}, &model.Cabinet{},
		&model.Asset{}, &model.Maintenance{},
		&model.Gateway{},
		&model.WorkOrder{}, &model.WorkOrderLog{},
		&model.InspectionPlan{}, &model.InspectionRecord{},
		&model.ChangeRequest{}, &model.PowerChain{}, &model.Notification{},
		&model.MonitoringPoint{}, &model.Reading{}, &model.AlertRule{}, &model.Alert{},
		&model.GatewayCommand{},
		&model.NetworkDevice{}, &model.NetworkPort{}, &model.NetworkLink{},
		&model.IPPool{}, &model.IPAssignment{}, &model.VLAN{}, &model.ConfigBackup{},
		&model.PUERecord{},
		&model.PUEPolicy{},
		&model.TopoDiagram{},
		&model.TrendTemplate{}, &model.AlertActionLog{},
		&model.SystemSetting{},
		&model.StocktakeTask{}, &model.StocktakeItem{},
		&model.NotifyChannel{},
		&model.InspectionCheckin{},
		&model.FaultCase{}, &model.AlertStorm{}, &model.DutyShift{},
		&model.RoomLayoutItem{}, &model.ModelAsset{}, &model.LayoutTemplate{},
		&model.SparePart{}, &model.SpareMovement{},
		&model.MaintenanceSpare{}, &model.PurchaseRequest{}, &model.SpareBOM{},
		&api.PushChannel{},
		&api.Attachment{},
		&api.DutyHandoverSign{},
		&model.AssetHealth{}, &model.AssetHealthHistory{}, &model.PowerRedundancy{}, &model.CapacityHistory{}, &model.CapacityForecast{},
		&model.EmergencyPlan{}, &model.DrillRecord{}, // V59 应急预案与演练
		&model.AssetRetirement{}, // V60 资产退役与处置
		&model.OpsReport{},       // V61 运维月报
		&model.InspectionTask{},  // V62 巡检任务
	); err != nil {
		log.Fatalf("建表失败: %v", err)
	}
	log.Println("数据表就绪")

	// 种子管理员 admin / 初始密码（可用环境变量 DCIM_ADMIN_INIT_PASSWORD 覆盖，首次登录后请立即修改）
	var cnt int64
	db.Model(&model.User{}).Count(&cnt)
	if cnt == 0 {
		initPwd := os.Getenv("DCIM_ADMIN_INIT_PASSWORD")
		if initPwd == "" {
			initPwd = "ChangeMe@123"
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(initPwd), bcrypt.DefaultCost)
		db.Create(&model.User{
			Username: "admin", PasswordHash: string(hash),
			DisplayName: "系统管理员", Role: "global_admin", Status: 1,
		})
		log.Println("已创建初始管理员 admin（初始密码见 DCIM_ADMIN_INIT_PASSWORD，默认 ChangeMe@123，请尽快修改）")
	}

	// Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.CORSMiddleware())
	r.Use(cors.Default())

	h := api.NewHandler(db, cfg.JWT.Secret, expire)
	// V49: 政策库种子（幂等，库空时导入34省）
	if n := h.SeedPUEPolicies(); n > 0 {
		log.Printf("[V49] PUE政策库种子导入 %d 条", n)
	}
	// V59: 应急预案种子（幂等，库空时导入6个内置预案）
	if n := h.SeedEmergencyPlans(); n > 0 {
		log.Printf("[V59] 应急预案种子导入 %d 条", n)
	}
	h.WxAppID = cfg.Wechat.AppID
	h.WxAppSecret = cfg.Wechat.AppSecret
	h.AIAPIKey = cfg.AI.APIKey
	h.AIBaseURL = cfg.AI.BaseURL
	h.AIModel = cfg.AI.Model
	// V58: env 原值留档（界面"恢复环境配置"用），DB 配置优先热覆盖
	h.AIEnvKey, h.AIEnvBase, h.AIEnvModel = cfg.AI.APIKey, cfg.AI.BaseURL, cfg.AI.Model
	if h.LoadAIConfigFromDB() {
		log.Printf("[V58] AI 大模型使用界面保存的配置（provider=%s model=%s）", h.AIProvider, h.AIModel)
	}
	if h.AIAPIKey != "" && h.AIBaseURL != "" {
		log.Printf("[V53] AI 运维助手已启用（模型 %s）", h.AIModel)
	} else {
		log.Printf("[V53] AI 大模型未配置，助手以本地规则模式运行")
	}
	h.RegisterRoutes(r)

	// 静态资源（前端构建产物）
	r.Static("/assets", "./static/assets")
	r.Static("/uploads", "./uploads") // V26 导入的3D模型文件
	r.StaticFile("/", "./static/index.html")
	r.StaticFile("/index.html", "./static/index.html")
	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:5] == "/api/" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "接口不存在"})
			return
		}
		c.File("./static/index.html")
	})

	// 网关离线检测定时任务
	go func() {
		for range time.Tick(15 * time.Second) {
			h.GatewayOfflineCheck()
			h.GatewayRedispatch() // V47: 超时未回报的 DO 命令回队重派
		}
	}()

	// 维保到期提醒：启动时检查一次，之后每 6 小时检查
	go func() {
		if n := h.CheckMaintenanceDue(); n > 0 {
			log.Printf("维保到期提醒：%d 条", n)
		}
		for range time.Tick(6 * time.Hour) {
			h.CheckMaintenanceDue()
		}
	}()

	// 巡检逾期扫描：启动时一次，之后每小时
	go func() {
		h.ScanOverdueInspections()
		h.ScanOverdueDrills() // V59 演练逾期扫描（7天幂等窗口）
		h.CheckAutoMonthlyReport() // V61 每月1日自动生成上月运维月报（幂等）
		h.V62TaskCycle()           // V62 巡检任务生成+核销+漏检扫描
		h.ScanAlertQuality()         // V64 高噪声告警规则评估（7天幂等）
		h.ScanSpareAutoPurchase()    // B3-2/G4 低库存备件自动生成采购申请（同备件待审批唯一+24h节奏幂等）
		h.ScanPUECompliance(0)       // V73/B3-3/G6 PUE 合规越线告警（同站点单活跃+24h通知幂等）
		for range time.Tick(1 * time.Hour) {
			h.ScanOverdueInspections()
			h.ScanOverdueDrills()
			h.CheckAutoMonthlyReport()
			h.V62TaskCycle()
			h.ScanAlertQuality()
			h.ScanSpareAutoPurchase()
			h.ScanPUECompliance(0)
		}
	}()

	// V13 告警升级扫描：每分钟检查超时未确认告警；V67/B2-2 网络设备离线判定同拍
	go func() {
		for range time.Tick(1 * time.Minute) {
			h.ScanAlertEscalation()
			h.ScanNetdevOffline()
		}
	}()

	// V18 SLA 扫描：每分钟检查工单响应/解决时限
	go func() {
		for range time.Tick(1 * time.Minute) {
			h.ScanSLABreaches()
		}
	}()

	// V22-② 告警风暴收尾扫描：每分钟关闭已结束风暴
	go func() {
		for range time.Tick(1 * time.Minute) {
			h.ScanStormEnd()
		}
	}()

	// V29-③ 趋势预测扫描：每10分钟检查24h内将越限的测点，主动预警
	go func() {
		for range time.Tick(10 * time.Minute) {
			h.ScanTrendForecasts()
		}
	}()

	// V50 资产健康评估：启动30秒后首评（错开建表/模拟器），之后每15分钟
	go func() {
		time.Sleep(30 * time.Second)
		stats := h.ScanAssetHealth()
		log.Printf("V50 资产健康首评: evaluated=%v risk=%v", stats["evaluated"], stats["risk"])
		for range time.Tick(15 * time.Minute) {
			h.ScanAssetHealth()
		}
	}()

	// V56 容量饱和预测：启动60秒后回填历史(表空才回填)+首评，之后每小时快照+预测
	go func() {
		time.Sleep(60 * time.Second)
		if n := h.BackfillCapacityHistory(); n > 0 {
			log.Printf("V56 容量历史回填: %d 行", n)
		}
		h.SnapshotCapacityToday()
		stats := h.ForecastCapacity()
		log.Printf("V56 容量预测首评: evaluated=%v risk=%v", stats["evaluated"], stats["risk"])
		for range time.Tick(1 * time.Hour) {
			h.BackfillCapacityHistory() // 新站点自动补历史
			h.SnapshotCapacityToday()
			h.ForecastCapacity()
		}
	}()

	// V54 供电冗余评估：启动45秒后首评（错开健康首评），之后每小时（拓扑变化频率低）
	go func() {
		time.Sleep(45 * time.Second)
		stats := h.ScanPowerRedundancy()
		log.Printf("V54 供电冗余首评: evaluated=%v risk=%v notified=%v", stats["evaluated"], stats["risk"], stats["notified"])
		for range time.Tick(1 * time.Hour) {
			h.ScanPowerRedundancy()
		}
	}()

	// 演示数据模拟器（simulate.enabled=true 时启动，真实网关接入后可关闭）
	go api.StartSimulator(db, api.SimulatorConfig{
		Enabled:     cfg.Simulate.Enabled,
		IntervalSec: cfg.Simulate.IntervalSec,
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("DCIM API 启动: http://%s (P2)", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
