<template>
  <div>
    <div class="page-head"><h1>告警规则</h1></div>
    <div class="tabs">
      <button class="tab" :class="{on:mainTab==='list'}" @click="mainTab='list'"><SvgIcon name="bolt" />规则列表</button>
      <button class="tab" :class="{on:mainTab==='quality'}" @click="switchQuality"><SvgIcon name="gauge" />质量治理</button>
    </div>

    <!-- V64 质量治理页签 -->
    <div v-if="mainTab==='quality'" class="q-panel">
      <div class="toolbar">
        <select class="input" style="width:130px" v-model.number="q.days" @change="loadQuality"><option :value="7">近 7 天</option><option :value="30">近 30 天</option><option :value="90">近 90 天</option></select>
        <button class="btn" @click="loadQuality"><SvgIcon name="refresh" />刷新</button>
        <span v-if="q.data" class="q-grade" :class="'g-'+q.data.kpi.grade" :title="'噪声率 '+q.data.kpi.noise_rate+'%（(疑似误报+被抑制)/总数），<20% 正常，20~40% 偏高，≥40% 严重'">{{ {ok:'告警质量正常',warn:'噪声偏高',bad:'噪声严重'}[q.data.kpi.grade] }}</span>
        <span style="flex:1"></span>
        <button class="btn" @click="runScan" title="手动触发一次高噪声规则评估：近7天触发≥5次且疑似误报率≥60%的规则将通知管理员（7天内不重复通知）"><SvgIcon name="gauge" />立即评估</button>
        <button class="btn" @click="exportQCsv"><SvgIcon name="download" />导出质量 CSV</button>
      </div>
      <div v-if="!q.data" class="card muted" style="padding:24px;text-align:center">加载中…</div>
      <template v-else>
      <div class="q-kpis">
        <div class="q-kpi"><span class="sk-label">告警总数</span><span class="sk-val">{{ q.data.kpi.total }}</span><span class="sk-sub">活跃 {{ q.data.kpi.open }} · 已结束 {{ q.data.kpi.resolved }}</span></div>
        <div class="q-kpi" :class="noiseCls(q.data.kpi.false_rate)"><span class="sk-label">疑似误报率</span><span class="sk-val">{{ q.data.kpi.false_rate }}%</span><span class="sk-sub">疑似误报 {{ q.data.kpi.suspected_false }} 条</span></div>
        <div class="q-kpi" :class="noiseCls(q.data.kpi.suppress_rate)"><span class="sk-label">风暴抑制率</span><span class="sk-val">{{ q.data.kpi.suppress_rate }}%</span><span class="sk-sub">抑制 {{ q.data.kpi.suppressed }} · 风暴 {{ q.data.kpi.storms }} 场</span></div>
        <div class="q-kpi" :class="ackCls(q.data.kpi.ack_rate)"><span class="sk-label">确认率</span><span class="sk-val">{{ q.data.kpi.ack_rate }}%</span><span class="sk-sub">平均确认 {{ q.data.kpi.mtta_min }} min</span></div>
        <div class="q-kpi"><span class="sk-label">工单转化率</span><span class="sk-val">{{ q.data.kpi.wo_rate }}%</span><span class="sk-sub">转工单 {{ q.data.kpi.wo_converted }} · 升级 {{ q.data.kpi.escalated }}</span></div>
        <div class="q-kpi" :class="noiseCls(q.data.kpi.repeat_rate)"><span class="sk-label">重复告警率</span><span class="sk-val">{{ q.data.kpi.repeat_rate }}%</span><span class="sk-sub">≥3次测点 {{ q.data.kpi.repeat_points }} 个</span></div>
      </div>
      <div class="q-cols">
        <div class="card">
          <h4 class="sec-title"><SvgIcon name="storm" />规则噪声榜（TOP {{ q.data.by_rule.length }}）<span class="muted" style="font-weight:400;font-size:12px;margin-left:8px">噪声分 = 疑似误报×2 + 被抑制 · 平均解决 {{ q.data.kpi.mttr_min }} min</span></h4>
          <div class="table-wrap"><table class="data-table">
            <thead><tr><th>规则</th><th>触发</th><th>疑似误报</th><th>被抑制</th><th>确认率</th><th style="min-width:130px">噪声分</th><th>调优建议</th><th></th></tr></thead>
            <tbody>
              <tr v-for="r in q.data.by_rule" :key="r.rule_id" :class="{'row-noisy':r.noise_score>0}">
                <td>{{ r.rule_name }}<span v-if="!r.enabled" class="badge st-0" style="margin-left:6px">已停用</span></td>
                <td>{{ r.total }}</td>
                <td :class="r.suspected_false>0?'td-breach':''">{{ r.suspected_false }}<span class="muted" style="font-size:11px"> ({{ r.false_rate }}%)</span></td>
                <td>{{ r.suppressed }}</td>
                <td>{{ r.ack_rate }}%</td>
                <td><div class="rate-bar"><div class="rate-fill" :class="r.noise_score>=10?'bad':r.noise_score>=4?'mid':'good'" :style="{width:Math.min(100,r.noise_score*5)+'%'}"></div><span class="rate-txt">{{ r.noise_score }}</span></div></td>
                <td class="q-sug" :class="{'q-sug-hot':r.suggestion}">{{ r.suggestion || '—' }}</td>
                <td class="ops"><button v-if="r.rule_id" class="btn btn-mini" @click="editFromQuality(r)">调优</button><button v-if="r.rule_id" class="btn btn-mini btn-mini-accent" @click="openTune(r)" title="带预览的处置：调阈值 / 停用 / 转规则优化工单">处置</button></td>
              </tr>
              <tr v-if="!q.data.by_rule.length"><td colspan="8" class="empty">期内暂无告警</td></tr>
            </tbody>
          </table></div>
        </div>
        <div class="q-side">
          <div class="card">
            <h4 class="sec-title"><SvgIcon name="layers" />按严重度</h4>
            <div class="table-wrap"><table class="data-table">
              <thead><tr><th>级别</th><th>数量</th><th>疑似误报</th><th>已结束</th><th>转工单</th></tr></thead>
              <tbody>
                <tr v-for="s in q.data.by_severity" :key="s.severity">
                  <td><span class="badge" :class="'sev-'+s.severity">{{ s.label }}</span></td>
                  <td>{{ s.total }}</td><td :class="s.suspected_false>0?'td-breach':''">{{ s.suspected_false }}</td><td>{{ s.resolved }}</td><td>{{ s.wo_converted }}</td>
                </tr>
                <tr v-if="!q.data.by_severity.length"><td colspan="5" class="empty">暂无数据</td></tr>
              </tbody>
            </table></div>
          </div>
          <div class="card">
            <h4 class="sec-title"><SvgIcon name="chart" />逐日趋势<span class="muted" style="font-weight:400;font-size:12px;margin-left:8px">柱=当日告警 · 红=疑似误报 · 黄=被抑制</span></h4>
            <div class="trend-bars">
              <div v-for="t in q.data.trend" :key="t.date" class="tb" :title="t.date+'：告警 '+t.total+' · 疑似误报 '+t.suspected+' · 抑制 '+t.suppressed+' · 转工单 '+t.wo">
                <div class="tb-col"><div class="tb-fill" :style="{height:qBarH(t.total)+'%'}"></div><div v-if="t.suppressed" class="tb-supp" :style="{height:qBarH(t.suppressed)+'%'}"></div><div v-if="t.suspected" class="tb-breach" :style="{height:qBarH(t.suspected)+'%'}"></div></div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="card">
        <h4 class="sec-title"><SvgIcon name="pin" />重复告警测点 TOP（≥2 次）<span class="muted" style="font-weight:400;font-size:12px;margin-left:8px">同一测点反复告警 = 阈值裕度不足或设备劣化信号，点击测点名穿透详情</span></h4>
        <div class="table-wrap"><table class="data-table">
          <thead><tr><th>测点</th><th>编码</th><th>告警次数</th><th>涉及天数</th><th>疑似误报</th><th>被抑制</th><th>最高级别</th><th>最近告警</th></tr></thead>
          <tbody>
            <tr v-for="p in q.data.by_point" :key="p.point_id">
              <td><a v-if="p.point_id" class="link" @click="$router.push({path:'/monitoring-points',query:{id:p.point_id,detail:'1'}})" title="打开测点详情">{{ p.point_name }}</a><span v-else>{{ p.point_name }}</span></td>
              <td class="mono muted">{{ p.point_code || '-' }}</td>
              <td><b>{{ p.count }}</b></td><td>{{ p.days }}</td>
              <td :class="p.suspected_false>0?'td-breach':''">{{ p.suspected_false }}</td>
              <td>{{ p.suppressed }}</td>
              <td><span class="badge" :class="'sev-'+p.severity_max">{{ SEV[p.severity_max] || '-' }}</span></td>
              <td class="muted">{{ fmtQ(p.last_at) }}</td>
            </tr>
            <tr v-if="!q.data.by_point.length"><td colspan="8" class="empty">期内无重复告警测点</td></tr>
          </tbody>
        </table></div>
      </div>
      <div class="card muted q-criteria"><SvgIcon name="info" /> 口径说明：{{ q.data.criteria.explain }}。周期评估红线：近 7 天触发 ≥{{ q.data.criteria.scan_min_triggers }} 次且误报率 ≥{{ q.data.criteria.scan_false_rate }}% 的规则，自动向管理员发送「告警规则噪声预警」通知（7 天幂等，系统设置 alert_quality_notify 可关）。</div>
      </template>
    </div>

    <template v-if="mainTab==='list'">
    <div class="toolbar">
      <select class="input" style="width:160px" v-model="f.point_type" @change="load(1)">
        <option value="">全部类型</option>
        <option v-for="(n, k) in TYPES" :key="k" :value="k">{{ n }}</option>
      </select>
      <span style="flex:1"></span>
      <button class="btn" @click="wizard.show = true"><SvgIcon name="bolt" />按模板批量生成</button>
      <button class="btn btn-primary" @click="openForm()">＋ 新建规则</button>
    </div>

    <!-- 类型模板速览 -->
    <div class="tpl-strip">
      <div v-for="(tpl, tk) in TEMPLATES" :key="tk" class="tpl-chip" @click="openFormForType(tk)">
        <b>{{ TYPES[tk] }}</b>
        <span class="muted">{{ tpl.map(t => `${OPS[t.operator]}${t.threshold}${t.unit || ''}`).join(' · ') }}</span>
      </div>
      <div class="tpl-chip tpl-hint">点击类型卡 = 用该类型默认模板新建规则</div>
    </div>

    <div class="card">
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>名称</th><th>适用目标</th><th>触发条件</th><th>严重度</th><th>自动建工单</th><th>状态</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="r in list" :key="r.id">
              <td>{{ r.name }}</td>
              <td>
                <a v-if="r.point_id" class="link" @click="$router.push({path:'/monitoring-points',query:{id:r.point_id,detail:'1'}})" :title="'打开测点详情'"><SvgIcon name="pin" />{{ pointName(r.point_id) }}</a>
                <span v-else-if="r.point_type" class="badge"><SvgIcon name="tag" />{{ TYPES[r.point_type] }}（同类型全部测点）</span>
                <span v-else class="badge"><SvgIcon name="globe" />全局</span>
              </td>
              <td class="mono">{{ condText(r) }}</td>
              <td><span class="badge" :class="'sev-' + r.severity">{{ SEV[r.severity] }}</span></td>
              <td><span class="badge" :class="r.auto_create_work_order ? 'st-1' : 'st-0'">{{ r.auto_create_work_order ? '是' : '否' }}</span></td>
              <td>
                <label class="sw"><input type="checkbox" :checked="r.enabled" @change="tog(r)" /> {{ r.enabled ? '启用' : '停用' }}</label>
              </td>
              <td class="ops">
                <button class="btn btn-mini" @click="openForm(r)">编辑</button>
                <button class="btn btn-mini" @click="cloneRule(r)">复制</button>
                <button class="btn btn-mini btn-mini-danger" @click="del(r)">删除</button>
              </td>
            </tr>
            <tr v-if="!list.length"><td colspan="7" class="empty">暂无告警规则</td></tr>
          </tbody>
        </table>
      </div>
      <Pagination :page="page" :total="total" :page-size="size" @change="load" />
    </div>
    </template>

    <!-- 新建/编辑：抽屉式 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show = false">
      <div class="drawer">
        <div class="drawer-head"><h3>{{ form.id ? '编辑告警规则' : '新建告警规则' }}</h3><button class="btn btn-mini" @click="form.show = false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <fieldset class="fs"><legend><SvgIcon name="target" />适用目标</legend>
            <div class="target-modes">
              <label class="tm" :class="{on: form.mode==='point'}"><input type="radio" value="point" v-model="form.mode" /> 指定测点<div class="tm-desc">仅对该测点生效，优先级最高</div></label>
              <label class="tm" :class="{on: form.mode==='type'}"><input type="radio" value="type" v-model="form.mode" /> 按测点类型<div class="tm-desc">该类型全部测点共用</div></label>
              <label class="tm" :class="{on: form.mode==='global'}"><input type="radio" value="global" v-model="form.mode" /> 全局<div class="tm-desc">兜底规则，所有测点</div></label>
            </div>
            <div class="form-grid" style="margin-top:12px">
              <label v-if="form.mode==='point'" class="full">选择测点 *
                <select class="input" v-model.number="form.point_id">
                  <option :value="0">请选择</option>
                  <option v-for="p in points" :key="p.id" :value="p.id">{{ p.name }}（{{ TYPES[p.type] }} · {{ p.code }}）</option>
                </select>
              </label>
              <label v-if="form.mode==='type'" class="full">测点类型 *
                <select class="input" v-model="form.point_type" @change="applyTemplate(form.point_type)">
                  <option value="">请选择</option>
                  <option v-for="(n, k) in TYPES" :key="k" :value="k">{{ n }}</option>
                </select>
              </label>
              <p v-if="form.mode!=='global'" class="muted small full"><SvgIcon name="bulb" />选择测点/类型后可点击下方「套用推荐模板」快速填入该类测点的行业常规阈值，再按实际微调</p>
            </div>
            <div v-if="tplForForm.length" class="tpl-apply">
              <button class="btn btn-mini" @click="applyTplItem()" v-if="form.mode!=='global'"><SvgIcon name="clipboard" />套用推荐模板（{{ tplForForm.length }} 条中的第 {{ tplIdx + 1 }} 条）</button>
              <span v-for="(t, i) in tplForForm" :key="i" class="tpl-pill" :class="{on: i===tplIdx}" @click="tplIdx=i;applyTplItem()">{{ t.label }} {{ OPS[t.operator] }}{{ t.threshold }}{{ t.unit }}</span>
            </div>
          </fieldset>

          <fieldset class="fs"><legend><SvgIcon name="gear" />触发条件</legend>
            <div class="form-grid">
              <label class="full">规则名称 *<input class="input" v-model="form.name" placeholder="如：A机房温度过高" /></label>
              <label>指标
                <select class="input" v-model="form.metric">
                  <option value="value">测点读数</option><option value="status">测点状态</option>
                </select>
              </label>
              <label>运算符
                <select class="input" v-model="form.operator">
                  <option v-for="(n, k) in OPS" :key="k" :value="k">{{ n }}</option>
                </select>
              </label>
              <label>阈值 *<input class="input" type="number" step="0.01" v-model.number="form.threshold" /></label>
              <label>严重度
                <select class="input" v-model.number="form.severity">
                  <option :value="1">提示</option><option :value="2">警告</option><option :value="3">严重</option>
                </select>
              </label>
              <label class="full">告警消息模板<input class="input" v-model="form.message" placeholder="留空则自动生成：{测点名} {条件描述}" /></label>
            </div>
          </fieldset>

          <fieldset class="fs"><legend><SvgIcon name="bell" />响应动作</legend>
            <div class="form-grid">
              <label class="sw-full"><input type="checkbox" v-model="form.enabled" /> 启用规则</label>
              <label class="sw-full"><input type="checkbox" v-model="form.auto_create_work_order" /> 触发后自动创建运维工单</label>
              <label>超时未确认自动升级（分钟）
                <input class="input" type="number" min="0" step="5" v-model.number="form.escalate_after_min" placeholder="0=不升级" />
              </label>
              <p class="muted small full" style="margin:0">告警触发后超过该时长仍无人确认 → 严重度自动 +1（封顶严重）、写升级留痕并推送通知；升级到「严重」且开启自动工单时会补开工单。</p>
            </div>
          </fieldset>

          <div class="rule-preview">
            <b>规则预览：</b>{{ previewText }}
          </div>
          <p v-if="form.err" class="err">{{ form.err }}</p>
        </div>
        <div class="drawer-foot">
          <button class="btn" @click="form.show = false">取消</button>
          <button class="btn btn-primary" :disabled="form.saving" @click="save">{{ form.saving ? '保存中...' : '保存规则' }}</button>
        </div>
      </div>
    </div>

    <!-- 批量生成向导：抽屉式 -->
    <div v-if="wizard.show" class="drawer-mask" @click.self="wizard.show = false">
      <div class="drawer" style="width:720px">
        <div class="drawer-head"><h3><SvgIcon name="bolt" />按模板批量生成规则</h3><button class="btn btn-mini" @click="wizard.show = false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <p class="muted small" style="margin-top:0">按测点类型套用行业常规阈值模板，为选定测点批量生成<b>每个测点专属</b>的规则（同测点+同条件已存在的自动跳过）。</p>
          <fieldset class="fs"><legend>① 选择测点类型</legend>
            <div class="wiz-types">
              <label v-for="(tpl, tk) in TEMPLATES" :key="tk" class="wt" :class="{on: wizard.types.includes(tk)}">
                <input type="checkbox" :value="tk" v-model="wizard.types" />
                <b>{{ TYPES[tk] }}</b>
                <span class="muted small">{{ tpl.map(t => t.label + ' ' + OPS[t.operator] + t.threshold + (t.unit||'')).join('；') }}</span>
              </label>
            </div>
          </fieldset>
          <fieldset class="fs"><legend>② 选择测点（{{ wizardPoints.length }} 个符合类型）</legend>
            <div class="wiz-pts-head">
              <label class="sw"><input type="checkbox" :checked="wizard.points.length===wizardPoints.length && wizardPoints.length>0" @change="toggleAllPoints" /> 全选</label>
              <span class="muted small">已选 {{ wizard.points.length }} 个</span>
            </div>
            <div class="wiz-pts">
              <label v-for="p in wizardPoints" :key="p.id" class="wp" :class="{on: wizard.points.includes(p.id)}">
                <input type="checkbox" :value="p.id" v-model="wizard.points" />
                {{ p.name }} <span class="muted small">{{ p.code }}</span>
              </label>
              <p v-if="!wizardPoints.length" class="muted small">先在第①步勾选类型</p>
            </div>
          </fieldset>
          <fieldset class="fs"><legend>③ 生成预览（{{ previewRules.length }} 条规则）</legend>
            <div class="wiz-preview">
              <div v-for="(r, i) in previewRules.slice(0, 60)" :key="i" class="wiz-line">
                <span class="badge" :class="'sev-'+r.severity">{{ SEV[r.severity] }}</span>
                {{ r.name }} <span class="mono muted">{{ OPS[r.operator] }} {{ r.threshold }}</span>
              </div>
              <p v-if="!previewRules.length" class="muted small">完成①②后此处显示将生成的规则清单</p>
              <p v-if="previewRules.length > 60" class="muted small">… 等共 {{ previewRules.length }} 条</p>
            </div>
          </fieldset>
          <p v-if="wizard.err" class="err">{{ wizard.err }}</p>
          <p v-if="wizard.done" class="ok-msg">{{ wizard.done }}</p>
        </div>
        <div class="drawer-foot">
          <button class="btn" @click="wizard.show = false">关闭</button>
          <button class="btn btn-primary" :disabled="wizard.saving || !previewRules.length" @click="runWizard">{{ wizard.saving ? '生成中...' : `生成 ${previewRules.length} 条规则` }}</button>
        </div>
      </div>
    </div>

    <!-- V70 高噪规则处置：抽屉式（预览→三动作，全人工确认） -->
    <div v-if="tune.show" class="drawer-mask" @click.self="tune.show = false">
      <div class="drawer">
        <div class="drawer-head"><h3><SvgIcon name="gauge" />高噪规则处置 · {{ tune.ruleName }}</h3><button class="btn btn-mini" @click="tune.show = false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <div v-if="tune.loading" class="card muted" style="padding:24px;text-align:center">正在统计噪声与仿真读数样本…</div>
          <template v-else-if="tune.data">
            <fieldset class="fs"><legend><SvgIcon name="storm" />近 7 天噪声统计</legend>
              <div class="tune-stats">
                <div class="ts"><span class="ts-v">{{ tune.data.stats.triggers }}</span><span class="ts-l">触发</span></div>
                <div class="ts"><span class="ts-v td-breach">{{ tune.data.stats.suspected }}</span><span class="ts-l">疑似误报</span></div>
                <div class="ts"><span class="ts-v">{{ tune.data.stats.suppressed }}</span><span class="ts-l">被抑制</span></div>
                <div class="ts"><span class="ts-v">{{ tune.data.stats.ack_rate }}%</span><span class="ts-l">确认率</span></div>
                <div class="ts"><span class="ts-v">{{ tune.data.stats.noise_score }}</span><span class="ts-l">噪声分</span></div>
              </div>
              <p class="muted small" style="margin:10px 0 0">当前条件：<b class="mono">{{ OPS[tune.data.rule.operator] || tune.data.rule.operator }} {{ tune.data.rule.threshold }}</b> · 严重度 {{ SEV[tune.data.rule.severity] }} · {{ tune.data.rule.enabled ? '启用中' : '已停用' }}</p>
            </fieldset>
            <fieldset v-if="tune.data.sample && tune.data.sample.p95 !== undefined" class="fs"><legend><SvgIcon name="chart" />读数样本仿真（近 7 天）</legend>
              <div class="tune-stats">
                <div class="ts"><span class="ts-v">{{ tune.data.sample.min }}</span><span class="ts-l">最小</span></div>
                <div class="ts"><span class="ts-v">{{ tune.data.sample.p50 }}</span><span class="ts-l">中位</span></div>
                <div class="ts"><span class="ts-v">{{ tune.data.sample.p95 }}</span><span class="ts-l">P95</span></div>
                <div class="ts"><span class="ts-v">{{ tune.data.sample.max }}</span><span class="ts-l">最大</span></div>
                <div class="ts"><span class="ts-v">{{ tune.data.sample.count }}</span><span class="ts-l">样本数</span></div>
              </div>
              <div class="rule-preview" style="margin-top:12px"><b>仿真结论：</b>{{ tune.data.explain }}</div>
              <p v-if="tune.data.sample.est_reduction_pct > 0" class="tune-reduce">采用建议阈值后，预计触发次数减少 <b>{{ tune.data.sample.est_reduction_pct }}%</b>（{{ tune.data.sample.cur_trigger }} → {{ tune.data.sample.sim_trigger }} 次）</p>
            </fieldset>
            <fieldset v-else class="fs"><legend><SvgIcon name="info" />仿真说明</legend>
              <div class="rule-preview">{{ tune.data.explain }}</div>
            </fieldset>
            <fieldset class="fs"><legend><SvgIcon name="gear" />处置动作（均需人工确认，全程审计留痕）</legend>
              <div class="form-grid">
                <label>新阈值（当前 {{ tune.data.rule.threshold }}<span v-if="tune.data.suggest_threshold !== undefined"> · 建议 {{ tune.data.suggest_threshold }}</span>）
                  <input class="input" type="number" step="0.01" v-model.number="tune.newThreshold" />
                </label>
                <label>处置备注（写入审计）<input class="input" v-model="tune.note" placeholder="如：按 P95 仿真建议调整" /></label>
              </div>
              <div class="tune-quick" v-if="tune.data.suggest_threshold !== undefined">
                <button class="btn btn-mini" @click="tune.newThreshold = tune.data.suggest_threshold">填入建议值 {{ tune.data.suggest_threshold }}</button>
              </div>
            </fieldset>
            <p v-if="tune.err" class="err">{{ tune.err }}</p>
          </template>
          <p v-else class="err">{{ tune.err || '加载失败' }}</p>
        </div>
        <div class="drawer-foot">
          <button class="btn" @click="tune.show = false">关闭</button>
          <button class="btn" :disabled="tune.busy || tune.loading" @click="doOptOrder" title="生成【规则优化】运维工单（同规则未闭环工单唯一，重复调用不重复建单）"><SvgIcon name="clipboard" />转优化工单</button>
          <button class="btn btn-mini-danger" :disabled="tune.busy || tune.loading || !tune.data || !tune.data.rule.enabled" @click="doDisable" style="padding:8px 14px;border-radius:6px"><SvgIcon name="stop" />停用规则</button>
          <button class="btn btn-primary" :disabled="tune.busy || tune.loading || tune.newThreshold === null || tune.newThreshold === undefined || isNaN(tune.newThreshold)" @click="doTune">{{ tune.busy ? '执行中...' : '保存新阈值' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { showConfirm, toast } from '../ui-dialog'
import Pagination from '../components/Pagination.vue'

const route = useRoute()
const mainTab = ref('list')

const TYPES = { power_dist: '配电', ups: 'UPS', hvac: '空调', temp_humidity: '温湿度', water_leak: '漏水', smoke: '烟感', cdu: '液冷CDU', leak_rope: '漏液绳', netdev: '网络设备' }
const OPS = { gt: '>', lt: '<', eq: '=', ne: '≠' }
const SEV = { 1: '提示', 2: '警告', 3: '严重' }

// 分类型行业常规阈值模板（每条都可被生成的规则继承后再微调）
const TEMPLATES = {
  temp_humidity: [
    { label: '温度过高', operator: 'gt', threshold: 28, severity: 2, unit: '°C', message: '温度超过 28°C，请检查制冷' },
    { label: '温度严重过高', operator: 'gt', threshold: 35, severity: 3, unit: '°C', message: '温度超过 35°C，立即处理' },
    { label: '温度过低', operator: 'lt', threshold: 15, severity: 1, unit: '°C', message: '温度低于 15°C，注意结露风险' },
  ],
  power_dist: [
    { label: '电流过载', operator: 'gt', threshold: 0.9, severity: 3, unit: '×额定', message: '电流超过额定 90%，存在过载风险' },
    { label: '电流偏高', operator: 'gt', threshold: 0.8, severity: 2, unit: '×额定', message: '电流超过额定 80%，请关注负载' },
    { label: '电压异常', operator: 'ne', threshold: 220, severity: 2, unit: 'V', message: '电压偏离 220V，请检查供电' },
  ],
  ups: [
    { label: '电池电量低', operator: 'lt', threshold: 30, severity: 2, unit: '%', message: 'UPS 电池电量低于 30%' },
    { label: '电池电量严重不足', operator: 'lt', threshold: 15, severity: 3, unit: '%', message: 'UPS 电池电量低于 15%，立即处理' },
    { label: '负载率过高', operator: 'gt', threshold: 80, severity: 2, unit: '%', message: 'UPS 负载率超过 80%' },
  ],
  hvac: [
    { label: '回风温度过高', operator: 'gt', threshold: 27, severity: 2, unit: '°C', message: '空调回风温度超过 27°C' },
    { label: '压缩机异常', operator: 'ne', threshold: 1, severity: 3, unit: '', message: '空调运行状态异常' },
  ],
  water_leak: [
    { label: '漏水报警', operator: 'gt', threshold: 0, severity: 3, unit: '', message: '检测到漏水，立即处理' },
  ],
  smoke: [
    { label: '烟感报警', operator: 'gt', threshold: 0, severity: 3, unit: '', message: '烟感报警，立即现场确认' },
  ],
  netdev: [
    { label: '设备离线（警告）', operator: 'eq', threshold: 0, severity: 2, unit: '', message: '网络设备离线（状态离线/心跳超时/绑定测点全部离线），请检查设备与链路' },
    { label: '核心设备离线（严重）', operator: 'eq', threshold: 0, severity: 3, unit: '', message: '网络设备离线，可能影响业务，立即现场确认' },
  ],
  cdu: [
    { label: '供液温度过高', operator: 'gt', threshold: 32, severity: 2, unit: '°C', message: 'CDU 供液温度超过 32°C，制冷能力不足' },
    { label: '回液温度过高', operator: 'gt', threshold: 45, severity: 2, unit: '°C', message: 'CDU 回液温度超过 45°C，请检查换热' },
    { label: '冷却液流量过低', operator: 'lt', threshold: 10, severity: 3, unit: 'L/min', message: 'CDU 流量过低，存在过热风险，立即处理' },
    { label: '管路压力异常', operator: 'gt', threshold: 0.6, severity: 2, unit: 'MPa', message: 'CDU 管路压力超过 0.6MPa，检查泄漏/堵塞' },
  ],
  leak_rope: [
    { label: '漏液报警', operator: 'gt', threshold: 0, severity: 3, unit: '', message: '漏液绳报警，冷却液泄漏，立即现场处理' },
  ],
}

const list = ref([])
const points = ref([])
const page = ref(1), total = ref(0), size = ref(20)
const f = reactive({ point_type: '' })
const form = reactive({ show: false, saving: false, err: '', id: 0, mode: 'type', name: '', point_id: 0, point_type: '', metric: 'value', operator: 'gt', threshold: 0, severity: 2, enabled: true, auto_create_work_order: false, escalate_after_min: 0, message: '' })
const tplIdx = ref(0)
const wizard = reactive({ show: false, saving: false, err: '', done: '', types: [], points: [] })

const pointName = id => { const p = points.value.find(x => x.id === id); return p ? p.name : '测点#' + id }
const unitOf = p => p?.unit || ''
function condText(r) {
  const unit = r.point_id ? unitOf(points.value.find(x => x.id === r.point_id)) : ''
  return `${OPS[r.operator] || r.operator} ${r.threshold}${unit ? ' ' + unit : ''}`
}

// 当前表单适用的模板列表
const tplForForm = computed(() => {
  if (form.mode === 'point') {
    const p = points.value.find(x => x.id === form.point_id)
    return p ? (TEMPLATES[p.type] || []) : []
  }
  if (form.mode === 'type') return TEMPLATES[form.point_type] || []
  return []
})
function applyTplItem() {
  const t = tplForForm.value[tplIdx.value]
  if (!t) return
  form.operator = t.operator; form.threshold = t.threshold; form.severity = t.severity; form.message = t.message
  if (!form.name || form.name.includes('（模板）')) {
    const who = form.mode === 'point' ? pointName(form.point_id) : (TYPES[form.point_type] || '')
    form.name = `${who} ${t.label}（模板）`
  }
}
function applyTemplate(tk) { tplIdx.value = 0; if (TEMPLATES[tk]) applyTplItem() }

const previewText = computed(() => {
  const who = form.mode === 'point' ? `测点「${pointName(form.point_id)}」`
    : form.mode === 'type' ? `所有 ${TYPES[form.point_type] || '?'} 类测点` : '全部测点'
  const unit = form.mode === 'point' ? unitOf(points.value.find(x => x.id === form.point_id)) : ''
  return `当 ${who} 的读数 ${OPS[form.operator]} ${form.threshold}${unit ? ' ' + unit : ''} 时，触发【${SEV[form.severity]}】告警${form.auto_create_work_order ? '，并自动创建工单' : ''}${form.escalate_after_min > 0 ? `；超过 ${form.escalate_after_min} 分钟未确认自动升级` : ''}`
})

async function load(p) {
  if (p) page.value = p
  const params = { page: page.value, size: size.value }
  if (f.point_type) params.point_type = f.point_type
  const r = await api.get('/alert-rules', params)
  list.value = r.data?.items || []
  total.value = r.data?.total || 0
}

function openForm(r) {
  form.err = ''
  if (r) Object.assign(form, { show: true, saving: false, id: r.id, mode: r.point_id ? 'point' : (r.point_type ? 'type' : 'global'), name: r.name, point_id: r.point_id || 0, point_type: r.point_type || '', metric: r.metric || 'value', operator: r.operator, threshold: r.threshold, severity: r.severity, enabled: r.enabled, auto_create_work_order: r.auto_create_work_order, escalate_after_min: r.escalate_after_min || 0, message: r.message || '' })
  else Object.assign(form, { show: true, saving: false, id: 0, mode: 'type', name: '', point_id: 0, point_type: '', metric: 'value', operator: 'gt', threshold: 0, severity: 2, enabled: true, auto_create_work_order: false, escalate_after_min: 0, message: '' })
  tplIdx.value = 0
}
function openFormForType(tk) {
  openForm(null)
  form.mode = 'type'; form.point_type = tk
  applyTemplate(tk)
  form.name = ''
}
function cloneRule(r) {
  openForm(r); form.id = 0; form.name = r.name + '（副本）'
}

async function save() {
  form.err = ''
  if (!form.name) { form.err = '请填写规则名称'; return }
  if (form.mode === 'point' && !form.point_id) { form.err = '请选择测点'; return }
  if (form.mode === 'type' && !form.point_type) { form.err = '请选择测点类型'; return }
  form.saving = true
  const body = {
    name: form.name, metric: form.metric, operator: form.operator, threshold: form.threshold,
    severity: form.severity, enabled: form.enabled, auto_create_work_order: form.auto_create_work_order, escalate_after_min: form.escalate_after_min || 0, message: form.message,
    point_id: form.mode === 'point' ? form.point_id : null,
    point_type: form.mode === 'type' ? form.point_type : '',
  }
  const r = form.id ? await api.put(`/alert-rules/${form.id}`, body) : await api.post('/alert-rules', body)
  form.saving = false
  if (r.code === 0) { form.show = false; load() }
  else form.err = r.msg || '保存失败'
}

async function tog(r) {
  await api.put(`/alert-rules/${r.id}`, { enabled: !r.enabled })
  load()
}
async function del(r) {
  if (!(await showConfirm({ message: `删除告警规则「${r.name}」？` }))) return
  await api.del(`/alert-rules/${r.id}`)
  load()
}

// ── 批量向导 ──
const wizardPoints = computed(() => points.value.filter(p => wizard.types.includes(p.type)))
watch(() => wizard.types, () => { wizard.points = wizard.points.filter(id => wizardPoints.value.some(p => p.id === id)) })
function toggleAllPoints() {
  wizard.points = wizard.points.length === wizardPoints.value.length ? [] : wizardPoints.value.map(p => p.id)
}
const previewRules = computed(() => {
  const out = []
  for (const pid of wizard.points) {
    const p = points.value.find(x => x.id === pid)
    if (!p) continue
    for (const t of (TEMPLATES[p.type] || [])) {
      out.push({
        name: `${p.name} ${t.label}`, point_id: p.id, point_type: '',
        operator: t.operator, threshold: t.threshold, severity: t.severity,
        message: `${p.name} ${t.message || t.label}（当前 {value}${t.unit || ''}）`,
        enabled: true, auto_create_work_order: t.severity === 3, metric: 'value',
      })
    }
  }
  return out
})
async function runWizard() {
  wizard.err = ''; wizard.done = ''; wizard.saving = true
  const r = await api.post('/alert-rules/batch', { rules: previewRules.value })
  wizard.saving = false
  if (r.code === 0) {
    wizard.done = `生成完成：新建 ${r.data.created} 条，跳过重复 ${r.data.skipped} 条`
    wizard.points = []
    load(1)
  } else wizard.err = r.msg || '生成失败'
}

onMounted(async () => {
  const pr = await api.get('/monitoring-points', { size: 500 })
  points.value = pr.data?.items || []
  load(1)
  applyDeepQ()
})

// ── V64 质量治理 ──
const q = reactive({ days: 30, data: null })
async function loadQuality() {
  const r = await api.get('/alerts/quality-analysis', { days: q.days })
  if (r.code === 0) q.data = r.data
}
function switchQuality() { mainTab.value = 'quality'; loadQuality() }
function noiseCls(v) { return v >= 40 ? 'bad' : v >= 20 ? 'mid' : 'good' }
function ackCls(v) { return v >= 80 ? 'good' : v >= 50 ? 'mid' : 'bad' }
function qBarH(v) { if (!q.data) return 0; const mx = Math.max(1, ...q.data.trend.map(t => t.total)); return Math.max(v > 0 ? 6 : 0, Math.round(v / mx * 100)) }
function fmtQ(t) { return t ? String(t).replace('T', ' ').slice(0, 16) : '-' }
async function editFromQuality(row) {
  if (!row.rule_id) return
  const r = await api.get('/alert-rules', { page: 1, size: 500 })
  const rule = (r.data?.items || []).find(x => x.id === row.rule_id)
  if (rule) { mainTab.value = 'list'; openForm(rule) }
  else toast('规则不存在或已删除')
}
async function runScan() {
  const r = await api.post('/alerts/quality-scan', {})
  if (r.code === 0) toast(`评估完成：检查规则 ${r.data.evaluated} 条，新发噪声预警 ${r.data.notified} 条（7天内已通知过的不重复）`)
  else toast(r.msg || '评估失败')
}
function exportQCsv() {
  if (!q.data) return
  const d = q.data
  const L = []
  L.push(`告警质量分析,近${d.days}天,生成时间,${fmtQ(d.generated_at)}`)
  L.push('')
  L.push('【总体指标】')
  L.push('指标,数值')
  const k = d.kpi
  L.push(`告警总数,${k.total}`)
  L.push(`活跃,${k.open}`)
  L.push(`已结束,${k.resolved}`)
  L.push(`疑似误报,${k.suspected_false}（${k.false_rate}%）`)
  L.push(`风暴抑制,${k.suppressed}（${k.suppress_rate}%）· 风暴 ${k.storms} 场`)
  L.push(`确认率,${k.ack_rate}%（平均确认 ${k.mtta_min} 分钟）`)
  L.push(`工单转化率,${k.wo_rate}%（升级 ${k.escalated} · 屏蔽 ${k.blocked}）`)
  L.push(`重复告警率,${k.repeat_rate}%（≥3次测点 ${k.repeat_points} 个）`)
  L.push(`噪声率,${k.noise_rate}%（${{ ok: '正常', warn: '偏高', bad: '严重' }[k.grade]}）`)
  L.push(`平均解决时长,${k.mttr_min} 分钟`)
  L.push('')
  L.push('【规则噪声榜】')
  L.push('规则ID,规则,触发,疑似误报,误报率%,被抑制,确认率%,平均确认min,转工单,噪声分,调优建议')
  for (const r of d.by_rule) L.push(`${r.rule_id},"${r.rule_name}",${r.total},${r.suspected_false},${r.false_rate},${r.suppressed},${r.ack_rate},${r.avg_ack_min},${r.wo_converted},${r.noise_score},"${r.suggestion || ''}"`)
  L.push('')
  L.push('【重复告警测点】')
  L.push('测点ID,测点,编码,告警次数,涉及天数,疑似误报,被抑制,最高级别,最近告警')
  for (const p of d.by_point) L.push(`${p.point_id},"${p.point_name}",${p.point_code || ''},${p.count},${p.days},${p.suspected_false},${p.suppressed},${SEV[p.severity_max] || ''},${fmtQ(p.last_at)}`)
  const blob = new Blob(['\uFEFF' + L.join('\n')], { type: 'text/csv' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `dcim_alert_quality_${d.days}d.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}
function applyDeepQ() {
  if (route.query.tab === 'quality') {
    mainTab.value = 'quality'
    if (route.query.days) q.days = Number(route.query.days) || 30
    loadQuality()
  }
  if (route.query.rule_id) openTune({ rule_id: Number(route.query.rule_id), rule_name: '' })
}
watch(() => route.query, () => applyDeepQ())

// ── V70 高噪规则处置（预览→调阈值/停用/转优化工单，全人工确认+审计留痕） ──
const tune = reactive({ show: false, loading: false, busy: false, ruleId: 0, ruleName: '', data: null, newThreshold: null, note: '', err: '' })
async function openTune(row) {
  if (!row.rule_id) return
  tune.show = true; tune.loading = true; tune.data = null; tune.err = ''
  tune.ruleId = row.rule_id; tune.ruleName = row.rule_name || `规则 #${row.rule_id}`; tune.note = ''; tune.newThreshold = null
  const r = await api.get(`/alert-rules/${row.rule_id}/tuning-preview`)
  tune.loading = false
  if (r.code === 0) {
    tune.data = r.data
    tune.ruleName = r.data.rule?.name || tune.ruleName
    if (r.data.suggest_threshold !== undefined) tune.newThreshold = r.data.suggest_threshold
  } else tune.err = r.msg || '预览加载失败'
}
async function doTune() {
  tune.busy = true; tune.err = ''
  const r = await api.post(`/alert-rules/${tune.ruleId}/tune`, { new_threshold: tune.newThreshold, note: tune.note })
  tune.busy = false
  if (r.code === 0) {
    toast(`阈值已调整：${r.data.old_threshold} → ${r.data.new_threshold}（已审计留痕）`)
    tune.show = false; loadQuality(); load()
  } else tune.err = r.msg || '调整失败'
}
async function doDisable() {
  if (!(await showConfirm({ title: '停用告警规则', message: `停用「${tune.ruleName}」后该规则不再触发新告警，历史告警不受影响。确认停用？`, tone: 'danger' }))) return
  tune.busy = true; tune.err = ''
  const r = await api.post(`/alert-rules/${tune.ruleId}/disable`, { note: tune.note })
  tune.busy = false
  if (r.code === 0) { toast('规则已停用（已审计留痕）'); tune.show = false; loadQuality(); load() }
  else tune.err = r.msg || '停用失败'
}
async function doOptOrder() {
  tune.busy = true; tune.err = ''
  const r = await api.post(`/alert-rules/${tune.ruleId}/optimization-order`, { note: tune.note })
  tune.busy = false
  if (r.code === 0) {
    const wo = r.data.work_order
    toast(r.data.existed ? `该规则已有未闭环优化工单（${wo.code}），未重复建单` : `已生成规则优化工单 ${wo.code}，指派当班人并计入 SLA`)
    tune.show = false; loadQuality()
  } else tune.err = r.msg || '生成工单失败'
}
</script>

<style scoped>
.link { color: var(--accent); cursor: pointer; font-weight: 600; }
.link:hover { text-decoration: underline; }
.badge.sev-3 { color: #f56c6c; border-color: rgba(245,108,108,.4); }
.badge.sev-2 { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.sev-1 { color: #409eff; border-color: rgba(64,158,255,.4); }
.badge.st-1 { color: #67c23a; border-color: rgba(103,194,58,.4); }
.badge.st-0 { color: #909399; border-color: rgba(144,147,153,.4); }
.sw { cursor: pointer; font-size: 13px; display: inline-flex; align-items: center; gap: 4px; }
.sw-full { cursor: pointer; font-size: 13px; display: flex; align-items: center; gap: 8px; padding: 6px 0; }
.small { font-size: 12px; }
.full { grid-column: 1 / -1; }

/* 模板速览条 */
.tpl-strip { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
.tpl-chip { display: flex; flex-direction: column; gap: 2px; padding: 8px 12px; border: 1px solid var(--border); border-radius: 8px; cursor: pointer; font-size: 12px; background: var(--card2, transparent); transition: border-color .2s; max-width: 280px; }
.tpl-chip:hover { border-color: var(--accent); }
.tpl-chip b { font-size: 13px; }
.tpl-hint { cursor: default; color: var(--muted); justify-content: center; border-style: dashed; }

/* 抽屉 */
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}
.drawer{width:620px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}
.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}
.drawer-head h3{margin:0;font-size:18px}
.drawer-body{flex:1;overflow-y:auto;padding:20px}
.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}
.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}

/* 适用目标三选 */
.target-modes{display:grid;grid-template-columns:repeat(3,1fr);gap:8px}
.tm{border:1px solid var(--border);border-radius:8px;padding:10px 12px;cursor:pointer;font-size:13px;display:flex;flex-direction:column;gap:4px}
.tm input{margin-right:4px}
.tm.on{border-color:var(--accent);background:rgba(64,158,255,.06)}
.tm-desc{font-size:11px;color:var(--muted);font-weight:400}

/* 模板套用 */
.tpl-apply{display:flex;gap:6px;flex-wrap:wrap;margin-top:12px;align-items:center}
.tpl-pill{padding:3px 10px;border:1px solid var(--border);border-radius:12px;font-size:12px;cursor:pointer;color:var(--muted)}
.tpl-pill.on{border-color:var(--accent);color:var(--accent)}

.rule-preview{background:rgba(64,158,255,.07);border:1px dashed rgba(64,158,255,.4);border-radius:8px;padding:10px 14px;font-size:13px;line-height:1.6}
.err{color:#f56c6c;font-size:13px;margin:8px 0 0}
.ok-msg{color:#67c23a;font-size:13px;margin:8px 0 0}

/* 向导 */
.wiz-types{display:grid;grid-template-columns:1fr 1fr;gap:8px}
.wt{border:1px solid var(--border);border-radius:8px;padding:10px 12px;cursor:pointer;font-size:13px;display:flex;flex-direction:column;gap:3px}
.wt.on{border-color:var(--accent);background:rgba(64,158,255,.06)}
.wiz-pts-head{display:flex;justify-content:space-between;margin-bottom:8px}
.wiz-pts{max-height:200px;overflow-y:auto;display:grid;grid-template-columns:1fr 1fr;gap:6px;border:1px solid var(--border);border-radius:8px;padding:10px}
.wp{font-size:12px;display:flex;align-items:center;gap:6px;padding:4px 6px;border-radius:6px;cursor:pointer}
.wp.on{background:rgba(64,158,255,.1)}
.wiz-preview{max-height:220px;overflow-y:auto;font-size:12px}
.wiz-line{padding:4px 6px;border-bottom:1px dashed var(--border);display:flex;align-items:center;gap:8px}
.mono{font-family:monospace}

/* ===== V64 质量治理 ===== */
.tabs{display:flex;gap:8px;margin-bottom:14px}.tab{padding:7px 18px;border:1px solid var(--border);border-radius:8px;background:var(--card);cursor:pointer;font-size:13.5px;display:inline-flex;align-items:center;gap:6px;color:var(--text)}.tab.on{color:var(--accent);border-color:var(--accent);font-weight:600}
.q-grade{display:inline-flex;align-items:center;font-size:12.5px;padding:4px 12px;border-radius:14px;border:1px solid;font-weight:600}
.q-grade.g-ok{color:#67c23a;border-color:rgba(103,194,58,.45);background:rgba(103,194,58,.07)}
.q-grade.g-warn{color:#e6a23c;border-color:rgba(230,162,60,.45);background:rgba(230,162,60,.07)}
.q-grade.g-bad{color:#f56c6c;border-color:rgba(245,108,108,.45);background:rgba(245,108,108,.08)}
.sec-title{margin:20px 0 10px;font-size:15px;display:flex;align-items:center;gap:6px}
.sec-title:first-child{margin-top:4px}
.q-kpis{display:grid;grid-template-columns:repeat(6,1fr);gap:12px;margin-bottom:14px}
.q-kpi{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:14px 16px;display:flex;flex-direction:column;gap:4px}
.q-kpi .sk-label{font-size:12.5px;color:var(--muted)}
.q-kpi .sk-val{font-size:26px;font-weight:700;line-height:1.1}
.q-kpi .sk-sub{font-size:11.5px;color:var(--muted)}
.q-kpi.good .sk-val{color:#67c23a}.q-kpi.mid .sk-val{color:#e6a23c}.q-kpi.bad .sk-val{color:#f56c6c}
.q-cols{display:grid;grid-template-columns:1.6fr 1fr;gap:12px;margin-bottom:14px}
.q-side{display:flex;flex-direction:column;gap:12px}
.row-noisy td:first-child{font-weight:600}
.td-breach{color:#f56c6c;font-weight:700}
.rate-bar{position:relative;height:14px;background:var(--bg2);border-radius:7px;overflow:hidden;min-width:110px}
.rate-fill{position:absolute;left:0;top:0;bottom:0;border-radius:7px;opacity:.75}
.rate-fill.good{background:#67c23a}.rate-fill.mid{background:#e6a23c}.rate-fill.bad{background:#f56c6c}
.rate-txt{position:relative;z-index:1;font-size:11px;line-height:14px;padding-left:6px}
.q-sug{font-size:12px;color:var(--muted);max-width:280px}
.q-sug-hot{color:#e6a23c}
.trend-bars{display:flex;gap:2px;align-items:flex-end;height:90px;padding:4px 2px 0}
.tb{flex:1;height:100%;display:flex;align-items:flex-end}
.tb-col{flex:1;height:100%;display:flex;flex-direction:column;justify-content:flex-end;gap:1px}
.tb-fill{background:var(--accent);opacity:.75;border-radius:2px 2px 0 0;min-height:0}
.tb-supp{background:#e6a23c;opacity:.85;border-radius:2px}
.tb-breach{background:#f56c6c;opacity:.9;border-radius:0 0 2px 2px}
.q-criteria{font-size:12.5px;line-height:1.7;padding:14px 16px;display:flex;gap:8px;align-items:flex-start}
@media (max-width:1200px){.q-kpis{grid-template-columns:repeat(3,1fr)}.q-cols{grid-template-columns:1fr}}
@media (max-width:900px){.q-kpis{grid-template-columns:repeat(2,1fr)}}

/* V70 高噪规则处置抽屉 */
.btn-mini-accent{color:var(--accent);border-color:color-mix(in srgb,var(--accent) 45%,transparent)}
.btn-mini-accent:hover{background:color-mix(in srgb,var(--accent) 10%,transparent)}
.tune-stats{display:grid;grid-template-columns:repeat(5,1fr);gap:8px}
.ts{display:flex;flex-direction:column;align-items:center;gap:2px;padding:10px 6px;border:1px solid var(--border);border-radius:8px;background:var(--card2,transparent)}
.ts-v{font-size:20px;font-weight:700;line-height:1.2}
.ts-l{font-size:11.5px;color:var(--muted)}
.tune-reduce{margin:10px 0 0;font-size:13px;color:#67c23a}
.tune-quick{margin-top:10px}
@media (max-width:600px){.tune-stats{grid-template-columns:repeat(3,1fr)}}
</style>
