<template>
  <div>
    <div class="page-head"><h1>测点管理</h1>
      <div style="display:flex;align-items:center;gap:8px">
        <span class="live-dot" :class="liveOn?'live-on':''"></span>
        <span class="muted small">{{ liveOn ? '● 实时刷新中(30s)' : '实时刷新暂停' }}</span>
        <button class="btn btn-mini" @click="toggleLive">{{ liveOn ? '暂停' : '开启' }}</button>
      </div>
    </div>

    <!-- V15 在线率统计条 -->
    <div class="stat-bar">
      <div class="sb-item" @click="f.status='';load(1)" :class="{on: f.status===''}"><b>{{ stat.total }}</b><span>总测点</span></div>
      <div class="sb-item" @click="f.status=1;load(1)" :class="{on: f.status===1}"><b class="c-ok">{{ stat.normal }}</b><span>正常</span></div>
      <div class="sb-item" @click="f.status=2;load(1)" :class="{on: f.status===2}"><b class="c-warn">{{ stat.alert }}</b><span>告警</span></div>
      <div class="sb-item" @click="f.status=0;load(1)" :class="{on: f.status===0}"><b class="c-off">{{ stat.offline }}</b><span>离线</span></div>
      <div class="sb-item" @click="f.status=3;load(1)" :class="{on: f.status===3}"><b class="c-off">{{ stat.fault }}</b><span>故障</span></div>
      <div class="sb-bar">
        <div class="sb-bar-in" :style="{width: stat.onlinePct+'%'}"></div>
        <span class="sb-bar-t">在线率 {{ stat.onlinePct.toFixed(1) }}%</span>
      </div>
    </div>

    <div class="toolbar">
      <select class="input" style="width:160px" v-model.number="f.site_id" @change="load(1)">
        <option :value="0">全部站点</option>
        <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
      </select>
      <select class="input" style="width:140px" v-model="f.type" @change="load(1)">
        <option value="">全部类型</option>
        <option v-for="(n, k) in TYPES" :key="k" :value="k">{{ n }}</option>
      </select>
      <select class="input" style="width:120px" v-model="f.status" @change="load(1)">
        <option value="">全部状态</option>
        <option :value="1">正常</option>
        <option :value="2">告警</option>
        <option :value="0">离线</option>
        <option :value="3">故障</option>
      </select>
      <select class="input" style="width:120px" v-model="f.protocol" @change="load(1)">
        <option value="">全部协议</option>
        <option value="modbus">Modbus</option>
        <option value="snmp">SNMP</option>
        <option value="mqtt">MQTT</option>
        <option value="manual">手动</option>
      </select>
      <input class="input search-input" v-model="f.keyword" placeholder="搜索名称/编码/地址" @keyup.enter="load(1)" />
      <button class="btn" @click="load(1)">查询</button>
      <span style="flex:1"></span>
      <!-- V15 批量操作 -->
      <template v-if="selIds.length">
        <span class="muted small">已选 {{ selIds.length }} 项</span>
        <button class="btn btn-mini" @click="batchOp('enable')">批量启用</button>
        <button class="btn btn-mini" @click="batchOp('disable')">批量停用</button>
        <button class="btn btn-mini btn-mini-danger" @click="batchOp('delete')">批量删除</button>
        <button class="btn btn-mini" @click="selIds=[]">取消</button>
      </template>
      <button class="btn" @click="openImport()"><SvgIcon name="upload" />批量导入</button>
      <button class="btn" @click="exportPointsCSV"><SvgIcon name="download" />导出CSV</button>
      <button class="btn" @click="openTemplatePicker()"><SvgIcon name="clipboard" />从模板添加</button>
      <button class="btn btn-primary" @click="openForm()" style="margin-left:8px">＋ 自定义添加</button>
    </div>

    <div class="card">
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr>
            <th style="width:30px"><input type="checkbox" :checked="pageAllChecked" @change="togglePageAll" /></th>
            <th>编码</th><th>名称</th><th>类型</th><th>协议</th><th>地址</th>
            <th>当前值</th><th>状态</th><th>采集间隔</th><th>阈值</th><th>更新时间</th><th>操作</th>
          </tr></thead>
          <tbody>
            <tr v-for="p in list" :key="p.id" :class="{'row-active': detail.show && detail.p?.id===p.id}">
              <td><input type="checkbox" :value="p.id" v-model="selIds" /></td>
              <td class="mono"><a class="link" @click="openDetail(p)" :title="'查看测点详情与历史趋势'">{{ p.code }}</a></td>
              <td>{{ p.name }}</td>
              <td><span class="badge">{{ TYPES[p.type] || p.type }}</span></td>
              <td>
                <span class="badge" :class="'proto-'+p.protocol">{{ PROTO[p.protocol] || p.protocol }}</span>
              </td>
              <td class="mono muted" style="font-size:12px">{{ p.address || '-' }}</td>
              <td><strong>{{ p.current_value !== null ? p.current_value + (p.unit || '') : '-' }}</strong></td>
              <td><span class="badge" :class="'ptst-' + p.status">{{ PTST[p.status] }}</span></td>
              <td class="muted">{{ p.poll_interval || 30 }}s</td>
              <td class="muted" style="font-size:12px">
                <span v-if="p.min_threshold !== null">≥{{ p.min_threshold }}</span>
                <span v-if="p.min_threshold !== null && p.max_threshold !== null"> / </span>
                <span v-if="p.max_threshold !== null">≤{{ p.max_threshold }}</span>
                <span v-if="p.high_threshold != null" class="th-high"> / 过高{{ p.high_threshold }}</span>
                <span v-if="p.min_threshold === null && p.max_threshold === null && p.high_threshold == null">未设置</span>
              </td>
              <td class="muted">{{ fmt(p.updated_at) }}</td>
              <td class="ops">
                <button class="btn btn-mini btn-mini-primary" @click="openDetail(p)">详情</button>
                <button class="btn btn-mini" @click="openForm(p)">编辑</button>
                <button class="btn btn-mini btn-mini-danger" @click="del(p)">删除</button>
              </td>
            </tr>
            <tr v-if="!list.length"><td colspan="12" class="empty">暂无测点</td></tr>
          </tbody>
        </table>
      </div>
      <Pagination :page="page" :total="total" :page-size="size" @change="load" />
    </div>

    <!-- 测点详情抽屉 -->
    <div v-if="detail.show" class="drawer-mask" @click.self="detail.show=false">
      <div class="drawer">
        <div class="drawer-head">
          <h3>测点详情 · {{ detail.p?.name }}</h3>
          <button class="btn btn-mini" @click="detail.show=false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body" v-if="detail.p">
          <div class="detail-grid">
            <div><span class="dl">编码</span>{{ detail.p.code }}</div>
            <div><span class="dl">类型</span>{{ TYPES[detail.p.type] || detail.p.type }}</div>
            <div><span class="dl">站点</span>{{ siteName(detail.p.site_id) }}</div>
            <div><span class="dl">状态</span><span class="badge" :class="'ptst-'+detail.p.status">{{ PTST[detail.p.status] }}</span></div>
            <div><span class="dl">当前值</span><strong>{{ detail.p.current_value !== null && detail.p.current_value !== undefined ? detail.p.current_value + (detail.p.unit||'') : '-' }}</strong></div>
            <div><span class="dl">协议</span>{{ PROTO[detail.p.protocol] || detail.p.protocol }}</div>
            <div><span class="dl">采集间隔</span>{{ detail.p.poll_interval || 30 }}s</div>
            <div><span class="dl">更新时间</span>{{ fmt(detail.p.updated_at) }}</div>
            <div class="full"><span class="dl">地址</span><span class="mono" style="font-size:12px">{{ detail.p.address || '-' }}</span></div>
            <div v-if="detail.p.cabinet_id" class="full"><span class="dl">所属机柜</span>
              <a class="link" @click="$router.push({path:'/capacity', query:{cabinet_id: detail.p.cabinet_id}})">查看机柜容量 →</a>
            </div>
            <div v-if="detail.p.network_device_id" class="full"><span class="dl">绑定网络设备</span>
              <a class="link" @click="$router.push({path:'/network-devices', query:{id: detail.p.network_device_id, detail:'1'}})">{{ netdevName(detail.p.network_device_id) }} →</a>
            </div>
            <div class="full"><span class="dl">描述</span>{{ detail.p.description || '-' }}</div>
          </div>

          <!-- V45: 配置白话解析——非技术人员也能看懂这个测点是怎么采数据的 -->
          <h4 class="sec-title"><SvgIcon name="info" />配置解析（白话版）</h4>
          <div class="cfg-explain">
            <div v-for="(line, i) in explainPoint(detail.p)" :key="i" class="cfg-line">
              <span class="cfg-dot"></span>{{ line }}
            </div>
          </div>

          <h4 class="sec-title"><SvgIcon name="trend" />历史趋势（近24小时）</h4>
          <div ref="trendEl" class="trend-box"></div>
          <div v-if="!detail.readings.length" class="muted" style="font-size:12.5px">暂无读数数据</div>

          <h4 class="sec-title"><SvgIcon name="bell" />告警规则（{{ detail.rules.length }}）
            <router-link :to="{path:'/alert-rules'}" class="link" style="font-size:12.5px;margin-left:8px">规则管理 →</router-link>
          </h4>
          <div class="mini-list">
            <div v-for="r in detail.rules" :key="r.id" class="mini-item">
              <span class="badge" :class="'sev-'+r.severity">{{ SEV[r.severity] }}</span>
              <span>{{ r.name }}：{{ OPNAME[r.operator]||r.operator }} {{ r.threshold }}{{ detail.p.unit||'' }}</span>
              <span class="muted" style="margin-left:auto">{{ r.enabled?'启用':'停用' }}<template v-if="r.auto_create_work_order"> · 自动建单</template></span>
            </div>
            <div v-if="!detail.rules.length" class="muted" style="font-size:12.5px">未配置规则</div>
          </div>

          <h4 class="sec-title"><SvgIcon name="alert" />关联告警（近10条）
            <router-link to="/alert-wall" class="link" style="font-size:12.5px;margin-left:8px">告警盯屏 →</router-link>
          </h4>
          <div class="mini-list">
            <div v-for="a in detail.alerts" :key="a.id" class="mini-item">
              <span class="badge" :class="'sev-'+a.severity">{{ SEV[a.severity] }}</span>
              <span class="muted">{{ fmt(a.triggered_at) }}</span>
              <span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" :title="a.message">{{ a.message }}</span>
              <span class="badge" :class="'ast-'+a.status">{{ AST[a.status] }}</span>
            </div>
            <div v-if="!detail.alerts.length" class="muted" style="font-size:12.5px">暂无告警</div>
          </div>
        </div>
        <div class="drawer-foot">
          <button class="btn" @click="detail.show=false">关闭</button>
          <router-link :to="{path:'/trend-analysis'}" class="btn">趋势分析 →</router-link>
          <button class="btn btn-primary" @click="openForm(detail.p)">编辑测点</button>
        </div>
      </div>
    </div>

    <!-- V46: 批量导入向导 -->
    <div v-if="imp.show" class="drawer-mask" @click.self="imp.show=false">
      <div class="drawer" style="width:860px">
        <div class="drawer-head">
          <h3><SvgIcon name="upload" />批量导入测点</h3>
          <button class="btn btn-mini" @click="imp.show=false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
        <!-- 步骤条 -->
        <div class="imp-steps">
          <div class="imp-step" :class="{on: imp.step>=1, cur: imp.step===1}"><b>1</b>选择文件</div>
          <div class="imp-step" :class="{on: imp.step>=2, cur: imp.step===2}"><b>2</b>预览校验</div>
          <div class="imp-step" :class="{on: imp.step>=3, cur: imp.step===3}"><b>3</b>导入结果</div>
        </div>

        <div class="imp-body">
          <!-- Step 1 -->
          <template v-if="imp.step===1">
            <div class="imp-guide">
              <p><b>三步导入大量测点</b>：下载模板 → 用 Excel/WPS 按行填写 → 传回本页自动校验导入。</p>
              <div class="imp-tips">
                <div class="imp-tip-line"><SvgIcon name="check-circle" />中文表头，填错会逐行提示哪一列什么问题（如「站点不存在」「编码重复」）</div>
                <div class="imp-tip-line"><SvgIcon name="check-circle" />类型/协议/寄存器类型可以直接填中文（如「温湿度」「手动」「输入寄存器」）</div>
                <div class="imp-tip-line"><SvgIcon name="check-circle" />下限/上限/过高线留空=不报警，只记录数值</div>
                <div class="imp-tip-line"><SvgIcon name="check-circle" />单次最多 500 行；已有编码不会被覆盖，会提示「编码已存在」</div>
              </div>
              <div class="imp-actions">
                <button class="btn" @click="downloadImportTemplate"><SvgIcon name="download" />下载 CSV 模板</button>
                <label class="btn btn-primary" style="margin:0"><SvgIcon name="upload" />选择 CSV 文件
                  <input type="file" accept=".csv,text/csv" style="display:none" @change="onImportFile" />
                </label>
              </div>
              <div v-if="imp.parseErr" class="imp-err">{{ imp.parseErr }}</div>
            </div>
          </template>

          <!-- Step 2: 预览+试校验 -->
          <template v-if="imp.step===2">
            <div class="imp-sum">
              共解析 <b>{{ imp.rows.length }}</b> 行，试校验：<span class="ok-b">可导入 {{ imp.preOk }}</span> /
              <span class="bad-b">有问题 {{ imp.preBad }}</span>
              <span class="muted small" style="margin-left:auto">有问题的行不会导入，修正后可重新上传</span>
            </div>
            <div class="imp-table-wrap">
              <table class="data-table imp-table">
                <thead><tr><th>行号</th><th>编码</th><th>名称</th><th>站点</th><th>类型</th><th>协议</th><th style="min-width:280px">校验</th></tr></thead>
                <tbody>
                  <tr v-for="r in imp.preview" :key="r.row_no" :class="{'imp-row-bad': r.errs && r.errs.length}">
                    <td>{{ r.row_no }}</td><td class="mono">{{ r.code }}</td><td>{{ r.name }}</td>
                    <td>{{ r.site_name }}</td><td>{{ typeCn(r.type) }}</td><td>{{ PROTO[r.protocol] || r.protocol }}</td>
                    <td>
                      <span v-if="!r.errs || !r.errs.length" class="ok-b"><SvgIcon name="check-circle" />可导入</span>
                      <div v-else><div v-for="(e,i) in r.errs" :key="i" class="bad-b imp-errline">{{ e }}</div></div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div class="modal-actions">
              <button class="btn" @click="imp.step=1">上一步</button>
              <button class="btn btn-primary" :disabled="imp.preOk===0 || imp.busy" @click="doImport(false)">
                {{ imp.busy ? '导入中...' : ('确认导入 ' + imp.preOk + ' 行') }}
              </button>
            </div>
          </template>

          <!-- Step 3: 结果 -->
          <template v-if="imp.step===3">
            <div class="imp-result">
              <div class="imp-result-big">
                <SvgIcon name="check-circle" /> 成功导入 <b>{{ imp.result.created }}</b> 个测点
                <span v-if="imp.result.failed" class="bad-b">，{{ imp.result.failed }} 行未导入（问题见下方）</span>
              </div>
              <div v-if="imp.result.errors && imp.result.errors.length" class="imp-table-wrap">
                <table class="data-table imp-table">
                  <thead><tr><th>行号</th><th>编码</th><th>名称</th><th>问题</th></tr></thead>
                  <tbody>
                    <tr v-for="(e,i) in imp.result.errors" :key="i" class="imp-row-bad">
                      <td>{{ e.row_no }}</td><td class="mono">{{ e.code || '-' }}</td><td>{{ e.name || '-' }}</td>
                      <td><div v-for="(m,j) in e.errors" :key="j" class="bad-b imp-errline">{{ m }}</div></td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="modal-actions">
                <button class="btn" @click="imp.show=false;imp.step=1;load();loadStat()">完成</button>
              </div>
            </div>
          </template>
        </div>
        </div>
      </div>
    </div>

    <!-- 模板选择器（V57 抽屉化） -->
    <div v-if="tpl.show" class="drawer-mask" @click.self="tpl.show=false">
      <div class="drawer" style="width:680px">
        <div class="drawer-head">
          <h3><SvgIcon name="clipboard" />选择测点模板</h3>
          <button class="btn btn-mini" @click="tpl.show=false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
        <div class="tpl-grid">
          <div class="tpl-item" v-for="t in TEMPLATES" :key="t.code" @click="applyTemplate(t)">
            <div class="tpl-icon">{{ t.icon }}</div>
            <div class="tpl-name">{{ t.name }}</div>
            <div class="tpl-desc">{{ t.desc }}</div>
            <div class="tpl-proto">{{ PROTO[t.protocol] }}</div>
          </div>
        </div>
        </div>
        <div class="drawer-foot"><button class="btn" @click="tpl.show=false">关闭</button></div>
      </div>
    </div>

    <!-- 添加/编辑侧滑面板 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false">
      <div class="drawer">
        <div class="drawer-head">
          <h3>{{ form.id ? '编辑测点' : '添加测点' }}</h3>
          <button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
          <!-- 基本信息 -->
          <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend>
            <div class="form-grid">
              <label>类型 *
                <select class="input" v-model="form.type" @change="onTypeChange">
                  <option v-for="(n, k) in TYPES" :key="k" :value="k">{{ n }}</option>
                </select>
              </label>
              <label>所属站点 *
                <select class="input" v-model.number="form.site_id">
                  <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
                </select>
              </label>
              <label>编码 *<input class="input" v-model="form.code" :placeholder="autoCode" /></label>
              <label>名称 *<input class="input" v-model="form.name" :placeholder="autoName" /></label>
              <label>所属房间
                <select class="input" v-model.number="form.room_id">
                  <option :value="0">（未指定）</option>
                  <option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option>
                </select>
              </label>
              <label>所属机柜
                <select class="input" v-model.number="form.cabinet_id">
                  <option :value="0">（未指定）</option>
                  <option v-for="c in cabinets" :key="c.id" :value="c.id">{{ c.name }}</option>
                </select>
              </label>
              <label class="full">绑定网络设备<i class="tip" data-tip="这个测点监测的是哪台网络设备（交换机/路由器/防火墙等）。&#10;绑定后可在设备详情里看到该设备的全部测点与实时读数，&#10;SNMP 采集类测点建议必填。选「未绑定」表示与网络设备无关。">?</i>
                <select class="input" v-model.number="form.network_device_id">
                  <option :value="0">（未绑定）</option>
                  <option v-for="nd in netdevs" :key="nd.id" :value="nd.id">{{ nd.name }}（{{ nd.code }}）</option>
                </select>
              </label>
              <label>数据格式 *<i class="tip" data-tip="这个测点采集的是什么形式的数：&#10;· 浮点数=带小数的数值（电压220.5、温度25.3），最常用&#10;· 整数=不带小数的数值（台数、次数）&#10;· 布尔值=只有开/关两种状态（漏水告警、烟感告警）&#10;· 字符串=文字内容（设备状态描述）">?</i>
                <select class="input" v-model="form.data_type">
                  <option value="float">浮点数（带小数的数值）</option>
                  <option value="int">整数</option>
                  <option value="bool">布尔值（开/关）</option>
                  <option value="string">字符串（文字）</option>
                </select>
              </label>
              <label>单位<i class="tip" data-tip="数值的单位，显示在页面上。常用：V电压 A电流 kW功率 °C温度 %RH湿度 ppm浓度">?</i>
                <input class="input" v-model="form.unit" placeholder="V / A / kW / °C / %RH / ppm" />
              </label>
              <label class="full">描述<input class="input" v-model="form.description" /></label>
            </div>
          </fieldset>

          <!-- 协议配置 -->
          <fieldset class="fs"><legend><SvgIcon name="plug" />协议配置</legend>
            <!-- V45: 协议选择卡片化（带白话说明），替代裸下拉 -->
            <div class="proto-cards">
              <div class="pcard" :class="{on: form.protocol==='modbus'}" @click="setProto('modbus')">
                <b>Modbus</b>
                <span>工业设备最常用。UPS、精密空调、PDU、传感器等，走串口线或网线对接。不确定设备协议时，先查设备说明书有没有写 Modbus</span>
              </div>
              <div class="pcard" :class="{on: form.protocol==='snmp'}" @click="setProto('snmp')">
                <b>SNMP</b>
                <span>网络设备标配。服务器、交换机、网络型空调大多支持，只要能 ping 通设备 IP 就能采集</span>
              </div>
              <div class="pcard" :class="{on: form.protocol==='mqtt'}" @click="setProto('mqtt')">
                <b>MQTT</b>
                <span>物联网消息订阅。设备主动把数据"发"到消息服务器，平台从里面订阅读取，适合无线传感器、智能网关</span>
              </div>
              <div class="pcard" :class="{on: form.protocol==='dio'}" @click="setProto('dio')">
                <b>网关DI</b>
                <span>采集网关自带的开关量输入口（XM-E16 有 12 路）。漏水绳、烟感、门磁等干接点信号直接接线即可，无需设备地址</span>
              </div>
              <div class="pcard" :class="{on: form.protocol==='manual'}" @click="setProto('manual')">
                <b>手动录入</b>
                <span>设备无法自动对接时选这个。数据由人工定期在页面上填写，如老式仪表抄表</span>
              </div>
            </div>
            <div class="form-grid">

              <!-- Modbus 配置 -->
              <template v-if="form.protocol === 'modbus'">
                <label>从站地址<i class="tip" data-tip="设备的「编号」。一台采集网关下可挂多台设备，每台设备有一个唯一编号（1-247），设备说明书或面板上能找到，最常见是 1">?</i>
                  <input class="input" type="number" v-model.number="pcfg.slave_id" min="1" max="247" />
                </label>
                <label>寄存器类型<i class="tip" data-tip="数据存放的「仓库类型」：&#10;· 输入寄存器(3x)=只读数据，温湿度/电压等传感器值大多是这种&#10;· 保持寄存器(4x)=可读可写，设置类参数在这种&#10;· 线圈(0x)/离散输入(1x)=开关量，只有「开/关」两种状态，如漏水告警、烟感告警&#10;对照设备点表(寄存器表)选择，不确定就选输入寄存器">?</i>
                  <select class="input" v-model="pcfg.reg_type">
                    <option value="holding">保持寄存器 (4x)</option>
                    <option value="input">输入寄存器 (3x)</option>
                    <option value="coil">线圈 (0x)</option>
                    <option value="discrete">离散输入 (1x)</option>
                  </select>
                </label>
                <label>寄存器地址<i class="tip" data-tip="数据在设备里的「门牌号」，从设备点表(寄存器地址表)里查。如点表写「输出电压 30100」，这里填 100 或 30100（按设备文档口径），填错会采不到数据">?</i>
                  <input class="input" type="number" v-model.number="pcfg.reg_addr" min="0" />
                </label>
                <label>字节序<i class="tip" data-tip="多字节数值的「拼装顺序」，不同厂家习惯不同。默认选 AB (大端)；如果采到的数值明显不对(如温度显示几百度)，改成 BA 或 CDAB 试一下">?</i>
                  <select class="input" v-model="pcfg.byte_order">
                    <option value="ab">AB (大端)</option>
                    <option value="ba">BA (小端)</option>
                    <option value="abcd">ABCD</option>
                    <option value="cdab">CDAB</option>
                  </select>
                </label>
                <label>缩放系数<i class="tip" data-tip="设备传回的原始值往往放大过：如实际 220.5V 传 2205，缩放系数就填 0.1（实际值=原始值×系数）。点表上通常标注「精度0.1」或「×0.1」。不确定填 1">?</i>
                  <input class="input" type="number" step="0.001" v-model.number="form.scale_factor" />
                </label>
                <label>连接方式<i class="tip" data-tip="· TCP(网络)=设备插网线/有IP地址，推荐，布线方便&#10;· RTU(串口)=设备走 RS485/RS232 串口线，需要串口服务器或网关转接">?</i>
                  <select class="input" v-model="pcfg.conn_type">
                    <option value="rtu">RTU (串口)</option>
                    <option value="tcp">TCP (网络)</option>
                  </select>
                </label>
                <label v-if="pcfg.conn_type==='rtu'">串口号<i class="tip" data-tip="网关/服务器上串口的名称。XM-E16 采集网关 16 路串口对应 /dev/com1 ~ /dev/com16（面板 COM 几就填几）；普通服务器 USB 转串口一般是 /dev/ttyUSB0，主板自带一般是 /dev/ttyS0">?</i>
                  <input class="input" v-model="pcfg.port" placeholder="/dev/com1" list="port-list" />
                  <datalist id="port-list"><option v-for="n in 16" :key="n" :value="'/dev/com'+n" /></datalist>
                </label>
                <label v-if="pcfg.conn_type==='rtu'">波特率<i class="tip" data-tip="串口通信速度，两端必须一致，否则采到乱码。设备说明书里找，9600 最常见">?</i>
                  <select class="input" v-model="pcfg.baud_rate">
                    <option value="9600">9600</option><option value="19200">19200</option>
                    <option value="38400">38400</option><option value="115200">115200</option>
                  </select>
                </label>
                <label v-if="pcfg.conn_type==='tcp'">设备 IP:端口<i class="tip" data-tip="设备的网络地址。IP 问网络管理员，端口 Modbus 标准是 502。例：192.168.1.100:502">?</i>
                  <input class="input" v-model="pcfg.host" placeholder="192.168.1.100:502" />
                </label>
              </template>

              <!-- SNMP 配置 -->
              <template v-if="form.protocol === 'snmp'">
                <label>SNMP 版本<i class="tip" data-tip="协议版本，选 v2c（现在最通用）。设备上如果只开了 v1 就选 v1">?</i>
                  <select class="input" v-model="pcfg.snmp_ver">
                    <option value="v1">v1</option><option value="v2c">v2c</option>
                  </select>
                </label>
                <label>Community（团体名）<i class="tip" data-tip="相当于 SNMP 的「只读密码」。默认是 public，很多设备出厂就是这个；如果被网管改过，向网络管理员要">?</i>
                  <input class="input" v-model="pcfg.community" placeholder="public" />
                </label>
                <label class="full">OID（数据编号）<i class="tip" data-tip="设备里每个指标都有一个全球唯一的编号串，就是 OID。不用自己编：&#10;· 温度常用: .1.3.6.1.4.1.2021.10.1.3.1&#10;· 其他设备在厂家 MIB 文档里查，或把设备型号发平台管理员帮你查">?</i>
                  <input class="input" v-model="pcfg.oid" placeholder=".1.3.6.1.4.1.2021.10.1.3.1" />
                  <span v-if="oidHint" class="field-hint">{{ oidHint }}</span>
                </label>
                <label>目标主机 IP<i class="tip" data-tip="被采集设备的 IP 地址，能 ping 通即可">?</i>
                  <input class="input" v-model="pcfg.host" placeholder="192.168.1.100" />
                </label>
                <label>端口<i class="tip" data-tip="SNMP 标准端口是 161，一般不用改">?</i>
                  <input class="input" type="number" v-model.number="pcfg.port" placeholder="161" />
                </label>
              </template>

              <!-- MQTT 配置 -->
              <template v-if="form.protocol === 'mqtt'">
                <label class="full">Topic（消息主题）<i class="tip" data-tip="相当于消息的「频道名」，设备往哪个频道发数据就订哪个。由设备配置人员提供，例：dcim/site01/ups/voltage。支持 / 分层，填错会收不到数据">?</i>
                  <input class="input" v-model="pcfg.topic" placeholder="dcim/site/bj01/ups/voltage" />
                </label>
                <label>QoS（投递保障）<i class="tip" data-tip="消息可靠等级：&#10;· 0 = 发了就不管（最快，可能丢）&#10;· 1 = 至少送达一次（推荐，日常够用）&#10;· 2 = 精确一次（最可靠但最慢，计费类数据用）">?</i>
                  <select class="input" v-model.number="pcfg.qos">
                    <option :value="0">0 (最多一次)</option>
                    <option :value="1">1 (至少一次)</option>
                    <option :value="2">2 (仅一次)</option>
                  </select>
                </label>
                <label>Broker 地址<i class="tip" data-tip="MQTT 消息服务器的地址（IP:端口），问部署人员，默认端口 1883">?</i>
                  <input class="input" v-model="pcfg.host" placeholder="192.168.1.100:1883" />
                </label>
                <label class="full">JSON 路径<i class="tip" data-tip="设备发的消息是一段 JSON 文本，这里告诉平台「取哪个字段当数值」。&#10;例：消息是 {&quot;data&quot;:{&quot;voltage&quot;:220}} → 填 $.data.voltage&#10;消息就是一个数或 {&quot;value&quot;:220} → 填 $.value">?</i>
                  <input class="input" v-model="pcfg.json_path" placeholder="$.data.voltage" />
                </label>
                <label>用户名<i class="tip" data-tip="Broker 开了账号密码才需要填，问部署人员；没开就留空">?</i>
                  <input class="input" v-model="pcfg.username" />
                </label>
                <label>密码<i class="tip" data-tip="同上，Broker 的登录密码">?</i>
                  <input class="input" type="password" v-model="pcfg.password" />
                </label>
              </template>

              <!-- V47: 网关DI 开关量输入（XM-E16 12路） -->
              <template v-if="form.protocol === 'dio'">
                <label>DI 通道<i class="tip" data-tip="网关面板上接线的 DI 端子编号。XM-E16 有 12 路 DI（DI1~DI12），接在哪路就选哪路。干接点设备（漏水绳/烟感/门磁）一根线接 DI 端子、一根接 GND">?</i>
                  <select class="input" v-model.number="pcfg.di_bit">
                    <option v-for="n in 12" :key="n" :value="n-1">DI{{ n }}（第{{ n }}路）</option>
                  </select>
                </label>
                <label>信号含义<i class="tip" data-tip="· 闭合=报警（默认）：设备触发时端子短接，如漏水绳遇水、烟感报警、门被打开&#10;· 闭合=正常：反逻辑设备用，如「门磁闭合=门关好」想反过来报「门开」">?</i>
                  <select class="input" v-model="pcfg.di_invert">
                    <option value="0">闭合 = 报警（默认，漏水/烟感/门开）</option>
                    <option value="1">闭合 = 正常（反逻辑）</option>
                  </select>
                </label>
                <div class="full field-hint" style="margin:2px 0 0">采集方式：网关每 {{ form.poll_interval || 5 }} 秒读一次 DI 位图；建议把采集间隔设为 5 秒、数据类型为开关量，并用告警规则「值 = 1 时报警」</div>
              </template>

              <!-- 手动模式 -->
              <template v-if="form.protocol === 'manual'">
                <label>数据来源<i class="tip" data-tip="· 人工录入 = 值班人员定期在测点页面填数值（如老式仪表抄表）&#10;· 文件导入 = 用 Excel/CSV 批量导入&#10;· API 推送 = 第三方系统调接口写入">?</i>
                  <select class="input" v-model="pcfg.source">
                    <option value="entry">人工录入</option>
                    <option value="file">文件导入</option>
                    <option value="api">API 推送</option>
                  </select>
                </label>
                <label class="full">备注<input class="input" v-model="pcfg.note" placeholder="手动录入说明，如：每天8点抄录UPS面板电压" /></label>
              </template>

              <label class="full">采集地址（自动生成，不用填）
                <input class="input" v-model="computedAddress" readonly style="background:#f0f0f0" />
                <span class="field-hint" v-if="addressHint">{{ addressHint }}</span>
              </label>
            </div>
          </fieldset>

          <!-- 阈值与高级设置 -->
          <fieldset class="fs"><legend><SvgIcon name="gear" />阈值与告警设置</legend>
            <div class="threshold-explain" v-if="thresholdExplain">{{ thresholdExplain }}</div>
            <div class="form-grid">
              <label>采集间隔<i class="tip" data-tip="多久自动采一次数据。越短越实时但设备/网络负担越大：&#10;· 漏水/烟感等安全类 → 5-10秒（要快）&#10;· 电压/电流/温湿度 → 10-60秒&#10;· 抄表类不重要的 → 5分钟以上">?</i>
                <select class="input" v-model.number="form.poll_interval">
                  <option :value="5">5 秒（安全类：漏水/烟感）</option>
                  <option :value="10">10 秒（重要：电压/电流）</option>
                  <option :value="30">30 秒（常规：温湿度）</option>
                  <option :value="60">1 分钟</option>
                  <option :value="300">5 分钟</option>
                  <option :value="600">10 分钟</option>
                  <option :value="3600">1 小时（抄表类）</option>
                </select>
              </label>
              <label>告警延迟<i class="tip" data-tip="数值连续超标多久才真正告警，防止瞬间毛刺误报。例：填 60 = 超标持续1分钟才告警。0 = 超标立即告警">?</i>
                <input class="input" type="number" v-model.number="form.alarm_delay" placeholder="连续超阈值N秒才告警" />
              </label>
              <label>下限（低于报警）<i class="tip" data-tip="数值低于这条线就告警。例：机房温度下限填 10 = 低于10°C报警（可能空调坏了太冷/结露风险）">?</i>
                <input class="input" type="number" step="0.01" v-model.number="form.min_threshold" />
              </label>
              <label>上限（高于报警）<i class="tip" data-tip="数值高于这条线就告警。例：机房温度上限填 35 = 高于35°C报警（制冷不足）">?</i>
                <input class="input" type="number" step="0.01" v-model.number="form.max_threshold" />
              </label>
              <label>过高线（严重级）<i class="tip" data-tip="比上限更严重的红线：超过直接触发【严重】告警并自动升级。例：上限35、过高线40 = 35°C报警提醒，40°C就是严重事故级">?</i>
                <input class="input" type="number" step="0.01" v-model.number="form.high_threshold" placeholder="超过即触发严重告警" />
              </label>
            </div>
          </fieldset>
        </div>
        <div class="drawer-foot">
          <button class="btn" @click="form.show=false">取消</button>
          <button class="btn btn-primary" @click="save" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
