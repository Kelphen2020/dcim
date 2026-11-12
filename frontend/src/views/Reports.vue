<template>
  <div>
    <div class="page-head"><h1>报表中心</h1>
      <div class="seg" style="margin-top:10px">
        <button class="seg-btn" :class="{on:tab==='realtime'}" @click="switchTab('realtime')"><SvgIcon name="chart" /> 实时报表</button>
        <button class="seg-btn" :class="{on:tab==='monthly'}" @click="switchTab('monthly')"><SvgIcon name="doc" /> 运维月报</button>
      </div>
    </div>

    <!-- ==================== 实时报表（原有） ==================== -->
    <div v-show="tab==='realtime'">
      <div class="toolbar" style="margin-top:8px">
        <select class="input" style="width:180px" v-model.number="siteId" @change="load">
          <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <div class="seg">
          <button v-for="d in [7,30,90]" :key="d" class="seg-btn" :class="{on:days===d}" @click="days=d;load()">近 {{ d }} 天</button>
        </div>
        <span style="flex:1"></span>
        <button class="btn" @click="exportCSV">导出 CSV</button>
        <button class="btn btn-primary" @click="printReport"><SvgIcon name="printer" />打印报表</button>
        <button class="btn" @click="gotoMonthly" :title="lastMr ? '打开上月月报详情' : '切到运维月报页签生成上月报告'"><SvgIcon name="doc" />{{ lastMrLabel }} ›</button>
      </div>

      <!-- 当前状态 -->
      <div class="sec-title">当前状态</div>
      <div class="stat-grid">
        <div class="dash-card"><div class="stat-label">资产总数</div><div class="stat-num">{{ r.total_assets }}</div></div>
        <div class="dash-card"><div class="stat-label">在架</div><div class="stat-num ok">{{ r.in_rack }}</div></div>
        <div class="dash-card"><div class="stat-label">维修中</div><div class="stat-num warn">{{ r.in_maint }}</div></div>
        <div class="dash-card"><div class="stat-label">机柜占用</div><div class="stat-num">{{ r.used_cabinets ?? 0 }}/{{ r.total_cabinets ?? 0 }}</div></div>
        <div class="dash-card"><div class="stat-label">未结工单</div><div class="stat-num warn">{{ r.open_orders }}</div></div>
        <div class="dash-card"><div class="stat-label">已结工单</div><div class="stat-num ok">{{ r.closed_orders }}</div></div>
        <div class="dash-card"><div class="stat-label">变更总数</div><div class="stat-num">{{ r.total_changes }}</div></div>
        <div class="dash-card"><div class="stat-label">告警/已恢复</div><div class="stat-num">{{ r.total_alerts }}/{{ r.resolved_alerts }}</div></div>
      </div>

      <!-- 期间指标 -->
      <div class="sec-title">期间指标（近 {{ days }} 天）</div>
      <div class="stat-grid">
        <div class="dash-card"><div class="stat-label">新增工单</div><div class="stat-num">{{ r.period_orders }}</div></div>
        <div class="dash-card"><div class="stat-label">新增告警</div><div class="stat-num warn">{{ r.period_alerts }}</div></div>
        <div class="dash-card"><div class="stat-label">变更申请</div><div class="stat-num">{{ r.period_changes }}</div></div>
        <div class="dash-card"><div class="stat-label">巡检次数/异常</div><div class="stat-num">{{ r.period_inspections }}<small v-if="r.period_abnormal" class="warn"> / {{ r.period_abnormal }}</small></div></div>
        <div class="dash-card"><div class="stat-label">工单 MTTR</div><div class="stat-num ok">{{ r.mttr_hours }} <small>h</small></div></div>
        <div class="dash-card"><div class="stat-label">期间能耗</div><div class="stat-num">{{ fmt(r.period_energy_kwh) }} <small>kWh</small></div></div>
        <div class="dash-card"><div class="stat-label">期间电费</div><div class="stat-num">¥{{ fmt(r.period_cost_yuan) }}</div></div>
        <div class="dash-card"><div class="stat-label">期间均PUE/碳排</div><div class="stat-num">{{ r.period_avg_pue || '-' }} <small>{{ fmt(r.period_co2_kg) }}kg</small></div></div>
      </div>

      <!-- V18 SLA 达标率 -->
      <div class="sec-title">工单 SLA 时效（近 {{ days }} 天）<a class="link" style="margin-left:8px;font-weight:400;font-size:12px" @click="$router.push('/work-orders')">运维工单 ›</a></div>
      <div class="stat-grid">
        <div class="dash-card"><div class="stat-label">响应达标率</div><div class="stat-num" :class="sla.respond_rate>=95?'ok':sla.respond_rate>=80?'warn':''">{{ sla.respond_rate ?? '-' }}<small>%</small></div><div class="stat-sub">{{ sla.respond_ok ?? 0 }}/{{ sla.total ?? 0 }} 达标，违约 {{ sla.respond_breach ?? 0 }}</div></div>
        <div class="dash-card"><div class="stat-label">解决达标率</div><div class="stat-num" :class="sla.resolve_rate>=95?'ok':sla.resolve_rate>=80?'warn':''">{{ sla.resolve_rate ?? '-' }}<small>%</small></div><div class="stat-sub">{{ sla.resolve_ok ?? 0 }} 达标，违约 {{ sla.resolve_breach ?? 0 }}</div></div>
        <div class="dash-card"><div class="stat-label">当前超期未闭环</div><div class="stat-num" :class="sla.open_overdue?'warn':'ok'">{{ sla.open_overdue ?? 0 }}</div><div class="stat-sub">已过解决时限仍未完成</div></div>
        <div class="dash-card"><div class="stat-label">紧急单响应违约</div><div class="stat-num" :class="urgentBreach?'warn':'ok'">{{ urgentBreach }}</div><div class="stat-sub">优先级=紧急</div></div>
      </div>
      <div class="card" style="margin-top:12px" v-if="sla.by_priority?.length">
        <div class="card-title">分优先级 SLA 明细</div>
        <div class="table-wrap"><table class="data-table">
          <thead><tr><th>优先级</th><th>工单数</th><th>响应违约</th><th>解决违约</th><th>响应达标率</th></tr></thead>
          <tbody><tr v-for="p in sla.by_priority" :key="p.priority">
            <td><span class="badge">{{ p.label }}</span></td><td>{{ p.total }}</td>
            <td :class="p.respond_breach?'warn-text':''">{{ p.respond_breach }}</td>
            <td :class="p.resolve_breach?'warn-text':''">{{ p.resolve_breach }}</td>
            <td><div class="cat-bar"><i :style="{width:p.respond_rate+'%',background:p.respond_rate>=95?'#67c23a':p.respond_rate>=80?'#e6a23c':'#f56c6c'}"></i></div><span class="muted small">{{ p.respond_rate }}%</span></td>
          </tr></tbody>
        </table></div>
      </div>

      <!-- 图表区 -->
      <div class="grid-2" style="margin-top:16px">
        <div class="card">
          <div class="card-title">每日告警 / 工单趋势 <a class="link" @click="$router.push('/alert-wall')">告警盯屏 ›</a></div>
          <div class="chart-box" ref="trendEl"></div>
        </div>
        <div class="card">
          <div class="card-title">PUE 趋势 <a class="link" @click="$router.push('/energy')">能耗总览 ›</a></div>
          <div class="chart-box" ref="pueEl"></div>
        </div>
        <div class="card">
          <div class="card-title">工单类型分布 <a class="link" @click="$router.push('/work-orders')">运维工单 ›</a></div>
          <div class="chart-box" ref="woEl"></div>
        </div>
        <div class="card">
          <div class="card-title">告警严重度分布 <a class="link" @click="$router.push('/alert-rules')">告警规则 ›</a></div>
          <div class="chart-box" ref="sevEl"></div>
        </div>
      </div>

      <div class="card" style="margin-top:16px">
        <div class="card-title">资产类型分布 <a class="link" @click="$router.push('/assets')">资产台账 ›</a></div>
        <div class="table-wrap"><table class="data-table">
          <thead><tr><th>类型</th><th>数量</th><th>占比</th></tr></thead>
          <tbody>
            <tr v-for="c in r.asset_cats" :key="c.category">
              <td>{{ c.category }}</td><td>{{ c.count }}</td>
              <td><div class="cat-bar"><i :style="{width: catPct(c)+'%'}"></i></div><span class="muted small">{{ catPct(c) }}%</span></td>
            </tr>
            <tr v-if="!r.asset_cats?.length"><td colspan="3" class="empty">暂无数据</td></tr>
          </tbody>
        </table></div>
      </div>
    </div>

    <!-- ==================== V61 运维月报 ==================== -->
    <div v-show="tab==='monthly'">
      <div class="toolbar" style="margin-top:8px">
        <input type="month" class="input" style="width:170px" v-model="mr.month" title="要生成的月份" />
        <select class="input" style="width:160px" v-model.number="mr.genSite" title="报告范围">
          <option :value="0">全站</option>
          <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <label v-if="auth.isAdmin" class="chk-inline" title="覆盖已有报告，按当前数据重算"><input type="checkbox" v-model="mr.force" /> 强制重算</label>
        <button v-if="auth.isAdmin" class="btn btn-primary" :disabled="mr.busy" @click="genReport"><SvgIcon name="save" />{{ mr.busy ? '生成中…' : '生成月报' }}</button>
        <span style="flex:1"></span>
        <span class="muted small">共 {{ mr.items.length }} 份</span>
        <button class="btn" @click="loadReports"><SvgIcon name="refresh" />刷新</button>
      </div>

      <div class="card" style="margin-top:12px">
        <div class="card-title">月报档案<a class="link" @click="switchTab('realtime')">实时报表 ›</a><a class="link" @click="$router.push('/settings')">自动生成开关在系统设置 ›</a></div>
        <div class="table-wrap"><table class="data-table">
          <thead><tr><th>编号</th><th>月份</th><th>范围</th><th>摘要</th><th>状态</th><th>生成方式</th><th>生成时间</th><th style="width:250px">操作</th></tr></thead>
          <tbody>
            <tr v-for="it in mr.items" :key="it.id">
              <td><a class="link" @click="openReport(it.id)">{{ it.code }}</a></td>
              <td>{{ it.month }}</td>
              <td>{{ it.site_name }}</td>
              <td class="muted small mr-summary">{{ it.summary }}</td>
              <td><span class="badge" :class="it.status===2?'mr-pub':''">{{ it.status===2?'已发布':'草稿' }}</span></td>
              <td class="small">{{ it.auto_gen ? '系统自动' : (it.generated_by_name || '手动') }}</td>
              <td class="muted small">{{ fmtTime(it.created_at) }}</td>
              <td class="mr-ops">
                <button class="btn btn-mini" @click="openReport(it.id)"><SvgIcon name="eye" />查看</button>
                <button class="btn btn-mini" @click="openReport(it.id,true)"><SvgIcon name="download" />CSV</button>
                <button v-if="auth.isAdmin&&it.status!==2" class="btn btn-mini btn-primary" @click="publishReport(it)"><SvgIcon name="check-circle" />发布</button>
                <button v-if="auth.isAdmin" class="btn btn-mini btn-danger" @click="delReport(it)"><SvgIcon name="x-circle" />删除</button>
              </td>
            </tr>
            <tr v-if="!mr.items.length"><td colspan="8" class="empty">暂无月报——选择月份点「生成月报」；每月 1 日系统也会自动生成上月全站报告并通知</td></tr>
          </tbody>
        </table></div>
      </div>
      <p class="muted small" style="margin-top:8px">月报是生成时刻的全模块指标快照归档（告警/工单SLA/巡检/变更/能耗PUE合规/资产健康/维保/盘点/备件采购/网络/应急演练/可用性），供月度汇报与检查留档；数据修正后可「强制重算」。</p>
    </div>

    <!-- 月报详情抽屉 -->
    <div v-if="mr.show" class="drawer-mask" @click.self="closeReport">
      <div class="drawer">
        <div class="drawer-head"><h3>{{ mr.detail?.title || '运维月报' }}</h3>
          <button class="btn btn-mini" @click="closeReport"><SvgIcon name="x" /></button></div>
        <div class="drawer-body" v-if="mr.detail">
          <div class="mr-head">
            <div><b>{{ mr.detail.code }}</b>
              <span class="badge" :class="mr.detail.status===2?'mr-pub':''" style="margin-left:8px">{{ mr.detail.status===2?'已发布':'草稿' }}</span>
              <span class="muted small" style="margin-left:8px">{{ mr.detail.site_name }}</span></div>
            <div class="muted small">统计期间 {{ pc('period.start') }} ~ {{ pc('period.end') }} · 生成于 {{ fmtTime(mr.detail.created_at) }} · {{ mr.detail.auto_gen?'系统自动':(mr.detail.generated_by_name||'手动') }}</div>
          </div>

          <div class="sec-title">告警与事件</div>
          <div class="stat-grid">
            <div class="dash-card"><div class="stat-label">新增告警</div><div class="stat-num warn">{{ pc('alerts.created') }}</div><div class="stat-sub">严重 {{ pc('alerts.sev3') }} · 警告 {{ pc('alerts.sev2') }} · 提示 {{ pc('alerts.sev1') }}</div></div>
            <div class="dash-card"><div class="stat-label">已恢复</div><div class="stat-num ok">{{ pc('alerts.resolved') }}</div><div class="stat-sub">恢复用时均值 {{ pc('alerts.mttr_hours') }} h</div></div>
            <div class="dash-card"><div class="stat-label">告警升级</div><div class="stat-num">{{ pc('alerts.escalated') }}</div><div class="stat-sub">超时未确认自动升级</div></div>
            <div class="dash-card"><div class="stat-label">告警风暴</div><div class="stat-num" :class="pc('alerts.storms')?'warn':''">{{ pc('alerts.storms') }}</div><div class="stat-sub">抑制派生告警 {{ pc('alerts.storm_suppressed') }} 条</div></div>
          </div>
          <div class="card" style="margin-top:10px" v-if="pc('alerts.top_points')?.length">
            <div class="card-title">告警最多的测点 TOP5</div>
            <div class="table-wrap"><table class="data-table">
              <thead><tr><th>测点</th><th>编码</th><th>告警次数</th></tr></thead>
              <tbody><tr v-for="(t,i) in pc('alerts.top_points')" :key="i"><td>{{ t.pname }}</td><td class="muted small">{{ t.pcode || '-' }}</td><td class="warn-text">{{ t.cnt }}</td></tr></tbody>
            </table></div>
          </div>

          <div class="sec-title">工单与 SLA</div>
          <div class="stat-grid">
            <div class="dash-card"><div class="stat-label">新建工单</div><div class="stat-num">{{ pc('work_orders.created') }}</div><div class="stat-sub">故障类 {{ pc('work_orders.fault_orders') }}</div></div>
            <div class="dash-card"><div class="stat-label">完成工单</div><div class="stat-num ok">{{ pc('work_orders.finished') }}</div><div class="stat-sub">平均处理 {{ pc('work_orders.mttr_hours') }} h</div></div>
            <div class="dash-card"><div class="stat-label">SLA 响应达标率</div><div class="stat-num" :class="slaClass(pc('work_orders.respond_rate'))">{{ pc('work_orders.respond_rate') }}<small>%</small></div><div class="stat-sub">违约 {{ pc('work_orders.respond_breach') }} 单</div></div>
            <div class="dash-card"><div class="stat-label">SLA 解决达标率</div><div class="stat-num" :class="slaClass(pc('work_orders.resolve_rate'))">{{ pc('work_orders.resolve_rate') }}<small>%</small></div><div class="stat-sub">违约 {{ pc('work_orders.resolve_breach') }} 单</div></div>
          </div>
          <div class="grid-2" style="margin-top:10px">
            <div class="card" v-if="pc('work_orders.by_type')?.length">
              <div class="card-title">类型分布</div>
              <div class="table-wrap"><table class="data-table">
                <tbody><tr v-for="t in pc('work_orders.by_type')" :key="t.t"><td>{{ WO_TYPES[t.t]||t.t }}</td><td>{{ t.cnt }}</td></tr></tbody>
              </table></div>
            </div>
            <div class="card" v-if="pc('work_orders.top_assignees')?.length">
              <div class="card-title">处理人 TOP5（完成量）</div>
              <div class="table-wrap"><table class="data-table">
                <thead><tr><th>处理人</th><th>完成</th><th>均时(h)</th><th>违约</th></tr></thead>
                <tbody><tr v-for="a in pc('work_orders.top_assignees')" :key="a.uname"><td>{{ a.uname }}</td><td>{{ a.cnt }}</td><td>{{ +(a.avg_hours||0).toFixed(1) }}</td><td :class="a.breach?'warn-text':''">{{ a.breach }}</td></tr></tbody>
              </table></div>
            </div>
          </div>

          <div class="sec-title">巡检与变更</div>
          <div class="stat-grid">
            <div class="dash-card"><div class="stat-label">巡检执行</div><div class="stat-num">{{ pc('inspections.executions') }}</div><div class="stat-sub">扫码 {{ pc('inspections.scan_count') }} · 签发 {{ pc('inspections.signed_reports') }}</div></div>
            <div class="dash-card"><div class="stat-label">巡检异常</div><div class="stat-num" :class="pc('inspections.abnormal')?'warn':'ok'">{{ pc('inspections.abnormal') }}</div><div class="stat-sub">转工单 {{ pc('inspections.to_work_orders') }}</div></div>
            <div class="dash-card"><div class="stat-label">漏检(逾期)</div><div class="stat-num" :class="pc('inspections.overdue')?'warn':'ok'">{{ pc('inspections.overdue') }}</div><div class="stat-sub">执行率 {{ pc('inspections.exec_rate') }}%</div></div>
            <div class="dash-card"><div class="stat-label">变更申请</div><div class="stat-num">{{ pc('changes.created') }}</div><div class="stat-sub">完成 {{ pc('changes.finished') }} · 回滚 {{ pc('changes.rollback') }} · 高风险 {{ pc('changes.high_risk') }}</div></div>
          </div>

          <div class="sec-title">能耗与 PUE 合规</div>
          <div class="stat-grid">
            <div class="dash-card"><div class="stat-label">月能耗</div><div class="stat-num">{{ fmt(pc('energy.energy_kwh')) }}<small>kWh</small></div><div class="stat-sub">电费 ¥{{ fmt(pc('energy.cost_yuan')) }} · 碳排 {{ fmt(pc('energy.co2_kg')) }}kg</div></div>
            <div class="dash-card"><div class="stat-label">平均 PUE</div><div class="stat-num" :class="pueClass(pc('energy.avg_pue'))">{{ pc('energy.avg_pue') || '-' }}</div><div class="stat-sub">峰值 {{ pc('energy.max_pue') || '-' }} · 谷值 {{ pc('energy.min_pue') || '-' }}</div></div>
            <div class="dash-card"><div class="stat-label">平均功率</div><div class="stat-num">{{ pc('energy.avg_power_kw') }}<small>kW</small></div><div class="stat-sub">电价 ¥{{ pc('energy.price_kwh') }}/kWh</div></div>
            <div class="dash-card"><div class="stat-label">合规站点</div><div class="stat-num" :class="compOver?'warn':'ok'">{{ compOk }}/{{ pc('energy.compliance')?.length || 0 }}</div><div class="stat-sub">超红线 {{ compOver }} 个</div></div>
          </div>
          <div class="card" style="margin-top:10px" v-if="pc('energy.compliance')?.length">
            <div class="card-title">分站点 PUE 合规（月均 vs 省份政策红线）</div>
            <div class="table-wrap"><table class="data-table">
              <thead><tr><th>站点</th><th>省份</th><th>月均PUE</th><th>红线</th><th>裕度</th><th>状态</th></tr></thead>
              <tbody><tr v-for="row in pc('energy.compliance')" :key="row.site_id">
                <td>{{ row.site_name }}</td><td class="muted small">{{ row.province || '未填写' }}</td>
                <td>{{ row.avg_pue || '-' }}</td><td>{{ row.limit || '-' }}</td><td>{{ row.margin || '-' }}</td>
                <td><span class="badge" :class="compBadge(row.status)">{{ COMP_ST[row.status]||row.status }}</span></td>
              </tr></tbody>
            </table></div>
          </div>
          <div class="card" style="margin-top:10px" v-if="pc('energy.compliance_event_count')">
            <div class="card-title">PUE 合规告警事件（本期 {{ pc('energy.compliance_event_count') }} 次 · 年化敞口合计 ¥{{ fmt(pc('energy.compliance_surcharge_annual')) }}）</div>
            <div class="table-wrap"><table class="data-table">
              <thead><tr><th>时间</th><th>站点</th><th>状态</th><th>PUE</th><th>红线</th><th>年化敞口</th><th>级别</th></tr></thead>
              <tbody><tr v-for="e in pc('energy.compliance_events')" :key="e.id">
                <td class="muted small">{{ e.created_at }}</td><td>{{ e.site_name }}</td>
                <td><span class="badge" :class="compBadge(e.status)">{{ COMP_ST[e.status]||e.status }}</span></td>
                <td>{{ e.pue }}</td><td>{{ e.limit }}</td><td>¥{{ fmt(e.surcharge_annual) }}</td>
                <td>{{ e.severity === 3 ? '严重' : '警告' }}</td>
              </tr></tbody>
            </table></div>
          </div>

          <div class="sec-title">资产与健康</div>
          <div class="stat-grid">
            <div class="dash-card"><div class="stat-label">资产总数</div><div class="stat-num">{{ pc('assets.total') }}</div><div class="stat-sub">在架 {{ pc('assets.in_rack') }} · 维修 {{ pc('assets.in_maint') }}</div></div>
            <div class="dash-card"><div class="stat-label">本月新增</div><div class="stat-num">{{ pc('assets.new_in_month') }}</div><div class="stat-sub">退役处置 {{ pc('assets.retire_done') }} 台</div></div>
            <div class="dash-card"><div class="stat-label">退役残值回收</div><div class="stat-num">¥{{ fmt(pc('assets.retire_amount')) }}</div><div class="stat-sub">处置留档可查</div></div>
            <div class="dash-card"><div class="stat-label">维保覆盖率</div><div class="stat-num" :class="pc('assets.maint_coverage_pct')>=80?'ok':'warn'">{{ pc('assets.maint_coverage_pct') }}<small>%</small></div><div class="stat-sub">30天内到期 {{ pc('assets.maint_expiring_30d') }} 份</div></div>
          </div>
          <div class="mr-chips" v-if="pc('assets.health_grades')?.length" style="margin-top:10px">
            <span class="muted small" style="margin-right:6px">健康度分布：</span>
            <span v-for="g in pc('assets.health_grades')" :key="g.grade" class="badge mr-chip" :class="'hg-'+g.grade">{{ GRADES[g.grade]||g.grade }} {{ g.cnt }}</span>
          </div>

          <div class="sec-title">备件 · 盘点 · 网络 · 应急演练</div>
          <div class="stat-grid">
            <div class="dash-card"><div class="stat-label">备件低库存</div><div class="stat-num" :class="pc('spares.low_stock')?'warn':'ok'">{{ pc('spares.low_stock') }}</div><div class="stat-sub">入库 {{ pc('spares.move_in') }} · 出库 {{ pc('spares.move_out') }}</div></div>
            <div class="dash-card"><div class="stat-label">采购申请</div><div class="stat-num">{{ pc('spares.purchase_created') }}</div><div class="stat-sub">待审 {{ pc('spares.purchase_pending') }}</div></div>
            <div class="dash-card"><div class="stat-label">盘点完成</div><div class="stat-num">{{ pc('stocktake.finished') }}</div><div class="stat-sub">差异 {{ pc('stocktake.diff_total') }} 项</div></div>
            <div class="dash-card"><div class="stat-label">网络链路在线率</div><div class="stat-num" :class="pc('network.links_up_pct')>=90?'ok':'warn'">{{ pc('network.links_up_pct') }}<small>%</small></div><div class="stat-sub">设备 {{ pc('network.devices') }} 台</div></div>
            <div class="dash-card"><div class="stat-label">应急演练</div><div class="stat-num">{{ pc('emergency.drills') }}</div><div class="stat-sub">整改工单 {{ pc('emergency.drill_to_work_orders') }}</div></div>
            <div class="dash-card"><div class="stat-label">预案逾期待演练</div><div class="stat-num" :class="pc('emergency.overdue_plans_now')?'warn':'ok'">{{ pc('emergency.overdue_plans_now') }}</div><div class="stat-sub">启用预案 {{ pc('emergency.plans_active') }}</div></div>
            <div class="dash-card"><div class="stat-label">测点在线率(当前)</div><div class="stat-num" :class="pc('availability.online_pct')>=90?'ok':'warn'">{{ pc('availability.online_pct') }}<small>%</small></div><div class="stat-sub">总 {{ pc('availability.points_total') }} · 告警 {{ pc('availability.alarm') }} · 离线 {{ pc('availability.offline') }}</div></div>
            <div class="dash-card"><div class="stat-label">演练结果</div><div class="stat-num">{{ drillOk }}<small>/{{ pc('emergency.drills') }}</small></div><div class="stat-sub">成功次数占比</div></div>
          </div>

          <div class="sec-title">每日告警 / 工单趋势</div>
          <div class="card"><div class="chart-box" ref="mrTrendEl"></div></div>
        </div>
        <div class="drawer-foot">
          <button class="btn" @click="exportMrCSV"><SvgIcon name="download" />导出 CSV</button>
          <button class="btn" @click="printMr"><SvgIcon name="printer" />打印月报</button>
          <button class="btn btn-primary" @click="closeReport">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>
