# DCIM 数据中心基础设施管理系统

轻量级数据中心基础设施管理（DCIM）平台：资产台账、动环告警、工单巡检、容量与能耗（PUE）、3D 机房可视化、移动端运维，一套系统覆盖机房日常运维闭环。

A lightweight Data Center Infrastructure Management platform: asset ledger, environmental alarms, work orders & inspections, capacity & PUE analytics, 3D room visualization, and mobile ops.

## 功能特性

- **可视化大屏**：机房总览 KPI、告警分级、能耗趋势，指标可穿透到过滤列表；随主题换色
- **3D 机房**：按面积规划机柜、面对面成列/冷通道布局、拖拽编辑、构件库、CAD→GLB 导入；透视柜门查看柜内设备，点击设备弹出信息卡，叠加测点/告警/PUE 面板
- **资产管理**：设备全生命周期台账（入库/上架/维修/退役）、分类与位置树、附件照片、盘点
- **告警中心**：动环测点阈值告警、分级/风暴抑制、飞书群机器人交互卡片推送、移动端详情跳转
- **工单与巡检**：工单全流程（创建→派发→处理→验收）、SLA 违约预警、巡检计划/扫码巡检/巡检报告
- **容量与能耗**：机柜 U 位/电力/制冷容量、PUE 计算与趋势、能耗分析、低库存备件预警与采购联动
- **运维协同**：值班排班与交接、变更管理全流程、维保合同与到期提醒、健康度评估与替换建议
- **报表与洞察**：运营报表、全局搜索（Cmd+K）、AI 洞察（可选，接入任意 OpenAI 兼容 LLM）
- **移动端**：uni-app 一套代码双端构建（H5 + 微信小程序），27 页移动运维界面，拍照上传、扫码巡检、消息推送

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.22 · Gin · GORM · PostgreSQL 16 |
| PC 前端 | Vue 3 · Vite · Pinia · ECharts · Three.js（Light-Tech 设计体系，6 套主题） |
| 移动端 | uni-app（Vue 3）→ H5 / 微信小程序 |
| 部署 | Nginx 反代 + systemd，单机即可运行 |

## 目录结构

```
backend/    Go API 服务（cmd/dcim-api 入口，internal/ 业务模块，configs/ 配置）
frontend/   Vue3 PC 管理端
mobile/     uni-app 移动端（H5 + 微信小程序双目标）
edge/       边缘采集网关适配
deploy/     部署脚本
```

## 快速开始

### 1. 准备数据库

```bash
sudo -u postgres psql -c "CREATE USER dcim WITH PASSWORD '你的密码';"
sudo -u postgres psql -c "CREATE DATABASE dcim_platform OWNER dcim;"
```

### 2. 配置后端

```bash
cd backend
cp configs/production.example.yaml configs/production.yaml
# 编辑 production.yaml：数据库连接、JWT secret（务必修改 CHANGE_ME）
# 可选：cp configs/ai.env.example configs/ai.env 并填入你的 LLM Key（AI 洞察功能）
```

### 3. 启动后端

```bash
cd backend
go run ./cmd/dcim-api -c configs/production.yaml
# 默认监听 127.0.0.1:8095；首次启动自动建表并创建管理员
# 初始账号 admin，初始密码由 DCIM_ADMIN_INIT_PASSWORD 指定（默认 ChangeMe@123，登录后立即修改）
```

### 4. 启动 PC 前端

```bash
cd frontend
npm install
npm run dev        # 开发模式；生产: npm run build 后由 nginx 托管 dist/
```

### 5. 移动端（可选）

```bash
cd mobile
npm install
npm run build:h5          # H5 → dist/build/h5，nginx 挂到 /m/
npm run build:mp-weixin   # 微信小程序 → dist/build/mp-weixin，微信开发者工具导入
```

小程序端需在 `src/config.js` 填入已备案的 HTTPS 域名（或在 App「我的-服务器设置」中配置）。

### Nginx 参考

```nginx
location / { root /path/to/frontend/dist; try_files $uri $uri/ /index.html; }
location /api/ { proxy_pass http://127.0.0.1:8095; client_max_body_size 20m; }
location ^~ /m/ { alias /path/to/mobile/dist/build/h5/; try_files $uri $uri/ /m/index.html; }
```

## 许可证

[MIT](LICENSE) © 2026 北京东青互联科技有限公司（DQINFOS）