const route = useRoute()
import * as echarts from 'echarts'
import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
import Pagination from '../components/Pagination.vue'

const TYPES = {
  power_dist: '配电', ups: 'UPS', hvac: '空调',
  temp_humidity: '温湿度', water_leak: '漏水', smoke: '烟感',
  cdu: '液冷CDU', leak_rope: '漏液绳'
}
const PTST = { 0: '离线', 1: '正常', 2: '告警', 3: '故障' }
const PROTO = { modbus: 'Modbus', snmp: 'SNMP', mqtt: 'MQTT', dio: '网关DI', manual: '手动' }
const SEV = { 1: '提示', 2: '警告', 3: '严重' }
const AST = { 0: '待处理', 1: '已挂起', 2: '已确认', 3: '已结束' }
const OPNAME = { gt: '＞', lt: '＜', eq: '＝', ne: '≠' }

// 预置模板
const TEMPLATES = [
  { icon:'', code:'TMP-UPS-V', name:'UPS 输出电压', desc:'单相/三相 UPS 输出电压监测', type:'ups', protocol:'modbus', data_type:'float', unit:'V', reg_type:'input', reg_addr:100, scale_factor:0.1, min_threshold:200, max_threshold:240, poll_interval:10 },
  { icon:'', code:'TMP-UPS-L', name:'UPS 负载率', desc:'UPS 负载百分比', type:'ups', protocol:'modbus', data_type:'float', unit:'%', reg_type:'input', reg_addr:102, scale_factor:0.1, min_threshold:0, max_threshold:80, poll_interval:10 },
  { icon:'', code:'TMP-TEMP', name:'机柜温度', desc:'机柜内温度传感器', type:'temp_humidity', protocol:'modbus', data_type:'float', unit:'°C', reg_type:'input', reg_addr:200, scale_factor:0.1, min_threshold:10, max_threshold:35, poll_interval:30 },
  { icon:'', code:'TMP-HUM', name:'机柜湿度', desc:'机柜内湿度传感器', type:'temp_humidity', protocol:'modbus', data_type:'float', unit:'%RH', reg_type:'input', reg_addr:202, scale_factor:0.1, min_threshold:20, max_threshold:80, poll_interval:30 },
  { icon:'', code:'TMP-HVAC', name:'空调回风温度', desc:'精密空调回风温度', type:'hvac', protocol:'snmp', data_type:'float', unit:'°C', community:'public', oid:'.1.3.6.1.4.1.2021.10.1.3.1', min_threshold:18, max_threshold:28, poll_interval:30 },
  { icon:'', code:'TMP-PDU', name:'PDU 电流', desc:'智能PDU每相电流', type:'power_dist', protocol:'snmp', data_type:'float', unit:'A', community:'public', oid:'.1.3.6.1.4.1.13742.6.5.2.3.1.4', min_threshold:0, max_threshold:32, poll_interval:10 },
  { icon:'', code:'TMP-LEAK', name:'漏水检测', desc:'漏水绳/漏水检测器', type:'water_leak', protocol:'modbus', data_type:'bool', unit:'', reg_type:'discrete', reg_addr:1, scale_factor:1, poll_interval:5 },
  { icon:'', code:'TMP-SMOKE', name:'烟感探测器', desc:'烟雾浓度传感器', type:'smoke', protocol:'modbus', data_type:'float', unit:'ppm', reg_type:'input', reg_addr:300, scale_factor:1, min_threshold:0, max_threshold:5, poll_interval:5 },
  { icon:'', code:'TMP-MQTT-T', name:'MQTT 温度', desc:'通过 MQTT 订阅的温度传感器', type:'temp_humidity', protocol:'mqtt', data_type:'float', unit:'°C', topic:'sensors/temperature', qos:1, json_path:'$.value', min_threshold:15, max_threshold:30, poll_interval:60 },
  { icon:'', code:'TMP-CDU-ST', name:'CDU 供液温度', desc:'冷量分配单元供液温度（冷板式液冷）', type:'cdu', protocol:'modbus', data_type:'float', unit:'°C', reg_type:'input', reg_addr:400, scale_factor:0.1, min_threshold:15, max_threshold:32, poll_interval:10 },
  { icon:'', code:'TMP-CDU-RT', name:'CDU 回液温度', desc:'冷量分配单元回液温度', type:'cdu', protocol:'modbus', data_type:'float', unit:'°C', reg_type:'input', reg_addr:402, scale_factor:0.1, min_threshold:20, max_threshold:45, poll_interval:10 },
  { icon:'', code:'TMP-CDU-FL', name:'CDU 冷却液流量', desc:'一次侧/二次侧冷却液流量', type:'cdu', protocol:'modbus', data_type:'float', unit:'L/min', reg_type:'input', reg_addr:404, scale_factor:0.1, min_threshold:10, max_threshold:120, poll_interval:10 },
  { icon:'', code:'TMP-CDU-PR', name:'CDU 管路压力', desc:'二次侧冷却液管路压力', type:'cdu', protocol:'modbus', data_type:'float', unit:'MPa', reg_type:'input', reg_addr:406, scale_factor:0.001, min_threshold:0.1, max_threshold:0.6, poll_interval:10 },
  { icon:'', code:'TMP-LROPE', name:'漏液绳', desc:'绳式漏液检测（沿冷却管路敷设，区别于点式漏水）', type:'leak_rope', protocol:'modbus', data_type:'bool', unit:'', reg_type:'discrete', reg_addr:2, scale_factor:1, poll_interval:5 },
]