<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../api'
import * as echarts from 'echarts'
import { useSettingsStore } from '../stores/settings'
import { useAuthStore } from '../stores/auth'
import { toast, toastAuto, showConfirm } from '../ui-dialog'
const settings = useSettingsStore()
const auth = useAuthStore()
const route = useRoute(), router = useRouter()
const sites=ref([]),siteId=ref(0),days=ref(30),r=ref({}),sla=ref({})
const tab=ref('realtime')
const urgentBreach=computed(()=>{const p=(sla.value.by_priority||[]).find(x=>x.priority===4);return p?(p.respond_breach||0)+(p.resolve_breach||0):0})
const trendEl=ref(null),pueEl=ref(null),woEl=ref(null),sevEl=ref(null)
let charts=[]
const fmt=v=>Number(v||0).toLocaleString('zh-CN',{maximumFractionDigits:1})
const fmtTime=t=>t?String(t).replace('T',' ').slice(0,16):'-'
const WO_TYPES={task:'任务',fault:'故障',maintenance:'维保',inspection:'巡检',change:'变更',other:'其他'}
const SEV={1:'提示',2:'警告',3:'严重'}
const catTotal=()=>(r.value.asset_cats||[]).reduce((a,c)=>a+c.count,0)||1
const catPct=c=>Math.round(c.count/catTotal()*100)

