import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/login', component: () => import('../views/Login.vue'), meta: { public: true } },
  { path: '/inspection-scan', component: () => import('../views/InspectionScan.vue'), meta: { title: '扫码巡检' } },
  {
    path: '/',
    component: () => import('../views/Layout.vue'),
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '运行总览' } },
      { path: 'topology', component: () => import('../views/Topology.vue'), meta: { title: '基础设施层级' } },
      { path: 'assets', component: () => import('../views/Assets.vue'), meta: { title: '资产台账' } },
      { path: 'stocktake', component: () => import('../views/Stocktake.vue'), meta: { title: '资产盘点' } },
      { path: 'asset-retirement', component: () => import('../views/AssetRetirement.vue'), meta: { title: '资产退役处置' } },
      { path: 'maintenances', component: () => import('../views/Maintenances.vue'), meta: { title: '维保管理' } },
      { path: 'capacity', component: () => import('../views/CabinetCapacity.vue'), meta: { title: '机柜容量' } },
      { path: 'work-orders', component: () => import('../views/WorkOrders.vue'), meta: { title: '运维工单' } },
      { path: 'inspection', component: () => import('../views/Inspection.vue'), meta: { title: '巡检管理' } },
      { path: 'inspection-reports', component: () => import('../views/InspectionReports.vue'), meta: { title: '巡检报告' } },
      { path: 'inspection-reports/:id', component: () => import('../views/InspectionReports.vue'), meta: { title: '巡检报告' } },
      { path: 'changes', component: () => import('../views/Changes.vue'), meta: { title: '变更管理' } },
      { path: 'fault-cases', component: () => import('../views/FaultCases.vue'), meta: { title: '故障案例库' } },
      { path: 'emergency-plans', component: () => import('../views/EmergencyPlans.vue'), meta: { title: '应急预案' } },
      { path: 'duty-roster', component: () => import('../views/DutyRoster.vue'), meta: { title: '值班与交接' } },
      { path: 'spare-parts', component: () => import('../views/SpareParts.vue'), meta: { title: '备件台账' } },
      { path: 'trend-forecast', component: () => import('../views/TrendForecast.vue'), meta: { title: '趋势预测' } },
      { path: 'power-chain', component: () => import('../views/PowerChain.vue'), meta: { title: '供电链路' } },
      { path: 'power-sld', component: () => import('../views/PowerSld.vue'), meta: { title: '供电单线图' } },
      { path: 'monitor', component: () => import('../views/MonitorDashboard.vue'), meta: { title: '动环监控' } },
      { path: 'monitoring-points', component: () => import('../views/MonitoringPoints.vue'), meta: { title: '测点管理' } },
      { path: 'alert-rules', component: () => import('../views/AlertRules.vue'), meta: { title: '告警规则' } },
      { path: 'alert-wall', component: () => import('../views/AlertWall.vue'), meta: { title: '告警盯屏' } },
      { path: 'trend-analysis', component: () => import('../views/TrendAnalysis.vue'), meta: { title: '趋势分析' } },
      { path: 'energy', component: () => import('../views/Energy.vue'), meta: { title: '能耗总览' } },
      { path: 'theme-center', component: () => import('../views/ThemeCenter.vue'), meta: { title: '主题中心' } },
      { path: 'room-3d', component: () => import('../views/Room3D.vue'), meta: { title: '3D机房' } },
      { path: 'network-devices', component: () => import('../views/NetworkDevices.vue'), meta: { title: '网络设备' } },
      { path: 'network-topology', component: () => import('../views/NetworkTopology.vue'), meta: { title: '网络拓扑' } },
      { path: 'network-links', component: () => import('../views/NetworkLinks.vue'), meta: { title: '链路台账' } },
      { path: 'ip-management', component: () => import('../views/IPManagement.vue'), meta: { title: 'IP管理' } },
      { path: 'vlan-management', component: () => import('../views/VLANManagement.vue'), meta: { title: 'VLAN管理' } },
      { path: 'config-backups', component: () => import('../views/ConfigBackups.vue'), meta: { title: '配置备份' } },
      { path: 'big-screen', component: () => import('../views/BigScreen.vue'), meta: { title: '可视化大屏' } },
      { path: 'reports', component: () => import('../views/Reports.vue'), meta: { title: '报表中心' } },
      { path: 'api-docs', component: () => import('../views/ApiDocs.vue'), meta: { title: '开放API' } },
      { path: 'gateways', component: () => import('../views/Gateways.vue'), meta: { title: '站点网关' } },
      { path: 'users', component: () => import('../views/Users.vue'), meta: { title: '用户与权限', roles: ['global_admin'] } },
      { path: 'audits', component: () => import('../views/Audits.vue'), meta: { title: '审计日志' } },
      { path: 'notify-channels', component: () => import('../views/NotifyChannels.vue'), meta: { title: '通知通道', roles: ['global_admin', 'site_admin'] } },
      { path: 'settings', component: () => import('../views/Settings.vue'), meta: { title: '系统设置' } },
      { path: 'user-center', component: () => import('../views/UserCenter.vue'), meta: { title: '用户中心' } }
    ]
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' }
]

const router = createRouter({ history: createWebHashHistory(), routes })

router.beforeEach((to, from, next) => {
  const auth = useAuthStore()
  if (to.meta.public) return next()
  if (!auth.loggedIn) return next({ path: '/login', query: { redirect: to.fullPath } })
  if (to.meta.roles && !to.meta.roles.includes(auth.role)) return next('/dashboard')
  if (to.meta.title) document.title = to.meta.title + ' · DCIM'
  next()
})

export default router