const sites = ref([]), rooms = ref([]), cabinets = ref([]), list = ref([])
const netdevs = ref([]) // V66 B2-1: 可绑定网络设备
const netdevName = id => { const nd = netdevs.value.find(x => x.id === id); return nd ? `${nd.name}（${nd.code}）` : `设备#${id}` }
const page = ref(1), total = ref(0), size = ref(20), saving = ref(false)

// V15 实时刷新 + 在线率统计 + 批量选择
const selIds = ref([])
const liveOn = ref(true)
let liveTimer = null
const stat = reactive({ total: 0, normal: 0, alert: 0, offline: 0, fault: 0, onlinePct: 0 })
const pageAllChecked = computed(() => list.value.length > 0 && list.value.every(p => selIds.value.includes(p.id)))
function togglePageAll(e) {
  if (e.target.checked) {
    const ids = list.value.map(p => p.id)
    selIds.value = [...new Set([...selIds.value, ...ids])]
  } else {
    const pageIds = list.value.map(p => p.id)
    selIds.value = selIds.value.filter(id => !pageIds.includes(id))
  }
}
async function loadStat() {
  const r = await api.get('/monitoring-summary')
  if (r.code !== 0) return
  const d = r.data || {}
  let total = 0, normal = 0, alert = 0, offline = 0, fault = 0
  for (const k of ['power_dist','ups','hvac','temp_humidity','water_leak','smoke','cdu','leak_rope']) {
    const c = d[k] || {}
    total += c.total || 0; normal += c.normal || 0; alert += c.alert || 0; offline += c.offline || 0
    fault += Math.max(0, (c.total || 0) - (c.normal || 0) - (c.alert || 0) - (c.offline || 0))
  }
  stat.total = total; stat.normal = normal; stat.alert = alert; stat.offline = offline; stat.fault = fault
  stat.onlinePct = total > 0 ? (normal + alert) / total * 100 : 0
}
async function refreshLive() {
  // 只刷新当前页测点的实时值，避免整表重载
  if (!list.value.length) return
  const ids = list.value.map(p => p.id).join(',')
  const r = await api.get('/points-live', { ids })
  if (r.code !== 0) return
  const m = {}
  for (const it of (r.data?.items || [])) m[it.id] = it
  for (const p of list.value) {
    if (m[p.id]) { p.current_value = m[p.id].current_value; p.status = m[p.id].status }
  }
}
function toggleLive() {
  liveOn.value = !liveOn.value
  if (liveOn.value) startLive(); else stopLive()
}
function startLive() {
  stopLive()
  liveTimer = setInterval(() => { refreshLive(); loadStat() }, 30000)
}
function stopLive() { if (liveTimer) { clearInterval(liveTimer); liveTimer = null } }
async function batchOp(action) {
  const labels = { enable: '启用', disable: '停用', delete: '删除' }
  if (!(await showConfirm({ message: `确认${labels[action]}选中的 ${selIds.value.length} 个测点？${action==='delete'?'\n（删除将同时移除其读数、告警、关联规则）':''}` }))) return
  const r = await api.post('/monitoring-points/batch', { ids: selIds.value, action })
  if (r.code === 0) { selIds.value = []; await load(); await loadStat() }
  else toastAuto(r.msg)
}
const f = reactive({ site_id: 0, type: '', status: '', protocol: '', keyword: '', cabinet_id: 0 })
const detail = reactive({ show: false, p: null, readings: [], rules: [], alerts: [] })
const trendEl = ref(null)
let trendChart = null
const siteName = id => (sites.value.find(s => s.id === id) || {}).name || '-'
const roomName = id => id ? ((rooms.value.find(r => r.id === id) || {}).name || id) : '-'
const form = reactive({ show: false, id: 0, code: '', name: '', type: 'temp_humidity', site_id: 0, room_id: 0, cabinet_id: 0, network_device_id: 0, protocol: 'modbus', address: '', data_type: 'float', unit: '', min_threshold: null, max_threshold: null, high_threshold: null, scale_factor: 1, poll_interval: 30, alarm_delay: 0, protocol_config: '{}', description: '' })
const pcfg = reactive({ slave_id: 1, reg_type: 'holding', reg_addr: 0, byte_order: 'ab', conn_type: 'tcp', port: '', baud_rate: '9600', host: '', snmp_ver: 'v2c', community: 'public', oid: '', topic: '', qos: 1, json_path: '$.value', username: '', password: '', source: 'entry', note: '', di_bit: 0, di_invert: '0' })
const tpl = reactive({ show: false })