function chartTheme(){const cs=getComputedStyle(document.documentElement);return{axis:cs.getPropertyValue('--muted').trim()||'#8a93a6',line:cs.getPropertyValue('--border2').trim()||'#3a4256',split:cs.getPropertyValue('--border').trim()||'rgba(138,147,166,.12)'}}
function mk(el){const c=echarts.init(el);charts.push(c);return c}

function renderAll(){
  const ct=chartTheme(),palette=settings.chartPalette
  const base={backgroundColor:'transparent',textStyle:{color:ct.axis}}
  const axis={axisLine:{lineStyle:{color:ct.line}},axisLabel:{color:ct.axis}}
  const split={splitLine:{lineStyle:{color:ct.split}}}
  // 每日趋势
  const trend=r.value.daily_trend||[]
  if(trendEl.value){
    const c=charts.find(x=>x.getDom()===trendEl.value)||mk(trendEl.value)
    c.setOption({...base,tooltip:{trigger:'axis',confine:true},legend:{bottom:0,textStyle:{color:ct.axis}},
      grid:{left:40,right:16,top:20,bottom:40},
      xAxis:{type:'category',data:trend.map(d=>d.day.slice(5)),...axis},
      yAxis:{type:'value',minInterval:1,...axis,...split},
      series:[
        {name:'告警',type:'line',smooth:true,data:trend.map(d=>d.alerts),itemStyle:{color:palette[2]||'#ffb454'},areaStyle:{opacity:.12}},
        {name:'工单',type:'line',smooth:true,data:trend.map(d=>d.orders),itemStyle:{color:palette[0]},areaStyle:{opacity:.12}}]},true)
  }
  // PUE
  const pue=r.value.pue_trend||[]
  if(pueEl.value){
    const c=charts.find(x=>x.getDom()===pueEl.value)||mk(pueEl.value)
    const labels=pue.map((p,i)=>i%24===0?String(p.recorded_at||'').slice(5,10):'')
    c.setOption({...base,tooltip:{trigger:'axis',confine:true},
      grid:{left:46,right:16,top:20,bottom:30},
      xAxis:{type:'category',data:labels,...axis,axisLabel:{color:ct.axis,interval:0}},
      yAxis:{type:'value',scale:true,...axis,...split},
      series:[{name:'PUE',type:'line',smooth:true,showSymbol:false,data:pue.map(p=>+(p.pue||0).toFixed(3)),
        itemStyle:{color:palette[0]},lineStyle:{width:2},
        areaStyle:{color:new echarts.graphic.LinearGradient(0,0,0,1,[{offset:0,color:palette[0]+'55'},{offset:1,color:palette[0]+'08'}])},
        markLine:{silent:true,symbol:'none',lineStyle:{type:'dashed',color:'#ffb454'},label:{color:ct.axis,formatter:'目标 1.40'},data:[{yAxis:1.4}]}}]},true)
  }
  // 工单类型
  const wo=r.value.wo_types||[]
  if(woEl.value){
    const c=charts.find(x=>x.getDom()===woEl.value)||mk(woEl.value)
    c.setOption({...base,tooltip:{trigger:'item',formatter:'{b}: {c} 单 ({d}%)'},legend:{bottom:0,textStyle:{color:ct.axis}},
      series:[{type:'pie',radius:['40%','64%'],center:['50%','44%'],label:{color:ct.axis,formatter:'{b}\n{c}'},
        data:wo.length?wo.map((t,i)=>({name:WO_TYPES[t.type]||t.type,value:t.count,itemStyle:{color:palette[i%palette.length]}})):[{name:'暂无数据',value:1,itemStyle:{color:ct.line}}]}]},true)
  }
  // 严重度
  const sev=r.value.alert_severity||[]
  if(sevEl.value){
    const c=charts.find(x=>x.getDom()===sevEl.value)||mk(sevEl.value)
    const sevColor={3:'#ff5252',2:'#ffb454',1:palette[0]}
    c.setOption({...base,tooltip:{trigger:'item',formatter:'{b}: {c} 条 ({d}%)'},legend:{bottom:0,textStyle:{color:ct.axis}},
      series:[{type:'pie',radius:['40%','64%'],center:['50%','44%'],label:{color:ct.axis,formatter:'{b}\n{c}'},
        data:sev.length?sev.map(s=>({name:SEV[s.type]||('级别'+s.type),value:s.count,itemStyle:{color:sevColor[s.type]||ct.axis}})):[{name:'暂无数据',value:1,itemStyle:{color:ct.line}}]}]},true)
  }
}

