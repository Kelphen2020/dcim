package api

import (
	"dcim/backend/internal/middleware"
	"dcim/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册全部路由
// 路由规范：/api/v1 前缀；写操作需登录；危险操作后续阶段加密码确认
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"service": "dcim-api", "phase": "P5"}})
	})

	v1 := r.Group("/api/v1")
	{
		// 公开端点
		v1.GET("/captcha", h.GetCaptcha)
		v1.POST("/login", middleware.LoginRateLimitMiddleware(), h.Login)
		// 微信小程序一键登录（code换openid；绑定走账号密码验证）
		v1.POST("/wx-login", middleware.LoginRateLimitMiddleware(), h.WxLogin)
		v1.POST("/wx-bind", middleware.LoginRateLimitMiddleware(), h.WxBind)
		v1.GET("/public-settings", h.GetPublicSettings)
		// 边缘网关接入（独立密钥认证）
		v1.POST("/gateway/register", h.GatewayRegister)
				v1.POST("/gateway/heartbeat", h.GatewayHeartbeat)
				// V47: XM-E16 边缘网关通道（X-Gateway-Code/Secret 头认证）
				gwEdge := v1.Group("/gateway")
				gwEdge.GET("/points", h.GatewayPoints)
				gwEdge.POST("/heartbeat2", h.GatewayHeartbeat2)
				gwEdge.GET("/commands/pull", h.GatewayCommandPull)
				gwEdge.POST("/commands/:cid/result", h.GatewayCommandResult)
				// 边缘网关推送读数（无需用户认证）
				r.POST("/api/v1/push-reading", h.PushReading)
				r.POST("/api/v1/push-readings", h.BatchPushReadings)

		// 登录保护组（SiteScope 注入当前用户可见站点范围）
		authed := v1.Group("", middleware.Auth(), middleware.SiteScope(), middleware.DenyViewerWrites())
		{
			// 用户下拉选项（工单改派/巡检责任人等，所有登录用户可用）
			authed.GET("/users-options", h.ListUserOptions)

			authed.GET("/me", h.Profile)
			authed.POST("/me/password", h.ChangePassword)
			authed.POST("/me/wx-unbind", h.WxUnbind)
			authed.PUT("/me/profile", h.UpdateProfile)

			// 仪表盘（所有角色可读）
			authed.GET("/dashboard", h.Dashboard)
			authed.GET("/search", h.GlobalSearch) // V76 B5-1 全局搜索（六类，SiteScope+菜单组权限）
			authed.GET("/dashboard/insights", h.DashboardInsights) // V65 B1-2 风险与洞察聚合区

			// 四级层级（读开放给所有登录用户，写由 RBAC 控制）
			authed.GET("/sites", h.ListSites)
			authed.GET("/rooms", h.ListRooms)
			authed.GET("/rows", h.ListRows)
			authed.GET("/cabinets", h.ListCabinets)
			authed.GET("/gateways", h.ListGateways)
			authed.GET("/gateway-stats", h.GatewayStats)
			authed.GET("/topology-stats", h.TopologyStats)
			authed.GET("/audits", h.ListAudits)

			// P1 资产（读开放；容量视图读开放）
			authed.GET("/assets", h.ListAssets)
			authed.GET("/cabinets/:id/capacity", h.CabinetCapacity)
			authed.GET("/cabinets/:id/what-if", h.CabinetWhatIf)
			authed.GET("/maintenances", h.ListMaintenances)
			// V13 资产盘点（读开放给所有登录用户）
			authed.GET("/stocktakes", h.ListStocktakes)
			authed.GET("/stocktakes/:id", h.GetStocktake)

			// P2 运维（读开放给所有登录用户）
			authed.GET("/work-orders", h.ListWorkOrders)
			authed.GET("/work-orders/assignee-suggestion", h.WorkOrderAssigneeSuggestion) // V74/B4-1 当班人建议
			authed.GET("/work-orders-logs/:id", h.GetWorkOrderLogs)
			authed.GET("/inspection-plans", h.ListInspectionPlans)
			authed.GET("/inspection-records", h.ListInspectionRecords)
			authed.GET("/changes", h.ListChanges)
			// V17 运维联动闭环
			authed.GET("/maintenances/coverage", h.MaintenanceCoverage)
			// V18 SLA + 外发通知通道（读开放）
			authed.GET("/sla-config", h.GetSLAConfig)
			authed.GET("/work-orders/sla-stats", h.SLAStats)
			authed.GET("/work-orders/sla-analysis", h.SLAAnalysis)
			authed.GET("/alerts/quality-analysis", h.AlertQualityAnalysis) // V64 告警质量治理
			// V53 AI 运维助手
			authed.GET("/ai/status", h.AIStatus)
			authed.POST("/ai/assistant", h.AIAssistant)
			authed.POST("/ai/report", h.AIReport)
			authed.GET("/notify-channels", h.ListNotifyChannels)
			// V19 扫码巡检 + 巡检报告（读开放）
			authed.GET("/inspection-scan/qr", h.GenScanQR)
			authed.GET("/inspection-scan/qr-batch", h.BatchScanQR)
			authed.GET("/inspection-reports", h.ListInspectionReports)
			authed.GET("/inspection-reports/:id", h.InspectionReport)
			// V22 故障案例库/告警风暴/值班（读开放）
			// V59 应急预案与演练（match 与 :id 同级共存，与 fault-cases/recommend 同模式）
			authed.GET("/emergency-plans", h.ListEmergencyPlans)
			authed.GET("/emergency-plans/match", h.MatchEmergencyPlans)
			authed.GET("/emergency-plans/:id", h.GetEmergencyPlan)
			// V60 资产退役与处置（stats 与 :id 同级共存）
			authed.GET("/asset-retirements", h.ListAssetRetirements)
			authed.GET("/asset-retirements/stats", h.RetirementStats)
			authed.GET("/asset-retirements/:id", h.GetAssetRetirement)
			// V61 运维月报
			authed.GET("/ops-reports", h.ListOpsReports)
			authed.GET("/ops-reports/:id", h.GetOpsReport)
			// V62 巡检任务与漏检治理
			authed.GET("/inspection-tasks", h.ListInspectionTasks)
			authed.GET("/inspection-tasks/stats", h.InspectionTaskStats)
			authed.GET("/fault-cases", h.ListFaultCases)
			authed.GET("/fault-cases/:id", h.GetFaultCase)
			authed.GET("/fault-cases/recommend", h.RecommendFaultCases)
			authed.GET("/alert-storms", h.ListAlertStorms)
			authed.GET("/alert-storms/:id", h.StormDetail)
			authed.GET("/duty-shifts", h.ListDutyShifts)
			authed.GET("/duty/current", h.CurrentDuty)
			// M1 移动端聚合接口
			authed.GET("/mobile/overview", h.MobileOverview)
			authed.POST("/mobile/upload", h.MobileUpload)
			authed.GET("/mobile/attachments", h.ListAttachments)
			authed.GET("/mobile/push-channels", h.ListMyPushChannels)
			authed.POST("/mobile/push-channels", h.UpsertPushChannel)
			authed.DELETE("/mobile/push-channels/:id", h.DeletePushChannel)
			// M2 移动端增值：交接签字
			authed.POST("/duty/handover/sign", h.SignDutyHandover)
			authed.GET("/duty/handover/signs", h.ListDutyHandoverSigns)
			// M3 移动端管理摘要
			authed.GET("/mobile/summary", h.MobileSummary)
			authed.GET("/duty/handover", h.DutyHandover)
			authed.GET("/changes/:id/impact", h.ChangeImpact)
			authed.GET("/changes/:id/backups", h.ChangeBackups)
			authed.POST("/inspection-items-live", h.InspectionItemsLive)
			authed.GET("/power-chains", h.ListPowerChainsRich)
			// V14 业务关联深化
			authed.GET("/power-chains/impact/:id", h.PowerChainImpact)
			authed.GET("/capacity-overview", h.CapacityOverview)
			authed.GET("/cabinets/:id/context", h.CabinetContext)
			authed.GET("/network-topology-rich", h.NetworkTopologyRich)
			authed.GET("/network-devices/:id/ports", h.DevicePorts)
			authed.GET("/network-devices/:id/points", h.DevicePoints)
			authed.GET("/network-devices/asset-options", h.NetdevAssetOptions) // V69/B2-4 从资产创建候选
			authed.POST("/network-devices/:id/heartbeat", h.NetdevHeartbeat) // V67/B2-2 心跳上报（离线判定数据源）

			// V2 拓扑图布局（读开放）
			authed.GET("/topo-diagram", h.GetTopoDiagram)

			// V3 趋势分析
			authed.GET("/point-tree", h.PointTree)
			authed.GET("/trend-series", h.TrendSeries)
			authed.GET("/trend-templates", h.ListTrendTemplates)
			authed.POST("/trend-templates", h.CreateTrendTemplate)
			authed.DELETE("/trend-templates/:id", h.DeleteTrendTemplate)
			// V3 告警五态流转 + 留痕
			authed.POST("/alerts/:id/action", h.AlertAction)
			authed.GET("/alerts/:id/logs", h.AlertLogs)

			// P2 通知中心
			authed.GET("/notifications", h.ListNotifications)
			authed.GET("/notifications-unread-count", h.UnreadCount)

			// V5 能耗总览
			authed.GET("/energy/overview", h.EnergyOverview)
			// V49 PUE 能效合规（读）
			authed.GET("/pue-policies", h.ListPUEPolicies)
			authed.GET("/energy/compliance", h.EnergyCompliance)

			// V50 资产健康度（读）
			authed.GET("/asset-health", h.ListAssetHealth)
			authed.GET("/asset-health/summary", h.GetAssetHealthSummary)
			authed.GET("/asset-health/config", h.GetAssetHealthConfig)
			authed.GET("/asset-health/:asset_id", h.GetAssetHealthDetail)
			authed.GET("/power/redundancy", h.ListPowerRedundancy)          // V54 供电冗余总览
			authed.GET("/power/redundancy/:site_id", h.GetPowerRedundancySite) // V54 站点冗余详情
			authed.GET("/capacity/forecast", h.ListCapacityForecast)            // V56 容量饱和预测总览
			authed.GET("/capacity/forecast/:site_id", h.GetCapacityForecastSite) // V56 站点历史+预测曲线

			// P1 上架/下架：值班员以上可操作（日常上下架作业）
			ops := authed.Group("", middleware.RequireRoles("global_admin", "site_admin", "operator"))
			{
				ops.POST("/assets/:id/mount", h.MountAsset)
				ops.POST("/assets/:id/unmount", h.UnmountAsset)

				// P2 工单（报单/流转/日志查看：值班员以上）
				ops.POST("/work-orders", h.CreateWorkOrder)
				ops.PUT("/work-orders/:id", h.UpdateWorkOrder)
				ops.POST("/work-orders/:id/:action", h.transitionWorkOrder)
				ops.POST("/asset-health/:asset_id/work-order", h.CreateHealthWorkOrderAPI) // V50 预测性维护工单
				ops.GET("/assets/:id/replacement-suggestion", h.ReplacementSuggestion) // V75 替换建议（健康度/退役）
				ops.POST("/assets/:id/replacement-request", h.CreateReplacementRequest) // V75 生成替换采购申请
				ops.POST("/power/redundancy/:site_id/work-order", h.CreateRedundancyWorkOrderAPI) // V54 供电冗余整改工单
				ops.POST("/capacity/forecast/:site_id/work-order", h.CreateCapacityWorkOrderAPI) // V56 容量扩容规划工单

				// P2 巡检（提交巡检结果：值班员以上）
				ops.POST("/inspection-records", h.SubmitInspection)
				// V19 扫码签到/提交（值班员以上）
				ops.POST("/inspection-scan/checkin", h.ScanCheckin)
				ops.POST("/inspection-scan/submit", h.ScanSubmit)
				// V22 故障案例沉淀（值班员以上）
				ops.POST("/emergency-plans", h.CreateEmergencyPlan)         // V59
				ops.POST("/emergency-plans/:id/drill", h.CreateDrillRecord) // V59 演练登记
				ops.POST("/drill-records/:id/work-order", h.DrillWorkOrder)   // V59 问题转整改工单
			// V60 资产退役与处置
			ops.POST("/asset-retirements", h.CreateAssetRetirement)
			ops.POST("/asset-retirements/from-stocktake-item/:id", h.CreateRetirementFromStocktakeItem)
			ops.POST("/asset-retirements/:id/disposal", h.DisposeAssetRetirement)
			ops.POST("/asset-retirements/:id/cancel", h.CancelAssetRetirement)
			ops.PUT("/inspection-tasks/:id/exempt", h.ExemptInspectionTask) // V62 漏检豁免（值班员以上）
				ops.POST("/fault-cases", h.CreateFaultCase)
				ops.POST("/fault-cases/from-work-order/:id", h.PromoteFromWorkOrder)

				// P2 变更（起草/提审/执行/完成/回滚：值班员以上；审批另限管理员）
				ops.POST("/changes", h.CreateChange)
				ops.PUT("/changes/:id", h.UpdateChange)
				ops.POST("/changes/:id/:action", h.TransitionChange)

				// P2 供电链路维护：值班员以上
				ops.POST("/power-chains", h.CreatePowerChain)
				ops.PUT("/power-chains/:id", h.UpdatePowerChain)
				ops.DELETE("/power-chains/:id", h.DeletePowerChain)

				// V2 拓扑图布局保存：值班员以上
				ops.PUT("/topo-diagram", h.SaveTopoDiagram)

				// P2 通知标记已读：值班员以上（自己的通知）
				ops.POST("/notifications-read/:id", h.ReadNotifications)
			}

			// 站点本身的增删改：仅全局管理员（站点是全局资源）
			siteAdmin := authed.Group("", middleware.RequireRoles("global_admin"))
			{
				siteAdmin.POST("/sites", h.CreateSite)
				siteAdmin.PUT("/sites/:id", h.UpdateSite)
				siteAdmin.DELETE("/sites/:id", h.DeleteSite)
			}

			// 站点下级资源：站点管理员以上
			admin := authed.Group("", middleware.RequireRoles("global_admin", "site_admin"))
			{
				// V49 PUE 政策库维护（管理员）
				admin.POST("/pue-policies", h.CreatePUEPolicy)
				admin.PUT("/pue-policies/:id", h.UpdatePUEPolicy)
				admin.DELETE("/pue-policies/:id", h.DeletePUEPolicy)
				admin.POST("/energy/pue-scan", h.TriggerPUEScan) // V73/B3-3 手动触发 PUE 合规扫描（可 ?site_id=）
				admin.POST("/asset-health/evaluate", h.EvaluateAssetHealth) // V50 手动触发全量健康评估
				admin.POST("/power/redundancy/evaluate", h.EvaluatePowerRedundancy) // V54 手动触发供电冗余评估
				admin.POST("/capacity/forecast/evaluate", h.EvaluateCapacityForecast) // V56 手动触发容量预测
				admin.GET("/settings/ai", h.GetAIConfig)          // V58 大模型配置读取（密钥掩码）
				admin.PUT("/settings/ai", h.PutAIConfig)          // V58 保存并热生效
				admin.POST("/settings/ai/test", h.TestAIConfig)   // V58 连接测试
				admin.DELETE("/settings/ai", h.ResetAIConfig)     // V58 清除并回退环境配置

				admin.POST("/rooms", h.CreateRoom)
				admin.PUT("/rooms/:id", h.UpdateRoom)
				admin.DELETE("/rooms/:id", h.DeleteRoom)

				admin.POST("/rows", h.CreateRow)
				admin.PUT("/rows/:id", h.UpdateRow)
				admin.DELETE("/rows/:id", h.DeleteRow)

				admin.POST("/cabinets", h.CreateCabinet)
				admin.PUT("/cabinets/:id", h.UpdateCabinet)
				admin.DELETE("/cabinets/:id", h.DeleteCabinet)

				admin.POST("/gateways", h.CreateGateway)
				admin.PUT("/gateways/:id", h.UpdateGateway)
				admin.DELETE("/gateways/:id", h.DeleteGateway)
				// V16 网关密钥重置 + 资产批量状态
				admin.POST("/gateways/:id/reset-secret", h.ResetGatewaySecret)
				admin.POST("/gateways/:id/command", h.GatewayCommand)
				authed2 := v1.Group("", middleware.Auth(), middleware.SiteScope())
				authed2.GET("/gateways/:id/commands", h.GatewayCommands)
				admin.POST("/assets/batch", h.BatchAssets)

				// V4 系统设置（站点默认主题等）
				admin.GET("/system-settings", h.ListSystemSettings)
				admin.PUT("/system-setting", h.SetSystemSetting)

				// V19 报告签发（站点管理员以上）
				admin.POST("/inspection-reports/:id/sign", h.SignInspectionReport)
				// V22 案例编辑/删除 + 值班排班管理（站点管理员以上）
				admin.POST("/emergency-plans/scan-overdue", h.ScanOverdueDrillsNow) // V59 手动触发逾期检查
				admin.PUT("/emergency-plans/:id", h.UpdateEmergencyPlan)   // V59
				admin.DELETE("/emergency-plans/:id", h.DeleteEmergencyPlan) // V59（内置/有演练记录拒删）
				// V61 运维月报（生成/发布/删除）
				admin.POST("/ops-reports/generate", h.GenerateOpsReport)
				admin.PUT("/ops-reports/:id/publish", h.PublishOpsReport)
				admin.DELETE("/ops-reports/:id", h.DeleteOpsReport)
				// V62 巡检任务（生成/漏检扫描/改派：管理员）
				admin.POST("/inspection-tasks/generate", h.GenerateInspectionTasksAPI)
				admin.POST("/inspection-tasks/scan-missed", h.ScanMissedTasksAPI)
				admin.PUT("/inspection-tasks/:id/assign", h.AssignInspectionTask)
			admin.POST("/asset-retirements/:id/approve", h.ApproveAssetRetirement) // V60 退役审批
				admin.PUT("/fault-cases/:id", h.UpdateFaultCase)
				admin.DELETE("/fault-cases/:id", h.DeleteFaultCase)
				admin.POST("/duty-shifts", h.CreateDutyShift)
				admin.DELETE("/duty-shifts/:id", h.DeleteDutyShift)

				// V18 SLA 配置 + 通知通道管理（站点管理员以上）
				admin.PUT("/sla-config", h.SetSLAConfig)
				admin.POST("/notify-channels", h.CreateNotifyChannel)
				admin.PUT("/notify-channels/:id", h.UpdateNotifyChannel)
				admin.DELETE("/notify-channels/:id", h.DeleteNotifyChannel)
				admin.POST("/notify-channels/:id/test", h.TestNotifyChannel)
				admin.POST("/sla-scan", h.TriggerSLAScan)

				// P1 资产台账与维保管理
				admin.POST("/assets", h.CreateAsset)
				admin.PUT("/assets/:id", h.UpdateAsset)
				admin.DELETE("/assets/:id", h.DeleteAsset)
				// V13 资产盘点写操作
				admin.POST("/stocktakes", h.CreateStocktake)
				admin.POST("/stocktakes/:id/start", h.StartStocktake)
				// H2: 现场核对下放已登录可写角色（viewer 被 DenyViewerWrites 拦截）
				authed.POST("/stocktakes/:id/check", h.CheckStocktakeItems)
				admin.POST("/stocktakes/:id/apply", h.ApplyStocktake)
				admin.DELETE("/stocktakes/:id", h.DeleteStocktake)
				authed.POST("/maintenances", h.CreateMaintenance)
				authed.PUT("/maintenances/:id", h.UpdateMaintenance)
				authed.DELETE("/maintenances/:id", h.DeleteMaintenance)
				authed.GET("/maintenances/expiring", h.ScanExpiringMaintenances)
				authed.POST("/notifications/push", h.PushNotifications)

				// P2 巡检计划管理：站点管理员以上
				admin.POST("/inspection-plans", h.CreateInspectionPlan)
				admin.PUT("/inspection-plans/:id", h.UpdateInspectionPlan)
				admin.DELETE("/inspection-plans/:id", h.DeleteInspectionPlan)

				// P2 通知广播：站点管理员以上
				admin.POST("/notifications-broadcast", h.BroadcastNotification)

				// P2 工单删除：站点管理员以上
				admin.DELETE("/work-orders/:id", h.DeleteWorkOrder)

				// P3 动环监控管理：站点管理员以上
				admin.POST("/monitoring-points", h.CreateMonitoringPoint)
				admin.PUT("/monitoring-points/:id", h.UpdateMonitoringPoint)
				admin.DELETE("/monitoring-points/:id", h.DeleteMonitoringPoint)
				admin.POST("/alert-rules", h.CreateAlertRule)
				admin.POST("/alert-rules/batch", h.BatchCreateAlertRules)
				admin.PUT("/alert-rules/:id", h.UpdateAlertRule)
				admin.DELETE("/alert-rules/:id", h.DeleteAlertRule)
				// V70 B3-1 高噪规则处置动作
				admin.GET("/alert-rules/:id/tuning-preview", h.RuleTuningPreview)
				admin.POST("/alert-rules/:id/tune", h.RuleTune)
				admin.POST("/alert-rules/:id/disable", h.RuleDisable)
				admin.POST("/alert-rules/:id/optimization-order", h.RuleOptimizationOrder)
				// V15 批量操作
				admin.POST("/monitoring-points/batch", h.BatchMonitoringPoints)
				admin.POST("/monitoring-points/import", h.ImportMonitoringPoints)
				admin.POST("/alerts/batch", h.BatchAlerts)
				// V13 告警升级：手动触发一次扫描（定时任务每分钟也会跑）
				admin.POST("/alert-escalation-scan", func(c *gin.Context) {
					n := h.ScanAlertEscalation()
					response.Success(c, gin.H{"escalated": n})
				})
				// V64 告警质量评估：手动触发一次高噪声规则扫描（定时任务每小时也会跑）
				admin.POST("/alerts/quality-scan", h.AlertQualityScan)

				// P4 网络管理：站点管理员以上
				admin.POST("/network-devices", h.CreateNetworkDevice)
				admin.PUT("/network-devices/:id", h.UpdateNetworkDevice)
				admin.DELETE("/network-devices/:id", h.DeleteNetworkDevice)
				admin.POST("/network-devices/offline-scan", h.TriggerNetdevScan) // V67/B2-2 手动触发离线判定
				admin.POST("/network-ports", h.CreateNetworkPort)
				admin.PUT("/network-ports/:id", h.UpdateNetworkPort)
				admin.DELETE("/network-ports/:id", h.DeleteNetworkPort)
				admin.POST("/network-links", h.CreateNetworkLink)
				admin.PUT("/network-links/:id", h.UpdateNetworkLink)
				admin.DELETE("/network-links/:id", h.DeleteNetworkLink)
				admin.POST("/ip-pools", h.CreateIPPool)
				admin.PUT("/ip-pools/:id", h.UpdateIPPool)
				admin.DELETE("/ip-pools/:id", h.DeleteIPPool)
				admin.POST("/ip-assignments", h.CreateIPAssignment)
				admin.PUT("/ip-assignments/:id", h.UpdateIPAssignment)
				admin.DELETE("/ip-assignments/:id", h.DeleteIPAssignment)
				admin.POST("/vlans", h.CreateVLAN)
				admin.PUT("/vlans/:id", h.UpdateVLAN)
				admin.DELETE("/vlans/:id", h.DeleteVLAN)
				admin.POST("/config-backups", h.CreateConfigBackup)
				admin.DELETE("/config-backups/:id", h.DeleteConfigBackup)
			}

			// 已登录用户可读 P4
			authed.GET("/network-devices", h.ListNetworkDevices)
			authed.GET("/network-ports", h.ListNetworkPorts)
			authed.GET("/network-links", h.ListNetworkLinks)
			authed.GET("/network-topology", h.NetworkTopology)
			authed.GET("/network-summary", h.NetworkSummary)
			authed.GET("/ip-pools", h.ListIPPools)
			authed.GET("/ip-assignments", h.ListIPAssignments)
			authed.GET("/vlans", h.ListVLANs)
			authed.GET("/config-backups", h.ListConfigBackups)
			authed.GET("/config-backups/:id", h.GetConfigBackup)

			// P5 大屏与高级能力
			authed.GET("/big-screen", h.BigScreenData)
		authed.GET("/room3d", h.Room3DData)
		// V27 3D模型库
		authed.GET("/model-assets", h.ListModelAssets)
		admin.POST("/model-assets", h.UploadModelAsset)
		admin.DELETE("/model-assets/:id", h.DeleteModelAsset)
		// V26 3D 自定义布局
		authed.GET("/room3d/layout", h.GetRoomLayout)
		authed.GET("/room3d/plan-hint", h.PlanHint)
		admin.PUT("/room3d/layout", h.SaveRoomLayout)
		admin.POST("/room3d/plan-cabinets", h.PlanCabinets)
		admin.POST("/room3d/model-upload", h.UploadModel)
		admin.DELETE("/room3d/layout/:id", h.DeleteRoomLayout)
		admin.DELETE("/room3d/layout", h.ResetRoomLayout)
		// V28 布局模板
		authed.GET("/layout-templates", h.ListLayoutTemplates)
		admin.POST("/layout-templates", h.SaveLayoutTemplate)
		admin.POST("/layout-templates/:id/apply", h.ApplyLayoutTemplate)
		admin.DELETE("/layout-templates/:id", h.DeleteLayoutTemplate)
		// V29 备件台账 + 趋势预测
		authed.GET("/spare-parts", h.ListSpareParts)
		authed.GET("/spare-parts/stats", h.SpareStats)
		authed.GET("/spare-parts/:id/movements", h.SpareMovements)
		authed.GET("/trend-forecast", h.TrendForecastAll)
		authed.GET("/trend-forecast/:id", h.TrendForecastOne)
		admin.POST("/spare-parts", h.CreateSparePart)
		admin.PUT("/spare-parts/:id", h.UpdateSparePart)
		admin.DELETE("/spare-parts/:id", h.DeleteSparePart)
		// M6-F: 出入库下放到已登录可写角色（viewer 被 DenyViewerWrites 拦截），移动端现场领用
		authed.POST("/spare-parts/:id/move", h.MoveSpare)
		// V30 维保备件清单 + 采购申请流
		authed.GET("/maintenances/:id/spares", h.ListMaintenanceSpares)
		authed.GET("/purchase-requests", h.ListPurchaseRequests)
		authed.POST("/purchase-requests", h.CreatePurchaseRequest)
		admin.PUT("/maintenances/:id/spares", h.SaveMaintenanceSpares)
		admin.POST("/maintenances/:id/consume", h.ConsumeMaintenanceSpares)
		admin.POST("/purchase-requests/:id/review", h.ReviewPurchaseRequest)
		admin.POST("/purchase-requests/:id/stock", h.StockPurchaseRequest)
		admin.POST("/spare-auto-purchase-scan", h.TriggerSpareAutoPurchase) // B3-2/G4 低库存自动采购扫描手动触发
		// V31 备件BOM关联
		authed.GET("/spare-boms", h.ListSpareBOMs)
		authed.GET("/spare-boms/recommend/:id", h.RecommendSpares)
		admin.POST("/spare-boms", h.CreateSpareBOM)
		admin.DELETE("/spare-boms/:id", h.DeleteSpareBOM)
			authed.GET("/report", h.ReportSummary)
			authed.GET("/pue", h.ListPUERecords)
			authed.POST("/pue", h.AddPUERecord)
			authed.GET("/api-docs", h.APIDocs)

			// 已登录用户可读
			authed.GET("/monitoring-points", h.ListMonitoringPoints)
			authed.GET("/monitoring-summary", h.MonitoringSummaryRich)
			authed.GET("/readings", h.QueryReadings)
			authed.GET("/alert-rules", h.ListAlertRules)
			authed.GET("/alerts", h.ListAlerts)
			authed.POST("/alerts/:id/acknowledge", h.AcknowledgeAlert)
			authed.POST("/alerts/:id/resolve", h.ResolveAlert)
			// V15 监控系统实用化
			authed.GET("/points-live", h.PointsLive)
			authed.GET("/trend-downsample", h.TrendDownsample)
			authed.GET("/trend-thresholds", h.TrendThresholds)
			authed.GET("/trend-compare", h.TrendSeriesCompare)

			// 用户管理：仅全局管理员
			um := authed.Group("/users", middleware.RequireRoles("global_admin"))
			{
				um.GET("", h.ListUsers)
				um.POST("", h.CreateUser)
				um.PUT("/:id", h.UpdateUser)
				um.POST("/:id/reset-password", h.ResetUserPassword)
				um.DELETE("/:id", h.DeleteUser)
			}
		}
	}
}