// ==================== V46: 批量导入向导 ====================
const imp = reactive({ show: false, step: 1, rows: [], preview: [], preOk: 0, preBad: 0, parseErr: '', busy: false, result: null })

const IMP_HEAD = ['站点名称*','机房名称','测点编码*','测点名称*','类型','协议','单位','设备IP:端口','从站地址','寄存器类型','寄存器地址','缩放系数','SNMP OID','MQTT Topic','数据类型','下限报警值','上限报警值','过高线(严重)','采集间隔秒','备注']
const IMP_SAMPLE = [
  ['示例数据中心','A栋1层机房','A1-TEMP-01','A1机柜温度','温湿度','Modbus','°C','10.1.2.3:502','3','输入寄存器','200','0.1','','','浮点数','10','35','40','30','机柜中部温度传感器'],
  ['示例数据中心','A栋1层机房','A1-LEAK-01','A1漏水检测','漏水','Modbus','','10.1.2.3:502','3','离散输入','1','1','','','开关','','','','5','漏水绳'],
  ['示例数据中心','','A1-HVAC-01','空调回风温度','空调','SNMP','°C','10.1.2.5','','','','','.1.3.6.1.4.1.2021.10.1.3.1','','浮点数','18','28','','30',''],
]

function openImport() {
  imp.show = true; imp.step = 1; imp.rows = []; imp.preview = []; imp.parseErr = ''; imp.result = null
}
function downloadImportTemplate() {
  const lines = [IMP_HEAD.map(csvCell).join(','), ...IMP_SAMPLE.map(r => r.map(csvCell).join(','))]
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const el = document.createElement('a')
  el.href = URL.createObjectURL(blob)
  el.download = '测点导入模板.csv'
  el.click(); URL.revokeObjectURL(el.href)
}
function csvCell(v) { return '"' + String(v ?? '').replace(/"/g, '""') + '"' }

// 解析 CSV（支持引号包裹、内嵌逗号、双引号转义、\r\n）
function parseCSV(text) {
  text = text.replace(/^\ufeff/, '')
  const rows = []; let cur = [], cell = '', inQ = false
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (inQ) {
      if (ch === '"') { if (text[i+1] === '"') { cell += '"'; i++ } else inQ = false }
      else cell += ch
    } else {
      if (ch === '"') inQ = true
      else if (ch === ',') { cur.push(cell); cell = '' }
      else if (ch === '\n') { cur.push(cell); rows.push(cur); cur = []; cell = '' }
      else if (ch === '\r') { /* skip */ }
      else cell += ch
    }
  }
  if (cell !== '' || cur.length) { cur.push(cell); rows.push(cur) }
  return rows.filter(r => r.some(c => String(c).trim() !== ''))
}

async function onImportFile(e) {
  imp.parseErr = ''
  const f = e.target.files[0]
  e.target.value = ''
  if (!f) return
  if (!/\.csv$/i.test(f.name)) { imp.parseErr = '请选择 .csv 文件（Excel 里「另存为」选 CSV 格式即可）'; return }
  const text = await f.text()
  const rows = parseCSV(text)
  if (rows.length < 2) { imp.parseErr = '文件里没有数据行（至少要有表头+1行数据）'; return }
  // 表头识别：宽松匹配（去 * 号、去空格），支持乱序列
  const head = rows[0].map(x => String(x).replace(/[*＊\s]/g, ''))
  const colIdx = {}
  const headMap = {
    '站点名称': 'site_name', '站点': 'site_name',
    '机房名称': 'room_name', '机房': 'room_name',
    '测点编码': 'code', '编码': 'code',
    '测点名称': 'name', '名称': 'name',
    '类型': 'type', '测点类型': 'type',
    '协议': 'protocol',
    '单位': 'unit',
    '设备IP:端口': 'host', '设备IP': 'host', 'IP': 'host', 'IP:端口': 'host',
    '从站地址': 'slave_id', '从站': 'slave_id',
    '寄存器类型': 'reg_type',
    '寄存器地址': 'reg_addr',
    '缩放系数': 'scale_factor', '系数': 'scale_factor',
    'SNMPOID': 'oid', 'OID': 'oid',
    'MQTTTopic': 'topic', 'Topic': 'topic',
    '数据类型': 'data_type',
    '下限报警值': 'min_threshold', '下限': 'min_threshold',
    '上限报警值': 'max_threshold', '上限': 'max_threshold',
    '过高线(严重)': 'high_threshold', '过高线': 'high_threshold',
    '采集间隔秒': 'poll_interval', '采集间隔': 'poll_interval',
    '备注': 'description', '描述': 'description',
  }
  head.forEach((hcell, i) => { if (headMap[hcell]) colIdx[headMap[hcell]] = i })
  const needCols = ['site_name', 'code', 'name']
  const missing = needCols.filter(k => colIdx[k] === undefined)
  if (missing.length) {
    const cnName = { site_name: '站点名称', code: '测点编码', name: '测点名称' }
    imp.parseErr = '表头缺少必需列：' + missing.map(k => '「' + cnName[k] + '」').join('、') + '。请下载最新模板，不要改动表头文字'
    return
  }
  if (rows.length - 1 > 500) { imp.parseErr = '数据行超过500行上限（当前 ' + (rows.length-1) + ' 行），请拆分文件'; return }
  const parsed = []
  for (let i = 1; i < rows.length; i++) {
    const r = rows[i], obj = { row_no: i + 1 }
    for (const [k, idx] of Object.entries(colIdx)) obj[k] = String(r[idx] ?? '').trim()
    parsed.push(obj)
  }
  imp.rows = parsed
  // 试校验（dry_run）
  imp.busy = true
  try {
    const res = await api.post('/monitoring-points/import', { dry_run: true, rows: parsed })
    if (res.code !== 0) { imp.parseErr = res.msg || '校验失败'; imp.busy = false; return }
    const d = res.data
    const errByRow = {}
    ;(d.errors || []).forEach(e => { errByRow[e.row_no] = e.errors })
    imp.preview = parsed.map(r => ({ ...r, errs: errByRow[r.row_no] || [] }))
    imp.preOk = d.created
    imp.preBad = d.failed
    imp.step = 2
  } catch (err) {
    imp.parseErr = '校验请求失败：' + (err.message || err)
  }
  imp.busy = false
}

async function doImport() {
  imp.busy = true
  try {
    const res = await api.post('/monitoring-points/import', { dry_run: false, rows: imp.rows })
    if (res.code !== 0) { toastAuto(res.msg || '导入失败'); return }
    imp.result = res.data
    imp.step = 3
  } catch (err) {
    toastAuto('导入失败：' + (err.message || err))
  }
  imp.busy = false
}

function typeCn(t) { return TYPES[t] || t || '-' }

// ==================== V46: 导出CSV ====================
async function exportPointsCSV() {
  const q = { page: 1, size: 2000 }
  if (f.site_id) q.site_id = f.site_id
  if (f.keyword) q.keyword = f.keyword
  const res = await api.get('/monitoring-points', q)
  const items = res.data?.items || []
  if (!items.length) { toastAuto('当前筛选下无数据可导出'); return }
  const head = ['测点编码','测点名称','类型','站点','机房','协议','数据格式','单位','下限报警值','上限报警值','过高线(严重)','缩放系数','采集间隔秒','状态','配置解析(白话)']
  const lines = [head.map(csvCell).join(',')]
  for (const p of items) {
    const ex = explainPoint(p)
    lines.push([
      p.code, p.name, TYPES[p.type] || p.type,
      siteName(p.site_id), roomName(p.room_id),
      PROTO[p.protocol] || p.protocol,
      { float: '浮点数', int: '整数', bool: '开关量', string: '字符串' }[p.data_type] || p.data_type,
      p.unit || '',
      p.min_threshold ?? '', p.max_threshold ?? '', p.high_threshold ?? '',
      p.scale_factor ?? '', p.poll_interval ?? '',
      PTST[p.status] || '-',
      ex.join(' / '),
    ].map(csvCell).join(','))
  }
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const el = document.createElement('a')
  el.href = URL.createObjectURL(blob)
  el.download = 'dcim测点_' + new Date().toISOString().slice(0, 10) + '.csv'
  el.click(); URL.revokeObjectURL(el.href)
}

const autoCode = computed(() => {
  const pre = form.type ? form.type.substring(0,3).toUpperCase() : 'PT'
  return `${pre}-${form.site_id || 'X'}-001`
})
const autoName = computed(() => {
  if (!form.type) return ''
  const m = { power_dist:'配电监测', ups:'UPS监测', hvac:'空调监测', temp_humidity:'温湿度', water_leak:'漏水检测', smoke:'烟感监测', cdu:'液冷CDU', leak_rope:'漏液绳' }
  return m[form.type] || '测点'
})

const computedAddress = computed(() => {
  switch (form.protocol) {
    case 'modbus': return `modbus://${pcfg.conn_type==='tcp'?pcfg.host||'127.0.0.1:502':pcfg.port||'/dev/ttyUSB0'}/${pcfg.slave_id}/${pcfg.reg_type}/${pcfg.reg_addr}`
    case 'snmp': return `snmp://${pcfg.host||'127.0.0.1'}:${pcfg.port||'161'}/${pcfg.oid||'.1.3.6.1...'}`
    case 'mqtt': return `mqtt://${pcfg.host||'127.0.0.1:1883'}/${pcfg.topic||'#'}`
    case 'dio': return `dio://bit${pcfg.di_bit}`
    default: return 'manual://'
  }
})

const fmt = t => t ? String(t).replace('T', ' ').slice(0, 16) : '-'

// V45: 协议卡片点击切换（等价旧下拉 + onProtoChange）
function setProto(p) {
  if (form.protocol === p) return
  form.protocol = p
  onProtoChange()
}

// V45: 常见 OID 白话注解——非技术人员看到编号串时给出可读说明
const OID_HINTS = [
  { re: /\.1\.3\.6\.1\.4\.1\.2021\.10\.1\.3/, txt: '常用温度 OID（UCD-SNMP 标准）：读取设备温度值' },
  { re: /\.1\.3\.6\.1\.4\.1\.13742/, txt: 'Server Technology PDU（智能电源分配单元）OID 族：读取每相电流等' },
  { re: /\.1\.3\.6\.1\.2\.1\.25\.3\.3\.1\.2/, txt: '主机 CPU 负载标准 OID（HOST-RESOURCES-MIB）' },
  { re: /\.1\.3\.6\.1\.2\.1\.25\.2\.3\.1\.6/, txt: '主机内存使用量标准 OID（HOST-RESOURCES-MIB）' },
  { re: /\.1\.3\.6\.1\.2\.1\.1\./, txt: '系统基础信息 OID（sysDescr/sysUpTime 等，不是数值测点，请确认用途）' },
]
const oidHint = computed(() => {
  const oid = (pcfg.oid || '').trim()
  if (!oid) return ''
  if (!/^\d+(\.\d+)*\.?$/.test(oid.replace(/^\./, ''))) return 'OID 格式不对：应为纯数字加点，如 .1.3.6.1.4.1.2021.10.1.3.1'
  const hit = OID_HINTS.find(h => h.re.test(oid))
  return hit ? hit.txt : ''
})

// V45: 采集地址的白话解读——把生成的协议串翻译成人话
const addressHint = computed(() => {
  const p = form.protocol
  if (p === 'modbus') {
    const tgt = pcfg.conn_type === 'tcp'
      ? (pcfg.host ? `网络方式连设备 ${pcfg.host}` : '还没填设备 IP:端口')
      : (pcfg.port ? `串口方式连 ${pcfg.port}@${pcfg.baud_rate}` : '还没填串口号')
    const regName = { holding:'保持寄存器(可读写)', input:'输入寄存器(只读)', coil:'线圈(开关量)', discrete:'离散输入(开关量)' }[pcfg.reg_type] || pcfg.reg_type
    return `${tgt}，读第 ${pcfg.slave_id} 号设备的${regName}第 ${pcfg.reg_addr} 号`
  }
  if (p === 'snmp') {
    return `从设备 ${pcfg.host || '(未填IP)'} 读取编号 ${pcfg.oid || '(未填OID)'} 的数据`
  }
  if (p === 'mqtt') {
    return `订阅消息服务器 ${pcfg.host || '(未填Broker)'} 的「${pcfg.topic || '(未填Topic)'}」频道，取 ${pcfg.json_path || '$.value'} 字段`
  }
  return '人工录入，无需设备连接'
})

// V45: 测点配置的白话解析（详情抽屉用）——把协议配置翻译成人话
function explainPoint(p) {
  if (!p) return []
  let pc = {}
  try { pc = JSON.parse(p.protocol_config || '{}') } catch {}
  const lines = []
  const tn = TYPES[p.type] || p.type
  if (p.protocol === 'modbus') {
    const conn = pc.conn_type === 'rtu'
      ? `通过串口线（${pc.port || '串口未配置'}，速率 ${pc.baud_rate || 9600}）连接`
      : `通过网线连接设备 ${pc.host || '(未配置IP)'}`
    const regName = { holding:'保持寄存器（可读写的参数区）', input:'输入寄存器（只读的传感器数据区）', coil:'线圈（开关状态）', discrete:'离散输入（开关状态）' }[pc.reg_type] || pc.reg_type
    lines.push(`这是一个【${tn}】类测点，用 Modbus 方式自动采集：${conn}，读取 ${pc.slave_id ?? 1} 号设备${regName}第 ${pc.reg_addr ?? 0} 号数据。`)
    if (p.scale_factor && p.scale_factor !== 1) lines.push(`设备传回的原始数会乘 ${p.scale_factor} 换算成实际值（厂家协议常见做法）。`)
  } else if (p.protocol === 'snmp') {
    lines.push(`这是一个【${tn}】类测点，用 SNMP 方式自动采集：向设备 ${pc.host || '(未配置IP)'} 查询数据编号 ${pc.oid || '(未配置)'}。`)
    const hit = OID_HINTS.find(h => h.re.test(pc.oid || ''))
    if (hit) lines.push(hit.txt + '。')
  } else if (p.protocol === 'mqtt') {
    lines.push(`这是一个【${tn}】类测点，用 MQTT 订阅方式采集：设备主动把数据发到消息服务器 ${pc.host || '(未配置)'} 的「${pc.topic || '(未配置频道)'}」频道，平台订阅后取 ${pc.json_path || '$.value'} 字段的数值。`)
  } else if (p.protocol === 'dio') {
    const bit = pc.bit != null ? pc.bit : (pc.di_bit != null ? pc.di_bit : 0)
    const inv = String(pc.invert || pc.di_invert || '0') === '1'
    lines.push(`这是一个【${tn}】类测点，接在采集网关的 DI${bit + 1}（第 ${bit + 1} 路开关量输入）上：设备是干接点信号（如漏水绳、烟感、门磁），${inv ? '闭合 = 正常，断开 = 报警（反逻辑）' : '闭合 = 报警，平台读到 1 即告警'}。不需要设备地址，接线即采集。`)
  } else {
    const srcName = { entry:'值班人员人工录入', file:'文件批量导入', api:'第三方系统接口推送' }[pc.source] || '人工录入'
    lines.push(`这是一个【${tn}】类测点，数据来源为${srcName}${pc.note ? '（' + pc.note + '）' : ''}，不连接设备。`)
  }
  lines.push(`每 ${p.poll_interval || 30} 秒采集一次；数据类型为${{float:'带小数数值',int:'整数',bool:'开/关状态',string:'文字'}[p.data_type] || p.data_type}${p.unit ? '，单位 ' + p.unit : ''}。`)
  const th = []
  if (p.min_threshold != null) th.push(`低于 ${p.min_threshold}${p.unit||''} 报警`)
  if (p.max_threshold != null) th.push(`高于 ${p.max_threshold}${p.unit||''} 报警`)
  if (p.high_threshold != null) th.push(`高于 ${p.high_threshold}${p.unit||''} 触发严重告警`)
  lines.push(th.length ? `告警规则：${th.join('；')}${p.alarm_delay ? `；持续超标 ${p.alarm_delay} 秒才报警（防瞬间毛刺误报）` : ''}。` : '未设置告警阈值：只记录数值，不会自动报警。')
  return lines
}

// V45: 阈值三线的白话汇总句，随填随变，帮助理解三线关系
const thresholdExplain = computed(() => {
  const u = form.unit ? form.unit : ''
  const parts = []
  if (form.min_threshold != null) parts.push(`低于 ${form.min_threshold}${u} 报警`)
  if (form.max_threshold != null) parts.push(`高于 ${form.max_threshold}${u} 报警`)
  if (form.high_threshold != null) parts.push(`高于 ${form.high_threshold}${u} 严重告警`)
  if (!parts.length) return '未设置阈值 = 该测点只记录数值、不告警（漏水/烟感类建议至少设上限）'
  // 逻辑校验提醒
  const warn = []
  if (form.min_threshold != null && form.max_threshold != null && form.min_threshold >= form.max_threshold) warn.push('下限应小于上限')
  if (form.max_threshold != null && form.high_threshold != null && form.high_threshold <= form.max_threshold) warn.push('过高线应大于上限')
  return '生效规则：' + parts.join('；') + (warn.length ? '。【注意】' + warn.join('；') : '')
})

function onProtoChange() {
  Object.assign(pcfg, { slave_id: 1, reg_type: 'holding', reg_addr: 0, byte_order: 'ab', conn_type: 'tcp', port: '', baud_rate: '9600', host: '', snmp_ver: 'v2c', community: 'public', oid: '', topic: '', qos: 1, json_path: '$.value', username: '', password: '', source: 'entry', note: '', di_bit: 0, di_invert: '0' })
  if (form.protocol === 'modbus') { form.data_type = 'float'; form.scale_factor = 0.1 }
  else if (form.protocol === 'snmp') { form.data_type = 'float'; form.scale_factor = 1 }
  else if (form.protocol === 'mqtt') { form.data_type = 'float'; form.scale_factor = 1 }
  else if (form.protocol === 'dio') { form.data_type = 'bool'; form.scale_factor = 1 }
  else { form.data_type = 'string'; form.scale_factor = 1 }
}

function onTypeChange() {
  const tu = { power_dist: 'A', ups: 'V', hvac: '°C', temp_humidity: '°C', water_leak: '', smoke: 'ppm', cdu: '°C', leak_rope: '' }
  form.unit = tu[form.type] || ''
}

async function load(p) {
  if (p) page.value = p
  const params = { page: page.value, size: size.value }
  if (route.query.id && !f.type && f.status === '') params.id = route.query.id
  if (f.site_id) params.site_id = f.site_id
  if (f.type) params.type = f.type
  if (f.status !== '') params.status = f.status
  if (f.protocol) params.protocol = f.protocol
  if (f.keyword) params.keyword = f.keyword
  if (f.cabinet_id) params.cabinet_id = f.cabinet_id
  const r = await api.get('/monitoring-points', params)
  list.value = r.data?.items || []
  total.value = r.data?.total || 0
}

// 打开测点详情：加载读数/规则/告警并渲染趋势图
async function openDetail(p) {
  detail.p = p
  detail.readings = []
  detail.rules = []
  detail.alerts = []
  detail.show = true
  const since = new Date(Date.now() - 24 * 3600 * 1000).toISOString()
  const [rd, ru, al] = await Promise.all([
    api.get('/readings', { point_id: p.id, start: since }),
    api.get('/alert-rules', { point_id: p.id }),
    api.get('/alerts', { point_id: p.id, size: 10 })
  ])
  detail.readings = rd.data?.items || []
  detail.rules = ru.data?.items || []
  detail.alerts = al.data?.items || []
  await nextTick()
  renderTrend()
}

function renderTrend() {
  if (!trendEl.value) return
  if (trendChart) { trendChart.dispose(); trendChart = null }
  trendChart = echarts.init(trendEl.value)
  const xs = detail.readings.map(r => String(r.timestamp).replace('T', ' ').slice(5, 16))
  const ys = detail.readings.map(r => r.value)
  const marks = []
  if (detail.p?.high_threshold != null) marks.push({ yAxis: detail.p.high_threshold, name: '过高线', lineStyle: { color: '#c62828', type: 'solid', width: 1.5 }, label: { color: '#c62828' } })
  if (detail.p?.max_threshold != null) marks.push({ yAxis: detail.p.max_threshold, name: '超高', lineStyle: { color: '#f56c6c' }, label: { color: '#f56c6c' } })
  if (detail.p?.min_threshold != null) marks.push({ yAxis: detail.p.min_threshold, name: '过低', lineStyle: { color: '#e6a23c' }, label: { color: '#e6a23c' } })
  trendChart.setOption({
    grid: { left: 44, right: 16, top: 20, bottom: 28 },
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: xs, axisLabel: { fontSize: 10 } },
    yAxis: { type: 'value', scale: true, axisLabel: { fontSize: 10 } },
    series: [{
      type: 'line', data: ys, smooth: true, showSymbol: false,
      lineStyle: { width: 2 }, areaStyle: { opacity: 0.12 },
      markLine: marks.length ? { silent: true, symbol: 'none', data: marks, lineStyle: { type: 'dashed' }, label: { fontSize: 10, formatter: '{b}' } } : undefined
    }]
  })
}