async function load(){
  const [rep,sl]=await Promise.all([
    api.get('/report',{site_id:siteId.value,days:days.value}),
    api.get('/work-orders/sla-stats',{site_id:siteId.value,days:days.value})])
  r.value=rep.data||{}
  sla.value=sl.data||{}
  await nextTick();renderAll()
}

function exportCSV(){
  const v=r.value,rows=[]
  rows.push(['DCIM 运营报表',`站点:${sites.value.find(s=>s.id===siteId.value)?.name||''}`,`周期:近${days.value}天`,`生成:${new Date().toLocaleString('zh-CN')}`])
  rows.push([])
  rows.push(['当前状态',''])
  rows.push(['资产总数',v.total_assets],['在架',v.in_rack],['维修中',v.in_maint])
  rows.push(['机柜已用/总数',`${v.used_cabinets}/${v.total_cabinets}`],['未结工单',v.open_orders],['已结工单',v.closed_orders])
  rows.push(['变更总数',v.total_changes],['告警总数',v.total_alerts],['已恢复告警',v.resolved_alerts])
  rows.push([])
  rows.push([`期间指标(近${days.value}天)`,''])
  rows.push(['新增工单',v.period_orders],['新增告警',v.period_alerts],['变更申请',v.period_changes])
  rows.push(['巡检次数',v.period_inspections],['巡检异常',v.period_abnormal],['MTTR(小时)',v.mttr_hours])
  rows.push(['期间能耗(kWh)',v.period_energy_kwh],['期间电费(元)',v.period_cost_yuan],['期间碳排(kg)',v.period_co2_kg],['期间均PUE',v.period_avg_pue])
  rows.push([])
  rows.push(['工单类型分布',''])
  ;(v.wo_types||[]).forEach(t=>rows.push([WO_TYPES[t.type]||t.type,t.count]))
  rows.push([])
  rows.push(['告警严重度分布',''])
  ;(v.alert_severity||[]).forEach(s=>rows.push([SEV[s.type]||('级别'+s.type),s.count]))
  rows.push([])
  rows.push(['资产类型分布',''])
  ;(v.asset_cats||[]).forEach(c=>rows.push([c.category,c.count]))
  rows.push([])
  rows.push([`工单SLA时效(近${days.value}天)`,''])
  const sv=sla.value
  rows.push(['响应达标率(%)',sv.respond_rate],['解决达标率(%)',sv.resolve_rate],['响应违约',sv.respond_breach],['解决违约',sv.resolve_breach],['超期未闭环',sv.open_overdue])
  rows.push(['优先级','工单数','响应违约','解决违约','响应达标率(%)'])
  ;(sv.by_priority||[]).forEach(p=>rows.push([p.label,p.total,p.respond_breach,p.resolve_breach,p.respond_rate]))
  rows.push([])
  rows.push(['每日趋势',''])
  rows.push(['日期','告警','工单'])
  ;(v.daily_trend||[]).forEach(d=>rows.push([d.day,d.alerts,d.orders]))
  downloadCSV(rows,`dcim_report_site${siteId.value}_${days.value}d.csv`)
}
function downloadCSV(rows,name){
  const csv='\uFEFF'+rows.map(rr=>rr.map(x=>`"${String(x??'').replace(/"/g,'""')}"`).join(',')).join('\n')
  const url=URL.createObjectURL(new Blob([csv],{type:'text/csv;charset=utf-8'}))
  const a=document.createElement('a');a.href=url;a.download=name;a.click();URL.revokeObjectURL(url)
}
function printReport(){window.print()}