function openTemplatePicker() { tpl.show = true }

function applyTemplate(t) {
  tpl.show = false
  Object.assign(form, {
    show: true, id: 0, code: '', name: t.name, type: t.type, site_id: sites.value[0]?.id || 0,
    room_id: 0, cabinet_id: 0, network_device_id: 0, protocol: t.protocol || 'modbus', address: '', data_type: t.data_type || 'float',
    unit: t.unit || '', min_threshold: t.min_threshold ?? null, max_threshold: t.max_threshold ?? null, high_threshold: t.high_threshold ?? null,
    scale_factor: t.scale_factor || 1, poll_interval: t.poll_interval || 30, alarm_delay: 0,
    protocol_config: '{}', description: t.desc || ''
  })
  // 设置协议参数
  if (t.protocol === 'modbus') {
    Object.assign(pcfg, { reg_type: t.reg_type || 'holding', reg_addr: t.reg_addr || 0, slave_id: 1, byte_order: 'ab', conn_type: 'tcp', host: '', port: '', baud_rate: '9600' })
  } else if (t.protocol === 'snmp') {
    Object.assign(pcfg, { snmp_ver: 'v2c', community: t.community || 'public', oid: t.oid || '', host: '', port: '161' })
  } else if (t.protocol === 'mqtt') {
    Object.assign(pcfg, { topic: t.topic || '', qos: t.qos || 1, json_path: t.json_path || '$.value', host: '', username: '', password: '' })
  }
}

function openForm(p) {
  if (p) {
    Object.assign(form, { show: true, id: p.id, code: p.code, name: p.name, type: p.type, site_id: p.site_id, room_id: p.room_id || 0, cabinet_id: p.cabinet_id || 0, network_device_id: p.network_device_id || 0, protocol: p.protocol, address: p.address, data_type: p.data_type || 'float', unit: p.unit, min_threshold: p.min_threshold, max_threshold: p.max_threshold, high_threshold: p.high_threshold ?? null, scale_factor: p.scale_factor || 1, poll_interval: p.poll_interval || 30, alarm_delay: p.alarm_delay || 0, protocol_config: p.protocol_config || '{}', description: p.description })
    try { const pc = JSON.parse(p.protocol_config || '{}'); Object.assign(pcfg, pc) } catch {}
  } else {
    Object.assign(form, { show: true, id: 0, code: '', name: '', type: 'temp_humidity', site_id: sites.value[0]?.id || 0, room_id: 0, cabinet_id: 0, network_device_id: 0, protocol: 'modbus', address: '', data_type: 'float', unit: '°C', min_threshold: null, max_threshold: null, high_threshold: null, scale_factor: 1, poll_interval: 30, alarm_delay: 0, protocol_config: '{}', description: '' })
    onProtoChange()
  }
}