// ==================== V61 运维月报 ====================
const PRI={1:'低',2:'中',3:'高',4:'紧急'}
const CHG_ST={1:'草稿',2:'待审批',3:'已批准',4:'已驳回',5:'执行中',6:'已完成',7:'已回滚'}
const GRADES={healthy:'健康',attention:'需关注',sub_healthy:'亚健康',risk:'风险'}
const COMP_ST={ok:'达标',edge:'临界',over:'超红线',nodata:'无数据',unknown:'无政策'}
const prevMonth=()=>{const d=new Date();d.setDate(1);d.setMonth(d.getMonth()-1);return d.getFullYear()+'-'+String(d.getMonth()+1).padStart(2,'0')}
const mr=reactive({month:prevMonth(),genSite:0,force:false,busy:false,items:[],loaded:false,show:false,detail:null})
// V65 B1-4 报表双出口归一：实时页签可见上月月报状态并直达；月报详情可打印
const lastMr=computed(()=>mr.items.find(x=>x.month===prevMonth()&&x.site_id===0))
const lastMrLabel=computed(()=>!lastMr.value?'生成上月月报':lastMr.value.status===2?'上月月报·已发布':'上月月报·草稿')
function gotoMonthly(){if(lastMr.value)openReport(lastMr.value.id);else switchTab('monthly')}
function printMr(){
  document.body.classList.add('mr-printing')
  const done=()=>document.body.classList.remove('mr-printing')
  window.addEventListener('afterprint',done,{once:true})
  setTimeout(()=>{window.print();setTimeout(done,800)},60)
}
const mrTrendEl=ref(null)
let mrChart=null
const pc=path=>String(path).split('.').reduce((a,k)=>(a==null?undefined:a[k]),mr.detail?.content||{})
const slaClass=v=>v>=95?'ok':v>=80?'warn':''
const pueClass=v=>!v?'':v>1.6?'warn':v<=1.4?'ok':''
const compOk=computed(()=>(pc('energy.compliance')||[]).filter(x=>x.status==='ok'||x.status==='edge').length)
const compOver=computed(()=>(pc('energy.compliance')||[]).filter(x=>x.status==='over').length)
const compBadge=s=>s==='ok'?'mr-pub':s==='over'?'mr-bad':s==='edge'?'mr-edge':''
const drillOk=computed(()=>((pc('emergency.drill_results')||[]).find(x=>x.result==='success')||{cnt:0}).cnt)