async function save() {
  saving.value = true
  form.address = computedAddress.value
  form.protocol_config = JSON.stringify({ ...pcfg })
  const body = { ...form }
  delete body.show
  body.network_device_id = form.network_device_id || null // 0=未绑定 → null（后端 *uint）
  const r = form.id ? await api.put(`/monitoring-points/${form.id}`, body) : await api.post('/monitoring-points', body)
  saving.value = false
  if (r.code === 0) { form.show = false; load() }
  else toastAuto(r.msg)
}

async function del(p) {
  if (!(await showConfirm({ message: `删除测点「${p.name}」？` }))) return
  const r = await api.del(`/monitoring-points/${p.id}`)
  if (r.code === 0) load()
  else toastAuto(r.msg)
}

// 同组件内 query 变化（如从测点列表页内点击告警链接）也要响应深链
watch(() => route.query, async (q) => {
  if (q.type !== undefined) f.type = q.type || ''
  if (q.status !== undefined && q.status !== '') f.status = Number(q.status)
  else if (q.status === '') f.status = ''
  await load(1)
  if (q.id) {
    const target = list.value.find(x => x.id === Number(q.id))
      || (await api.get('/monitoring-points', { id: q.id })).data?.items?.[0]
    if (target && q.detail === '1') openDetail(target)
  } else {
    detail.show = false
  }
})

onMounted(async () => {
  const [rs, rr, rc, rn] = await Promise.all([
    api.get('/sites', { size: 100 }),
    api.get('/rooms', { size: 200 }),
    api.get('/cabinets', { size: 200 }),
    api.get('/network-devices', { size: 200 })
  ])
  sites.value = rs.data?.items || []
  rooms.value = rr.data?.items || []
  cabinets.value = rc.data?.items || []
  netdevs.value = rn.data?.items || []
  form.site_id = sites.value[0]?.id || 0
  // 深链：?type=xx&status=n 穿透过滤；?id=n&detail=1 直接打开测点详情
  const q = route.query
  if (q.type) f.type = q.type
  if (q.status !== undefined && q.status !== '') f.status = Number(q.status)
  if (q.keyword) f.keyword = q.keyword
  if (q.cabinet_id) f.cabinet_id = Number(q.cabinet_id)
  await load(1)
  await loadStat()
  startLive()
  if (q.id) {
    const target = list.value.find(x => x.id === Number(q.id))
      || (await api.get('/monitoring-points', { id: q.id })).data?.items?.[0]
    if (target && q.detail === '1') openDetail(target)
    else if (target) {
      // 高亮定位所在行
      const idx = list.value.findIndex(x => x.id === target.id)
      if (idx >= 0) document.querySelectorAll('tbody tr')[idx]?.scrollIntoView({ block: 'center' })
    }
  }
})

onBeforeUnmount(() => { stopLive() })
</script>

<style scoped>
.th-high{color:#c62828;font-weight:600}
/* V15 实时标记与统计条 */
.live-dot { width: 8px; height: 8px; border-radius: 50%; background: #909399; display: inline-block; }
.live-dot.live-on { background: #67c23a; animation: pulse-dot 2s infinite; }
@keyframes pulse-dot { 0%,100%{opacity:1} 50%{opacity:.4} }
.stat-bar { display: flex; align-items: center; gap: 10px; background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 10px 16px; margin-bottom: 12px; }
.sb-item { display: flex; flex-direction: column; align-items: center; padding: 2px 14px; border-radius: 8px; cursor: pointer; transition: background .15s; }
.sb-item:hover { background: var(--bg2); }
.sb-item.on { background: rgba(64,158,255,.1); }
.sb-item b { font-size: 18px; color: var(--text); }
.sb-item b.c-ok { color: #67c23a; } .sb-item b.c-warn { color: #e6a23c; } .sb-item b.c-off { color: #909399; }
.sb-item span { font-size: 11px; color: var(--muted); }
.sb-bar { flex: 1; height: 8px; background: var(--bg2); border-radius: 4px; position: relative; margin-left: 12px; min-width: 120px; }
.sb-bar-in { height: 100%; background: #67c23a; border-radius: 4px; transition: width .4s; }
.sb-bar-t { position: absolute; right: 0; top: -18px; font-size: 11px; color: var(--muted); }
/* 详情抽屉 */
.detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px 16px; font-size: 13.5px; }
.detail-grid .full { grid-column: 1 / -1; }
.dl { display: inline-block; min-width: 70px; color: var(--muted); }
.sec-title { margin: 20px 0 10px; font-size: 15px; display: flex; align-items: center; }
.trend-box { width: 100%; height: 220px; }
.mini-list { display: flex; flex-direction: column; gap: 6px; }
.mini-item { display: flex; align-items: center; gap: 10px; font-size: 12.5px; padding: 8px 10px; background: var(--bg2); border-radius: 8px; }
.badge.sev-3 { color: #f56c6c; border-color: rgba(245,108,108,.4); }
.badge.sev-2 { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.sev-1 { color: #409eff; border-color: rgba(64,158,255,.4); }
.badge.ast-0 { color: #f56c6c; border-color: rgba(245,108,108,.4); }
.badge.ast-1 { color: #909399; border-color: rgba(144,147,153,.4); }
.badge.ast-2 { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.ast-3 { color: #67c23a; border-color: rgba(103,194,58,.4); }
.link { color: var(--accent); cursor: pointer; font-weight: 600; }
.link:hover { text-decoration: underline; }
.btn-mini-primary { color: var(--accent); border-color: var(--accent); }
.row-active td { background: rgba(64,158,255,.06); }
/* 表状态 */
.badge.ptst-1 { color: #67c23a; border-color: rgba(103,194,58,.4); }
.badge.ptst-2 { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.ptst-0 { color: #909399; border-color: rgba(144,147,153,.4); }
.badge.ptst-3 { color: #f56c6c; border-color: rgba(245,108,108,.4); }
/* 协议 */
.badge.proto-modbus { color: #409eff; border-color: rgba(64,158,255,.4); }
.badge.proto-snmp { color: #67c23a; border-color: rgba(103,194,58,.4); }
.badge.proto-mqtt { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.proto-manual { color: #909399; border-color: rgba(144,147,153,.4); }

/* 模板选择器 */
.tpl-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; padding: 0 4px; }
.tpl-item { border: 1px solid var(--border); border-radius: 10px; padding: 14px; cursor: pointer; transition: all .2s; }
.tpl-item:hover { border-color: var(--accent); background: rgba(64,158,255,.05); transform: translateY(-2px); box-shadow: 0 4px 12px rgba(0,0,0,.1); }
.tpl-icon { font-size: 28px; margin-bottom: 6px; }
.tpl-name { font-weight: 600; font-size: 14px; margin-bottom: 4px; }
.tpl-desc { font-size: 12px; color: var(--muted); margin-bottom: 6px; }
.tpl-proto { font-size: 11px; color: var(--accent); }

/* 侧滑面板 */
.drawer-mask { position: fixed; inset: 0; background: rgba(0,0,0,.35); z-index: 1000; display: flex; justify-content: flex-end; }
.drawer { width: 600px; max-width: 95vw; height: 100%; background: var(--bg); box-shadow: -4px 0 20px rgba(0,0,0,.15); display: flex; flex-direction: column; overflow: hidden; }
.drawer-head { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.drawer-head h3 { margin: 0; font-size: 18px; }
.drawer-body { flex: 1; overflow-y: auto; padding: 20px; }
.drawer-foot { padding: 14px 20px; border-top: 1px solid var(--border); display: flex; justify-content: flex-end; gap: 10px; flex-shrink: 0; }

/* 字段集 */
.fs { border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin-bottom: 16px; }
.fs legend { font-weight: 600; font-size: 14px; padding: 0 8px; color: var(--accent); }

/* 表单 */
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.form-grid label { display: flex; flex-direction: column; gap: 4px; font-size: 13px; color: var(--muted); }
.form-grid label.full { grid-column: 1 / -1; }

/* V45: 字段白话提示（悬停气泡，纯 CSS） */
.tip {
  display: inline-flex; align-items: center; justify-content: center;
  width: 15px; height: 15px; margin-left: 5px; border-radius: 50%;
  border: 1px solid var(--muted); color: var(--muted);
  font-size: 10px; font-style: normal; font-weight: 700; cursor: help;
  position: relative; vertical-align: middle; flex-shrink: 0;
}
.tip:hover { border-color: var(--accent); color: var(--accent); }
.tip::after {
  content: attr(data-tip);
  position: absolute; left: 50%; bottom: calc(100% + 8px); transform: translateX(-50%);
  width: 300px; padding: 10px 12px; border-radius: 8px;
  background: #1c2333; color: #f0f3fa; font-size: 12px; font-weight: 400;
  line-height: 1.6; white-space: pre-line; text-align: left;
  opacity: 0; visibility: hidden; transition: opacity .15s; pointer-events: none;
  z-index: 1200; box-shadow: 0 6px 20px rgba(0,0,0,.35);
}
.tip:hover::after { opacity: 1; visibility: visible; }
/* 靠右字段的气泡防溢出：向左对齐 */
.form-grid label:nth-child(even) .tip::after { left: auto; right: -8px; transform: none; }

/* V45: 协议选择卡片 */
.proto-cards { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 14px; }
.pcard {
  border: 1.5px solid var(--border); border-radius: 10px; padding: 10px 12px;
  cursor: pointer; transition: all .15s; background: var(--card);
}
.pcard b { display: block; font-size: 13.5px; color: var(--text); margin-bottom: 4px; }
.pcard span { font-size: 11.5px; color: var(--muted); line-height: 1.5; display: block; }
.pcard:hover { border-color: var(--accent); }
.pcard.on { border-color: var(--accent); background: rgba(64,158,255,.07); box-shadow: 0 0 0 1px var(--accent) inset; }
.pcard.on b { color: var(--accent); }

/* V45: 字段下方动态提示 */
.field-hint { font-size: 11.5px; color: var(--accent); line-height: 1.5; margin-top: 2px; }
.threshold-explain {
  font-size: 12px; color: var(--muted); background: var(--bg2);
  border-left: 3px solid var(--accent); border-radius: 4px;
  padding: 8px 12px; margin-bottom: 12px; line-height: 1.6;
}
/* V45: 详情抽屉配置白话解析 */
.cfg-explain {
  background: var(--bg2); border-radius: 10px; padding: 12px 14px;
  display: flex; flex-direction: column; gap: 8px;
}
.cfg-line { font-size: 12.5px; color: var(--text); line-height: 1.7; display: flex; gap: 8px; }
.cfg-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--accent); flex-shrink: 0; margin-top: 7px; }
/* V46: 导入向导 */
.imp-steps { display: flex; gap: 24px; padding: 4px 20px 12px; border-bottom: 1px solid var(--border); }
.imp-step { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--muted); }
.imp-step b { display: inline-flex; width: 22px; height: 22px; border-radius: 50%; background: var(--bg2); border: 1px solid var(--border); align-items: center; justify-content: center; font-size: 12px; }
.imp-step.on { color: var(--accent); }
.imp-step.on b { background: var(--accent); color: #fff; border-color: var(--accent); }
.imp-step.cur { font-weight: 600; }
.imp-body { padding: 16px 20px; overflow: auto; flex: 1; }
.imp-guide p { font-size: 13.5px; line-height: 1.8; color: var(--text); margin-bottom: 12px; }
.imp-tips { background: var(--bg2); border-radius: 10px; padding: 12px 14px; display: flex; flex-direction: column; gap: 8px; margin-bottom: 16px; }
.imp-tip-line { display: flex; gap: 8px; align-items: flex-start; font-size: 12.5px; color: var(--muted); line-height: 1.6; }
.imp-tip-line svg { color: var(--green); flex-shrink: 0; margin-top: 2px; }
.imp-actions { display: flex; gap: 12px; }
.imp-err { color: var(--red); font-size: 13px; margin-top: 12px; line-height: 1.6; }
.imp-sum { display: flex; align-items: center; gap: 6px; font-size: 13px; margin-bottom: 10px; }
.ok-b { color: var(--green); font-weight: 600; }
.bad-b { color: var(--red); font-weight: 600; }
.imp-table-wrap { max-height: 44vh; overflow: auto; border: 1px solid var(--border); border-radius: 8px; }
.imp-table { font-size: 12px; }
.imp-table td, .imp-table th { padding: 6px 10px; }
.imp-row-bad { background: color-mix(in srgb, var(--red) 6%, transparent); }
.imp-errline { font-weight: 400; font-size: 12px; line-height: 1.6; }
.imp-result-big { font-size: 15px; padding: 8px 0 14px; display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.imp-result-big svg { color: var(--green); }
.imp-result-big b { color: var(--green); font-size: 20px; }
.form-grid .input { width: 100%; box-sizing: border-box; }
</style>