function switchTab(t){
  if(tab.value===t)return
  tab.value=t
  const q={...route.query}
  if(t==='monthly')q.tab='monthly';else delete q.tab
  router.replace({query:q}).catch(()=>{})
  if(t==='monthly'&&!mr.loaded)loadReports()
}
async function loadReports(){
  const res=await api.get('/ops-reports',{size:200})
  if(res.code===0){mr.items=res.data?.items||[];mr.loaded=true}else toastAuto(res.msg)
}
async function genReport(){
  if(!mr.month){toast('请选择月份','error');return}
  mr.busy=true
  try{
    const res=await api.post('/ops-reports/generate',{month:mr.month,site_id:mr.genSite,force:mr.force})
    if(res.code===0){
      toast(res.data?.msg||'月报已生成')
      await loadReports()
      if(res.data?.id&&!res.data?.existed)openReport(res.data.id)
      else if(res.data?.id)openReport(res.data.id)
    }else toastAuto(res.msg)
  }finally{mr.busy=false}
}
async function openReport(id,csvOnly){
  if(!csvOnly&&mr.show&&mr.detail?.id===id)return
  const res=await api.get(`/ops-reports/${id}`)
  if(res.code!==0){toastAuto(res.msg);return}
  mr.detail=res.data
  if(csvOnly){exportMrCSV();return}
  if(tab.value!=='monthly')tab.value='monthly'
  mr.show=true
  router.replace({query:{tab:'monthly',id:String(id),detail:'1'}}).catch(()=>{})
  await nextTick();renderMrTrend()
}
function closeReport(){
  mr.show=false
  if(mrChart){mrChart.dispose();mrChart=null}
  const q={...route.query};delete q.id;delete q.detail
  router.replace({query:q}).catch(()=>{})
}
async function publishReport(it){
  if(!(await showConfirm({title:'发布月报',message:`确认发布「${it.title}」？发布后状态归档为已发布，并通知订阅了 ops_report 事件的 Webhook 渠道。如需修正数据可删除或强制重算。`})))return
  const res=await api.put(`/ops-reports/${it.id}/publish`,{})
  if(res.code===0){toast('已发布');loadReports()}else toastAuto(res.msg)
}
async function delReport(it){
  if(!(await showConfirm({title:'删除月报',message:`确认删除「${it.code}」？删除后可重新生成。`,tone:'danger'})))return
  const res=await api.del(`/ops-reports/${it.id}`)
  if(res.code===0){toast('已删除');if(mr.detail?.id===it.id)mr.show=false;loadReports()}else toastAuto(res.msg)
}
function renderMrTrend(){
  if(!mrTrendEl.value)return
  if(mrChart){mrChart.dispose();mrChart=null}
  const trend=pc('daily_trend')||[]
  if(!trend.length)return
  mrChart=echarts.init(mrTrendEl.value)
  const ct=chartTheme(),palette=settings.chartPalette
  mrChart.setOption({backgroundColor:'transparent',textStyle:{color:ct.axis},
    tooltip:{trigger:'axis',confine:true},legend:{bottom:0,textStyle:{color:ct.axis}},
    grid:{left:40,right:16,top:20,bottom:40},
    xAxis:{type:'category',data:trend.map(d=>String(d.day).slice(5)),axisLine:{lineStyle:{color:ct.line}},axisLabel:{color:ct.axis}},
    yAxis:{type:'value',minInterval:1,axisLine:{lineStyle:{color:ct.line}},axisLabel:{color:ct.axis},splitLine:{lineStyle:{color:ct.split}}},
    series:[
      {name:'告警',type:'line',smooth:true,data:trend.map(d=>d.alerts),itemStyle:{color:palette[2]||'#ffb454'},areaStyle:{opacity:.12}},
      {name:'工单',type:'line',smooth:true,data:trend.map(d=>d.orders),itemStyle:{color:palette[0]},areaStyle:{opacity:.12}}]},true)
}
function exportMrCSV(){
  const d=mr.detail;if(!d)return
  const rows=[];const push=(...a)=>rows.push(a)
  push('DCIM 运维月报',d.title)
  push('编号',d.code,'范围',d.site_name,'状态',d.status===2?'已发布':'草稿')
  push('统计期间',`${pc('period.start')} ~ ${pc('period.end')}`,'生成时间',pc('period.generated_at'),'生成方式',d.auto_gen?'系统自动':(d.generated_by_name||'手动'))
  push()
  push('【告警与事件】')
  push('新增告警',pc('alerts.created'),'严重',pc('alerts.sev3'),'警告',pc('alerts.sev2'),'提示',pc('alerts.sev1'))
  push('已恢复',pc('alerts.resolved'),'恢复均时(h)',pc('alerts.mttr_hours'),'当前活跃',pc('alerts.active_now'),'升级',pc('alerts.escalated'),'风暴',pc('alerts.storms'),'风暴抑制',pc('alerts.storm_suppressed'))
  push('告警TOP测点','编码','次数')
  ;(pc('alerts.top_points')||[]).forEach(t=>push(t.pname,t.pcode||'-',t.cnt))
  push()
  push('【工单与SLA】')
  push('新建',pc('work_orders.created'),'完成',pc('work_orders.finished'),'当前未结',pc('work_orders.open_now'),'故障类',pc('work_orders.fault_orders'))
  push('响应达标率(%)',pc('work_orders.respond_rate'),'响应违约',pc('work_orders.respond_breach'),'解决达标率(%)',pc('work_orders.resolve_rate'),'解决违约',pc('work_orders.resolve_breach'),'MTTR(h)',pc('work_orders.mttr_hours'))
  push('类型','数量');(pc('work_orders.by_type')||[]).forEach(t=>push(WO_TYPES[t.t]||t.t,t.cnt))
  push('优先级','数量');(pc('work_orders.by_priority')||[]).forEach(x=>push(PRI[x.pri]||x.pri,x.cnt))
  push('处理人','完成','均时(h)','违约');(pc('work_orders.top_assignees')||[]).forEach(a=>push(a.uname,a.cnt,+(a.avg_hours||0).toFixed(1),a.breach))
  push()
  push('【巡检】')
  push('启用计划',pc('inspections.plans_active'),'执行',pc('inspections.executions'),'异常',pc('inspections.abnormal'),'漏检(逾期)',pc('inspections.overdue'),'转工单',pc('inspections.to_work_orders'),'扫码巡检',pc('inspections.scan_count'),'签发报告',pc('inspections.signed_reports'),'执行率(%)',pc('inspections.exec_rate'))
  push()
  push('【变更】')
  push('申请',pc('changes.created'),'完成',pc('changes.finished'),'回滚',pc('changes.rollback'),'高风险',pc('changes.high_risk'))
  push('状态','数量');(pc('changes.by_status')||[]).forEach(x=>push(CHG_ST[x.st]||x.st,x.cnt))
  push()
  push('【能耗与PUE合规】')
  push('月能耗(kWh)',pc('energy.energy_kwh'),'电费(元)',pc('energy.cost_yuan'),'碳排(kg)',pc('energy.co2_kg'),'平均PUE',pc('energy.avg_pue'),'峰值',pc('energy.max_pue'),'谷值',pc('energy.min_pue'),'平均功率(kW)',pc('energy.avg_power_kw'))
  push('站点','省份','月均PUE','红线','裕度','状态')
  ;(pc('energy.compliance')||[]).forEach(x=>push(x.site_name,x.province||'未填写',x.avg_pue||'-',x.limit||'-',x.margin||'-',COMP_ST[x.status]||x.status))
  if (pc('energy.compliance_event_count')) {
    push('【PUE合规告警事件】本期',pc('energy.compliance_event_count'),'其中超标',pc('energy.compliance_over_count'),'临界',pc('energy.compliance_edge_count'),'年化敞口合计(元)',pc('energy.compliance_surcharge_annual'))
    push('时间','站点','状态','PUE','红线','年化敞口(元)','级别')
    ;(pc('energy.compliance_events')||[]).forEach(e=>push(e.created_at,e.site_name,COMP_ST[e.status]||e.status,e.pue,e.limit,e.surcharge_annual,e.severity===3?'严重':'警告'))
  }
  push()
  push('【资产】')
  push('总数',pc('assets.total'),'在架',pc('assets.in_rack'),'维修',pc('assets.in_maint'),'报废(累计)',pc('assets.scrapped'),'本月新增',pc('assets.new_in_month'))
  push('退役申请',pc('assets.retire_applied'),'处置完成',pc('assets.retire_done'),'残值回收(元)',pc('assets.retire_amount'),'维保覆盖率(%)',pc('assets.maint_coverage_pct'),'在保',pc('assets.maint_covered'),'30天到期',pc('assets.maint_expiring_30d'))
  push('健康等级','数量');(pc('assets.health_grades')||[]).forEach(g=>push(GRADES[g.grade]||g.grade,g.cnt))
  push()
  push('【盘点/网络/备件/应急演练】')
  push('盘点任务',pc('stocktake.created'),'完成',pc('stocktake.finished'),'差异',pc('stocktake.diff_total'))
  push('网络设备',pc('network.devices'),'链路',pc('network.links_total'),'在线',pc('network.links_up'),'在线率(%)',pc('network.links_up_pct'))
  push('备件种类',pc('spares.kinds'),'低库存',pc('spares.low_stock'),'入库',pc('spares.move_in'),'出库',pc('spares.move_out'),'采购申请',pc('spares.purchase_created'),'待审',pc('spares.purchase_pending'))
  push('启用预案',pc('emergency.plans_active'),'演练',pc('emergency.drills'),'整改工单',pc('emergency.drill_to_work_orders'),'逾期待演练',pc('emergency.overdue_plans_now'))
  ;(pc('emergency.drill_results')||[]).forEach(x=>push('演练结果:'+(x.result||'-'),x.cnt))
  push()
  push('【监控可用性(当前快照)】')
  push('测点总数',pc('availability.points_total'),'正常',pc('availability.normal'),'告警',pc('availability.alarm'),'离线',pc('availability.offline'),'在线率(%)',pc('availability.online_pct'))
  push()
  push('【每日趋势】','日期','告警','工单')
  ;(pc('daily_trend')||[]).forEach(x=>push('',x.day,x.alerts,x.orders))
  downloadCSV(rows,`dcim_monthly_${d.month}_S${d.site_id}.csv`)
}
watch(()=>route.query,q=>{
  if(q.tab==='monthly'&&tab.value!=='monthly'){tab.value='monthly';if(!mr.loaded)loadReports()}
  if(q.detail==='1'&&q.id){openReport(Number(q.id))}
})

function onThemeChange(){charts.forEach(c=>c.dispose());charts=[];nextTick(()=>{renderAll();if(mr.show)renderMrTrend()})}
function onResize(){charts.forEach(c=>c.resize());if(mrChart)mrChart.resize()}
onMounted(async()=>{
  const r1=await api.get('/sites',{size:100});sites.value=r1.data?.items||[];siteId.value=sites.value[0]?.id||0
  loadReports() // V65: 预载月报列表供实时页签「上月月报」徽章（不 await，不阻塞实时报表渲染）
  await load()
  if(route.query.tab==='monthly'){tab.value='monthly';await loadReports()}
  if(route.query.id&&route.query.detail==='1'){openReport(Number(route.query.id))}
  document.addEventListener('dcim-theme-change',onThemeChange)
  window.addEventListener('resize',onResize)})
onBeforeUnmount(()=>{document.removeEventListener('dcim-theme-change',onThemeChange);window.removeEventListener('resize',onResize);charts.forEach(c=>c.dispose());if(mrChart)mrChart.dispose();document.body.classList.remove('mr-printing')})
</script>

<style scoped>
.sec-title{font-size:13px;font-weight:600;color:var(--text2,var(--muted));margin:18px 0 8px}
.stat-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}
.dash-card{background:var(--bg-card,var(--card));border:1px solid var(--border);border-radius:8px;padding:14px;text-align:center}
.stat-num{font-size:26px;font-weight:700;color:#409eff}.stat-num.ok{color:#67c23a}.stat-num.warn{color:#e6a23c}
.stat-num small{font-size:13px;color:var(--muted);font-weight:400}
.stat-label{font-size:12px;color:var(--muted);margin-bottom:4px}
.grid-2{display:grid;grid-template-columns:1fr 1fr;gap:16px}
.chart-box{height:260px}
.card-title{display:flex;align-items:center;font-size:14px;font-weight:600;margin-bottom:8px}
.card-title .link{margin-left:auto;font-size:12px;font-weight:400;color:var(--accent);cursor:pointer}
.link{color:var(--accent);cursor:pointer}
.cat-bar{display:inline-block;width:120px;height:8px;border-radius:4px;background:var(--border);overflow:hidden;vertical-align:middle;margin-right:8px}
.cat-bar i{display:block;height:100%;background:#409eff;border-radius:4px}
.small{font-size:12px}
.stat-sub{font-size:11px;color:var(--muted);margin-top:3px}
.warn-text{color:#f56c6c;font-weight:600}
.seg{display:flex;border:1px solid var(--border);border-radius:6px;overflow:hidden;width:fit-content}
.seg-btn{padding:6px 12px;font-size:12px;background:transparent;border:0;color:var(--muted);cursor:pointer;display:inline-flex;align-items:center;gap:4px}
.seg-btn.on{background:var(--accent,#409eff);color:#fff}
/* V61 月报 */
.chk-inline{display:inline-flex;align-items:center;gap:4px;font-size:12px;color:var(--muted);cursor:pointer}
.mr-summary{max-width:340px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.mr-ops{display:flex;gap:6px;flex-wrap:wrap}
.mr-head{border-bottom:1px solid var(--border);padding-bottom:10px;margin-bottom:4px}
.badge.mr-pub{border-color:#67c23a;color:#67c23a}
.badge.mr-bad{border-color:#f56c6c;color:#f56c6c}
.badge.mr-edge{border-color:#e6a23c;color:#e6a23c}
.mr-chips{display:flex;align-items:center;flex-wrap:wrap;gap:6px}
.mr-chip{font-size:12px}
.hg-healthy{border-color:#67c23a;color:#67c23a}
.hg-attention{border-color:#409eff;color:#409eff}
.hg-sub_healthy{border-color:#e6a23c;color:#e6a23c}
.hg-risk{border-color:#f56c6c;color:#f56c6c}
@media (max-width:900px){.stat-grid{grid-template-columns:repeat(2,1fr)}.grid-2{grid-template-columns:1fr}}
@media print{.toolbar,.seg{display:none!important}}
</style>
<style>
/* 打印报表：隐藏侧栏/顶栏，只留内容区 */
@media print{
  .sidebar,.topbar,.nav-backdrop,.page-head .toolbar{display:none!important}
  .main,.main-content,.content,.layout-main{margin:0!important;padding:8px!important;width:100%!important}
  body *{visibility:visible}
}
/* V65 打印月报：只印详情抽屉（visibility 技巧，抽屉不 Teleport 无法用祖先 display:none） */
@media print{
  body.mr-printing *{visibility:hidden!important}
  body.mr-printing .drawer-mask,body.mr-printing .drawer-mask *{visibility:visible!important}
  body.mr-printing .drawer-mask{position:absolute!important;inset:0!important;background:#fff!important}
  body.mr-printing .drawer{width:100%!important;max-width:none!important;height:auto!important;box-shadow:none!important;overflow:visible!important}
  body.mr-printing .drawer-body{overflow:visible!important;height:auto!important;flex:none!important}
  body.mr-printing .drawer-foot,body.mr-printing .drawer-head button{display:none!important}
}
</style>
