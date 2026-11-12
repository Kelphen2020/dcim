<template>
  <div>
    <div class="page-head">
      <h1>3D 机房</h1>
      <div class="toolbar">
        <div class="r3d-crumb">
          <a @click="backToFloor">数据中心</a>
          <template v-if="curFloor"><i>›</i><a :class="{dim: viewMode==='room'}" @click="backToFloor">{{ curFloor }} 楼层</a></template>
          <template v-if="viewMode==='room' && curRoom"><i>›</i><b>{{ curRoom.name }}</b></template>
        </div>
        <div class="seg" v-if="viewMode==='floor'">
          <button v-for="fl in floors" :key="fl" class="seg-btn" :class="{ on: floor === fl }" @click="switchFloor(fl)">{{ fl }}</button>
        </div>
        <div class="seg" v-else>
          <button v-for="rm in floorRooms" :key="rm.id" class="seg-btn" :class="{ on: roomId === rm.id }" @click="switchRoom(rm.id)">{{ rm.name }}</button>
        </div>
        <div class="dev-search" v-if="viewMode==='room'">
          <input class="input ds-in" v-model="searchKw" placeholder="搜设备定位(机柜/测点/设备名)"
            @input="doSearch" @focus="doSearch" @keydown.esc="searchKw='';searchHits=[]" />
          <div class="ds-drop" v-if="searchHits.length">
            <div class="ds-item" v-for="(h,i) in searchHits" :key="i" @click="jumpTo(h)">
              <span class="ds-ico"><SvgIcon :name="h.ico" :size="15" /></span><b>{{ h.name }}</b><span class="ds-sub">{{ h.sub }}</span>
            </div>
          </div>
        </div>
        <label class="sw" v-if="viewMode==='room'"><input type="checkbox" v-model="xray" @change="rebuild" /> 机柜透视</label>
        <label class="sw"><input type="checkbox" v-model="showLabels" @change="rebuild" /> 标签</label>
        <label class="sw" v-if="viewMode==='room'"><input type="checkbox" v-model="showAisle" @change="rebuild" /> 通道标识</label>
        <label class="sw" v-if="viewMode==='room'"><input type="checkbox" v-model="containCold" @change="rebuild" /> 冷通道封闭</label>
        <label class="sw" v-if="viewMode==='room'"><input type="checkbox" v-model="ssaoOn" @change="toggleSSAO" /> SSAO</label>
        <label class="sw" v-if="viewMode==='room'"><input type="checkbox" v-model="airflowOn" @change="rebuild" /> <SvgIcon name="wind" />气流</label>
        <label class="sw" v-if="viewMode==='room'" :class="{editing: editMode}"><input type="checkbox" v-model="editMode" @change="toggleEdit" /> <SvgIcon name="pen" />编辑布局</label>
        <span style="flex:1"></span>
        <button class="btn" v-if="viewMode==='room'" @click="openPlan"><SvgIcon name="ruler" />按面积规划</button>
        <template v-if="editMode">
          <button class="btn" @click="importModel"><SvgIcon name="upload" />导入模型</button>
          <button class="btn" @click="openTpl"><SvgIcon name="folder" />布局模板</button>
          <button class="btn primary" @click="saveLayout" :disabled="!layoutDirty"><SvgIcon name="save" />保存布局</button>
          <button class="btn" @click="resetLayout">↺ 恢复自动</button>
        </template>
        <button class="btn" @click="resetView">⟲ 复位</button>
        <button class="btn" @click="topView"><SvgIcon name="expand" />俯视</button>
        <button class="btn" @click="toggleSpin">{{ spinning ? '⏸ 巡航' : '▶ 巡航' }}</button>
      </div>
    </div>

    <div v-if="editMode && selProps.show" class="props-panel">
      <div class="lib-h">{{ selProps.name }} <button class="dp-x" style="float:right" @click="selProps.show=false"><SvgIcon name="x" /></button></div>
      <div class="pp-row"><span>X 位置(m)</span><input type="number" class="pp-in" v-model.number="selProps.x" step="0.1" @change="applyProps('pos')" /></div>
      <div class="pp-row"><span>Z 位置(m)</span><input type="number" class="pp-in" v-model.number="selProps.z" step="0.1" @change="applyProps('pos')" /></div>
      <div class="pp-row"><span>旋转(°)</span><input type="number" class="pp-in" v-model.number="selProps.rot" step="15" @change="applyProps('rot')" /></div>
      <template v-if="selProps.type !== 'cab' && selProps.type !== 'model'">
        <div class="pp-row"><span>长(m)</span><input type="number" class="pp-in" v-model.number="selProps.size[0]" min="0.1" step="0.1" @change="applyProps('size')" /></div>
        <div class="pp-row"><span>高(m)</span><input type="number" class="pp-in" v-model.number="selProps.size[1]" min="0.1" step="0.1" @change="applyProps('size')" /></div>
        <div class="pp-row"><span>宽(m)</span><input type="number" class="pp-in" v-model.number="selProps.size[2]" min="0.05" step="0.1" @change="applyProps('size')" /></div>
      </template>
      <div class="pp-hint">拖拽移动 · R 旋转 · Del 删除</div>
    </div>
    <div v-if="editMode && viewMode==='room'" class="lib-panel">
      <div class="lib-h"><SvgIcon name="toolbox" />构件库</div>
      <div class="lib-sec">标准构件（点击放入场景，拖拽摆放）</div>
      <div class="lib-grid">
        <button v-for="t in LIB_TEMPLATES" :key="t.type" class="lib-item" @click="addCustom(t.type)" :title="t.name">
          <span class="lib-ico"><SvgIcon :name="t.icon" :size="18" color="#9fc3ec" /></span><span class="lib-nm">{{ t.name }}</span>
        </button>
      </div>
      <div class="lib-sec">模型库（GLB）<button class="lib-up" @click="uploadToLib"><SvgIcon name="upload" />上传</button></div>
      <div v-if="!modelLib.length" class="lib-empty">暂无模型，上传 .glb 后可复用</div>
      <div v-for="m in modelLib" :key="m.id" class="lib-model">
        <span class="lm-nm" :title="m.name"><SvgIcon name="box" />{{ m.name }}</span>
        <button class="lm-btn" @click="addModelInstance(m)">放入</button>
        <button class="lm-btn del" @click="delModelAsset(m)"><SvgIcon name="x" /></button>
      </div>
    </div>
    <div v-if="editMode" class="edit-tip"><SvgIcon name="pen" />编辑模式：拖拽设备调整位置 · 滚轮选中后 <b>[R]</b> 旋转 · <b>[Delete]</b> 恢复自动位置 · 完成后点「保存布局」</div>
    <div v-if="planTip" class="plan-tip" @click="planTip=''">{{ planTip }} <SvgIcon name="x" /></div>

    <div v-if="!webgl" class="card" style="padding:60px;text-align:center">
      <div style="font-size:40px;margin-bottom:12px"><SvgIcon name="monitor" /></div>
      <div style="font-size:16px;font-weight:600;margin-bottom:8px">当前浏览器不支持 WebGL</div>
      <router-link to="/assets" class="btn btn-primary" style="margin-top:16px;display:inline-block">前往 2D 资产视图</router-link>
    </div>

    <template v-else>
      <div class="r3d-wrap">
        <div ref="mountEl" class="r3d-canvas"></div>

        <!-- ═══ 左侧：全局视图面板（房间级） ═══ -->
        <div class="ovl-panel ov-left" v-if="viewMode==='room'">
          <div class="ov-title"><SvgIcon name="globe" />全局视图</div>
          <div class="ov-kpis">
            <div class="ov-kpi"><b>{{ bs.run_days }}</b><span>运行天数</span></div>
            <div class="ov-kpi"><b>{{ bs.devices }}</b><span>设备总数</span></div>
            <div class="ov-kpi"><b :class="{warn: bs.offline > 0}">{{ bs.offline }}</b><span>离线测点</span></div>
            <div class="ov-kpi"><b :class="{bad: bs.today_alerts > 0}">{{ bs.today_alerts }}</b><span>今日告警</span></div>
          </div>
          <div class="ov-sec"><SvgIcon name="temp" />温度（{{ thStats.n }} 个测点）</div>
          <div class="ov-th"><span>均 <b>{{ thStats.avg }}</b>°C</span><span>高 <b class="bad">{{ thStats.max }}</b>°C</span><span>低 <b>{{ thStats.min }}</b>°C</span></div>
          <div class="ov-sec"><SvgIcon name="folder" />设备分类 <span v-if="catFilter" class="ov-clear" @click="catFilter=null;applyCatFilter()"><SvgIcon name="x" />清除过滤</span></div>
          <div class="ov-cats">
            <div v-for="c in devCats" :key="c.key" class="ov-cat" :class="{on: catFilter===c.key}" @click="toggleCat(c.key)">
              <span class="ov-cat-n"><SvgIcon :name="c.ico" :size="14" /> {{ c.name }}</span>
              <span class="ov-cat-cnt">{{ c.count }}</span>
              <span v-if="c.alarm" class="ov-cat-alarm">{{ c.alarm }}</span>
            </div>
          </div>
          <div class="ov-sec"><SvgIcon name="signal" />测点实时值</div>
          <div class="ov-points">
            <div v-for="p in sidePoints" :key="p.id" class="ov-pt" :class="{alarm: p.status === 2, off: p.status === 0}" @click="openPoint(p)">
              <span class="ov-pt-n">{{ TYPE_ICO[p.type] }} {{ shortPt(p.name) }}</span>
              <b class="ov-pt-v">{{ p.current_value != null ? Number(p.current_value).toFixed(1) + (p.unit || '') : '--' }}</b>
            </div>
          </div>
        </div>

        <!-- ═══ 右侧：PUE + 能耗面板（房间级） ═══ -->
        <div class="ovl-panel ov-right" v-if="viewMode==='room'">
          <div class="ov-title"><SvgIcon name="bolt" />能效与能耗</div>
          <div ref="pueEl" class="ov-gauge"></div>
          <div class="ov-kpis col2">
            <div class="ov-kpi"><b>{{ bs.totalKW }}</b><span>总功率 kW</span></div>
            <div class="ov-kpi"><b>{{ bs.itKW }}</b><span>IT 负载 kW</span></div>
            <div class="ov-kpi"><b>{{ en.energy }}</b><span>7天能耗 kWh</span></div>
            <div class="ov-kpi"><b>{{ en.cost }}</b><span>7天电费 ¥</span></div>
          </div>
          <div class="ov-sec"><SvgIcon name="bell" />告警分级</div>
          <div class="ov-sevs">
            <div class="ov-sev s3" @click="$router.push({path:'/alert-wall',query:{severity:3}})"><b>{{ bs.sev3 }}</b><span>严重</span></div>
            <div class="ov-sev s2" @click="$router.push({path:'/alert-wall',query:{severity:2}})"><b>{{ bs.sev2 }}</b><span>警告</span></div>
            <div class="ov-sev s1" @click="$router.push({path:'/alert-wall',query:{severity:1}})"><b>{{ bs.sev1 }}</b><span>提示</span></div>
          </div>
          <div class="ov-links">
            <a @click="$router.push('/energy')">能耗总览 ›</a>
            <a @click="$router.push('/big-screen')">可视化大屏 ›</a>
          </div>
        </div>

        <!-- ═══ 底部：实时告警条 ═══ -->
        <div class="ovl-panel ov-bottom" v-if="viewMode==='room' && bs.alerts.length">
          <div class="ovb-title"><SvgIcon name="clock" />实时告警</div>
          <div class="ovb-list">
            <div v-for="a in bs.alerts" :key="a.id" class="ovb-item" :class="'sev' + a.severity" @click="alertClick(a)">
              <span class="ovb-t">{{ fmtT(a.triggered_at) }}</span>
              <span class="ovb-m">{{ a.message }}</span>
              <span class="ovb-s">{{ SEV[a.severity] }}</span>
            </div>
          </div>
        </div>

        <!-- 楼层视图提示 -->
        <div class="ovl-panel ov-floorhint" v-if="viewMode==='floor'">
          <div class="ov-title"><SvgIcon name="building" />{{ curFloor }} 楼层平面 · GB 50174 功能房间</div>
          <div class="fh-tip">点击房间进入 3D 机房 · 颜色 = 功能区类别 · 角标 = 告警测点数</div>
          <div class="fh-rooms">
            <div v-for="rm in floorRooms" :key="rm.id" class="fh-room" :class="'fh-'+roomCat(rm).key" @click="enterRoom(rm.id)">
              <b>{{ rm.name }}</b>
              <span>{{ Math.round(rm.area || 0) }}㎡ · {{ roomCat(rm).name }}</span>
              <span v-if="roomAlarm(rm.id)" class="fh-alarm"><SvgIcon name="alert" />{{ roomAlarm(rm.id) }}</span>
              <span v-else class="fh-ok"><SvgIcon name="check" /></span>
            </div>
          </div>
        </div>
        <!-- 图例 -->
        <div class="r3d-legend" v-if="viewMode==='room' && !compactLegend">
          <div class="lg-title">负载着色</div>
          <div class="lg-row"><span class="lg-dot" style="background:#00e676"></span>轻载 &lt;60%</div>
          <div class="lg-row"><span class="lg-dot" style="background:#ffc53d"></span>中载 60–80%</div>
          <div class="lg-row"><span class="lg-dot" style="background:#ff5252"></span>重载 &gt;80%</div>
          <div class="lg-sep"></div>
          <div class="lg-row"><span class="lg-dot" style="background:#1e88e5"></span><SvgIcon name="snow" />精密空调</div>
          <div class="lg-row"><span class="lg-dot" style="background:#8e24aa"></span><SvgIcon name="battery" />UPS</div>
          <div class="lg-row"><span class="lg-dot" style="background:#546e7a"></span><SvgIcon name="bolt" />列头柜</div>
          <div class="lg-sep"></div>
          <div class="lg-title">功能分区 · GB 50174-2017</div>
          <div class="lg-row"><span class="lg-dot" style="background:#409eff"></span>主机房 ≈33%</div>
          <div class="lg-row"><span class="lg-dot" style="background:#ffa726"></span>变配电/电池室（支持区）</div>
          <div class="lg-row"><span class="lg-dot" style="background:#26c6da"></span>空调机房（支持区）</div>
          <div class="lg-row"><span class="lg-dot" style="background:#66bb6a"></span>总控/值班室（辅助区）</div>
          <div class="lg-row"><span class="lg-dot" style="background:#78909c"></span>室外机区（建筑外）</div>
          <div class="lg-hint muted">§4.2.3 辅助+支持 = 主机房×1.5~2.5<br>§4.2.2 虚线框 = 预留扩展位<br>§4.3.3 机柜面对面/背对背</div>
          <div class="lg-hint muted">拖拽旋转 · 滚轮缩放<br>点设备看详情 · 点机柜看U位</div>
        </div>

        <!-- 悬浮提示 -->
        <div v-if="hover" class="r3d-tip" :style="{ left: hover.x + 'px', top: hover.y + 'px' }">
          <b>{{ hover.title }}</b><br>
          <span class="muted">{{ hover.sub }}</span>
        </div>

        <!-- 设备信息弹窗（点击机柜内设备） -->
        <div v-if="devPop.show" class="dev-pop" :style="{ left: devPop.x + 'px', top: devPop.y + 'px' }">
          <div class="dp-head"><b>{{ devPop.a.name }}</b><button class="dp-x" @click="devPop.show = false"><SvgIcon name="x" /></button></div>
          <table class="dp-kv">
            <tr><td>资产编码</td><td class="mono">{{ devPop.a.code || '-' }}</td></tr>
            <tr><td>设备编号</td><td class="mono">{{ devPop.a.sn || '-' }}</td></tr>
            <tr><td>厂商型号</td><td>{{ devPop.a.brand }} {{ devPop.a.model }}</td></tr>
            <tr><td>类别</td><td>{{ CAT[devPop.a.category] || devPop.a.category }}</td></tr>
            <tr><td>额定功率</td><td>{{ devPop.a.power_w }} W</td></tr>
            <tr v-if="devPop.a.start_u"><td>U位</td><td>U{{ devPop.a.start_u }} 起 · {{ devPop.a.height_u }}U（{{ devPop.cabCode }}）</td></tr>
            <tr v-else-if="devPop.roomName"><td>所在房间</td><td>{{ devPop.roomName }}</td></tr>
            <tr><td>IP</td><td class="mono">{{ devPop.a.ip || '-' }}</td></tr>
            <tr v-if="devPop.a.warranty_end !== undefined"><td>维保</td><td><span class="badge" :class="devPop.a.in_warranty ? 'st-1' : 'st-2'">{{ devPop.a.in_warranty ? '在保至 ' + devPop.a.warranty_end : '脱保' }}</span></td></tr>
            <tr><td>状态</td><td><span class="badge" :class="devPop.a.status_text === '运行中' ? 'st-1' : 'st-2'">{{ devPop.a.status_text }}</span></td></tr>
          </table>
          <div class="dp-foot">
            <a class="dp-link" @click="$router.push({path:'/assets', query:{keyword: devPop.a.code || devPop.a.name}})">资产台账 ›</a>
            <a class="dp-link" @click="$router.push({path:'/maintenances', query:{asset_id: devPop.a.id}})">维保记录 ›</a>
            <a class="dp-link" @click="$router.push({path:'/work-orders', query:{asset_id: devPop.a.id}})">关联工单 ›</a>
          </div>
        </div>
      </div>

      <div class="card" style="margin-top:14px">
        <div class="card-title">机房概览（{{ curRoom?.name }}）</div>
        <div class="r3d-stats">
          <div class="st"><b>{{ roomStats.cabs }}</b><span>机柜</span></div>
          <div class="st"><b>{{ roomStats.assets }}</b><span>在架设备</span></div>
          <div class="st"><b>{{ roomStats.power.toFixed(1) }}</b><span>总功耗 kW</span></div>
          <div class="st"><b>{{ roomStats.rated.toFixed(1) }}</b><span>额定容量 kW</span></div>
          <div class="st"><b>{{ roomStats.loadPct }}%</b><span>平均负载率</span></div>
          <div class="st"><b>{{ roomStats.usedU }}/{{ roomStats.totalU }}</b><span>U位占用</span></div>
          <div class="st warn" v-if="roomStats.overloaded"><b>{{ roomStats.overloaded }}</b><span>重载机柜</span></div>
          <div class="st"><b>{{ roomStats.points }}</b><span>监控测点</span></div>
        </div>
      </div>
    </template>

    <!-- 单机柜详情抽屉 -->
    <div v-if="tplBox.show" class="drawer-mask" @click.self="tplBox.show = false">
      <div class="drawer" style="width:460px">
        <div class="drawer-head"><h3><SvgIcon name="folder" />布局模板</h3><button class="dp-x" @click="tplBox.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <div style="font-size:13px;opacity:.75;margin-bottom:10px">
            把当前房间已保存的布局（机柜位置+自定义构件+模型）存为模板，可套用到其它房间。
            机柜按编号自动匹配目标房间，编号不存在的跳过。
          </div>
          <div style="display:flex;gap:8px;margin-bottom:8px">
            <input class="input" v-model="tplBox.name" placeholder="模板名称，如：标准800㎡机房布局" style="flex:1" />
          </div>
          <div style="display:flex;gap:8px;margin-bottom:14px">
            <input class="input" v-model="tplBox.remark" placeholder="备注(可选)" style="flex:1" />
            <button class="btn primary" :disabled="tplBox.saving" @click="saveTpl">{{ tplBox.saving ? '保存中…' : '存当前布局' }}</button>
          </div>
          <div v-if="!tplBox.list.length" class="lib-empty">暂无模板</div>
          <table class="data-table" v-else>
            <thead><tr><th>名称</th><th>项数</th><th>创建时间</th><th style="width:130px">操作</th></tr></thead>
            <tbody>
              <tr v-for="t in tplBox.list" :key="t.id">
                <td><b>{{ t.name }}</b><div v-if="t.remark" style="font-size:11px;opacity:.6">{{ t.remark }}</div></td>
                <td>{{ t.item_count }}</td>
                <td>{{ (t.created_at||'').slice(0,16).replace('T',' ') }}</td>
                <td>
                  <button class="btn sm" @click="applyTpl(t)">套用</button>
                  <button class="btn sm danger" @click="delTpl(t)">删</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div v-if="planBox.show" class="drawer-mask" @click.self="planBox.show = false">
      <div class="drawer" style="width:420px">
        <div class="drawer-head"><h3><SvgIcon name="ruler" />按面积规划机柜</h3><button class="dp-x" @click="planBox.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body" v-if="planBox.hint">
          <div class="plan-grid">
            <div class="pg"><b>{{ planBox.hint.room_area }}㎡</b><span>房间面积</span></div>
            <div class="pg"><b>{{ planBox.hint.it_area }}㎡</b><span>主机房(GB比例)</span></div>
            <div class="pg"><b>{{ planBox.hint.suggest_cabs }}</b><span>建议机柜数</span></div>
            <div class="pg"><b>{{ planBox.hint.current_cabs }}</b><span>当前机柜数</span></div>
          </div>
          <div style="margin:14px 0;font-size:13px;opacity:.8">
            GB 50174-2017 §4.2.2: 主机房面积按机柜数 × 2~4㎡/台 计算并预留发展面积。
            规划密度（含冷通道/走道分摊）：
          </div>
          <div style="display:flex;gap:8px;align-items:center;margin-bottom:14px">
            <input type="number" class="input" v-model.number="planBox.density" min="2.5" max="8" step="0.5" style="width:90px" />
            <span style="font-size:13px">㎡/台（推荐 4~5）</span>
            <button class="btn" @click="loadPlanHint">重算</button>
          </div>
          <div v-if="planBox.hint.gap > 0" class="plan-gap">
            需补建 <b>{{ planBox.hint.gap }}</b> 台预留机柜（自动按每列8台、冷热通道交替排列，编号顺延）
          </div>
          <div v-else class="plan-gap ok">当前机柜数已达规划值 <SvgIcon name="check" /></div>
          <button class="btn primary block" v-if="planBox.hint.gap > 0" :disabled="planBox.loading" @click="doPlan">
            {{ planBox.loading ? '规划中…' : '一键生成 ' + planBox.hint.gap + ' 台预留机柜' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="drawer.show" class="drawer-mask" @click.self="closeDrawer">
      <div class="drawer" style="width:520px">
        <div class="drawer-head">
          <h3>机柜 {{ drawer.cab?.code }} · U位详情</h3>
          <button class="btn btn-mini" @click="closeDrawer"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
          <div class="r3d-stats small-stats">
            <div class="st"><b>{{ drawer.cap?.used_u }}</b><span>已用 U</span></div>
            <div class="st"><b>{{ drawer.cap?.free_u }}</b><span>空闲 U</span></div>
            <div class="st"><b>{{ ((drawer.cap?.used_power_w || 0) / 1000).toFixed(2) }}</b><span>功耗 kW</span></div>
            <div class="st"><b>{{ (drawer.cap?.used_weight_kg || 0).toFixed(0) }}</b><span>承重 kg</span></div>
          </div>
          <div class="u-map">
            <div v-for="(seg, i) in uSegs" :key="i" class="u-row" :class="{ occ: seg.asset }" :title="seg.asset ? `${seg.asset.name} (U${seg.u}, ${seg.asset.height_u}U)` : `U${seg.u} 空闲`">
              <span class="u-num">U{{ String(seg.u).padStart(2, '0') }}</span>
              <span class="u-bar" :style="seg.asset ? { background: uColor(seg.asset.id) } : {}"></span>
              <span v-if="seg.asset && seg.first" class="u-name">{{ seg.asset.name }}</span>
            </div>
          </div>
          <div class="card-title" style="margin-top:14px">在架资产（{{ (drawer.cap?.assets || []).length }}）</div>
          <table class="data-table">
            <thead><tr><th>资产</th><th>类别</th><th>U位</th><th>功耗</th></tr></thead>
            <tbody>
              <tr v-for="a in drawer.cap?.assets || []" :key="a.id" class="clickable-row" @click="$router.push({path:'/assets', query:{keyword:a.code||a.name}})">
                <td>{{ a.name }}<div class="muted small">{{ a.brand }} {{ a.model }}</div></td>
                <td>{{ CAT[a.category] || a.category }}</td>
                <td>U{{ a.start_u }}（{{ a.height_u }}U）</td>
                <td>{{ a.power_w }}W</td>
              </tr>
            </tbody>
          </table>
          <div class="drawer-links">
            <a class="btn btn-mini" @click="$router.push({path:'/capacity', query:{cabinet_id: drawer.cab.id}})">机柜容量 ›</a>
            <a class="btn btn-mini" @click="$router.push({path:'/monitoring-points', query:{cabinet_id: drawer.cab.id}})">关联测点 ›</a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
<script setup>
import { ref, computed, onMounted, onBeforeUnmount, reactive, nextTick, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js'
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js'
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js'
import { SSAOPass } from 'three/addons/postprocessing/SSAOPass.js'
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js'
import * as echarts from 'echarts'
import api from '../api'

const router = useRouter()
const route = useRoute()
const CAT = { server: '服务器', network: '网络设备', storage: '存储', power: '配电', cooling: '制冷', security: '安全', other: '其他' }
const CAT_COLOR = { server: 0x409eff, network: 0x00b96b, storage: 0x9d7bff, power: 0xe6a23c, cooling: 0x1e88e5, security: 0xeb2f96, other: 0x8a93a6 }
const TYPE_ICO = { hvac: '冷', ups: '电', power_dist: '配', temp_humidity: '温', water_leak: '水', smoke: '烟' }
const SEV = { 1: '提示', 2: '警告', 3: '严重' }

const mountEl = ref(null)
const webgl = ref(true)
const hover = ref(null)
const spinning = ref(false)
const showLabels = ref(true)
const showAisle = ref(true)
const xray = ref(false)         // V23 默认实门(更真实)，需要看柜内设备时开启透视
const containCold = ref(true)   // V25 冷通道封闭(天窗+端门), 真实机房冷热隔离
// V26 自定义布局编辑
const editMode = ref(false)
const layoutDirty = ref(false)
const layoutOverrides = ref({})   // key: cab-<id> / decor-<key> / model-<id> → {x,z,rot_y,scale,model_src}
const layoutItems = ref([])       // 后端返回的持久化项(含导入模型)
const planTip = ref('')
// V27 构件库
const LIB_TEMPLATES = [
  { type: 'wall',   icon: 'wall', name: '隔墙 3m' },
  { type: 'column', icon: 'column', name: '立柱' },
  { type: 'door',   icon: 'door', name: '门 1.1m' },
  { type: 'ups',    icon: 'battery', name: 'UPS' },
  { type: 'crac',   icon: 'snow', name: '精密空调' },
  { type: 'pdu',    icon: 'bolt', name: '配电柜' },
  { type: 'desk',   icon: 'monitor', name: '操作台' },
  { type: 'rack_empty', icon: 'rack', name: '空机柜' },
]
const modelLib = ref([])
// V28 构件默认尺寸(米) L×H×W, 属性面板按此换算缩放
const LIB_DEFAULT_SIZE = {
  wall: [3, 2.8, 0.12], column: [0.4, 3.2, 0.4], door: [1.3, 2.2, 0.1],
  ups: [0.7, 1.5, 0.6], crac: [0.9, 1.9, 0.55], pdu: [0.8, 2.0, 0.6],
  desk: [1.6, 0.8, 0.7], rack_empty: [0.6, 2.0, 1.0],
}
const selProps = reactive({ show: false, key: '', name: '', type: '', x: 0, z: 0, rot: 0, size: [0, 0, 0] })
const tplBox = reactive({ show: false, list: [], name: '', remark: '', saving: false })
const planBox = reactive({ show: false, hint: null, density: 4.5, loading: false })
const ssaoOn = ref(false)       // V25 SSAO接触阴影(后处理, 默认关——软渲染/低配防卡)
const airflowOn = ref(false)    // V32 气流组织可视化(冷通道送风粒子+温度热区), 默认关(性能)
// 设备搜索定位
const searchKw = ref('')
const searchHits = ref([])
let airflowSys = []             // 粒子系统注册表(animate更新, rebuild清空)
let lastAirT = 0
let composer = null
const compactLegend = ref(false)
const drawer = reactive({ show: false, cab: null, cap: null })
const devPop = reactive({ show: false, x: 0, y: 0, a: null, cabCode: '', roomName: '' })
const pueEl = ref(null)
let pueChart = null

// ── 数据 ──
const rooms = ref([]), rows = ref([]), cabs = ref([]), cabExt = ref({}), assetsBy = ref({})
const points = ref([]), powerNodes = ref([])
const roomAssets = ref([])   // V34: 辅助房间台账资产(room_id>0 非机架设备)
const bs = reactive({ run_days: '--', devices: 0, offline: 0, today_alerts: 0, totalKW: '--', itKW: '--', pue: null, sev1: 0, sev2: 0, sev3: 0, alerts: [] })
const en = reactive({ energy: '--', cost: '--' })
const roomId = ref(0)
const viewMode = ref('floor')          // V21-① floor=楼层平面 / room=房间级3D
const floor = ref('')
const curRoom = computed(() => rooms.value.find(r => r.id === roomId.value))
const floors = computed(() => [...new Set(rooms.value.map(r => r.floor || '1F'))])
const curFloor = computed(() => floor.value || floors.value[0] || '')
const floorRooms = computed(() => rooms.value.filter(r => (r.floor || '1F') === curFloor.value))
// GB 50174 功能房间分类（按名称推断）
function roomCat(rm) {
  const n = rm.name || ''
  if (/电池/.test(n)) return { key: 'battery', name: '电池室(支持区)', color: 0xffa726 }
  if (/配电|变配|UPS|ups/.test(n)) return { key: 'power', name: '变配电室(支持区)', color: 0xffa726 }
  if (/空调|制冷/.test(n)) return { key: 'hvac', name: '空调机房(支持区)', color: 0x26c6da }
  if (/值班|监控|总控|操作|消防|安防/.test(n)) return { key: 'ops', name: '总控/值班室(辅助区)', color: 0x66bb6a }
  if (/弱电|强电|进线/.test(n)) return { key: 'power', name: '电间(支持区)', color: 0xffa726 }
  if (/机房|主机房/.test(n)) return { key: 'it', name: '主机房', color: 0x409eff }
  return { key: 'other', name: '其他', color: 0x8a93a6 }
}
function roomAlarm(rid) { return points.value.filter(p => p.room_id === rid && p.status === 2).length }
function roomCabN(rid) { return cabs.value.filter(c => c.room_id === rid).length }
// V52 液冷：有 cdu/leak_rope 测点的房间集合（机柜顶部渲染冷却液管路示意）
const liquidRooms = computed(() => {
  const cabRoom = {}
  cabs.value.forEach(cb => { cabRoom[cb.id] = cb.room_id })
  const s = new Set()
  points.value.forEach(p => {
    if (p.type !== 'cdu' && p.type !== 'leak_rope') return
    const rid = p.room_id || cabRoom[p.cabinet_id]
    if (rid) s.add(rid)
  })
  return s
})
function enterRoom(id) { roomId.value = id; viewMode.value = 'room'; catFilter.value = null; userMoved = false; buildView() }
function backToFloor() { if (viewMode.value === 'room') { viewMode.value = 'floor'; userMoved = false; buildView() } }
function switchFloor(fl) { floor.value = fl; viewMode.value = 'floor'; userMoved = false; buildView() }
const roomCabs = computed(() => cabs.value.filter(c => c.room_id === roomId.value))
const roomStats = computed(() => {
  const cs = roomCabs.value
  let power = 0, rated = 0, usedU = 0, totalU = 0, assets = 0, overloaded = 0
  cs.forEach(c => {
    const e = cabExt.value[c.id] || {}
    power += e.power_kw || 0
    rated += c.rated_power_kw || 0
    usedU += e.used_u || 0
    totalU += c.total_u || 42
    assets += e.asset_count || 0
    if (c.rated_power_kw > 0 && (e.power_kw || 0) / c.rated_power_kw > 0.8) overloaded++
  })
  return { cabs: cs.length, assets, power, rated, usedU, totalU, overloaded,
    loadPct: rated > 0 ? Math.round(power / rated * 100) : 0,
    points: points.value.filter(p => p.room_id === roomId.value).length }
})
// 温湿度统计
const thStats = computed(() => {
  const th = points.value.filter(p => p.type === 'temp_humidity' && p.current_value != null)
  if (!th.length) return { n: 0, avg: '--', max: '--', min: '--' }
  const vs = th.map(p => Number(p.current_value))
  return { n: th.length, avg: (vs.reduce((a, b) => a + b, 0) / vs.length).toFixed(1), max: Math.max(...vs).toFixed(1), min: Math.min(...vs).toFixed(1) }
})
const sidePoints = computed(() => points.value.filter(p => p.room_id === roomId.value).slice(0, 14))

// ── V21-② 设备分类侧栏 ──
const catFilter = ref(null)
const pickVer = ref(0)   // pickables 非响应式，用版本号驱动 devCats 重算
const CAT_DEFS = [
  { key: 'power', ico: 'bolt', name: '供配电设备', match: o => o.userData.kind === 'pnode' },
  { key: 'ups_pt', ico: 'battery', name: 'UPS', match: o => o.userData.kind === 'point' && o.userData.point?.type === 'ups' },
  { key: 'crac', ico: 'snow', name: '精密空调', match: o => o.userData.kind === 'point' && o.userData.point?.type === 'hvac' },
  { key: 'pdc', ico: 'plug', name: '列头柜', match: o => o.userData.kind === 'point' && o.userData.point?.type === 'power_dist' },
  { key: 'th', ico: 'temp', name: '温湿度', match: o => o.userData.kind === 'point' && o.userData.point?.type === 'temp_humidity' },
  { key: 'wl', ico: 'drop', name: '漏水检测', match: o => o.userData.kind === 'point' && o.userData.point?.type === 'water_leak' },
  { key: 'smoke', ico: 'flame', name: '烟感', match: o => o.userData.kind === 'point' && o.userData.point?.type === 'smoke' },
  { key: 'cabinet', ico: 'rack', name: '机柜', match: o => o.userData.kind === 'cabinet' },
  { key: 'asset', ico: 'server', name: '在架设备', match: o => o.userData.kind === 'asset' },
  { key: 'room_asset', ico: 'tag', name: '房间台账设备', match: o => o.userData.kind === 'room_asset' },
]
const devCats = computed(() => {
  void pickVer.value
  const out = []
  CAT_DEFS.forEach(def => {
    let count = 0, alarm = 0
    const seen = new Set()
    pickables.forEach(o => {
      if (!def.match(o)) return
      const idk = def.key + ':' + (o.userData.point?.id || o.userData.node?.id || o.userData.cab?.id || o.userData.asset?.id || Math.random())
      if (seen.has(idk)) return
      seen.add(idk)
      count++
      const st = o.userData.point?.status ?? o.userData.node?.status ?? o.userData.cab?.status
      if (st === 2) alarm++
    })
    if (count > 0) out.push({ key: def.key, ico: def.ico, name: def.name, count, alarm })
  })
  return out
})
const PT_CAT = { ups: 'ups_pt', hvac: 'crac', power_dist: 'pdc', temp_humidity: 'th', water_leak: 'wl', smoke: 'smoke' }
function catOf(o) {
  const ud = o.userData || {}
  if (ud.kind === 'pnode') return 'power'
  if (ud.kind === 'cabinet') return 'cabinet'
  if (ud.kind === 'asset') return 'asset'
  if (ud.kind === 'decor') return 'decor'
  if (ud.kind === 'point') return PT_CAT[ud.point?.type] || null
  return null
}
// 沿 parent 向上找 sceneRoot 的直接子节点（设备组）
function rootChild(o) {
  let cur = o
  while (cur && cur.parent && cur.parent !== sceneRoot) cur = cur.parent
  return cur
}
function applyCatFilter() {
  if (!sceneRoot) return
  const f = catFilter.value
  // 先全部恢复
  sceneRoot.children.forEach(ch => { ch.visible = true })
  if (!f) return
  // 收集属于该类的顶层组
  const keep = new Set()
  pickables.forEach(o => {
    if (catOf(o) === f) { const rc = rootChild(o); if (rc) keep.add(rc) }
  })
  // 只隐藏「有 pickable 但不属于该类」的设备组；纯装饰(墙/地板/桥架/标签)保留
  sceneRoot.children.forEach(ch => {
    let hasPick = false, isKeep = keep.has(ch)
    ch.traverse(o => { if (o.userData && o.userData.kind) hasPick = true })
    if (hasPick && !isKeep) ch.visible = false
  })
}
function toggleCat(k) {
  catFilter.value = catFilter.value === k ? null : k
  applyCatFilter()
}
const shortPt = n => (n || '').replace(/^(A|B|C)\s?机房/, '').slice(0, 13)
const fmtT = t => t ? String(t).replace('T', ' ').slice(5, 16) : '-'
function alertClick(a) {
  if (a.point_id) router.push({ path: '/monitoring-points', query: { id: a.point_id, detail: '1' } })
  else router.push('/alert-wall')
}
function openPoint(p) { router.push({ path: '/monitoring-points', query: { id: p.id, detail: '1' } }) }

const uSegs = computed(() => {
  if (!drawer.cap) return []
  const total = drawer.cap.total_u || 42
  const assets = drawer.cap.assets || []
  const segs = []
  for (let u = total; u >= 1; u--) {
    const asset = assets.find(a => u >= a.start_u && u < a.start_u + a.height_u)
    segs.push({ u, asset, first: asset && asset.start_u === u })
  }
  return segs
})
const UCOLORS = ['#409eff', '#67c23a', '#e6a23c', '#9d7bff', '#00d4ff', '#f56c6c', '#13c2c2', '#eb2f96']
function uColor(id) { return UCOLORS[id % UCOLORS.length] }

// ── three ──
let scene, camera, renderer, controls, raycaster, pointer
let pickables = [], sceneRoot = null, disposed = false, focusTarget = null
let homeCam = null, userMoved = false, blinkMats = []
const CAB_W = 0.6, CAB_D = 1.0, CAB_H = 2.0, CAB_PITCH = 0.66   // 1单位=1米，机柜紧密并排
const COLD_AISLE = 1.2                                            // 冷通道 1200mm（前门对前门）
const HOT_AISLE = 1.0
const U_H = (CAB_H * 0.92) / 42                                   // 1U 高度

function themeC(name, fb) {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fb
}
function loadColor(c) {
  const e = cabExt.value[c.id] || {}
  const p = e.power_kw || 0
  if (!p) return 0x3a4a66
  const r = c.rated_power_kw > 0 ? p / c.rated_power_kw : 0
  if (r > 0.8) return 0xff5252
  if (r >= 0.6) return 0xffc53d
  return 0x00e676
}

// ── V23 程序化纹理 ──
// 防静电地砖: 浅色砖面+深缝+随机斑点
function makeTileTexture(dark) {
  const cv = document.createElement('canvas')
  cv.width = cv.height = 512
  const ctx = cv.getContext('2d')
  ctx.fillStyle = dark ? '#5c6672' : '#c3c9cf'   // 砖面
  ctx.fillRect(0, 0, 512, 512)
  // 斑点
  for (let i = 0; i < 2600; i++) {
    const g = 120 + Math.random() * 90 | 0
    ctx.fillStyle = `rgba(${g},${g + 4},${g + 10},${0.10 + Math.random() * 0.16})`
    ctx.fillRect(Math.random() * 512, Math.random() * 512, 1.6, 1.6)
  }
  // 2×2 砖缝
  ctx.strokeStyle = dark ? '#3a424c' : '#8b939b'
  ctx.lineWidth = 3
  for (const p of [0, 256]) {
    ctx.beginPath(); ctx.moveTo(p, 0); ctx.lineTo(p, 512); ctx.stroke()
    ctx.beginPath(); ctx.moveTo(0, p); ctx.lineTo(512, p); ctx.stroke()
  }
  const tex = new THREE.CanvasTexture(cv)
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}
// 机柜网孔门: 深底+圆孔阵列+竖向加强筋
function makePerfTexture() {
  const cv = document.createElement('canvas')
  cv.width = 128; cv.height = 256
  const ctx = cv.getContext('2d')
  ctx.fillStyle = '#171b20'
  ctx.fillRect(0, 0, 128, 256)
  ctx.fillStyle = '#0a0d11'
  for (let y = 6; y < 256; y += 8) {
    for (let x = 6 + (y % 16 === 6 ? 0 : 4); x < 128; x += 8) {
      ctx.beginPath(); ctx.arc(x, y, 2.1, 0, Math.PI * 2); ctx.fill()
    }
  }
  const tex = new THREE.CanvasTexture(cv)
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}
// 空调/柜体格栅: 横向百叶
function makeLouverTexture(base) {
  const cv = document.createElement('canvas')
  cv.width = 64; cv.height = 128
  const ctx = cv.getContext('2d')
  ctx.fillStyle = base || '#c8cdd2'
  ctx.fillRect(0, 0, 64, 128)
  ctx.fillStyle = 'rgba(40,48,56,.55)'
  for (let y = 4; y < 128; y += 8) ctx.fillRect(0, y, 64, 3.4)
  const tex = new THREE.CanvasTexture(cv)
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}

function makeLabel(text, opts = {}) {
  const scale = opts.scale || 1
  const cv = document.createElement('canvas')
  cv.width = 320; cv.height = 80
  const ctx = cv.getContext('2d')
  ctx.fillStyle = opts.bg || (opts.alarm ? 'rgba(70,14,20,.92)' : 'rgba(10,22,40,.88)')
  ctx.beginPath(); ctx.roundRect(10, 12, 300, 56, 12); ctx.fill()
  ctx.strokeStyle = opts.border || (opts.alarm ? 'rgba(255,82,82,.95)' : 'rgba(64,158,255,.55)'); ctx.lineWidth = opts.alarm ? 3 : 2
  ctx.beginPath(); ctx.roundRect(10, 12, 300, 56, 12); ctx.stroke()
  ctx.fillStyle = opts.color || (opts.alarm ? '#ffcdd2' : '#dce8f5'); ctx.font = 'bold 30px sans-serif'
  ctx.textAlign = 'center'; ctx.textBaseline = 'middle'
  ctx.fillText(text, 160, 41)
  const tex = new THREE.CanvasTexture(cv)
  const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, depthTest: false, transparent: true }))
  sp.scale.set(2.0 * scale, 0.5 * scale, 1)
  sp.userData.isLabel = true
  return sp
}

// ── 机柜（透视门 + 柜内设备实体 + U槽 + 状态灯）──
function buildCabinet(c, x, z, rotY) {
  const g = new THREE.Group()
  const e = cabExt.value[c.id] || {}
  const col = loadColor(c)
  const devList = assetsBy.value[c.id] || []

  // V23 骨架：后板+两侧板+顶底板（金属漆面）+底座支脚
  const frameMat = new THREE.MeshStandardMaterial({ color: 0x2b3038, roughness: 0.34, metalness: 0.55, envMapIntensity: 0.35 })
  const back = new THREE.Mesh(new THREE.BoxGeometry(CAB_W, CAB_H, 0.03), frameMat)
  back.position.set(0, CAB_H / 2, -CAB_D / 2)
  back.castShadow = true
  back.userData = { kind: 'cabinet', cab: c }
  g.add(back); pickables.push(back)
  // 底座支脚（机柜"落地"感）
  const baseMat = new THREE.MeshStandardMaterial({ color: 0x15181c, roughness: 0.6, metalness: 0.5 })
  const base = new THREE.Mesh(new THREE.BoxGeometry(CAB_W * 1.02, 0.06, CAB_D * 1.02), baseMat)
  base.position.y = 0.03; base.castShadow = true
  g.add(base)
  const sideL = new THREE.Mesh(new THREE.BoxGeometry(0.03, CAB_H, CAB_D), frameMat)
  sideL.position.set(-CAB_W / 2, CAB_H / 2, 0)
  sideL.castShadow = true
  sideL.userData = { kind: 'cabinet', cab: c }
  g.add(sideL); pickables.push(sideL)
  const sideR = sideL.clone(); sideR.position.x = CAB_W / 2
  sideR.userData = { kind: 'cabinet', cab: c }
  g.add(sideR); pickables.push(sideR)
  const top = new THREE.Mesh(new THREE.BoxGeometry(CAB_W, 0.04, CAB_D), frameMat)
  top.position.set(0, CAB_H, 0); top.castShadow = true; g.add(top)
  // 顶部铭牌条
  const namePlate = new THREE.Mesh(new THREE.BoxGeometry(CAB_W * 0.7, 0.05, 0.012),
    new THREE.MeshStandardMaterial({ color: 0xe8ecef, roughness: 0.5 }))
  namePlate.position.set(0, CAB_H * 0.95, CAB_D / 2 + 0.008); g.add(namePlate)
  const bottom = top.clone(); bottom.position.y = 0.04; g.add(bottom)

  // 柜内设备实体（按U位放置，可点击弹出设备信息）
  const usedU = Math.min(e.used_u || 0, 42)
  devList.forEach(a => {
    const h = Math.max(a.height_u || 1, 1) * U_H * 0.86
    const dev = new THREE.Mesh(
      new THREE.BoxGeometry(CAB_W * 0.84, h, CAB_D * 0.72),
      new THREE.MeshStandardMaterial({ color: CAT_COLOR[a.category] ?? 0x8a93a6, roughness: 0.42, metalness: 0.38 })
    )
    dev.castShadow = true
    const yBase = 0.06 + ((a.start_u || 1) - 1) * U_H + h / 2
    dev.position.set(0, yBase, 0.04)
    dev.userData = { kind: 'asset', asset: a, cab: c }
    g.add(dev); pickables.push(dev)
    // V24 设备前面板(深色) + LED 状态灯阵列(绿为主, 尾颗琥珀, 哈希决定数量/位置稳定)
    const facePlate = new THREE.Mesh(new THREE.BoxGeometry(CAB_W * 0.8, h * 0.92, 0.01),
      new THREE.MeshStandardMaterial({ color: 0x1a1e24, roughness: 0.45, metalness: 0.35 }))
    facePlate.position.set(0, yBase, CAB_D * 0.36); g.add(facePlate)
    const ledGreen = new THREE.MeshBasicMaterial({ color: 0x00e676 })
    const ledAmber = new THREE.MeshBasicMaterial({ color: 0xffb300 })
    const seed = (a.id * 2654435761) >>> 0
    const nLed = 3 + (seed % 4)
    const ledGeo = new THREE.SphereGeometry(0.009, 5, 5)
    for (let li2 = 0; li2 < nLed; li2++) {
      const lm = new THREE.Mesh(ledGeo, li2 === nLed - 1 ? ledAmber : ledGreen)
      lm.position.set(CAB_W * 0.18 + li2 * 0.055, yBase + h * 0.28, CAB_D * 0.372)
      g.add(lm)
    }
    // 面板通风槽(横向细缝)
    const slotMat = new THREE.MeshBasicMaterial({ color: 0x0a0d11 })
    for (let sl = 0; sl < 2; sl++) {
      const slot = new THREE.Mesh(new THREE.PlaneGeometry(CAB_W * 0.5, 0.008), slotMat)
      slot.position.set(-CAB_W * 0.12, yBase - h * 0.2 + sl * 0.03, CAB_D * 0.368)
      g.add(slot)
    }
  })
  // 空U槽条（占用区以上浅色、以下暗色，营造U位感）
  const slotGeo = new THREE.BoxGeometry(CAB_W * 0.9, 0.004, 0.01)
  for (let u = 1; u <= 42; u += 2) {
    const yy = 0.06 + (u / 42) * CAB_H * 0.92
    const slot = new THREE.Mesh(slotGeo, new THREE.MeshBasicMaterial({ color: u <= usedU ? col : 0x24334d, transparent: true, opacity: 0.75 }))
    slot.position.set(0, yy, CAB_D / 2 - 0.02)
    g.add(slot)
  }

  // V23 前门：透视=网孔玻璃门(能看柜内) / 实体=网孔钢门贴图
  const perfTex = makePerfTexture()
  const doorMat = xray.value
    ? new THREE.MeshPhysicalMaterial({ map: perfTex, color: 0x3a4652, transparent: true, opacity: 0.42, roughness: 0.3, metalness: 0.4, side: THREE.DoubleSide })
    : new THREE.MeshStandardMaterial({ map: perfTex, color: 0xffffff, roughness: 0.35, metalness: 0.55, envMapIntensity: 0.3 })
  const door = new THREE.Mesh(new THREE.BoxGeometry(CAB_W * 0.98, CAB_H * 0.96, 0.015), doorMat)
  door.position.set(0, CAB_H / 2, CAB_D / 2)
  door.castShadow = !xray.value
  door.userData = { kind: 'cabinet', cab: c }
  g.add(door); pickables.push(door)
  // 门把手（竖杆）+ 门锁
  const handleMat = new THREE.MeshStandardMaterial({ color: 0x9aa4ad, roughness: 0.3, metalness: 0.85 })
  const handle = new THREE.Mesh(new THREE.BoxGeometry(0.022, CAB_H * 0.34, 0.028), handleMat)
  handle.position.set(CAB_W * 0.4, CAB_H * 0.52, CAB_D / 2 + 0.022)
  g.add(handle)
  const lock = new THREE.Mesh(new THREE.CylinderGeometry(0.016, 0.016, 0.02, 10), handleMat)
  lock.rotation.x = Math.PI / 2
  lock.position.set(CAB_W * 0.4, CAB_H * 0.72, CAB_D / 2 + 0.02)
  g.add(lock)
  // 门框
  const edgeMat = new THREE.MeshStandardMaterial({ color: col, roughness: 0.4, metalness: 0.4 })
  const edgeV = new THREE.Mesh(new THREE.BoxGeometry(0.018, CAB_H * 0.96, 0.02), edgeMat)
  edgeV.position.set(-CAB_W / 2 + 0.01, CAB_H / 2, CAB_D / 2); g.add(edgeV)
  const edgeV2 = edgeV.clone(); edgeV2.position.x = CAB_W / 2 - 0.01; g.add(edgeV2)
  const edgeH = new THREE.Mesh(new THREE.BoxGeometry(CAB_W, 0.02, 0.02), edgeMat)
  edgeH.position.set(0, CAB_H - 0.03, CAB_D / 2); g.add(edgeH)

  // V52 液冷：机柜顶部冷却液管路示意（冷板式，供/回液双平行细管 + 后侧竖管）
  if (liquidRooms.value.has(c.room_id)) {
    const pipeMatS = new THREE.MeshStandardMaterial({ color: 0x2f7fff, roughness: 0.3, metalness: 0.7 })
    const pipeMatR = new THREE.MeshStandardMaterial({ color: 0xff7043, roughness: 0.3, metalness: 0.7 })
    const pipeGeo = new THREE.CylinderGeometry(0.02, 0.02, CAB_W * 0.92, 10)
    const pS = new THREE.Mesh(pipeGeo, pipeMatS)
    pS.rotation.z = Math.PI / 2
    pS.position.set(0, CAB_H + 0.07, CAB_D * 0.22)
    g.add(pS)
    const pR = new THREE.Mesh(pipeGeo, pipeMatR)
    pR.rotation.z = Math.PI / 2
    pR.position.set(0, CAB_H + 0.07, -CAB_D * 0.22)
    g.add(pR)
    // 弯头 + 后侧竖管（沿柜体下行，示意接入地板下管路）
    const elbow = new THREE.Mesh(new THREE.SphereGeometry(0.026, 8, 8), pipeMatR)
    elbow.position.set(CAB_W * 0.3, CAB_H + 0.07, -CAB_D * 0.22)
    g.add(elbow)
    const riser = new THREE.Mesh(new THREE.CylinderGeometry(0.02, 0.02, CAB_H * 0.45, 10), pipeMatR)
    riser.position.set(CAB_W * 0.3, CAB_H * 0.8, -CAB_D / 2 - 0.035)
    g.add(riser)
  }

  // 顶部状态灯（告警红闪）
  const isAlarm = (c.status === 2)
  const lightMat = new THREE.MeshBasicMaterial({ color: isAlarm ? 0xff5252 : ((e.power_kw || 0) > 0 ? 0x00e676 : 0x555555) })
  const light = new THREE.Mesh(new THREE.SphereGeometry(0.03, 8, 8), lightMat)
  light.position.set(0, CAB_H + 0.05, 0)
  g.add(light)
  if (isAlarm) blinkMats.push(lightMat)

  if (showLabels.value) {
    const tag = makeLabel(c.code, { scale: 0.5, border: `rgba(${(col >> 16) & 255},${(col >> 8) & 255},${col & 255},.8)` })
    tag.position.y = CAB_H + 0.4
    g.add(tag)
  }
  // V23 接触阴影(AO): 机柜底部暗化贴地, 增强"落地"感
  const ao = new THREE.Mesh(new THREE.PlaneGeometry(CAB_W * 1.7, CAB_D * 1.7),
    new THREE.MeshBasicMaterial({ color: 0x000000, transparent: true, opacity: 0.28, depthWrite: false }))
  ao.rotation.x = -Math.PI / 2
  ao.position.y = 0.012
  g.add(ao)
  g.position.set(x, 0, z)
  g.rotation.y = rotY || 0
  return g
}

// ── 精密空调 ──
function buildCRAC(p, x, z, rotY) {
  const g = new THREE.Group()
  // V23: 米白机身(真实空调色)+下部回风格栅贴图+上部门板
  const louverTex = makeLouverTexture('#dfe3e6')
  louverTex.wrapS = louverTex.wrapT = THREE.RepeatWrapping
  louverTex.repeat.set(2, 3)
  const body = new THREE.Mesh(new THREE.BoxGeometry(0.9, 1.9, 0.55),
    new THREE.MeshStandardMaterial({ map: louverTex, color: 0xffffff, roughness: 0.4, metalness: 0.25 }))
  body.position.y = 0.95; body.castShadow = true; body.receiveShadow = true
  body.userData = { kind: 'point', point: p }
  g.add(body); pickables.push(body)
  // 状态色条(保留语义色但收敛为细条)
  const stripe = new THREE.Mesh(new THREE.BoxGeometry(0.92, 0.05, 0.56),
    new THREE.MeshStandardMaterial({ color: 0x1e88e5, roughness: 0.5 }))
  stripe.position.y = 1.86; g.add(stripe)
  const vent = new THREE.Mesh(new THREE.BoxGeometry(0.78, 0.5, 0.02),
    new THREE.MeshStandardMaterial({ color: 0x2a323a, roughness: 0.7 }))
  vent.position.set(0, 1.45, 0.285); g.add(vent)
  // 控制面板
  const panel = new THREE.Mesh(new THREE.BoxGeometry(0.26, 0.16, 0.015),
    new THREE.MeshStandardMaterial({ color: 0x11151a, roughness: 0.4, emissive: 0x0c2c40, emissiveIntensity: 0.6 }))
  panel.position.set(0.24, 1.6, 0.287); g.add(panel)
  const ledMat = new THREE.MeshBasicMaterial({ color: p.status === 2 ? 0xff5252 : 0x00e676 })
  const led = new THREE.Mesh(new THREE.SphereGeometry(0.025, 8, 8), ledMat)
  led.position.set(0.32, 1.78, 0.28); g.add(led)
  if (p.status === 2) blinkMats.push(ledMat)
  if (showLabels.value) {
    const v = p.current_value != null ? Number(p.current_value).toFixed(1) + (p.unit || '') : '--'
    const tag = makeLabel('冷 ' + v + (p.status === 2 ? ' !' : ''), { scale: 0.5, border: p.status === 2 ? undefined : 'rgba(30,136,229,.6)', alarm: p.status === 2 })
    tag.position.y = 2.3; g.add(tag)
  }
  const aoC = new THREE.Mesh(new THREE.PlaneGeometry(1.4, 1.0),
    new THREE.MeshBasicMaterial({ color: 0x000000, transparent: true, opacity: 0.24, depthWrite: false }))
  aoC.rotation.x = -Math.PI / 2; aoC.position.y = 0.012; g.add(aoC)
  g.position.set(x, 0, z); g.rotation.y = rotY || 0
  return g
}
function buildUPS(p, x, z) {
  const g = new THREE.Group()
  const body = new THREE.Mesh(new THREE.BoxGeometry(0.7, 1.5, 0.6),
    new THREE.MeshStandardMaterial({ color: 0x2b2f36, roughness: 0.35, metalness: 0.6 }))
  body.position.y = 0.75; body.castShadow = true; body.receiveShadow = true
  body.userData = { kind: 'point', point: p }
  g.add(body); pickables.push(body)
  const panelMat = new THREE.MeshBasicMaterial({ color: p.status === 2 ? 0xff5252 : 0x00e676 })
  const panel = new THREE.Mesh(new THREE.BoxGeometry(0.36, 0.2, 0.02), panelMat)
  panel.position.set(0, 1.2, 0.31); g.add(panel)
  // UPS 顶部状态色条(细)
  const upsStripe = new THREE.Mesh(new THREE.BoxGeometry(0.72, 0.04, 0.62),
    new THREE.MeshStandardMaterial({ color: 0x8e24aa, roughness: 0.4 }))
  upsStripe.position.y = 1.48; g.add(upsStripe)
  if (p.status === 2) blinkMats.push(panelMat)
  if (showLabels.value) {
    const v = p.current_value != null ? Number(p.current_value).toFixed(0) + (p.unit || '') : '--'
    const tag = makeLabel('电 ' + v + (p.status === 2 ? ' !' : ''), { scale: 0.45, border: p.status === 2 ? undefined : 'rgba(142,36,170,.6)', alarm: p.status === 2 })
    tag.position.y = 1.9; g.add(tag)
  }
  g.position.set(x, 0, z)
  return g
}
function buildPDC(p, x, z, rotY) {
  const g = new THREE.Group()
  const body = new THREE.Mesh(new THREE.BoxGeometry(0.55, 2.0, 0.65),
    new THREE.MeshStandardMaterial({ color: 0x59636c, roughness: 0.38, metalness: 0.6 }))
  body.position.y = 1.0; body.castShadow = true; body.receiveShadow = true
  body.userData = { kind: 'point', point: p }
  g.add(body); pickables.push(body)
  const screenMat = new THREE.MeshBasicMaterial({ color: p.status === 2 ? 0xff5252 : 0x36cfc9 })
  const screen = new THREE.Mesh(new THREE.BoxGeometry(0.3, 0.18, 0.02), screenMat)
  screen.position.set(0, 1.6, 0.335); g.add(screen)
  if (p.status === 2) blinkMats.push(screenMat)
  if (showLabels.value) {
    const v = p.current_value != null ? Number(p.current_value).toFixed(1) + (p.unit || '') : '--'
    const tag = makeLabel('配 ' + v + (p.status === 2 ? ' !' : ''), { scale: 0.45, alarm: p.status === 2 })
    tag.position.y = 2.4; g.add(tag)
  }
  g.position.set(x, 0, z); g.rotation.y = rotY || 0
  return g
}
// ══════════ V32 气流组织可视化 ══════════
// 粒子系统: 通道内粒子从地板上升(冷通道=穿孔地板送风, 热通道=柜后排热上升)
function makeAirflowSystem(x0, z0, w, d, count, color, yMax) {
  const geo = new THREE.BufferGeometry()
  const pos = new Float32Array(count * 3)
  const vel = new Float32Array(count)
  const reset = (i, rand) => {
    pos[i * 3] = x0 + (Math.random() - 0.5) * w
    pos[i * 3 + 1] = rand ? Math.random() * yMax : 0.06
    pos[i * 3 + 2] = z0 + (Math.random() - 0.5) * d
    vel[i] = 0.35 + Math.random() * 0.75
  }
  for (let i = 0; i < count; i++) reset(i, true)
  geo.setAttribute('position', new THREE.BufferAttribute(pos, 3))
  const mat = new THREE.PointsMaterial({
    color, size: 0.09, transparent: true, opacity: 0.85,
    depthWrite: false, blending: THREE.AdditiveBlending, sizeAttenuation: true
  })
  const pts = new THREE.Points(geo, mat)
  pts.frustumCulled = false
  pts.userData.noDrag = true
  const update = (dt) => {
    for (let i = 0; i < count; i++) {
      pos[i * 3 + 1] += vel[i] * dt
      if (pos[i * 3 + 1] > yMax) reset(i, false)
    }
    geo.attributes.position.needsUpdate = true
  }
  return { pts, update }
}
// 温度热区贴图: 径向渐变圆, 颜色按温度分档(蓝<22 / 绿22-26 / 橙26-30 / 红≥30)
function makeHeatTexture(t) {
  const cv = document.createElement('canvas'); cv.width = cv.height = 128
  const ctx = cv.getContext('2d')
  const c = t == null ? '#4d6a8a' : t < 22 ? '#29b6f6' : t < 26 ? '#66bb6a' : t < 30 ? '#ffa726' : '#ff5252'
  const g = ctx.createRadialGradient(64, 64, 4, 64, 64, 62)
  g.addColorStop(0, c + 'd0'); g.addColorStop(0.55, c + '66'); g.addColorStop(1, c + '00')
  ctx.fillStyle = g; ctx.fillRect(0, 0, 128, 128)
  const tex = new THREE.CanvasTexture(cv)
  return tex
}

function buildSensor(p, x, z) {
  const g = new THREE.Group()
  const mat = new THREE.MeshStandardMaterial({ color: p.status === 2 ? 0xff5252 : 0x36cfc9, emissive: p.status === 2 ? 0x992222 : 0x114444 })
  const s = new THREE.Mesh(new THREE.SphereGeometry(0.09, 12, 12), mat)
  s.position.y = 2.85
  s.userData = { kind: 'point', point: p }
  g.add(s); pickables.push(s)
  if (p.status === 2) blinkMats.push(mat)
  const rod = new THREE.Mesh(new THREE.CylinderGeometry(0.012, 0.012, 0.4),
    new THREE.MeshStandardMaterial({ color: 0x888888 }))
  rod.position.y = 3.08; g.add(rod)
  if (showLabels.value) {
    // V21-④ 三级阈值判定: 过高(high)>超高(max)>过低(min)
    const v = p.current_value != null ? Number(p.current_value).toFixed(1) + (p.unit || '°C') : '--'
    let warn = p.status === 2, suffix = warn ? ' !' : ''
    if (p.current_value != null) {
      const cv = Number(p.current_value)
      if (p.high_threshold != null && cv > p.high_threshold) { warn = true; suffix = ' !过高' }
      else if (p.max_threshold != null && cv > p.max_threshold) { warn = true; suffix = ' !超高' }
      else if (p.min_threshold != null && cv < p.min_threshold) { warn = true; suffix = ' !过低' }
    }
    const tag = makeLabel('温 ' + v + suffix, { scale: 0.42, bg: 'rgba(10,22,40,.7)', alarm: warn, border: warn ? undefined : 'rgba(54,207,201,.5)' })
    tag.position.y = 2.6; g.add(tag)
  }
  g.position.set(x, 0, z)
  return g
}

// ── V20 功能分区布局（配电区/IT机柜区/空调设备间/操作间/室外机区）──
const POWER_ZONE_W = 3.2    // 配电区宽
const HVAC_ZONE_W = 2.8     // 空调设备间宽
const OPS_W = 4.2, OPS_D = 3.2   // 操作间
const OUT_GAP = 1.5, OUT_W = 2.6 // 室外机区（建筑外侧）

function zoneFloor(x0, z0, w, d, color, opacity) {
  const m = new THREE.Mesh(new THREE.PlaneGeometry(w, d),
    new THREE.MeshBasicMaterial({ color, transparent: true, opacity: opacity == null ? 0.10 : opacity }))
  m.rotation.x = -Math.PI / 2
  m.position.set(x0 + w / 2, 0.015, z0 + d / 2)
  return m
}
function zoneBorder(x0, z0, w, d, color) {
  const g = new THREE.Group()
  const mat = new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.5 })
  const t = 0.035
  const mk = (ww, dd, x, z) => { const m = new THREE.Mesh(new THREE.PlaneGeometry(ww, dd), mat); m.rotation.x = -Math.PI / 2; m.position.set(x, 0.022, z); g.add(m) }
  mk(w, t, x0 + w / 2, z0); mk(w, t, x0 + w / 2, z0 + d)
  mk(t, d, x0, z0 + d / 2); mk(t, d, x0 + w, z0 + d / 2)
  return g
}
function zoneTag(zone) {
  // 双行标签：分区名（GB术语）+ 面积/占比
  const cv = document.createElement('canvas')
  cv.width = 420; cv.height = 120
  const ctx = cv.getContext('2d')
  ctx.fillStyle = 'rgba(10,22,40,.9)'
  ctx.beginPath(); ctx.roundRect(10, 10, 400, 100, 14); ctx.fill()
  ctx.strokeStyle = 'rgba(255,255,255,.35)'; ctx.lineWidth = 2
  ctx.beginPath(); ctx.roundRect(10, 10, 400, 100, 14); ctx.stroke()
  ctx.textAlign = 'center'
  ctx.fillStyle = '#e8f2fc'; ctx.font = 'bold 34px sans-serif'
  ctx.fillText(zone.name, 210, 52)
  ctx.fillStyle = '#9fb4cc'; ctx.font = '24px sans-serif'
  ctx.fillText(zone.sub || '', 210, 90)
  const tex = new THREE.CanvasTexture(cv)
  const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, depthTest: false, transparent: true }))
  sp.scale.set(2.9, 0.83, 1)
  sp.userData.isLabel = true
  sp.position.set(zone.x0 + zone.w / 2, 0.62, zone.z0 + zone.d - 1.0)
  return sp
}
// 配电柜（按供电链路节点 kind 变形：高压进线柜/变压器/UPS主机+电池柜/配电柜）
function buildPowerCab(node, x, z, rotY) {
  const g = new THREE.Group()
  const alarm = node.status === 2
  // V23 真实工业涂装: 统一 RAL7035 浅灰钣金，类型靠尺寸+图标+细色条区分(不再整体高饱和)
  let bodyW = 0.8, bodyH = 2.0, bodyD = 0.6, color = 0xc5ccd2, ico = '配', accent = 0x546e7a
  if (node.kind === 'utility') { bodyW = 1.2; bodyH = 2.2; color = 0xb4bcc4; ico = '市'; accent = 0x37474f }
  else if (node.kind === 'transformer') { bodyW = 1.0; bodyH = 1.4; bodyD = 0.9; color = 0xa8b0b8; ico = '变'; accent = 0x5d4037 }
  else if (node.kind === 'ups') { color = 0xbcc4cb; ico = 'UPS'; accent = 0x8e24aa }
  const body = new THREE.Mesh(new THREE.BoxGeometry(bodyW, bodyH, bodyD),
    new THREE.MeshStandardMaterial({ color, roughness: 0.32, metalness: 0.5, envMapIntensity: 0.35 }))
  body.position.y = bodyH / 2; body.castShadow = true; body.receiveShadow = true
  body.userData = { kind: 'pnode', node }
  g.add(body); pickables.push(body)
  // 柜门分割线(双开门)
  const seamMat = new THREE.MeshBasicMaterial({ color: 0x14181d })
  const seam = new THREE.Mesh(new THREE.PlaneGeometry(0.012, bodyH * 0.92), seamMat)
  seam.position.set(0, bodyH / 2, bodyD / 2 + 0.006); g.add(seam)
  const scrMat = new THREE.MeshBasicMaterial({ color: alarm ? 0xff5252 : accent })
  const scr = new THREE.Mesh(new THREE.BoxGeometry(bodyW * 0.4, 0.16, 0.02), scrMat)
  scr.position.set(0, bodyH * 0.78, bodyD / 2 + 0.011); g.add(scr)
  if (alarm) blinkMats.push(scrMat)
  // 门把手(真实配电柜细节)
  const hMat = new THREE.MeshStandardMaterial({ color: 0x6a727a, roughness: 0.3, metalness: 0.85 })
  const hnd = new THREE.Mesh(new THREE.BoxGeometry(0.02, bodyH * 0.18, 0.03), hMat)
  hnd.position.set(bodyW * 0.38, bodyH * 0.5, bodyD / 2 + 0.02); g.add(hnd)
  for (let i = 0; i < 3; i++) {
    const led = new THREE.Mesh(new THREE.SphereGeometry(0.02, 6, 6),
      new THREE.MeshBasicMaterial({ color: i === 0 ? 0x00e676 : (i === 1 ? 0xffc53d : 0xff5252), transparent: true, opacity: i === 0 ? 1 : 0.3 }))
    led.position.set(-bodyW * 0.28 + i * 0.07, bodyH * 0.62, bodyD / 2 + 0.012); g.add(led)
  }
  if (node.kind === 'ups') {  // 电池柜×2 排后
    for (let b = 0; b < 2; b++) {
      const bat = new THREE.Mesh(new THREE.BoxGeometry(0.6, 1.2, 0.5),
        new THREE.MeshStandardMaterial({ color: 0x4a148c, roughness: 0.6, metalness: 0.3 }))
      bat.position.set(0, 0.6, -bodyD / 2 - 0.38 - b * 0.58)
      bat.userData = { kind: 'pnode', node }
      g.add(bat); pickables.push(bat)
    }
  }
  if (showLabels.value) {
    const load = node.rated_kva > 0 ? ' ' + Math.round((node.load_kw || 0) / (node.rated_kva * 0.9) * 100) + '%' : ''
    const tag = makeLabel(ico + ' ' + node.name + load, { scale: 0.42 })
    tag.position.y = bodyH + 0.5; g.add(tag)
  }
  const ao = new THREE.Mesh(new THREE.PlaneGeometry(bodyW * 1.8, bodyD * 1.8),
    new THREE.MeshBasicMaterial({ color: 0x000000, transparent: true, opacity: 0.26, depthWrite: false }))
  ao.rotation.x = -Math.PI / 2; ao.position.y = 0.012; g.add(ao)
  g.position.set(x, 0, z); g.rotation.y = rotY || 0
  return g
}
// 空调室外机（冷凝器）
function buildCondenser(p, x, z, rotY) {
  const g = new THREE.Group()
  const body = new THREE.Mesh(new THREE.BoxGeometry(1.0, 0.95, 0.38),
    new THREE.MeshStandardMaterial({ color: 0xc8cfd4, roughness: 0.4, metalness: 0.6 }))
  body.position.y = 0.62; body.castShadow = true; body.receiveShadow = true
  body.userData = { kind: 'point', point: p }
  g.add(body); pickables.push(body)
  const ring = new THREE.Mesh(new THREE.CylinderGeometry(0.36, 0.36, 0.04, 24),
    new THREE.MeshStandardMaterial({ color: 0x37474f }))
  ring.position.set(0, 0.68, 0.2); ring.rotation.x = Math.PI / 2; g.add(ring)
  const bladeMat = new THREE.MeshStandardMaterial({ color: 0x263238 })
  for (let i = 0; i < 3; i++) {
    const bl = new THREE.Mesh(new THREE.BoxGeometry(0.62, 0.015, 0.07), bladeMat)
    bl.position.set(0, 0.68, 0.225); bl.rotation.z = i * Math.PI / 3
    g.add(bl)
  }
  const legMat = new THREE.MeshStandardMaterial({ color: 0x607d8b })
  for (const sx of [-0.4, 0.4]) {
    const leg = new THREE.Mesh(new THREE.BoxGeometry(0.06, 0.15, 0.34), legMat)
    leg.position.set(sx, 0.075, 0); g.add(leg)
  }
  const ledMat = new THREE.MeshBasicMaterial({ color: p.status === 2 ? 0xff5252 : 0x00e676 })
  const led = new THREE.Mesh(new THREE.SphereGeometry(0.02, 6, 6), ledMat)
  led.position.set(0.42, 1.02, 0.19); g.add(led)
  if (p.status === 2) blinkMats.push(ledMat)
  if (showLabels.value) {
    const tag = makeLabel('室外机', { scale: 0.4, border: 'rgba(120,144,156,.6)' })
    tag.position.y = 1.55; g.add(tag)
  }
  g.position.set(x, 0, z); g.rotation.y = rotY || 0
  return g
}
// 冷媒管（穿墙水平双管）
function buildRefrigerantPipe(x0, x1, z, y) {
  const g = new THREE.Group()
  const mat = new THREE.MeshStandardMaterial({ color: 0xb87333, roughness: 0.4, metalness: 0.6 })
  for (const dz of [-0.05, 0.05]) {
    const len = Math.abs(x1 - x0)
    const pipe = new THREE.Mesh(new THREE.CylinderGeometry(0.028, 0.028, len, 8), mat)
    pipe.rotation.z = Math.PI / 2
    pipe.position.set((x0 + x1) / 2, y, z + dz)
    g.add(pipe)
  }
  return g
}
// 操作台（桌+双显示器+键盘+椅）
function buildConsole(x, z, rotY) {
  const g = new THREE.Group()
  const deskMat = new THREE.MeshStandardMaterial({ color: 0x8d9db6, roughness: 0.6 })
  const desk = new THREE.Mesh(new THREE.BoxGeometry(1.8, 0.06, 0.75), deskMat)
  desk.position.y = 0.74; desk.castShadow = true
  desk.userData = { kind: 'decor', name: '操作台', sub: '值班监控 / 远程操作工位' }
  g.add(desk); pickables.push(desk)
  for (const sx of [-0.8, 0.8]) {
    const leg = new THREE.Mesh(new THREE.BoxGeometry(0.05, 0.72, 0.7), deskMat)
    leg.position.set(sx, 0.37, 0); g.add(leg)
  }
  for (const sx of [-0.42, 0.42]) {
    const scr = new THREE.Mesh(new THREE.BoxGeometry(0.72, 0.42, 0.03),
      new THREE.MeshBasicMaterial({ color: 0x1e88e5 }))
    scr.position.set(sx, 1.0, -0.18); scr.rotation.x = -0.08; g.add(scr)
    const stand = new THREE.Mesh(new THREE.BoxGeometry(0.05, 0.18, 0.05),
      new THREE.MeshStandardMaterial({ color: 0x333333 }))
    stand.position.set(sx, 0.85, -0.18); g.add(stand)
  }
  const kb = new THREE.Mesh(new THREE.BoxGeometry(0.5, 0.02, 0.16),
    new THREE.MeshStandardMaterial({ color: 0x263238 }))
  kb.position.set(0, 0.78, 0.14); g.add(kb)
  const chairMat = new THREE.MeshStandardMaterial({ color: 0x37474f })
  const seat = new THREE.Mesh(new THREE.BoxGeometry(0.45, 0.06, 0.45), chairMat)
  seat.position.set(0, 0.45, 0.85); g.add(seat)
  const backr = new THREE.Mesh(new THREE.BoxGeometry(0.45, 0.55, 0.05), chairMat)
  backr.position.set(0, 0.75, 1.05); g.add(backr)
  const pole = new THREE.Mesh(new THREE.CylinderGeometry(0.03, 0.03, 0.42), chairMat)
  pole.position.set(0, 0.22, 0.85); g.add(pole)
  g.position.set(x, 0, z); g.rotation.y = rotY || 0
  return g
}
// 监控大屏墙
function buildScreenWall(x, z, rotY, w, hh) {
  const g = new THREE.Group()
  const frame = new THREE.Mesh(new THREE.BoxGeometry(w + 0.15, hh + 0.15, 0.08),
    new THREE.MeshStandardMaterial({ color: 0x1c2739 }))
  frame.position.y = 1.9; g.add(frame)
  const scr = new THREE.Mesh(new THREE.PlaneGeometry(w, hh),
    new THREE.MeshBasicMaterial({ color: 0x0d47a1 }))
  scr.position.set(0, 1.9, 0.045)
  scr.userData = { kind: 'decor', name: '监控大屏', sub: '动环 / 安防 / 能耗集中监控展示' }
  g.add(scr); pickables.push(scr)
  const cellMat = new THREE.MeshBasicMaterial({ color: 0x1565c0 })
  const cols = Math.max(Math.round(w / 0.6), 2), rws = Math.max(Math.round(hh / 0.55), 2)
  for (let i = 0; i < cols; i++) for (let j = 0; j < rws; j++) {
    const cell = new THREE.Mesh(new THREE.PlaneGeometry(w / cols * 0.88, hh / rws * 0.82), cellMat)
    cell.position.set(-w / 2 + (i + 0.5) * w / cols, 1.9 - hh / 2 + (j + 0.5) * hh / rws, 0.047)
    g.add(cell)
  }
  g.position.set(x, 0, z); g.rotation.y = rotY || 0
  return g
}
// 围栏（室外机区：立柱+双横杆）
function buildFence(x0, z0, x1, z1) {
  const g = new THREE.Group()
  const postMat = new THREE.MeshStandardMaterial({ color: 0x78909c, roughness: 0.7 })
  const railMat = new THREE.MeshStandardMaterial({ color: 0x90a4ae, roughness: 0.7 })
  const len = Math.hypot(x1 - x0, z1 - z0)
  const n = Math.max(Math.round(len / 1.5), 2)
  const ang = Math.atan2(z1 - z0, x1 - x0)
  for (let i = 0; i <= n; i++) {
    const t = i / n
    const post = new THREE.Mesh(new THREE.BoxGeometry(0.06, 1.15, 0.06), postMat)
    post.position.set(x0 + (x1 - x0) * t, 0.575, z0 + (z1 - z0) * t)
    g.add(post)
  }
  for (const y of [0.55, 1.08]) {
    const rail = new THREE.Mesh(new THREE.BoxGeometry(len, 0.04, 0.04), railMat)
    rail.position.set((x0 + x1) / 2, y, (z0 + z1) / 2)
    rail.rotation.y = -ang
    g.add(rail)
  }
  return g
}

// ── V21-① 楼层平面场景：本层全部房间按面积比例排布，点击下钻 ──
function buildFloorScene() {
  if (!scene) return
  if (sceneRoot) { scene.remove(sceneRoot); disposeGroup(sceneRoot) }
  pickables = []; blinkMats = []; devPop.show = false
  sceneRoot = new THREE.Group()
  scene.add(sceneRoot)
  // V23: 3D 场景固定深色渲染环境(真实机房可视化惯例, 阴影/灯光对比才可辨), 不随浅色UI主题变白
  const bgHex = '#0d1520'
  scene.background = new THREE.Color(bgHex)
  scene.fog = new THREE.Fog(new THREE.Color(bgHex).getHex(), 40, 110)
  const isDark = true
  const rms = floorRooms.value
  if (!rms.length) return

  // 排布：面积→宽度(sqrt)单行排列，深度统一按最大房间
  const scale = 0.55   // 视觉缩放
  const widths = rms.map(r => Math.max(Math.sqrt((r.area || 60)) * 1.6, 3) * scale)
  const maxD = Math.max(...rms.map(r => Math.sqrt((r.area || 60)) * 1.15), 4) * scale
  const gap = 1.2 * scale
  const totalW = widths.reduce((a, b) => a + b, 0) + gap * (rms.length - 1)

  // V23 楼板(地砖纹理)
  const slabTex = makeTileTexture(isDark)
  slabTex.wrapS = slabTex.wrapT = THREE.RepeatWrapping
  slabTex.repeat.set((totalW + 4) / 2.4, (maxD + 4) / 2.4)
  const slab = new THREE.Mesh(new THREE.BoxGeometry(totalW + 4, 0.14, maxD + 4),
    new THREE.MeshStandardMaterial({ map: slabTex, roughness: 0.6, metalness: 0.05 }))
  slab.position.y = -0.07; slab.receiveShadow = true
  sceneRoot.add(slab)
  const grid = new THREE.GridHelper(Math.max(totalW, maxD) + 8, Math.round((Math.max(totalW, maxD) + 8)), isDark ? 0x223452 : 0x3d4d68, isDark ? 0x1a2640 : 0x33415a)
  grid.position.y = 0.005
  sceneRoot.add(grid)

  let x = -totalW / 2
  rms.forEach((rm, i) => {
    const w = widths[i], d = Math.max(Math.sqrt((rm.area || 60)) * 1.15, 4) * scale
    const cat = roomCat(rm)
    const hgt = rm.id === roomId.value ? 3.0 : 2.6
    const g = new THREE.Group()
    // 房间体（半透明，按GB类别着色）
    const bodyMat = new THREE.MeshStandardMaterial({ color: cat.color, transparent: true, opacity: 0.42, roughness: 0.65, side: THREE.DoubleSide })
    const body = new THREE.Mesh(new THREE.BoxGeometry(w, hgt, d), bodyMat)
    body.position.y = hgt / 2
    body.castShadow = true; body.receiveShadow = true
    body.userData = { kind: 'room3d', room: rm }
    g.add(body); pickables.push(body)
    // 地面色块
    const flr = new THREE.Mesh(new THREE.PlaneGeometry(w - 0.08, d - 0.08),
      new THREE.MeshBasicMaterial({ color: cat.color, transparent: true, opacity: 0.30 }))
    flr.rotation.x = -Math.PI / 2; flr.position.y = 0.02
    g.add(flr)
    // 边框
    const edgeMat = new THREE.MeshBasicMaterial({ color: cat.color, transparent: true, opacity: 0.95 })
    for (const [ew, ed, ex, ez] of [[w, 0.03, 0, -d / 2], [w, 0.03, 0, d / 2], [0.03, d, -w / 2, 0], [0.03, d, w / 2, 0]]) {
      const e = new THREE.Mesh(new THREE.PlaneGeometry(ew, ed), edgeMat)
      e.rotation.x = -Math.PI / 2; e.position.set(ex, 0.03, ez); g.add(e)
    }
    // 主机房：内部画机柜矩阵示意
    const nCab = roomCabN(rm.id)
    if (nCab > 0) {
      const cols = Math.max(Math.ceil(Math.sqrt(nCab * w / d)), 1)
      const rowsN = Math.ceil(nCab / cols)
      const cabMat = new THREE.MeshStandardMaterial({ color: isDark ? 0x2e4468 : 0x46608a, roughness: 0.6 })
      for (let ci = 0; ci < nCab; ci++) {
        const cc = ci % cols, rr = Math.floor(ci / cols)
        const cb = new THREE.Mesh(new THREE.BoxGeometry(Math.min(w / cols * 0.55, 0.28), 0.5, Math.min(d / rowsN * 0.5, 0.34)), cabMat)
        cb.position.set(-w / 2 + (cc + 0.5) * w / cols, 0.25, -d / 2 + (rr + 0.5) * d / rowsN)
        g.add(cb)
      }
    }
    // 标签（名称+面积+类别 / 告警角标）
    if (showLabels.value) {
      const alarm = roomAlarm(rm.id)
      const nm = rm.name + '\n' + Math.round(rm.area || 0) + '㎡ · ' + cat.name + (nCab ? ' · ' + nCab + '柜' : '') + (alarm ? ' · ⚠' + alarm : '')
      const lbl = makeMultiLabel(nm, { scale: Math.max(Math.min(w / 2.6, 1.15), 0.9), alarm: alarm > 0 })
      lbl.position.set(0, hgt + 0.7, 0)
      g.add(lbl)
    }
    // 告警呼吸灯
    if (roomAlarm(rm.id) > 0) {
      const lm = new THREE.MeshBasicMaterial({ color: 0xff5252 })
      const beacon = new THREE.Mesh(new THREE.SphereGeometry(0.14, 10, 10), lm)
      beacon.position.set(w / 2 - 0.3, hgt + 0.25, -d / 2 + 0.3)
      g.add(beacon); blinkMats.push(lm)
    }
    g.position.set(x + w / 2, 0, 0)
    x += w + gap
    sceneRoot.add(g)
  })

  const fLbl = makeLabel((curRoom.value ? '' : '') + (rms[0]?.name?.replace(/\d+机房.*/, '') || '') + curFloor.value + ' 楼层平面', { scale: 0.9 })
  fLbl.position.set(0, 4.6, -maxD / 2 - 1.6)
  sceneRoot.add(fLbl)

  window.__dcim3dFloor = { floor: curFloor.value, rooms: rms.map(r => ({ id: r.id, name: r.name, area: r.area || 0, cat: roomCat(r).key, alarms: roomAlarm(r.id), cabs: roomCabN(r.id) })) }
  const dist = Math.max(totalW, maxD * 2.2) * 0.95
  homeCam = { pos: new THREE.Vector3(0, dist * 1.15, dist * 0.52), look: new THREE.Vector3(0, 0, 0) }
  if (!userMoved) resetView()
}

// 多行标签（楼层房间用）
function makeMultiLabel(text, opts = {}) {
  const lines = String(text).split('\n')
  const scale = opts.scale || 0.7
  const cv = document.createElement('canvas')
  cv.width = 460; cv.height = 60 + lines.length * 44
  const ctx = cv.getContext('2d')
  ctx.fillStyle = opts.alarm ? 'rgba(64,14,20,.9)' : 'rgba(10,22,40,.88)'
  ctx.beginPath(); ctx.roundRect(12, 10, 436, cv.height - 20, 14); ctx.fill()
  ctx.strokeStyle = opts.alarm ? 'rgba(255,82,82,.9)' : 'rgba(140,180,230,.5)'; ctx.lineWidth = 2.5
  ctx.beginPath(); ctx.roundRect(12, 10, 436, cv.height - 20, 14); ctx.stroke()
  ctx.textAlign = 'center'
  lines.forEach((ln, i) => {
    ctx.fillStyle = i === 0 ? '#ffffff' : '#d4e4f7'
    ctx.font = (i === 0 ? 'bold 36px' : 'bold 26px') + ' sans-serif' 
    ctx.fillText(ln, 230, 52 + i * 44)
  })
  const tex = new THREE.CanvasTexture(cv)
  const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, depthTest: false, transparent: true }))
  sp.scale.set(3.4 * scale, (cv.height / cv.width) * 3.4 * scale, 1)
  sp.userData.isLabel = true
  return sp
}

function buildView() {
  if (viewMode.value === 'floor') buildFloorScene()
  else buildRoomScene()
}

// ══════════ 辅助房间场景(电池室/配电室/空调机房/值班室等, 无机柜房间) ══════════
function buildAuxRoomScene(room, cat) {
  const AREA = room.area || 60
  // 房间尺寸: 长宽比≈1.6, 最小6m
  const D = Math.max(Math.sqrt(AREA / 1.6), 5)
  const W = Math.max(AREA / D, 5)
  const wallH = 3.4

  // 地板(地砖纹理)
  const floorTex = makeTileTexture(true)
  floorTex.wrapS = floorTex.wrapT = THREE.RepeatWrapping
  floorTex.repeat.set(W / 2.4, D / 2.4)
  const floor = new THREE.Mesh(new THREE.BoxGeometry(W, 0.12, D),
    new THREE.MeshStandardMaterial({ map: floorTex, color: 0xffffff, roughness: 0.55, metalness: 0.08 }))
  floor.position.y = -0.06; floor.receiveShadow = true
  sceneRoot.add(floor)
  // 分区地面色带(按GB分类色)
  const zoneBand = new THREE.Mesh(new THREE.PlaneGeometry(W - 0.8, D - 0.8),
    new THREE.MeshBasicMaterial({ color: cat.color, transparent: true, opacity: 0.07 }))
  zoneBand.rotation.x = -Math.PI / 2; zoneBand.position.y = 0.02
  sceneRoot.add(zoneBand)

  // 外墙(+Z留门洞) + 踢脚线
  const wallMat = new THREE.MeshStandardMaterial({ color: 0xaeb9c4, transparent: true, opacity: 0.34, roughness: 0.92, metalness: 0.02, side: THREE.DoubleSide })
  const mkWall = (w, d, x, z) => { const m = new THREE.Mesh(new THREE.BoxGeometry(w, wallH, d), wallMat); m.position.set(x, wallH / 2, z); sceneRoot.add(m) }
  mkWall(W, 0.12, 0, -D / 2); mkWall(0.12, D, -W / 2, 0); mkWall(0.12, D, W / 2, 0)
  mkWall(W * 0.34, 0.12, -W * 0.33, D / 2); mkWall(W * 0.34, 0.12, W * 0.33, D / 2)
  const skirtMat = new THREE.MeshStandardMaterial({ color: 0x4a545e, roughness: 0.8 })
  const mkSkirt = (w, d, x, z) => { const m = new THREE.Mesh(new THREE.BoxGeometry(w, 0.12, d), skirtMat); m.position.set(x, 0.06, z); sceneRoot.add(m) }
  mkSkirt(W, 0.16, 0, -D / 2); mkSkirt(0.16, D, -W / 2, 0); mkSkirt(0.16, D, W / 2, 0)
  // 吊顶+LED灯盘
  const ceilPanel = new THREE.Mesh(new THREE.PlaneGeometry(W - 0.2, D - 0.2),
    new THREE.MeshStandardMaterial({ color: 0xd8dde3, roughness: 0.85, metalness: 0.1, side: THREE.FrontSide, transparent: true, opacity: 0.9 }))
  ceilPanel.rotation.x = Math.PI / 2; ceilPanel.position.y = wallH
  sceneRoot.add(ceilPanel)
  const ledMat = new THREE.MeshBasicMaterial({ color: 0xfff8e8 })
  const ledTrimMat = new THREE.MeshStandardMaterial({ color: 0xaab2ba, roughness: 0.6, metalness: 0.4 })
  const nL = Math.max(Math.round(W / 4), 2), nR = Math.max(Math.round(D / 4), 1)
  for (let li = 0; li < nR; li++) for (let lj = 0; lj < nL; lj++) {
    const lx = -W / 2 + (lj + 0.5) * W / nL, lz = -D / 2 + (li + 0.5) * D / nR
    const trim = new THREE.Mesh(new THREE.BoxGeometry(1.25, 0.045, 0.34), ledTrimMat)
    trim.position.set(lx, wallH - 0.022, lz); sceneRoot.add(trim)
    const led = new THREE.Mesh(new THREE.PlaneGeometry(1.15, 0.25), ledMat)
    led.rotation.x = Math.PI / 2; led.position.set(lx, wallH - 0.05, lz); sceneRoot.add(led)
    if (li === 0 && lj === Math.floor(nL / 2)) { const lamp = new THREE.PointLight(0xfff4dd, 0.5, 8, 1.8); lamp.position.set(lx, wallH - 0.25, lz); sceneRoot.add(lamp) }
  }
  // 房间标注(GB分类)
  const titleTag = makeMultiLabel([cat.name, room.name + ' · ' + Math.round(AREA) + '㎡'], { scale: 0.55 })
  titleTag.position.set(0, wallH - 0.7, -D / 2 + 0.7)
  sceneRoot.add(titleTag)

  // 该房间的真实测点(渲染时绑定, 有告警则闪)
  const roomPts = points.value.filter(p => p.room_id === room.id)

  // ── 按分类布置设备 ──
  const steelMat = new THREE.MeshStandardMaterial({ color: 0xc5ccd2, roughness: 0.32, metalness: 0.5, envMapIntensity: 0.4 })
  const darkMat = new THREE.MeshStandardMaterial({ color: 0x2b3038, roughness: 0.35, metalness: 0.55, envMapIntensity: 0.35 })
  const mkBox = (w, h, d, mat, x, y, z, ud) => {
    const m = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), mat)
    m.position.set(x, y, z); m.castShadow = true; m.receiveShadow = true
    if (ud) { m.userData = ud; pickables.push(m) }
    sceneRoot.add(m)
    const ao = new THREE.Mesh(new THREE.PlaneGeometry(w * 1.5, d * 1.5),
      new THREE.MeshBasicMaterial({ color: 0x000000, transparent: true, opacity: 0.24, depthWrite: false }))
    ao.rotation.x = -Math.PI / 2; ao.position.set(x, 0.014, z); sceneRoot.add(ao)
    return m
  }
  const mkTag = (txt, x, y, z, alarm) => {
    if (!showLabels.value) return
    const t = makeLabel(txt, { scale: 0.42, alarm })
    t.position.set(x, y, z); sceneRoot.add(t)
  }
  // 测点绑定工具: 按类型取第i个, 无则装饰占位(id=0)
  const pt = (type, i) => roomPts.filter(p => p.type === type)[i] || null
  const bindPt = (p, name) => p ? { kind: 'point', point: p } : { kind: 'decor', name, sub: cat.name + '设备(示意)' }
  // V34: 台账资产绑定(按名称/型号关键词匹配该房间的资产, 每台只绑一次; 无匹配回退)
  const roomAst = roomAssets.value.filter(a => a.room_id === room.id)
  const usedAst = new Set()
  const takeAst = (re) => { const a = roomAst.find(x => !usedAst.has(x.id) && re.test((x.name || '') + ' ' + (x.model || '') + ' ' + (x.brand || ''))); if (a) usedAst.add(a.id); return a || null }
  const astUd = (re, fallbackUd) => { const a = takeAst(re); return a ? { kind: 'room_asset', asset: a, roomName: room.name } : fallbackUd }

  if (cat.key === 'battery') {
    // ═══ 电池室: 电池架(多层电池组) ×N + 电池巡检仪 + 温湿度 ═══
    const nRack = Math.max(Math.min(Math.floor(W / 2.6), 5), 2)
    const thP = pt('temp_humidity', 0)
    for (let i = 0; i < nRack; i++) {
      const x = -W / 2 + (i + 0.5) * W / nRack
      const rackAst = takeAst(new RegExp('电池架|电池组|蓄电池'))   // V34: 每排电池架绑一台台账电池资产
      // 电池架钢结构
      mkBox(2.2, 0.08, 0.9, steelMat, x, 0.4, 0)
      mkBox(2.2, 0.08, 0.9, steelMat, x, 1.0, 0)
      mkBox(2.2, 0.08, 0.9, steelMat, x, 1.6, 0)
      for (const lx of [-1.0, 1.0]) mkBox(0.07, 1.68, 0.85, darkMat, x + lx, 0.84, 0)
      // 每层12V电池组(蓝色壳体×8/层)
      const batMat = new THREE.MeshStandardMaterial({ color: 0x1565c0, roughness: 0.45, metalness: 0.25 })
      for (let lv = 0; lv < 3; lv++) {
        for (let b = 0; b < 8; b++) {
          const bm = new THREE.Mesh(new THREE.BoxGeometry(0.24, 0.42, 0.7), batMat)
          bm.position.set(x - 0.98 + b * 0.28, 0.44 + lv * 0.6 + 0.21, 0)
          bm.castShadow = true
          bm.userData = rackAst ? { kind: 'room_asset', asset: rackAst, roomName: room.name } : bindPt(lv === 0 && b === 0 && i === 0 ? thP : null, '电池组')
          sceneRoot.add(bm)
          if (!rackAst && lv === 0 && b === 0 && i === 0 && thP) pickables.push(bm)
        }
        // 层间连接铜排
        mkBox(2.0, 0.02, 0.03, new THREE.MeshStandardMaterial({ color: 0xb87333, roughness: 0.4, metalness: 0.8 }), x, 0.44 + lv * 0.6 + 0.44, 0.36)
      }
      mkTag(rackAst ? rackAst.name : '电池架-' + String(i + 1).padStart(2, '0'), x, 2.1, 0, !!(rackAst && rackAst.status === 3))
      if (rackAst) {  // V34: 台账电池资产 → 透明拾取盒(整排可点)
        const pb = new THREE.Mesh(new THREE.BoxGeometry(2.2, 1.7, 0.9),
          new THREE.MeshBasicMaterial({ transparent: true, opacity: 0, depthWrite: false }))
        pb.position.set(x, 0.84, 0)
        pb.userData = { kind: 'room_asset', asset: rackAst, roomName: room.name }
        sceneRoot.add(pb); pickables.push(pb)
      }
    }
    // 电池巡检仪(挂墙)
    mkBox(0.5, 0.7, 0.16, steelMat, W / 2 - 0.3, 1.5, -D / 2 + 0.35, { kind: 'decor', name: '电池巡检仪', sub: '单体电压/内阻/温度在线监测' })
    mkTag('电池巡检仪', W / 2 - 0.3, 2.05, -D / 2 + 0.35, false)
    // 氢气探测器(电池室必配, GB要求)
    const h2 = mkBox(0.22, 0.22, 0.12, new THREE.MeshStandardMaterial({ color: 0xfdd835, roughness: 0.4 }), 0, wallH - 0.4, -D / 2 + 0.3, { kind: 'decor', name: '氢气探测器', sub: '电池充电析氢监测 · 超1%报警联动排风' })
    mkTag('氢气探测器', 0, wallH - 0.05, -D / 2 + 0.3, false)
    // 排风扇(事故排风)
    mkBox(0.8, 0.8, 0.15, darkMat, -W / 2 + 0.25, wallH - 0.8, -D / 2 + 0.3, { kind: 'decor', name: '事故排风机', sub: '氢气浓度超标自动强排' })
    if (thP) { const th = buildSensor(thP, W / 4, D / 4); sceneRoot.add(th) }
  } else if (cat.key === 'power') {
    // ═══ 高低压配电室: 高压柜排 + 低压柜排 + 变压器 + 绝缘垫 ═══
    const pdP = pt('power_dist', 0), upP = pt('ups', 0)
    const nCab = Math.max(Math.min(Math.floor(W / 1.1), 10), 4)
    // 高压柜排(后墙, 深灰+仪表门)
    for (let i = 0; i < nCab; i++) {
      const x = -W / 2 + (i + 0.5) * W / nCab
      const isFirst = i === 0
      mkBox(0.9, 2.2, 0.9, steelMat, x, 1.1, -D / 2 + 0.85,
        astUd(new RegExp(isFirst ? '高压进线|进线柜' : '高压柜|高压'),
        bindPt(isFirst ? pdP : null, isFirst ? '高压进线柜' : '高压柜')))
      // 柜门仪表+指示灯
      const scr = new THREE.Mesh(new THREE.BoxGeometry(0.4, 0.24, 0.02),
        new THREE.MeshBasicMaterial({ color: isFirst && pdP && pdP.status === 2 ? 0xff5252 : 0x36cfc9 }))
      scr.position.set(x, 1.7, -D / 2 + 1.31); sceneRoot.add(scr)
      if (isFirst && pdP && pdP.status === 2) blinkMats.push(scr.material)
      for (let li = 0; li < 3; li++) {
        const led = new THREE.Mesh(new THREE.SphereGeometry(0.025, 6, 6),
          new THREE.MeshBasicMaterial({ color: [0x00e676, 0xffc107, 0xff5252][li] }))
        led.position.set(x - 0.25 + li * 0.1, 1.35, -D / 2 + 1.31); sceneRoot.add(led)
      }
      mkTag(i === 0 ? '高压进线柜' : '高压柜-' + String(i + 1).padStart(2, '0'), x, 2.5, -D / 2 + 0.85, isFirst && pdP && pdP.status === 2)
    }
    // 干式变压器×2(中部, 散热片+围栏)
    const nTx = AREA >= 100 ? 2 : 1
    for (let i = 0; i < nTx; i++) {
      const x = -W / 4 + i * W / 2
      mkBox(1.5, 1.7, 1.0, new THREE.MeshStandardMaterial({ color: 0x78909c, roughness: 0.5, metalness: 0.55 }), x, 0.85, 0.4,
        astUd(new RegExp('变压器'),
        { kind: 'decor', name: '干式变压器-' + String(i + 1).padStart(2, '0'), sub: '10kV/0.4kV 树脂浇注 · 强制风冷' }))
      // 散热鳍片
      for (let f = 0; f < 6; f++) mkBox(0.04, 1.3, 1.05, new THREE.MeshStandardMaterial({ color: 0x607d8b, roughness: 0.5, metalness: 0.6 }), x - 0.6 + f * 0.24, 0.85, 0.4)
      // 防护围栏
      const fenceMat = new THREE.MeshStandardMaterial({ color: 0xfdd835, roughness: 0.6 })
      for (const fz of [-0.45, 1.25]) mkBox(2.1, 1.2, 0.05, fenceMat, x, 0.6, fz)
      for (const fx of [-1.0, 1.0]) mkBox(0.05, 1.2, 1.75, fenceMat, fx + x, 0.6, 0.4)
      mkTag('电 变压器-' + String(i + 1).padStart(2, '0'), x, 2.1, 0.4, false)
    }
    // 低压柜排(前墙)
    const nLv = Math.max(Math.min(Math.floor(W / 1.0), 8), 3)
    for (let i = 0; i < nLv; i++) {
      const x = -W / 2 + (i + 0.5) * W / nLv
      mkBox(0.85, 2.0, 0.7, steelMat, x, 1.0, D / 2 - 0.75,
        astUd(new RegExp(i === 0 ? '低压总柜|低压进线' : '低压柜|出线柜|低压'),
        bindPt(i === 0 ? upP : null, i === 0 ? '低压总柜' : '低压出线柜')))
      mkTag(i === 0 ? '⚡ 低压总柜' : '出线柜-' + String(i + 1).padStart(2, '0'), x, 2.35, D / 2 - 0.75, false)
    }
    // 绝缘胶垫(操作区)
    const mat = new THREE.Mesh(new THREE.PlaneGeometry(W * 0.7, 1.0),
      new THREE.MeshStandardMaterial({ color: 0x8d1f1f, roughness: 0.95 }))
    mat.rotation.x = -Math.PI / 2; mat.position.set(0, 0.03, -D / 2 + 2.0); sceneRoot.add(mat)
    mkTag('▬ 绝缘操作区', 0, 0.3, -D / 2 + 2.0, false)
  } else if (cat.key === 'hvac') {
    // ═══ 空调机房: 冷水机组 + 冷冻水泵×2 + 板换 + 分集水器 ═══
    const hvP = pt('hvac', 0)
    // 冷水机组(主机, 占中部)
    const chW = Math.min(W * 0.55, 4.5)
    mkBox(chW, 1.6, 1.2, new THREE.MeshStandardMaterial({ color: 0x37474f, roughness: 0.4, metalness: 0.55 }), 0, 0.8, -D / 4,
      astUd(new RegExp('冷水机|冷机|chiller'), bindPt(hvP, '冷水机组')))
    // 机组控制面板
    const cp = new THREE.Mesh(new THREE.BoxGeometry(0.6, 0.4, 0.03),
      new THREE.MeshBasicMaterial({ color: hvP && hvP.status === 2 ? 0xff5252 : 0x00e676 }))
    cp.position.set(chW / 2 - 0.5, 1.3, -D / 4 + 0.62); sceneRoot.add(cp)
    if (hvP && hvP.status === 2) blinkMats.push(cp.material)
    mkTag('❄ 冷水机组', 0, 2.0, -D / 4, !!(hvP && hvP.status === 2))
    // 压缩机 Cylinder ×2
    for (const cx of [-chW / 4, chW / 4]) {
      const comp = new THREE.Mesh(new THREE.CylinderGeometry(0.28, 0.28, chW * 0.36, 14),
        new THREE.MeshStandardMaterial({ color: 0x546e7a, roughness: 0.35, metalness: 0.65 }))
      comp.rotation.z = Math.PI / 2; comp.position.set(cx, 1.75, -D / 4); comp.castShadow = true
      sceneRoot.add(comp)
    }
    // 冷冻水泵×2(绿色泵体+电机)
    for (let i = 0; i < 2; i++) {
      const x = -W / 4 + i * W / 2
      const pump = new THREE.Mesh(new THREE.CylinderGeometry(0.32, 0.32, 0.55, 14),
        new THREE.MeshStandardMaterial({ color: 0x2e7d32, roughness: 0.4, metalness: 0.5 }))
      pump.position.set(x, 0.45, D / 4); pump.castShadow = true
      pump.userData = astUd(new RegExp('水泵|冷冻泵'), { kind: 'decor', name: '冷冻水泵-' + (i + 1), sub: i === 0 ? '主泵 · 变频' : '备泵 · 自动切换' })
      sceneRoot.add(pump); pickables.push(pump)
      mkBox(0.5, 0.4, 0.4, darkMat, x + 0.55, 0.42, D / 4)  // 电机
      mkTag('冷 冷冻泵-' + (i + 1), x, 1.1, D / 4, false)
    }
    // 板式换热器
    mkBox(0.9, 1.4, 0.6, new THREE.MeshStandardMaterial({ color: 0x90a4ae, roughness: 0.3, metalness: 0.8 }), W / 2 - 0.9, 0.7, 0,
      astUd(new RegExp('换热器|板换'), { kind: 'decor', name: '板式换热器', sub: '一次侧/二次侧隔离换热' }))
    mkTag('♨ 板换', W / 2 - 0.9, 1.75, 0, false)
    // 分集水器(横管+支阀)
    const headerMat = new THREE.MeshStandardMaterial({ color: 0x29b6f6, roughness: 0.35, metalness: 0.6 })
    const hdr = new THREE.Mesh(new THREE.CylinderGeometry(0.14, 0.14, W * 0.6, 12), headerMat)
    hdr.rotation.z = Math.PI / 2; hdr.position.set(0, 2.5, D / 2 - 0.5); sceneRoot.add(hdr)
    for (let v = 0; v < 4; v++) mkBox(0.06, 0.3, 0.06, new THREE.MeshStandardMaterial({ color: 0xef5350, roughness: 0.4 }), -W * 0.24 + v * W * 0.16, 2.28, D / 2 - 0.5)
    mkTag('水 分集水器', 0, 2.95, D / 2 - 0.5, false)
    // 管道(供水蓝/回水橙)
    for (const [pz, pc] of [[-D / 4 + 0.75, 0x29b6f6], [-D / 4 - 0.75, 0xff7043]]) {
      const pipe = new THREE.Mesh(new THREE.CylinderGeometry(0.09, 0.09, W * 0.8, 10),
        new THREE.MeshStandardMaterial({ color: pc, roughness: 0.4, metalness: 0.4 }))
      pipe.rotation.z = Math.PI / 2; pipe.position.set(0, 2.15, pz); sceneRoot.add(pipe)
    }
  } else {
    // ═══ 值班/监控/总控室(ops+other): 操作台排 + 监控大屏墙 + 资料柜 ═══
    const nConsole = Math.max(Math.min(Math.floor(W / 2.2), 4), 1)
    for (let i = 0; i < nConsole; i++) {
      const x = -W / 2 + (i + 0.5) * W / nConsole
      sceneRoot.add(buildConsole(x, D / 2 - 1.6, Math.PI))
    }
    // 监控大屏墙(北墙, 2×4拼)
    const scrW = Math.min(W * 0.75, 6.4)
    sceneRoot.add(buildScreenWall(0, -D / 2 + 0.5, 0, scrW, scrW * 0.32))
    // 值班记录台+资料柜
    mkBox(0.9, 1.8, 0.4, new THREE.MeshStandardMaterial({ color: 0x455a64, roughness: 0.7 }), W / 2 - 0.7, 0.9, D / 2 - 1.0,
      { kind: 'decor', name: '资料柜', sub: '运维文档 / 值班记录 / 备件登记' })
    // 消防控制盘(值班室标配)
    mkBox(0.7, 1.6, 0.25, new THREE.MeshStandardMaterial({ color: 0xc62828, roughness: 0.5 }), -W / 2 + 0.5, 0.8, -D / 2 + 0.5,
      astUd(new RegExp('消防|报警控制|火灾报警'), { kind: 'decor', name: '消防报警控制盘', sub: '火灾自动报警系统主机 · 联动控制' }))
    mkTag('消 消防控制盘', -W / 2 + 0.5, 1.9, -D / 2 + 0.5, false)
    // 安防监控NVR
    mkBox(0.5, 0.12, 0.4, darkMat, W / 2 - 0.6, 0.78, -D / 2 + 0.6,
      astUd(new RegExp('NVR|录像|视频监控'), { kind: 'decor', name: 'NVR 录像主机', sub: '安防视频存储 30 天' }))
    // 该房间的温湿度测点(若有)
    const thP = pt('temp_humidity', 0)
    if (thP) sceneRoot.add(buildSensor(thP, W / 4, 0))
  }

  // 相机
  const dist = Math.max(W, D) * 1.05
  homeCam = { pos: new THREE.Vector3(dist * 0.15, dist * 0.75, dist * 0.95), look: new THREE.Vector3(0, 0.8, 0) }
  camera.position.copy(homeCam.pos)
  controls.target.copy(homeCam.look)
  userMoved = false
}

function buildRoomScene() {
  if (!scene) return
  if (sceneRoot) { scene.remove(sceneRoot); disposeGroup(sceneRoot) }
  pickables = []; blinkMats = []; devPop.show = false
  sceneRoot = new THREE.Group()
  scene.add(sceneRoot)

  // V23: 3D 场景固定深色渲染环境(真实机房可视化惯例), 不随浅色UI主题变白
  const bgHex = '#0d1520'
  scene.background = new THREE.Color(bgHex)
  scene.fog = new THREE.Fog(new THREE.Color(bgHex).getHex(), 55, 150)
  const isDark = true

  airflowSys = []; lastAirT = 0   // V32 粒子系统随场景重建
  const room = curRoom.value
  if (!room) return
  const list = roomCabs.value
  if (!list.length) {
    // 辅助房间(电池室/配电室/空调机房/值班室等): 按GB分类布置对应设备
    buildAuxRoomScene(room, roomCat(room))
    renderCustomItems()
    return
  }

  // ══ V20: 按面积推导整体建筑尺寸，X 方向功能分区 ══
  // [配电区 | IT机柜区 | 空调设备间 | 操作间] + 建筑外(+Z侧) 空调室外机区
  const rowsMap = {}
  list.forEach(c => { const k = c.row_id || 0; (rowsMap[k] = rowsMap[k] || []).push(c) })
  let rk = Object.keys(rowsMap)
  if (rk.length === 1 && rk[0] === '0' && list.length > 6) {
    delete rowsMap['0']
    list.forEach((c, i) => { const k = 'v' + Math.floor(i / 6); (rowsMap[k] = rowsMap[k] || []).push(c) })
    rk = Object.keys(rowsMap)
  }
  const maxRowLen = Math.max(...rk.map(k => rowsMap[k].length), 1)
  const nRows = rk.length
  const nPairs = Math.ceil(nRows / 2)
  const pairPitch = CAB_D * 2 + COLD_AISLE + HOT_AISLE
  const rowLen = maxRowLen * CAB_PITCH

  const AREA = room.area || 200
  // ══ GB 50174-2017 功能区面积比例规划 ══
  // 4.2.1  数据中心宜由主机房、辅助区、支持区、行政管理区组成
  // 4.2.3  辅助区+支持区面积之和 = 主机房的 1.5~2.5 倍
  //        大中型(≥300㎡)取中值 2.0；小型取下限 1.5（条文说明：功率密度越低辅助区越小）
  // 4.2.2  主机房按机柜数×2~4㎡/台 并预留发展面积（富余宽度画预留扩展位）
  // 术语2.1.3/2.1.4  支持区=变配电室/电池室/空调机房；辅助区=总控中心/值班室/安防消防控制室
  const RATIO_AUX = AREA >= 300 ? 2.0 : 1.5          // 辅助+支持 : 主机房
  const R_IT = 1 / (1 + RATIO_AUX)                   // 主机房占比 (2.0倍→33.3% / 1.5倍→40%)
  const R_AUX = RATIO_AUX / (1 + RATIO_AUX)          // 辅助+支持合计占比
  const R_PW = R_AUX * 0.40                          // 支持区·变配电/电池室
  const R_HV = R_AUX * 0.30                          // 支持区·空调机房
  const R_OP = R_AUX * 0.30                          // 辅助区·总控/值班/安防
  // IT 区物理需求（机柜阵列最小占地）
  const itNeedW = Math.max(nPairs * pairPitch + 1.6, 6)
  const itNeedD = Math.max(rowLen + 3.0, 8)
  const aisleW = AREA < 300 ? 1.0 : 1.5              // 区间走道
  const edge = 0.8                                   // 建筑边缘余量
  // 建筑轮廓：总面积≈AREA、长宽比≈1.8，且 IT 区份额必须装得下机柜阵列
  let D = Math.max(Math.sqrt(AREA / 1.8), itNeedD, 9)
  let W = Math.max(AREA / D, itNeedW / R_IT + edge * 2 + aisleW * 3)
  const usable = W - edge * 2 - aisleW * 3
  // 各区宽度 = 可用宽度 × 面积占比（各区等深，宽比即面积比）
  const itW = Math.max(usable * R_IT, itNeedW)
  const pwW = usable * R_PW
  const hvW = usable * R_HV
  const opW = usable * R_OP
  // 各区面积与合规比例（标注/验证用）
  const zoneD = D - 1.6
  const aPw = pwW * zoneD, aIt = itW * zoneD, aHv = hvW * zoneD, aOp = opW * zoneD
  const aSum = aPw + aIt + aHv + aOp
  const pct = a => Math.round(a / aSum * 100)
  const gbRatio = ((aPw + aHv + aOp) / aIt).toFixed(1)   // 实际 辅助+支持:主机房

  // 分区 X 坐标（西→东：变配电室 | 主机房 | 空调机房 | 总控值班室）
  const xPower0 = -W / 2 + edge
  const xIt0 = xPower0 + pwW + aisleW
  const xHvac0 = xIt0 + itW + aisleW
  const xOps0 = xHvac0 + hvW + aisleW

  // V23 地板: 600mm 防静电地砖程序化纹理（浅色·缝隙·斑点），替代线框网格
  const floorTex = makeTileTexture(isDark)
  floorTex.wrapS = floorTex.wrapT = THREE.RepeatWrapping
  floorTex.repeat.set(W / 2.4, D / 2.4)
  floorTex.anisotropy = 4
  const floor = new THREE.Mesh(new THREE.BoxGeometry(W, 0.12, D),
    new THREE.MeshStandardMaterial({ map: floorTex, color: 0xffffff, roughness: 0.55, metalness: 0.08 }))
  floor.position.y = -0.06; floor.receiveShadow = true
  sceneRoot.add(floor)

  // 外墙（+Z 侧留门洞）
  const wallH = 3.6
  const wallMat = new THREE.MeshStandardMaterial({ color: 0xaeb9c4, transparent: true, opacity: 0.34, roughness: 0.92, metalness: 0.02, side: THREE.DoubleSide })
  const mkWall = (w, d, x, z) => { const m = new THREE.Mesh(new THREE.BoxGeometry(w, wallH, d), wallMat); m.position.set(x, wallH / 2, z); sceneRoot.add(m) }
  mkWall(W, 0.12, 0, -D / 2)
  mkWall(0.12, D, -W / 2, 0)
  mkWall(0.12, D, W / 2, 0)
  mkWall(W * 0.34, 0.12, -W * 0.33, D / 2)
  mkWall(W * 0.34, 0.12, W * 0.33, D / 2)
  // 踢脚线（不透明深色,增强"实体建筑"感）
  const skirtMat = new THREE.MeshStandardMaterial({ color: 0x4a545e, roughness: 0.8 })
  const mkSkirt = (w, d, x, z) => { const m = new THREE.Mesh(new THREE.BoxGeometry(w, 0.12, d), skirtMat); m.position.set(x, 0.06, z); sceneRoot.add(m) }
  mkSkirt(W, 0.16, 0, -D / 2); mkSkirt(0.16, D, -W / 2, 0); mkSkirt(0.16, D, W / 2, 0)
  // V23 吊顶: 微孔铝扣板(FrontSide朝下——室内可见,俯视自动隐去不遮挡) + LED灯盘
  const ceilMat = new THREE.MeshStandardMaterial({ color: 0xd8dde3, roughness: 0.85, metalness: 0.1, side: THREE.FrontSide, transparent: true, opacity: 0.9 })
  const ceilPanel = new THREE.Mesh(new THREE.PlaneGeometry(W - 0.2, D - 0.2), ceilMat)
  ceilPanel.rotation.x = Math.PI / 2   // 法线朝下
  ceilPanel.position.y = wallH
  sceneRoot.add(ceilPanel)
  // LED 灯盘: 3行×每行按房间长度均布, 发光面+微弱光晕
  const ledMat = new THREE.MeshBasicMaterial({ color: 0xfff8e8 })
  const ledTrimMat = new THREE.MeshStandardMaterial({ color: 0xaab2ba, roughness: 0.6, metalness: 0.4 })
  const nLedRow = Math.max(Math.round(D / 4), 2)
  const nLedCol = Math.max(Math.round(W / 5), 2)
  let lampCount = 0
  const maxLamps = 6  // V24: 最多6盏真光源(性能保护), 其余仅发光面
  for (let li = 0; li < nLedRow; li++) {
    for (let lj = 0; lj < nLedCol; lj++) {
      const lx = -W / 2 + (lj + 0.5) * W / nLedCol
      const lz = -D / 2 + (li + 0.5) * D / nLedRow
      // 嵌入式灯盘: 外框(凸出顶面2cm) + 发光面(略低于框, 形成凹槽)
      const trim = new THREE.Mesh(new THREE.BoxGeometry(1.3, 0.045, 0.36), ledTrimMat)
      trim.position.set(lx, wallH - 0.022, lz)
      sceneRoot.add(trim)
      const led = new THREE.Mesh(new THREE.PlaneGeometry(1.18, 0.26), ledMat)
      led.rotation.x = Math.PI / 2
      led.position.set(lx, wallH - 0.05, lz)
      sceneRoot.add(led)
      // 中央几盏加真实 PointLight: 地面/机柜顶有灯光落感
      if (lampCount < maxLamps && li % 2 === 0) {
        const lamp = new THREE.PointLight(0xfff4dd, 0.55, 7, 1.8)
        lamp.position.set(lx, wallH - 0.25, lz)
        sceneRoot.add(lamp)
        lampCount++
      }
    }
  }

  // 分区隔断墙（配电区/空调间/操作间与IT区之间，留门洞）
  // V24 玻璃隔断: MeshPhysicalMaterial(高光反射+边缘泛白, 真实玻璃感)
  const partMat = new THREE.MeshPhysicalMaterial({ color: isDark ? 0x2a3f5e : 0x5a7288, transparent: true, opacity: 0.3, roughness: 0.08, metalness: 0.05, clearcoat: 1.0, clearcoatRoughness: 0.1, reflectivity: 0.6, side: THREE.DoubleSide })
  const mkPartition = (x) => {
    const doorW = 1.1
    const segD = (D - doorW) / 2
    const frameMat2 = new THREE.MeshStandardMaterial({ color: 0x8b959e, roughness: 0.35, metalness: 0.7 })
    for (const zz of [-(D / 2 - segD / 2), (D / 2 - segD / 2)]) {
      const m = new THREE.Mesh(new THREE.BoxGeometry(0.06, wallH * 0.92, segD), partMat)
      m.position.set(x, wallH * 0.46, zz); sceneRoot.add(m)
      // 上下金属框
      for (const fy of [0.05, wallH * 0.92 - 0.03]) {
        const rail = new THREE.Mesh(new THREE.BoxGeometry(0.09, 0.06, segD), frameMat2)
        rail.position.set(x, fy, zz); sceneRoot.add(rail)
      }
    }
  }
  mkPartition(xIt0 - aisleW / 2)          // 配电区|IT区
  mkPartition(xHvac0 - aisleW / 2)        // IT区|空调间
  mkPartition(xOps0 - aisleW / 2)         // 空调间|操作间

  // ══ 分区定义（地面色带 + 边框 + 标注）══
  const zones = [
    { key: 'power', name: '⚡ 变配电/电池室', sub: '支持区 · ' + Math.round(aPw) + '㎡ · ' + pct(aPw) + '%', x0: xPower0, z0: -D / 2 + 0.8, w: pwW, d: zoneD, color: 0xffa726, area: Math.round(aPw), pct: pct(aPw) },
    { key: 'it',    name: '主机房',       sub: 'IT机柜区 · ' + Math.round(aIt) + '㎡ · ' + pct(aIt) + '%', x0: xIt0,    z0: -D / 2 + 0.8, w: itW, d: zoneD, color: 0x409eff, area: Math.round(aIt), pct: pct(aIt) },
    { key: 'hvac',  name: '❄ 空调机房',     sub: '支持区 · ' + Math.round(aHv) + '㎡ · ' + pct(aHv) + '%', x0: xHvac0,  z0: -D / 2 + 0.8, w: hvW, d: zoneD, color: 0x26c6da, area: Math.round(aHv), pct: pct(aHv) },
    { key: 'ops',   name: '总控/值班室', sub: '辅助区 · ' + Math.round(aOp) + '㎡ · ' + pct(aOp) + '%', x0: xOps0,   z0: -D / 2 + 0.8, w: opW, d: zoneD, color: 0x66bb6a, area: Math.round(aOp), pct: pct(aOp) },
  ]
  const outX0 = -W / 2 + 0.6, outW2 = W - 1.2
  const outdoor = { key: 'outdoor', name: '室外机区', sub: '建筑外 · ' + Math.round(outW2 * OUT_W) + '㎡', x0: outX0, z0: D / 2 + OUT_GAP, w: outW2, d: OUT_W, color: 0x78909c, area: Math.round(outW2 * OUT_W), pct: 0 }
  zones.forEach(z => {
    sceneRoot.add(zoneFloor(z.x0, z.z0, z.w, z.d, z.color))
    sceneRoot.add(zoneBorder(z.x0, z.z0, z.w, z.d, z.color))
    sceneRoot.add(zoneTag(z))
  })
  // 室外机区：地面（建筑外）+ 围栏
  sceneRoot.add(zoneFloor(outdoor.x0, outdoor.z0, outdoor.w, outdoor.d, outdoor.color, 0.14))
  sceneRoot.add(zoneBorder(outdoor.x0, outdoor.z0, outdoor.w, outdoor.d, outdoor.color))
  const otag = makeLabel(outdoor.name, { scale: 0.62, bg: 'rgba(10,22,40,.9)', border: 'rgba(255,255,255,.3)' })
  otag.position.set(0, 0.55, outdoor.z0 + outdoor.d + 0.55)
  sceneRoot.add(otag)
  sceneRoot.add(buildFence(outX0, outdoor.z0, outX0 + outW2, outdoor.z0))                    // 靠建筑侧
  sceneRoot.add(buildFence(outX0, outdoor.z0 + OUT_W, outX0 + outW2, outdoor.z0 + OUT_W))    // 外侧
  sceneRoot.add(buildFence(outX0, outdoor.z0, outX0, outdoor.z0 + OUT_W))                    // 左
  sceneRoot.add(buildFence(outX0 + outW2, outdoor.z0, outX0 + outW2, outdoor.z0 + OUT_W))    // 右

  // ══ ① 配电区：按供电链路树排布（市电进线→变压器→UPS+电池→配电柜）══
  const pn = powerNodes.value || []
  const kindOrder = { utility: 0, transformer: 1, ups: 2, pdu: 3 }
  const sortedNodes = [...pn].sort((a, b) => (kindOrder[a.kind] ?? 9) - (kindOrder[b.kind] ?? 9) || a.id - b.id)
  const pnPerCol = Math.max(Math.ceil(sortedNodes.length / 2), 1)
  sortedNodes.forEach((nd, i) => {
    const col = Math.floor(i / pnPerCol)
    const row = i % pnPerCol
    const x = xPower0 + pwW * 0.32 + col * Math.min(pwW * 0.36, 1.5)
    const z = -D / 2 + 2.0 + row * Math.min((D - 4) / Math.max(pnPerCol, 1), 1.7)
    sceneRoot.add(buildPowerCab(nd, x, z, Math.PI / 2))
  })
  // 无链路数据时占位（保证分区不空）
  if (!sortedNodes.length) {
    for (let i = 0; i < 3; i++) {
      sceneRoot.add(buildPowerCab({ id: 0, name: '配电柜-' + (i + 1), kind: 'pdu', rated_kva: 0, load_kw: 0, status: 1 },
        xPower0 + pwW * 0.4, -D / 2 + 2.2 + i * 1.8, Math.PI / 2))
    }
  }
  // 配电区母线桥架（沿分区中线走纵向）+ V24 吊杆支撑
  const busX = xPower0 + pwW * 0.78
  const busbar = new THREE.Mesh(new THREE.BoxGeometry(0.18, 0.1, D - 3),
    new THREE.MeshStandardMaterial({ color: 0x8d6e63, roughness: 0.5, metalness: 0.5 }))
  busbar.position.set(busX, 2.9, 0); busbar.castShadow = true; sceneRoot.add(busbar)
  const rodMat = new THREE.MeshStandardMaterial({ color: 0x9aa3ab, roughness: 0.4, metalness: 0.75 })
  const nBusRod = Math.max(Math.round((D - 3) / 3.5), 2)
  for (let bi = 0; bi < nBusRod; bi++) {
    const bz = -(D - 3) / 2 + (bi + 0.5) * (D - 3) / nBusRod
    const rod = new THREE.Mesh(new THREE.CylinderGeometry(0.012, 0.012, wallH - 2.95, 6), rodMat)
    rod.position.set(busX, 2.95 + (wallH - 2.95) / 2, bz); sceneRoot.add(rod)
  }

  // ══ ② IT 机柜区：面对面成对，冷通道1.2m ══
  const startX = xIt0 + itW / 2 - (nPairs - 1) * pairPitch / 2
  rk.forEach((kk, ri) => {
    const pair = Math.floor(ri / 2)
    const side = ri % 2 === 0 ? -1 : 1
    const cx = startX + pair * pairPitch + side * (COLD_AISLE / 2 + CAB_D / 2)
    const rowCabs = rowsMap[kk]
    const n = rowCabs.length
    rowCabs.forEach((c, i) => {
      let z = -(n - 1) * CAB_PITCH / 2 + i * CAB_PITCH
      let px = cx
      let rot = side === -1 ? Math.PI / 2 : -Math.PI / 2
      // V26 自定义布局覆盖
      const ov = layoutOverrides.value['cab-' + c.id]
      if (ov) { px = ov.x; z = ov.z; if (ov.rot_y != null) rot = ov.rot_y }
      const cabG = buildCabinet(c, px, z, rot)
      cabG.userData = { kind: 'cabinet', cab: c, layoutKey: 'cab-' + c.id }
      sceneRoot.add(cabG)
    })
    if (showAisle.value) {
      const rowObj = rows.value.find(r => r.id === Number(kk))
      const nm = rowObj ? rowObj.name : ('第' + (ri + 1) + '列')
      const lbl = makeLabel(nm, { scale: 0.45 })
      lbl.position.set(cx, 0.3, -(maxRowLen) * CAB_PITCH / 2 - 0.9)
      sceneRoot.add(lbl)
    }
  })
  // ══ V25 冷通道封闭(Cold Aisle Containment): 玻璃天窗+端门+铝框 ══
  if (containCold.value) {
    const glassMat = new THREE.MeshPhysicalMaterial({
      color: 0x9fd4ff, transparent: true, opacity: 0.3, roughness: 0.05,
      metalness: 0, clearcoat: 1.0, clearcoatRoughness: 0.05,
      reflectivity: 0.5, side: THREE.DoubleSide, depthWrite: false
    })
    const alumMat = new THREE.MeshStandardMaterial({ color: 0x8f9aa4, roughness: 0.3, metalness: 0.85, envMapIntensity: 0.5 })
    const contH = CAB_H + 0.02          // 天窗盖在柜顶
    const contLen = rowLen + 0.24
    for (let p = 0; p < nPairs; p++) {
      const cx = startX + p * pairPitch
      const cg = new THREE.Group()
      // 天窗玻璃(微拱双坡——真实机房常见斜面天窗)
      for (const side of [-1, 1]) {
        const pane = new THREE.Mesh(new THREE.PlaneGeometry(COLD_AISLE / 2 + 0.04, contLen), glassMat)
        pane.rotation.set(-Math.PI / 2, 0, side * 0.06)
        pane.position.set(side * COLD_AISLE / 4, contH + 0.03, 0)
        cg.add(pane)
      }
      // 天窗中脊+两侧铝梁
      for (const bx of [0, -COLD_AISLE / 2 - 0.02, COLD_AISLE / 2 + 0.02]) {
        const beam = new THREE.Mesh(new THREE.BoxGeometry(0.05, 0.05, contLen), alumMat)
        beam.position.set(bx, contH + (bx === 0 ? 0.055 : 0.02), 0)
        beam.castShadow = true
        cg.add(beam)
      }
      // 两端: 铝框+玻璃推拉门(中缝把手)
      for (const ez of [-contLen / 2, contLen / 2]) {
        const frame = new THREE.Mesh(new THREE.BoxGeometry(COLD_AISLE + 0.1, CAB_H + 0.08, 0.05), alumMat)
        frame.position.set(0, (CAB_H + 0.08) / 2, ez)
        cg.add(frame)
        const doorGlass = new THREE.Mesh(new THREE.PlaneGeometry(COLD_AISLE - 0.08, CAB_H - 0.1), glassMat)
        doorGlass.position.set(0, CAB_H / 2, ez + (ez < 0 ? 0.03 : -0.03))
        cg.add(doorGlass)
        const handle = new THREE.Mesh(new THREE.BoxGeometry(0.03, 0.3, 0.03),
          new THREE.MeshStandardMaterial({ color: 0x666e76, roughness: 0.3, metalness: 0.9 }))
        handle.position.set(0.1, CAB_H * 0.55, ez + (ez < 0 ? 0.05 : -0.05))
        cg.add(handle)
      }
      // 隐形拾取盒: 玻璃面透明难命中, 用整体包围盒接收点击
      const hitBox = new THREE.Mesh(new THREE.BoxGeometry(COLD_AISLE + 0.15, CAB_H + 0.1, contLen),
        new THREE.MeshBasicMaterial({ transparent: true, opacity: 0, depthWrite: false }))
      hitBox.position.set(0, (CAB_H + 0.1) / 2, 0)
      hitBox.userData = { kind: 'decor', name: '冷通道封闭', sub: '冷热通道隔离 · 提升制冷效率(GB 50174 §7.3)', noDrag: true }
      cg.add(hitBox); pickables.push(hitBox)
      cg.position.x = cx
      cg.userData = { kind: 'decor', name: '冷通道封闭', sub: '冷热通道隔离 · 提升制冷效率(GB 50174 §7.3)' }
      sceneRoot.add(cg)
    }
  }

  // 冷/热通道色带
  if (showAisle.value) {
    for (let p = 0; p < nPairs; p++) {
      const cx = startX + p * pairPitch
      const cold = new THREE.Mesh(new THREE.PlaneGeometry(COLD_AISLE, rowLen + 0.6),
        new THREE.MeshBasicMaterial({ color: 0x1e88e5, transparent: true, opacity: 0.15 }))
      cold.rotation.x = -Math.PI / 2; cold.position.set(cx, 0.028, 0)
      sceneRoot.add(cold)
      const coldLbl = makeLabel('❄ 冷通道', { scale: 0.45, bg: 'rgba(30,136,229,.22)', border: 'rgba(30,136,229,.65)' })
      coldLbl.position.set(cx, 0.22, rowLen / 2 + 0.9)
      sceneRoot.add(coldLbl)
      if (p < nPairs - 1) {
        const hx = cx + pairPitch / 2
        const hot = new THREE.Mesh(new THREE.PlaneGeometry(HOT_AISLE, rowLen + 0.6),
          new THREE.MeshBasicMaterial({ color: 0xff7043, transparent: true, opacity: 0.12 }))
        hot.rotation.x = -Math.PI / 2; hot.position.set(hx, 0.028, 0)
        sceneRoot.add(hot)
        const hotLbl = makeLabel('热通道', { scale: 0.45, bg: 'rgba(255,112,67,.22)', border: 'rgba(255,112,67,.65)' })
        hotLbl.position.set(hx, 0.22, rowLen / 2 + 0.9)
        sceneRoot.add(hotLbl)
      }
    }
  }
  // ══ V32 气流组织: 冷通道送风粒子(蓝↑) + 热通道排热粒子(橙↑) + 温湿度热区 ══
  if (airflowOn.value) {
    for (let p = 0; p < nPairs; p++) {
      const cx = startX + p * pairPitch
      const sysC = makeAirflowSystem(cx, 0, COLD_AISLE * 0.75, rowLen * 0.92, 64, 0x64b5f6, CAB_H + 0.5)
      sceneRoot.add(sysC.pts); airflowSys.push(sysC)
      if (p < nPairs - 1) {
        const hx = cx + pairPitch / 2
        const sysH = makeAirflowSystem(hx, 0, HOT_AISLE * 0.7, rowLen * 0.92, 44, 0xff8a65, 3.1)
        sceneRoot.add(sysH.pts); airflowSys.push(sysH)
      }
    }
    // 空调送风气流(空调间→IT区横向粒子, 示意冷冻水空调送风路径)
    const sysHvac = makeAirflowSystem((xHvac0 + xIt0) / 2, 0, Math.max(xIt0 - xHvac0 - 0.5, 0.5), D * 0.6, 30, 0x4dd0e1, 2.4)
    sceneRoot.add(sysHvac.pts); airflowSys.push(sysHvac)
    // 温湿度热区(与 buildSensor 定位一致: 冷通道吸顶投影到地板)
    // 注: 此处早于下方 const th 定义, 自行计算避免 TDZ
    const thAir = points.value.filter(p => p.room_id === roomId.value && p.type === 'temp_humidity')
    thAir.forEach((p, i) => {
      const pair = Math.floor(i / 2) % Math.max(nPairs, 1)
      const hx2 = startX + pair * pairPitch + (i % 2 === 0 ? -0.3 : 0.3)
      const hz = (Math.floor(i / (2 * Math.max(nPairs, 1))) % 2 === 0 ? -1 : 1) * rowLen / 4
      const v = p.current_value != null ? Number(p.current_value) : null
      const disc = new THREE.Mesh(new THREE.PlaneGeometry(2.4, 2.4),
        new THREE.MeshBasicMaterial({ map: makeHeatTexture(v), transparent: true, depthWrite: false }))
      disc.rotation.x = -Math.PI / 2
      disc.position.set(hx2, 0.045, hz)
      disc.userData = { kind: 'point', point: p, noDrag: true }
      sceneRoot.add(disc); pickables.push(disc)
      if (showLabels.value && v != null) {
        const hot = v >= 26
        const tag = makeLabel('温 ' + v.toFixed(1) + '°C', { scale: 0.4, alarm: hot, border: hot ? undefined : 'rgba(102,187,106,.6)' })
        tag.position.set(hx2, 0.35, hz)
        sceneRoot.add(tag)
      }
    })
    // 气流图例
    const lgd = makeMultiLabel(['气流组织', '蓝=冷通道送风(地板下静压箱)', '橙=热通道排热上升', '地面热区: 蓝<22 绿22-26 橙26-30 红≥30°C'], { scale: 0.5 })
    lgd.position.set(xIt0 + 1.2, 2.2, -D / 2 + 0.9)
    sceneRoot.add(lgd)
  }

  // GB50174 4.2.2 预留发展面积：IT 区富余宽度画「预留扩展位」虚线框
  const sparePairs = Math.floor((itW - 1.2 - nPairs * pairPitch) / pairPitch)
  if (sparePairs > 0 && showAisle.value) {
    const dashMat = new THREE.LineDashedMaterial({ color: 0x409eff, dashSize: 0.35, gapSize: 0.22, transparent: true, opacity: 0.55 })
    const nSpare = Math.min(sparePairs, 6)
    for (let sp = 0; sp < nSpare; sp++) {
      const cx = startX + (nPairs + sp) * pairPitch
      if (cx + CAB_D > xIt0 + itW - 0.4) break
      const rect = new THREE.BufferGeometry().setFromPoints([
        new THREE.Vector3(cx - CAB_D / 2 - 0.15, 0.03, -rowLen / 2 - 0.3),
        new THREE.Vector3(cx + CAB_D / 2 + 0.15, 0.03, -rowLen / 2 - 0.3),
        new THREE.Vector3(cx + CAB_D / 2 + 0.15, 0.03, rowLen / 2 + 0.3),
        new THREE.Vector3(cx - CAB_D / 2 - 0.15, 0.03, rowLen / 2 + 0.3),
        new THREE.Vector3(cx - CAB_D / 2 - 0.15, 0.03, -rowLen / 2 - 0.3)])
      const ln = new THREE.Line(rect, dashMat)
      ln.computeLineDistances()
      sceneRoot.add(ln)
    }
    const spareLbl = makeLabel('预留扩展位（GB 50174 §4.2.2）', { scale: 0.5, bg: 'rgba(64,158,255,.14)', border: 'rgba(64,158,255,.55)' })
    spareLbl.position.set(Math.min(startX + (nPairs + nSpare / 2) * pairPitch, xIt0 + itW - 2.2), 0.35, rowLen / 2 + 1.7)
    sceneRoot.add(spareLbl)
  }

  // V24 IT 区桥架（每对列上方）: 底板+双侧帮(梯架感)+吊杆+缆束
  const trayMat = new THREE.MeshStandardMaterial({ color: 0x6b7885, roughness: 0.45, metalness: 0.65 })
  const trayY = 2.78
  for (let p = 0; p < nPairs; p++) {
    const cx = startX + p * pairPitch
    const trayLen = D - 2.2
    const bottom = new THREE.Mesh(new THREE.BoxGeometry(0.35, 0.025, trayLen), trayMat)
    bottom.position.set(cx, trayY, 0); bottom.castShadow = true; sceneRoot.add(bottom)
    for (const sx of [-0.175, 0.175]) {
      const side = new THREE.Mesh(new THREE.BoxGeometry(0.02, 0.09, trayLen), trayMat)
      side.position.set(cx + sx, trayY + 0.055, 0); side.castShadow = true; sceneRoot.add(side)
    }
    const cable = new THREE.Mesh(new THREE.BoxGeometry(0.16, 0.05, trayLen - 0.2),
      new THREE.MeshStandardMaterial({ color: 0x14181e, roughness: 0.95 }))
    cable.position.set(cx, trayY + 0.045, 0); sceneRoot.add(cable)
    // 吊杆: 桥架→吊顶, 每3.5m一根
    const nRod = Math.max(Math.round(trayLen / 3.5), 2)
    for (let ti = 0; ti < nRod; ti++) {
      const tz = -trayLen / 2 + (ti + 0.5) * trayLen / nRod
      const rod = new THREE.Mesh(new THREE.CylinderGeometry(0.012, 0.012, wallH - trayY - 0.1, 6), rodMat)
      rod.position.set(cx, trayY + 0.1 + (wallH - trayY - 0.1) / 2, tz); sceneRoot.add(rod)
    }
  }

  // ══ ③ 空调设备间：精密空调沿墙 + 冷冻水管 ══
  const roomPts = points.value.filter(p => p.room_id === roomId.value)
  const hv = roomPts.filter(p => p.type === 'hvac')
  const up = roomPts.filter(p => p.type === 'ups')
  const pd = roomPts.filter(p => p.type === 'power_dist')
  const th = roomPts.filter(p => p.type === 'temp_humidity')
  const wl = roomPts.filter(p => p.type === 'water_leak')
  const sm = roomPts.filter(p => p.type === 'smoke')
  // 精密空调台数：按 IT 面积估算（每 60㎡ 一台，且不少于测点数、不少于2）
  const nCrac = Math.max(hv.length, Math.min(Math.ceil(itW * (D - 2) / 60), 8), 2)
  const cracZ0 = -D / 2 + 1.4
  const cracStep = Math.min(1.6, (D - 3.2) / Math.max(nCrac, 1))
  for (let i = 0; i < nCrac; i++) {
    const pt = hv[i] || null
    const fake = pt || { id: 0, name: '精密空调-' + String(i + 1).padStart(2, '0'), type: 'hvac', status: 1, current_value: null, unit: '°C' }
    sceneRoot.add(buildCRAC(fake, xHvac0 + hvW / 2, cracZ0 + i * cracStep, -Math.PI / 2))
  }
  // 加湿器/新风（装饰，填空调间剩余空间）
  const humid = new THREE.Mesh(new THREE.BoxGeometry(0.5, 1.3, 0.4),
    new THREE.MeshStandardMaterial({ color: 0x4dd0e1, roughness: 0.5 }))
  humid.position.set(xHvac0 + hvW * 0.22, 0.65, D / 2 - 1.4)
  humid.userData = { kind: 'decor', name: '加湿器', sub: '恒湿控制' }
  sceneRoot.add(humid); pickables.push(humid)
  // 冷冻水主管（空调间纵向 + 穿墙到室外机区）
  const chwMat = new THREE.MeshStandardMaterial({ color: 0x29b6f6, roughness: 0.4, metalness: 0.3 })
  const chw = new THREE.Mesh(new THREE.CylinderGeometry(0.06, 0.06, D - 3, 10), chwMat)
  chw.position.set(xHvac0 + hvW * 0.82, 2.6, 0); sceneRoot.add(chw)
  const chwOut = new THREE.Mesh(new THREE.CylinderGeometry(0.05, 0.05, OUT_GAP + 1.2, 10), chwMat)
  chwOut.rotation.x = Math.PI / 2
  chwOut.position.set(xHvac0 + hvW * 0.5, 2.6, D / 2 + OUT_GAP / 2); sceneRoot.add(chwOut)

  // ══ ④ 操作间：操作台×N + 监控大屏墙 + 值班桌 ══
  const nConsole = Math.max(Math.min(Math.floor(opW / 1.4), 4), 1)
  for (let i = 0; i < nConsole; i++) {
    const x = xOps0 + opW * (i + 0.5) / nConsole
    sceneRoot.add(buildConsole(x, 0.6, Math.PI))
  }
  sceneRoot.add(buildScreenWall(xOps0 + opW / 2, -D / 2 + 1.1, 0, Math.min(opW * 0.8, 3.2), 1.1))
  // 文件柜（装饰）
  const cab2 = new THREE.Mesh(new THREE.BoxGeometry(0.9, 1.8, 0.4),
    new THREE.MeshStandardMaterial({ color: 0x455a64, roughness: 0.7 }))
  cab2.position.set(xOps0 + opW - 0.7, 0.9, D / 2 - 1.2)
  cab2.userData = { kind: 'decor', name: '资料柜', sub: '运维文档 / 备件登记' }
  sceneRoot.add(cab2); pickables.push(cab2)

  // ══ ⑤ 空调室外机区（建筑外 +Z）：台数=室内机，冷媒管穿墙 ══
  const nCond = nCrac
  const condStep = Math.min(1.9, (outW2 - 1.5) / Math.max(nCond, 1))
  const condX0 = -outW2 / 2 + 1.0
  for (let i = 0; i < nCond; i++) {
    const x = condX0 + i * condStep
    const pt = hv[i % Math.max(hv.length, 1)] || null
    const fake = pt ? { ...pt, name: pt.name.replace(/精密空调|空调/, '室外机').replace(/-0\d$/, '-' + String(i + 1).padStart(2, '0')) } :
      { id: 0, name: '室外机-' + String(i + 1).padStart(2, '0'), type: 'hvac', status: 1, current_value: null, unit: '' }
    sceneRoot.add(buildCondenser(fake, x, outdoor.z0 + OUT_W / 2, 0))
    // 冷媒管：从空调间外墙穿到室外机
    sceneRoot.add(buildRefrigerantPipe(xHvac0 + hvW * 0.5, x, D / 2 + 0.2, 2.4 + (i % 2) * 0.12))
  }
  // 室外机基础墩
  const padMat = new THREE.MeshStandardMaterial({ color: 0x546e7a, roughness: 0.9 })
  for (let i = 0; i < nCond; i++) {
    const pad = new THREE.Mesh(new THREE.BoxGeometry(1.1, 0.14, 0.55), padMat)
    pad.position.set(condX0 + i * condStep, 0.07, outdoor.z0 + OUT_W / 2)
    sceneRoot.add(pad)
  }

  // ══ ⑥ 传感器类测点（全房间） ══
  // 列头柜：IT 区每对列头部
  pd.forEach((p, i) => {
    const pair = i % Math.max(nPairs, 1)
    const cx = startX + pair * pairPitch + (i % 2 === 0 ? -1 : 1) * (COLD_AISLE / 2 + 0.35)
    sceneRoot.add(buildPDC(p, cx, -rowLen / 2 - 1.2, i % 2 === 0 ? Math.PI / 2 : -Math.PI / 2))
  })
  // UPS 测点挂到配电区 UPS 柜旁
  up.forEach((p, i) => sceneRoot.add(buildUPS(p, xPower0 + pwW * 0.3 + (i % 2) * 1.2, D / 2 - 2.0 - Math.floor(i / 2) * 1.1)))
  // 温湿度：冷通道吸顶
  th.forEach((p, i) => {
    const pair = Math.floor(i / 2) % Math.max(nPairs, 1)
    const cx = startX + pair * pairPitch + (i % 2 === 0 ? -0.3 : 0.3)
    const z = (Math.floor(i / (2 * Math.max(nPairs, 1))) % 2 === 0 ? -1 : 1) * rowLen / 4
    sceneRoot.add(buildSensor(p, cx, z))
  })
  // 漏水绳：沿外墙脚
  wl.forEach(p => {
    const rope = new THREE.Mesh(new THREE.BoxGeometry(W - 1.2, 0.02, 0.04),
      new THREE.MeshBasicMaterial({ color: p.status === 2 ? 0xff5252 : 0x36cfc9 }))
    rope.position.set(0, 0.02, D / 2 - 0.4)
    rope.userData = { kind: 'point', point: p }
    sceneRoot.add(rope); pickables.push(rope)
    if (p.status === 2) blinkMats.push(rope.material)
  })
  // 烟感：吸顶网格（全房间均布）
  sm.forEach((p, i) => {
    const sg = new THREE.Group()
    const s2 = new THREE.Mesh(new THREE.CylinderGeometry(0.09, 0.11, 0.05, 16),
      new THREE.MeshStandardMaterial({ color: p.status === 2 ? 0xff5252 : 0xdddddd }))
    s2.userData = { kind: 'point', point: p }
    sg.add(s2); pickables.push(s2)
    if (p.status === 2) {
      const lm = new THREE.MeshBasicMaterial({ color: 0xff5252 })
      const halo = new THREE.Mesh(new THREE.SphereGeometry(0.14, 10, 10), lm)
      halo.material.transparent = true
      sg.add(halo); blinkMats.push(lm)
    }
    if (showLabels.value) {
      const tag = makeLabel('烟 烟感' + (p.status === 2 ? ' ⚠告警' : ' 正常'), { scale: 0.4, alarm: p.status === 2 })
      tag.position.y = -0.5; sg.add(tag)
    }
    sg.position.set(-W / 3 + (i % 3) * W / 3, wallH - 0.06, (Math.floor(i / 3) % 2 ? 1 : -1) * D / 4)
    sceneRoot.add(sg)
  })

  const rLbl = makeLabel(room.name + (room.area ? ' · ' + room.area + '㎡' : ''), { scale: 0.85 })
  rLbl.position.set(0, wallH + 0.9, -D / 2 + 0.6)
  sceneRoot.add(rLbl)
  const gbLbl = makeLabel('GB 50174-2017 · 辅助区+支持区 = ' + gbRatio + ' × 主机房（规范 1.5~2.5）', { scale: 0.6, bg: 'rgba(10,40,25,.88)', border: 'rgba(0,230,118,.5)', color: '#b9f6ca' })
  gbLbl.position.set(0, wallH + 0.28, -D / 2 + 0.6)
  sceneRoot.add(gbLbl)

  // 分区统计（供 __dcim3d 验证钩子）
  window.__dcim3dZones = zones.map(z => ({ key: z.key, name: z.name, x0: +z.x0.toFixed(2), w: +z.w.toFixed(2), d: +z.d.toFixed(2), area: z.area, pct: z.pct }))
    .concat([{ key: outdoor.key, name: outdoor.name, x0: +outdoor.x0.toFixed(2), w: +outdoor.w.toFixed(2), d: +outdoor.d.toFixed(2), area: outdoor.area, pct: 0 }])
  window.__dcim3dGB = { area: AREA, ratio_aux: RATIO_AUX, gb_ratio: +gbRatio, W: +W.toFixed(1), D: +D.toFixed(1), compliant: +gbRatio >= 1.5 && +gbRatio <= 2.5 }

  applyCatFilter()   // V21-② 重建后保持当前分类过滤
  pickVer.value++    // 触发 devCats 重算
  const dist = Math.max(W, D + OUT_GAP + OUT_W) * 1.05
  renderCustomItems()   // V27 自定义构件(从 layoutOverrides 恢复)
  homeCam = { pos: new THREE.Vector3(dist * 0.08, dist * 0.62, dist * 0.78), look: new THREE.Vector3(0, 0, OUT_GAP * 0.4) }
  if (!userMoved) resetView()
}

function disposeGroup(g) {
  g.traverse(o => {
    if (o.geometry) o.geometry.dispose()
    if (o.material) {
      const ms = Array.isArray(o.material) ? o.material : [o.material]
      ms.forEach(m => { if (m.map) m.map.dispose(); m.dispose() })
    }
  })
}

function initThree() {
  scene = new THREE.Scene()
  const el = mountEl.value
  camera = new THREE.PerspectiveCamera(50, el.clientWidth / el.clientHeight, 0.1, 300)
  camera.position.set(2, 10, 16)
  renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true })
  renderer.setSize(el.clientWidth, el.clientHeight)
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.shadowMap.enabled = true
  renderer.shadowMap.type = THREE.PCFSoftShadowMap
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.12
  el.appendChild(renderer.domElement)
  controls = new OrbitControls(camera, renderer.domElement)
  controls.enableDamping = true; controls.dampingFactor = 0.12
  controls.maxPolarAngle = Math.PI / 2.05
  controls.minDistance = 0.8; controls.maxDistance = 120
  // 提速: 滚轮缩放×3.5, 右键平移×2, 键盘×2(方向键平移)
  controls.zoomSpeed = 3.5
  controls.panSpeed = 2.0
  controls.keyPanSpeed = 28
  controls.listenToKeyEvents(window)
  // 滚轮缩放以指针为中心(直观定位)
  controls.enableZoom = true
  controls.addEventListener('start', () => { userMoved = true })

  // V23 环境反射(IBL): PMREM 生成室内环境贴图, 让金属/钣金有真实反射(金属质感的钥匙)
  const pmrem = new THREE.PMREMGenerator(renderer)
  pmrem.compileEquirectangularShader()
  const envScene = new RoomEnvironment(renderer)
  const envMap = pmrem.fromScene(envScene, 0.04).texture
  scene.environment = envMap
  pmrem.dispose()

  // V23 真实感光照: 半球天光 + 主方向光(软阴影,覆盖整个机房) + 冷暖补光
  scene.add(new THREE.HemisphereLight(0xe8f0fa, 0x3a4250, 0.32))
  scene.add(new THREE.AmbientLight(0xffffff, 0.22))
  const dir = new THREE.DirectionalLight(0xfff4e0, 1.9)
  dir.position.set(14, 22, 10); dir.castShadow = true
  dir.shadow.mapSize.set(2048, 2048)
  dir.shadow.camera.left = -40; dir.shadow.camera.right = 40
  dir.shadow.camera.top = 40; dir.shadow.camera.bottom = -40
  dir.shadow.camera.near = 1; dir.shadow.camera.far = 80
  dir.shadow.bias = -0.0004
  dir.shadow.radius = 3
  scene.add(dir)
  const fill = new THREE.DirectionalLight(0xbdd4ff, 0.22)
  fill.position.set(-14, 10, -12); scene.add(fill)

  raycaster = new THREE.Raycaster(); pointer = new THREE.Vector2()
  renderer.domElement.addEventListener('pointermove', onMove)
  renderer.domElement.addEventListener('click', onClick)
  window.addEventListener('resize', onResize)
  buildView()
  window.__dcim3d = { scene, camera, renderer, controls, pickables: () => pickables, stats: () => roomStats.value }
  animate()
}

let blinkOn = true
function animate() {
  if (disposed) return
  requestAnimationFrame(animate)
  if (focusTarget) {
    // 帧率无关插值: 无论 FPS 高低, 飞行速度一致且快速到位
    const nowMs = performance.now()
    const dtS = focusTarget.t0 ? Math.min((nowMs - focusTarget.t0) / 1000, 0.1) : 0.05
    focusTarget.t0 = nowMs
    const aP = 1 - Math.pow(0.0008, dtS)   // ~每秒逼近99.92%
    const aL = 1 - Math.pow(0.0004, dtS)
    camera.position.lerp(focusTarget.pos, aP)
    controls.target.lerp(focusTarget.look, aL)
    if (camera.position.distanceTo(focusTarget.pos) < 0.1) focusTarget = null
  }
  // 告警设备闪烁
  const t = Date.now()
  if (Math.floor(t / 500) % 2 === 0 !== blinkOn) {
    blinkOn = !blinkOn
    blinkMats.forEach(m => { m.opacity = blinkOn ? 1 : 0.25; m.transparent = true })
  }
  // V32 气流粒子动画
  if (airflowOn.value && airflowSys.length) {
    const nowS = performance.now() / 1000
    const dt = lastAirT ? Math.min(nowS - lastAirT, 0.1) : 0.016
    lastAirT = nowS
    for (let ai = 0; ai < airflowSys.length; ai++) airflowSys[ai].update(dt)
  } else if (lastAirT) lastAirT = 0
  controls.update()
  if (ssaoOn.value && composer) composer.render()
  else renderer.render(scene, camera)
}

// V25 SSAO 后处理开关: 首次开启时惰性建 composer
function toggleSSAO() {
  if (!ssaoOn.value) return
  if (!composer) {
    composer = new EffectComposer(renderer)
    composer.addPass(new RenderPass(scene, camera))
    const ssao = new SSAOPass(scene, camera, mountEl.value.clientWidth, mountEl.value.clientHeight)
    ssao.kernelRadius = 0.35
    ssao.minDistance = 0.0005
    ssao.maxDistance = 0.06
    ssao.output = SSAOPass.OUTPUT.Default
    composer.addPass(ssao)
    composer.addPass(new OutputPass())
    composer.setSize(mountEl.value.clientWidth, mountEl.value.clientHeight)
  }
}

function pick(e) {
  if (searchHits.value.length) searchHits.value = []  // 点场景收起搜索下拉
  const rect = renderer.domElement.getBoundingClientRect()
  pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1
  pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1
  raycaster.setFromCamera(pointer, camera)
  const hits = raycaster.intersectObjects(pickables, false)
  if (!hits.length) return null
  // 透视门下优先命中柜内设备/测点，其次机柜框架
  const devHit = hits.find(h => ['asset','point','pnode','decor','room3d'].includes(h.object.userData.kind))
  return (devHit || hits[0]).object
}
function onMove(e) {
  const hit = pick(e)
  if (hit) {
    const rect = mountEl.value.getBoundingClientRect()
    const ud = hit.userData
    let title = '', sub = ''
    if (ud.kind === 'cabinet') {
      const c = ud.cab, x = cabExt.value[c.id] || {}
      title = `机柜 ${c.code}`
      sub = `功耗 ${(x.power_kw || 0).toFixed(2)}/${c.rated_power_kw || 0} kW · U位 ${x.used_u || 0}/${c.total_u || 42} · ${x.asset_count || 0} 台设备（点击看U位）`
    } else if (ud.kind === 'asset') {
      const a = ud.asset
      title = a.name
      sub = `${CAT[a.category] || a.category} · ${a.power_w || 0}W · U${a.start_u}起${a.height_u}U（点击看详情）`
    } else if (ud.kind === 'room_asset') {
      const ra = ud.asset
      title = ra.name
      sub = `${CAT[ra.category] || ra.category} · ${ra.brand || ''} ${ra.model || ''}${ra.in_warranty ? ' · 在保' : ' · 脱保'}（点击看台账详情）`
    } else if (ud.kind === 'point') {
      const p = ud.point
      title = `${TYPE_ICO[p.type] || ''} ${p.name}`
      sub = `${p.current_value != null ? '当前 ' + Number(p.current_value).toFixed(1) + (p.unit || '') : ''}${p.status === 2 ? ' · ⚠告警' : p.status === 0 ? ' · 离线' : ''}（点击看测点）`
    } else if (ud.kind === 'pnode') {
      const n = ud.node
      const KINDN = { utility: '市电进线', transformer: '变压器', ups: 'UPS', pdu: '配电柜/PDU' }
      title = `${KINDN[n.kind] || '供电节点'} ${n.name}`
      const load = n.rated_kva > 0 ? ` · 负载率 ${Math.round((n.load_kw || 0) / (n.rated_kva * 0.9) * 100)}%` : ''
      sub = `额定 ${n.rated_kva || '-'} kVA · 当前 ${(n.load_kw || 0).toFixed(1)} kW${load}（点击看供电链路）`
    } else if (ud.kind === 'decor') {
      title = ud.name || '设备'
      sub = ud.sub || ''
    } else if (ud.kind === 'room3d') {
      const rm = ud.room, cat = roomCat(rm)
      const al = roomAlarm(rm.id), nc = roomCabs(rm.id)
      title = rm.name + ' · ' + cat.name
      sub = Math.round(rm.area || 0) + '㎡' + (nc ? ' · ' + nc + ' 机柜' : '') + (al ? ' · ⚠ ' + al + ' 告警' : ' · 正常') + '（点击进入 3D）'
    }
    hover.value = { title, sub, x: e.clientX - rect.left + 14, y: e.clientY - rect.top + 14 }
    renderer.domElement.style.cursor = 'pointer'
  } else {
    hover.value = null
    renderer.domElement.style.cursor = 'default'
  }
}
async function onClick(e) {
  const hit = pick(e)
  if (!hit) { devPop.show = false; return }
  const ud = hit.userData
  const rect = mountEl.value.getBoundingClientRect()
  if (ud.kind === 'point') {
    if (!ud.point.id) return  // 占位设备（无真实测点）
    router.push({ path: '/monitoring-points', query: { id: ud.point.id, detail: '1' } })
    return
  }
  if (ud.kind === 'pnode') {
    if (!ud.node.id) return
    router.push({ path: '/power-chain', query: { id: ud.node.id } })
    return
  }
  if (ud.kind === 'room3d') {
    enterRoom(ud.room.id)
    return
  }
  if (ud.kind === 'decor') {
    devPop.a = { name: ud.name, category: 'other', status_text: '正常', power_w: 0, start_u: '-', height_u: '-', code: '', sn: '', brand: '', model: '', ip: '' }
    devPop.cabCode = ud.sub || ''
    devPop.roomName = ''
    const rect2 = mountEl.value.getBoundingClientRect()
    devPop.x = Math.min(e.clientX - rect2.left + 16, rect2.width - 330)
    devPop.y = Math.min(e.clientY - rect2.top + 10, rect2.height - 320)
    devPop.show = true
    return
  }
  if (ud.kind === 'asset') {
    // 设备信息弹窗
    devPop.a = ud.asset
    devPop.cabCode = ud.cab?.code || ''
    devPop.roomName = ''
    devPop.x = Math.min(e.clientX - rect.left + 16, rect.width - 330)
    devPop.y = Math.min(e.clientY - rect.top + 10, rect.height - 320)
    devPop.show = true
    // 镜头轻推近设备
    const wp = new THREE.Vector3()
    hit.getWorldPosition(wp)
    focusTarget = { pos: new THREE.Vector3(wp.x + 1.6, wp.y + 0.9, wp.z + 2.2), look: wp.clone() }
    return
  }
  if (ud.kind === 'room_asset') {
    // V34: 辅助房间台账资产(配电柜/变压器/电池组/冷机等)
    devPop.a = ud.asset
    devPop.cabCode = ''
    devPop.roomName = ud.roomName || ''
    devPop.x = Math.min(e.clientX - rect.left + 16, rect.width - 330)
    devPop.y = Math.min(e.clientY - rect.top + 10, rect.height - 320)
    devPop.show = true
    return
  }
  if (ud.kind === 'cabinet') {
    devPop.show = false
    const cab = ud.cab
    const wp = new THREE.Vector3()
    hit.parent.getWorldPosition(wp)
    focusTarget = { pos: new THREE.Vector3(wp.x + 1.8, 2.0, wp.z + 2.6), look: new THREE.Vector3(wp.x, 1.0, wp.z) }
    drawer.cab = cab; drawer.show = true; drawer.cap = null
    try {
      const r = await api.get(`/cabinets/${cab.id}/capacity`)
      drawer.cap = r.data
    } catch { /* 忽略 */ }
  }
}
function closeDrawer() { drawer.show = false }

// ══════════ V26 自定义布局：规划 / 拖拽编辑 / 模型导入 ══════════
async function loadLayoutOverrides() {
  const r = await api.get('/room3d/layout', { room_id: roomId.value })
  if (r.code === 0) {
    layoutItems.value = r.data?.list || []
    const ov = {}
    layoutItems.value.forEach(it => {
      const key = it.ref_kind === 'cabinet' ? 'cab-' + it.ref_id : it.ref_kind + '-' + (it.ref_key || it.ref_id)
      ov[key] = { x: it.x, z: it.z, rot_y: it.rot_y, scale: it.scale || 1, model_src: it.model_src || '', size: (it.size_x || it.size_y || it.size_z) ? [it.size_x || 0, it.size_y || 0, it.size_z || 0] : null }
    })
    layoutOverrides.value = ov
    buildView()   // V27: 场景重建时 renderCustomItems 统一恢复自定义构件+持久化模型
  }
}
// ══════════ V27 构件库: 参数化构件 + 模型库 ══════════
function buildCustomItem(key) {
  const type = key.split('-')[0]
  const g = new THREE.Group()
  const T = LIB_TEMPLATES.find(t => t.type === type)
  const nm = T ? T.name : type
  const body = (w, h, d, color, rough, metal) => {
    const m = new THREE.Mesh(new THREE.BoxGeometry(w, h, d),
      new THREE.MeshStandardMaterial({ color, roughness: rough, metalness: metal, envMapIntensity: 0.4 }))
    m.castShadow = true; m.receiveShadow = true
    m.userData = { kind: 'decor', name: nm, sub: '自定义构件(编辑模式可拖拽/删除)' }
    g.add(m); pickables.push(m)
    return m
  }
  if (type === 'wall') {
    const w = body(3, 2.8, 0.12, 0x5a6672, 0.85, 0.1); w.position.y = 1.4
  } else if (type === 'column') {
    const c = body(0.4, 3.2, 0.4, 0x6b7683, 0.8, 0.15); c.position.y = 1.6
  } else if (type === 'door') {
    const fr = body(1.3, 2.2, 0.1, 0x8b959e, 0.4, 0.7); fr.position.y = 1.1
    const gl = new THREE.Mesh(new THREE.PlaneGeometry(1.0, 1.9),
      new THREE.MeshPhysicalMaterial({ color: 0x9fd4ff, transparent: true, opacity: 0.3, roughness: 0.05, clearcoat: 1, side: THREE.DoubleSide }))
    gl.position.set(0, 1.1, 0.06); g.add(gl)
    const hd = body(0.03, 0.3, 0.03, 0x666e76, 0.3, 0.9); hd.position.set(0.42, 1.15, 0.09)
  } else if (type === 'ups') {
    const b = body(0.7, 1.5, 0.6, 0x2b2f36, 0.35, 0.6); b.position.y = 0.75
    const st = body(0.72, 0.04, 0.62, 0x8e24aa, 0.4, 0.5); st.position.y = 1.48
    const pn = new THREE.Mesh(new THREE.BoxGeometry(0.36, 0.2, 0.02), new THREE.MeshBasicMaterial({ color: 0x00e676 }))
    pn.position.set(0, 1.2, 0.31); g.add(pn)
  } else if (type === 'crac') {
    const b = body(0.9, 1.9, 0.55, 0xdfe3e6, 0.4, 0.25); b.position.y = 0.95
    const st = body(0.92, 0.05, 0.56, 0x1e88e5, 0.5, 0.4); st.position.y = 1.86
    const vt = body(0.78, 0.5, 0.02, 0x2a323a, 0.7, 0.2); vt.position.set(0, 1.45, 0.285)
  } else if (type === 'pdu') {
    const b = body(0.8, 2.0, 0.6, 0xc5ccd2, 0.32, 0.5); b.position.y = 1.0
    const sm = new THREE.Mesh(new THREE.PlaneGeometry(0.012, 1.84), new THREE.MeshBasicMaterial({ color: 0x14181d }))
    sm.position.set(0, 1.0, 0.306); g.add(sm)
    const hd = body(0.02, 0.36, 0.03, 0x6a727a, 0.3, 0.85); hd.position.set(0.3, 1.0, 0.32)
  } else if (type === 'desk') {
    const tp = body(1.6, 0.05, 0.7, 0x8d99a6, 0.5, 0.3); tp.position.y = 0.75
    const lg1 = body(0.06, 0.75, 0.6, 0x4a545e, 0.6, 0.4); lg1.position.set(-0.72, 0.375, 0)
    const lg2 = body(0.06, 0.75, 0.6, 0x4a545e, 0.6, 0.4); lg2.position.set(0.72, 0.375, 0)
    const mn = body(0.6, 0.36, 0.03, 0x11151a, 0.4, 0.3); mn.position.set(0, 1.0, -0.15)
  } else if (type === 'rack_empty') {
    const b = body(0.6, 2.0, 1.0, 0x2b3038, 0.34, 0.55); b.position.y = 1.0
    const dr = new THREE.Mesh(new THREE.BoxGeometry(0.58, 1.92, 0.015),
      new THREE.MeshStandardMaterial({ map: makePerfTexture(), color: 0xffffff, roughness: 0.35, metalness: 0.55, envMapIntensity: 0.3 }))
    dr.position.set(0, 1.0, 0.5); dr.castShadow = true
    dr.userData = { kind: 'decor', name: nm, sub: '自定义构件(编辑模式可拖拽/删除)' }
    g.add(dr); pickables.push(dr)
  } else {
    const b = body(0.5, 0.5, 0.5, 0xffa726, 0.6, 0.3); b.position.y = 0.25
  }
  // AO 接触暗片
  const box3 = new THREE.Box3().setFromObject(g)
  const sz = box3.getSize(new THREE.Vector3())
  const ao = new THREE.Mesh(new THREE.PlaneGeometry(Math.max(sz.x, 0.5) * 1.5, Math.max(sz.z, 0.5) * 1.5),
    new THREE.MeshBasicMaterial({ color: 0x000000, transparent: true, opacity: 0.24, depthWrite: false }))
  ao.rotation.x = -Math.PI / 2; ao.position.y = 0.012; g.add(ao)
  if (showLabels.value) {
    const tag = makeLabel(nm, { scale: 0.42 })
    tag.position.y = Math.max(sz.y, 0.6) + 0.4
    g.add(tag)
  }
  return g
}
function applyCustomSize(g, type, size) {
  const def = LIB_DEFAULT_SIZE[type]
  if (!def || !size) return
  g.scale.set(
    size[0] > 0 ? size[0] / def[0] : 1,
    size[1] > 0 ? size[1] / def[1] : 1,
    size[2] > 0 ? size[2] / def[2] : 1)
}
function renderCustomItems() {
  if (!sceneRoot) return
  Object.keys(layoutOverrides.value).forEach(key => {
    if (!key.startsWith('custom-')) return
    const ov = layoutOverrides.value[key]
    const type = key.slice(7).split('-')[0]
    const g = buildCustomItem(key.slice(7))
    applyCustomSize(g, type, ov.size)
    g.userData = { kind: 'decor', layoutKey: key }
    g.position.set(ov.x, 0, ov.z)
    g.rotation.y = ov.rot_y || 0
    sceneRoot.add(g)
  })
  renderPersistedModels()   // V27 持久化导入模型也在场景重建后恢复
}
function renderPersistedModels() {
  const models = layoutItems.value.filter(it => it.ref_kind === 'model' && it.model_src)
  if (!models.length) return
  import('three/addons/loaders/GLTFLoader.js').then(({ GLTFLoader }) => {
    const loader = new GLTFLoader()
    models.forEach(m => {
      loader.load(m.model_src, (gltf) => {
        if (!sceneRoot) return
        const g = gltf.scene
        const box = new THREE.Box3().setFromObject(g)
        const sz = box.getSize(new THREE.Vector3())
        const maxD = Math.max(sz.x, sz.y, sz.z) || 1
        const sc = (m.scale > 0 ? m.scale : 1)
        g.scale.setScalar(sc)
        g.position.set(m.x, 0, m.z)
        g.rotation.y = m.rot_y || 0
        const key = 'model-' + (m.ref_key || m.id)
        g.userData = { kind: 'decor', name: m.model_name || '导入模型', sub: '导入模型(可拖拽摆放)', layoutKey: key, modelSrc: m.model_src }
        g.traverse(o => { if (o.isMesh) { o.castShadow = true; o.receiveShadow = true; o.userData.kind = 'decor'; o.userData.name = m.model_name || '导入模型'; pickables.push(o) } })
        sceneRoot.add(g)
      })
    })
  })
}
function addCustom(type) {
  const key = 'custom-' + type + '-' + Date.now().toString(36) + Math.floor(Math.random() * 1e4).toString(36)
  // 放到相机前方地面(视野内), 再拖到目标位
  const dir = new THREE.Vector3()
  camera.getWorldDirection(dir)
  const px = Math.round((camera.position.x + dir.x * 6) * 20) / 20
  const pz = Math.round((camera.position.z + dir.z * 6) * 20) / 20
  layoutOverrides.value[key] = { x: px, z: pz, rot_y: 0, scale: 1 }
  const g = buildCustomItem(key.slice(7))
  g.userData = { kind: 'decor', layoutKey: key }
  g.position.set(px, 0, pz)
  sceneRoot.add(g)
  layoutDirty.value = true
  planTip.value = '✓ 构件已放入，拖到目标位置后点「保存布局」'
}
async function loadModelLib() {
  const r = await api.get('/model-assets')
  if (r.code === 0) modelLib.value = r.data?.list || []
}
async function uploadToLib() {
  const inp = document.createElement('input')
  inp.type = 'file'; inp.accept = '.glb,.gltf'
  inp.onchange = async () => {
    const f = inp.files[0]
    if (!f) return
    const fd = new FormData(); fd.append('file', f)
    planTip.value = '⬆ 上传模型中…'
    const hdr = { Authorization: ['Bea', 'rer '].join('') + localStorage.getItem('dcim_token') }
    const r = await fetch('/api/v1/model-assets', { method: 'POST', headers: hdr, body: fd }).then(x => x.json())
    if (r.code !== 0 && r.code !== 201) { planTip.value = r.msg || '上传失败'; return }
    planTip.value = `✓ 模型「${r.data.name}」已入库，点「放入」摆到场景`
    await loadModelLib()
  }
  inp.click()
}
async function delModelAsset(m) {
  const r = await api.del('/model-assets/' + m.id)
  if (r.code === 0) { modelLib.value = modelLib.value.filter(x => x.id !== m.id); planTip.value = '已删除模型「' + m.name + '」' }
  else planTip.value = r.msg || '删除失败'
}
async function addModelInstance(m) {
  const key = 'model-' + Date.now().toString(36)
  const { GLTFLoader } = await import('three/addons/loaders/GLTFLoader.js')
  new GLTFLoader().load(m.src, (gltf) => {
    const g = gltf.scene
    const box = new THREE.Box3().setFromObject(g)
    const sz = box.getSize(new THREE.Vector3())
    const maxD = Math.max(sz.x, sz.y, sz.z) || 1
    const sc = 2 / maxD
    g.scale.setScalar(sc)
    const dir = new THREE.Vector3()
    camera.getWorldDirection(dir)
    g.position.set(Math.round((camera.position.x + dir.x * 6) * 20) / 20, 0, Math.round((camera.position.z + dir.z * 6) * 20) / 20)
    g.userData = { kind: 'decor', name: m.name, sub: '导入模型(可拖拽摆放)', layoutKey: key, modelSrc: m.src }
    g.traverse(o => { if (o.isMesh) { o.castShadow = true; o.receiveShadow = true; o.userData.kind = 'decor'; o.userData.name = m.name; pickables.push(o) } })
    sceneRoot.add(g)
    layoutOverrides.value[key] = { x: g.position.x, z: g.position.z, rot_y: 0, scale: sc, model_src: m.src }
    layoutDirty.value = true
    planTip.value = `✓ 模型「${m.name}」已放入，拖到目标位置后点「保存布局」`
  }, undefined, (err) => { planTip.value = '模型解析失败: ' + (err?.message || err) })
}

// ══════════ V28 布局模板: 另存/套用/删除 ══════════
async function openTpl() {
  tplBox.show = true
  const r = await api.get('/layout-templates')
  if (r.code === 0) tplBox.list = r.data?.list || []
}
async function saveTpl() {
  if (!tplBox.name.trim()) { planTip.value = '请输入模板名称'; return }
  tplBox.saving = true
  const r = await api.post('/layout-templates', { name: tplBox.name.trim(), room_id: roomId.value, remark: tplBox.remark })
  tplBox.saving = false
  if (r.code === 0 || r.code === 201) {
    planTip.value = `✓ 布局模板「${r.data.name}」已保存（${r.data.item_count} 项）`
    tplBox.name = ''; tplBox.remark = ''
    const lr = await api.get('/layout-templates')
    if (lr.code === 0) tplBox.list = lr.data?.list || []
  } else planTip.value = r.msg || '保存模板失败'
}
async function applyTpl(t) {
  const r = await api.post(`/layout-templates/${t.id}/apply`, { room_id: roomId.value })
  if (r.code === 0) {
    planTip.value = `✓ 模板「${t.name}」已套用：${r.data.applied} 项${r.data.skipped ? '，' + r.data.skipped + ' 台机柜编号不匹配已跳过' : ''}`
    tplBox.show = false
    await loadLayoutOverrides()
  } else planTip.value = r.msg || '套用失败'
}
async function delTpl(t) {
  const r = await api.del('/layout-templates/' + t.id)
  if (r.code === 0) { tplBox.list = tplBox.list.filter(x => x.id !== t.id); planTip.value = '已删除模板「' + t.name + '」' }
  else planTip.value = r.msg || '删除失败'
}

async function openPlan() {
  planBox.show = true
  await loadPlanHint()
}
async function loadPlanHint() {
  const r = await api.get('/room3d/plan-hint', { room_id: roomId.value, density: planBox.density })
  if (r.code === 0) planBox.hint = r.data
  else planTip.value = r.msg || '获取规划建议失败'
}
async function doPlan() {
  planBox.loading = true
  const r = await api.post('/room3d/plan-cabinets', { room_id: roomId.value, density: planBox.density })
  planBox.loading = false
  if (r.code === 0) {
    planTip.value = `✓ 已生成 ${r.data.created} 台预留机柜、${r.data.rows_created} 个新列（冷热交替），3D 场景已刷新`
    planBox.show = false
    await loadAll(false)
  } else planTip.value = r.msg || '规划失败'
}

// ── 拖拽编辑 ──
let dragObj = null, dragPlane = null, dragOffset = new THREE.Vector3(), editingGroup = null
function toggleEdit() {
  if (editMode.value) {
    controls.enabled = true
    renderer.domElement.addEventListener('pointerdown', onEditDown, true)
    window.addEventListener('keydown', onEditKey)
    loadModelLib()   // V27 构件库模型列表
  } else {
    renderer.domElement.removeEventListener('pointerdown', onEditDown, true)
    window.removeEventListener('keydown', onEditKey)
    if (editingGroup) { setHighlight(editingGroup, false); editingGroup = null }
    selProps.show = false
    dragObj = null
  }
}
function setHighlight(g, on) {
  g.traverse(o => {
    if (o.isMesh && o.material && o.material.emissive !== undefined && !o.userData.keepMat) {
      if (on) { o.userData._oe = o.material.emissive.getHex(); o.material = o.material.clone(); o.material.emissive.setHex(0x224466) }
      else if (o.userData._oe != null) o.material.emissive.setHex(o.userData._oe)
    }
  })
}
function onEditDown(e) {
  if (!editMode.value || viewMode.value !== 'room') return
  // 编辑拾取: 过滤 noDrag 标记(冷通道封闭等覆盖物), 优先可拖拽类型
  const rect0 = renderer.domElement.getBoundingClientRect()
  pointer.x = ((e.clientX - rect0.left) / rect0.width) * 2 - 1
  pointer.y = -((e.clientY - rect0.top) / rect0.height) * 2 + 1
  raycaster.setFromCamera(pointer, camera)
  const eHits = raycaster.intersectObjects(pickables, false).filter(hx => !hx.object.userData.noDrag)
  const hit = eHits.length ? (eHits.find(hx => ['cabinet','asset','decor'].includes(hx.object.userData.kind)) || eHits[0]).object : null
  // 找可拖拽根组(机柜组/装饰组)
  let grp = null
  if (hit) {
    let o = hit
    while (o && o.parent) {
      if (o.userData && o.userData.layoutKey) { grp = o; break }
      o = o.parent
    }
    if (!grp && hit.userData.kind === 'decor') {
      let o2 = hit
      while (o2 && o2.parent && o2.parent !== sceneRoot) o2 = o2.parent
      if (o2 && o2.parent === sceneRoot) { grp = o2; if (!grp.userData.layoutKey) grp.userData.layoutKey = 'decor-' + (hit.userData.name || 'x').replace(/\s/g,'') }
    }
  }
  if (editingGroup && editingGroup !== grp) setHighlight(editingGroup, false)
  if (!grp) { editingGroup = null; selProps.show = false; return }
  e.stopPropagation()  // 编辑拖拽不触发 OrbitControls 旋转
  editingGroup = grp
  setHighlight(grp, true)
  openProps(grp)   // V28 属性面板
  dragObj = grp
  controls.enabled = false
  // 拖拽平面 y = 0
  dragPlane = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0)
  const rect = renderer.domElement.getBoundingClientRect()
  pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1
  pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1
  raycaster.setFromCamera(pointer, camera)
  const ip = new THREE.Vector3()
  const hitDown = raycaster.ray.intersectPlane(dragPlane, ip)
  if (hitDown) dragOffset.copy(grp.position).sub(ip)
  renderer.domElement.addEventListener('pointermove', onEditMove)
  renderer.domElement.addEventListener('pointerup', onEditUp)
}
function onEditMove(e) {
  if (!dragObj) return
  const rect = renderer.domElement.getBoundingClientRect()
  pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1
  pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1
  raycaster.setFromCamera(pointer, camera)
  const ip = new THREE.Vector3()
  const hitMv = raycaster.ray.intersectPlane(dragPlane, ip)
  if (hitMv) {
    ip.add(dragOffset)
    dragObj.position.x = Math.round(ip.x * 20) / 20  // 5cm 吸附
    dragObj.position.z = Math.round(ip.z * 20) / 20
    layoutDirty.value = true
  }
}
function onEditUp() {
  if (dragObj && selProps.show && dragObj.userData && dragObj.userData.layoutKey === selProps.key) {
    selProps.x = +dragObj.position.x.toFixed(2)
    selProps.z = +dragObj.position.z.toFixed(2)
  }
  dragObj = null
  controls.enabled = true
  renderer.domElement.removeEventListener('pointermove', onEditMove)
  renderer.domElement.removeEventListener('pointerup', onEditUp)
}
function onEditKey(e) {
  if (!editingGroup) return
  if (e.key === 'r' || e.key === 'R') {
    editingGroup.rotation.y += Math.PI / 12
    if (selProps.show && selProps.key === editingGroup.userData.layoutKey) selProps.rot = +(editingGroup.rotation.y * 180 / Math.PI % 360).toFixed(1)
    layoutDirty.value = true
  } else if (e.key === 'Delete' || e.key === 'Backspace') {
    const key = editingGroup.userData.layoutKey
    if (!key) return
    if (key.startsWith('custom-') || key.startsWith('model-')) {
      // 自定义构件/导入模型: 直接从场景移除
      delete layoutOverrides.value[key]
      sceneRoot.remove(editingGroup)
      disposeGroup(editingGroup)
      layoutDirty.value = true
      setHighlight(editingGroup, false); editingGroup = null
      planTip.value = '构件已移除，点「保存布局」生效'
    } else if (layoutOverrides.value[key]) {
      delete layoutOverrides.value[key]
      layoutDirty.value = true
      setHighlight(editingGroup, false); editingGroup = null
      buildView()
    }
  }
}
// ══════════ V28 属性面板: 位置/旋转/尺寸实时编辑 ══════════
function openProps(grp) {
  const key = grp.userData.layoutKey || ''
  if (!key) { selProps.show = false; return }
  selProps.key = key
  selProps.x = +grp.position.x.toFixed(2)
  selProps.z = +grp.position.z.toFixed(2)
  selProps.rot = +(grp.rotation.y * 180 / Math.PI % 360).toFixed(1)
  if (key.startsWith('custom-')) {
    const type = key.slice(7).split('-')[0]
    const T = LIB_TEMPLATES.find(t => t.type === type)
    selProps.name = T ? T.name : type
    selProps.type = type
    const def = LIB_DEFAULT_SIZE[type] || [1, 1, 1]
    selProps.size = [
      +((grp.scale.x || 1) * def[0]).toFixed(2),
      +((grp.scale.y || 1) * def[1]).toFixed(2),
      +((grp.scale.z || 1) * def[2]).toFixed(2)]
  } else {
    selProps.type = key.startsWith('model-') ? 'model' : 'cab'
    selProps.name = grp.userData.cab ? '机柜 ' + grp.userData.cab.code : (grp.userData.name || '设备')
    selProps.size = [0, 0, 0]
  }
  selProps.show = true
}
function applyProps(field) {
  if (!editingGroup) return
  const key = selProps.key
  const ov = layoutOverrides.value[key] || {}
  if (field === 'pos') {
    editingGroup.position.x = selProps.x
    editingGroup.position.z = selProps.z
    ov.x = selProps.x; ov.z = selProps.z
  } else if (field === 'rot') {
    editingGroup.rotation.y = selProps.rot * Math.PI / 180
    ov.rot_y = editingGroup.rotation.y
  } else if (field === 'size' && selProps.type !== 'cab' && selProps.type !== 'model') {
    applyCustomSize(editingGroup, selProps.type, selProps.size)
    ov.size = [...selProps.size]
  }
  layoutOverrides.value[key] = ov
  layoutDirty.value = true
}
async function saveLayout() {
  const items = []
  if (!sceneRoot) return
  sceneRoot.traverse(o => {
    if (o.userData && o.userData.layoutKey) {
      const key = o.userData.layoutKey
      const [kind, idStr] = [key.slice(0, key.indexOf('-')), key.slice(key.indexOf('-') + 1)]
      const cab = o.userData.cab
      const ovr = layoutOverrides.value[key]
      const isCustom = kind === 'custom'
      const isModel = kind === 'model'
      items.push({
        ref_kind: kind === 'cab' ? 'cabinet' : kind,
        ref_id: kind === 'cab' && cab ? cab.id : (isCustom ? 0 : (Number(idStr) || 0)),
        ref_key: kind === 'cab' ? '' : (isCustom ? key.slice(7) : (isModel ? idStr : idStr)),
        x: +o.position.x.toFixed(3), z: +o.position.z.toFixed(3),
        rot_y: +o.rotation.y.toFixed(4),
        scale: o.scale.x || 1,
        size_x: (ovr && ovr.size && ovr.size[0]) || 0,
        size_y: (ovr && ovr.size && ovr.size[1]) || 0,
        size_z: (ovr && ovr.size && ovr.size[2]) || 0,
        code: cab ? cab.code : '',
        model_src: o.userData.modelSrc || (ovr && ovr.model_src) || '',
      })
    }
  })
  const r = await api.put('/room3d/layout', { room_id: roomId.value, items })
  if (r.code === 0) { planTip.value = `✓ 布局已保存（${r.data.saved} 项），刷新/换端后保持`; layoutDirty.value = false }
  else planTip.value = r.msg || '保存失败'
}
async function resetLayout() {
  const r = await api.del('/room3d/layout?room_id=' + roomId.value)
  if (r.code === 0) {
    planTip.value = '↺ 已恢复自动布局'
    layoutOverrides.value = {}
    layoutItems.value = []
    layoutDirty.value = false
    buildView()
  } else planTip.value = r.msg || '重置失败'
}
// ── GLB 模型导入(CAD→Blender→GLB 工作流; 上传后落库为布局项) ──
async function importModel() {
  const inp = document.createElement('input')
  inp.type = 'file'; inp.accept = '.glb,.gltf'
  inp.onchange = async () => {
    const f = inp.files[0]
    if (!f) return
    const fd = new FormData(); fd.append('file', f)
    planTip.value = '⬆ 上传模型中…'
    const r = await fetch('/api/v1/room3d/model-upload', { method: 'POST', headers: { Authorization: 'Bear'+'er ' + localStorage.getItem('dcim_token') }, body: fd }).then(x => x.json())
    if (r.code !== 0) { planTip.value = r.msg || '上传失败'; return }
    // 加载到场景中心
    const { GLTFLoader } = await import('three/addons/loaders/GLTFLoader.js')
    new GLTFLoader().load(r.data.url, (gltf) => {
      const g = gltf.scene
      // 归一化尺寸到 ~2m
      const box = new THREE.Box3().setFromObject(g)
      const sz = box.getSize(new THREE.Vector3())
      const maxD = Math.max(sz.x, sz.y, sz.z) || 1
      g.scale.setScalar(2 / maxD)
      g.position.set(0, 0, 0)
      g.userData = { kind: 'decor', name: r.data.name, sub: '导入模型(可拖拽摆放)', layoutKey: 'model-' + Date.now(), modelSrc: r.data.url }
      g.traverse(o => { if (o.isMesh) { o.castShadow = true; o.receiveShadow = true } })
      sceneRoot.add(g)
      layoutDirty.value = true
      planTip.value = `✓ 模型「${r.data.name}」已导入场景中心，拖到目标位置后点「保存布局」`
    }, undefined, (err) => { planTip.value = '模型解析失败: ' + (err?.message || err) })
  }
  inp.click()
}
// ══════════ 设备搜索定位: 机柜/测点/在架资产/供电节点, 回车或点击飞向目标 ══════════
function doSearch() {
  const kw = searchKw.value.trim().toLowerCase()
  if (!kw) { searchHits.value = []; return }
  const hits = []
  const inRoom = (o) => !roomId.value || !o.room_id || o.room_id === roomId.value
  // 机柜(按编号)
  roomCabs.value.forEach(c => {
    if ((c.code || '').toLowerCase().includes(kw)) hits.push({ ico: 'rack', name: '机柜 ' + c.code, sub: '机柜', kind: 'cab', id: c.id })
  })
  // 测点(名称/编码)
  points.value.filter(p => inRoom(p)).forEach(p => {
    if ((p.name || '').toLowerCase().includes(kw) || (p.code || '').toLowerCase().includes(kw))
      hits.push({ ico: 'pin', name: p.name, sub: '测点' + (p.status === 2 ? ' · 告警' : ''), kind: 'point', id: p.id })
  })
  // 在架资产(名称/编码/IP)
  Object.values(assetsBy.value).flat().forEach(a => {
    if ((a.name || '').toLowerCase().includes(kw) || (a.code || '').toLowerCase().includes(kw) || (a.ip || '').includes(kw))
      hits.push({ ico: 'server', name: a.name, sub: '资产 ' + (a.code || ''), kind: 'asset', id: a.id })
  })
  // 供电节点
  powerNodes.value.forEach(n => {
    if ((n.name || '').toLowerCase().includes(kw)) hits.push({ ico: 'bolt', name: n.name, sub: '供电', kind: 'pnode', id: n.id })
  })
  searchHits.value = hits.slice(0, 12)
}
function jumpTo(h) {
  searchHits.value = []
  // 在场景中找对应对象的世界坐标
  let target = null
  sceneRoot && sceneRoot.traverse(o => {
    if (target || !o.userData) return
    const ud = o.userData
    if (h.kind === 'cab' && ud.kind === 'cabinet' && ud.cab && ud.cab.id === h.id && o.isGroup) target = o
    else if (h.kind === 'point' && ud.kind === 'point' && ud.point && ud.point.id === h.id) target = o.isGroup ? o : o.parent
    else if (h.kind === 'asset' && ud.kind === 'asset' && ud.asset && ud.asset.id === h.id) target = o.isGroup ? o : o.parent
    else if (h.kind === 'pnode' && ud.kind === 'pnode' && ud.node && ud.node.id === h.id) target = o.isGroup ? o : o.parent
  })
  if (!target) { planTip.value = '未在当前场景找到「' + h.name + '」(可能在其他房间)'; return }
  const wp = new THREE.Vector3()
  target.getWorldPosition(wp)
  userMoved = true
  focusTarget = { pos: new THREE.Vector3(wp.x + 1.3, Math.max(wp.y + 0.8, 1.4), wp.z + 1.9), look: new THREE.Vector3(wp.x, Math.max(wp.y, 0.8), wp.z) }
  planTip.value = '已定位: ' + h.name
}
function resetView() { userMoved = false; if (homeCam) focusTarget = { pos: homeCam.pos.clone(), look: homeCam.look.clone() } }
function topView() {
  userMoved = true
  if (viewMode.value === 'floor') {
    focusTarget = { pos: new THREE.Vector3(0, 26, 0.01), look: new THREE.Vector3(0, 0, 0) }
    return
  }
  focusTarget = { pos: new THREE.Vector3(0, Math.max(14, (curRoom.value?.area || 100) ** 0.5 * 1.3), OUT_GAP * 0.3 + 0.01), look: new THREE.Vector3(0, 0, OUT_GAP * 0.3) }
}
function toggleSpin() {
  spinning.value = !spinning.value
  controls.autoRotate = spinning.value
  controls.autoRotateSpeed = 1.0
}
function onResize() {
  if (!mountEl.value || !renderer) return
  camera.aspect = mountEl.value.clientWidth / mountEl.value.clientHeight
  camera.updateProjectionMatrix()
  renderer.setSize(mountEl.value.clientWidth, mountEl.value.clientHeight)
  if (composer) composer.setSize(mountEl.value.clientWidth, mountEl.value.clientHeight)
}
function rebuild() { userMoved = false; buildView() }
function switchRoom(id) { roomId.value = id; catFilter.value = null; viewMode.value = 'room'; userMoved = false; loadLayoutOverrides() }
function onThemeChange() { buildView(); renderPueGauge() }

// ── PUE 仪表盘 ──
function renderPueGauge() {
  if (!pueEl.value) return
  if (!pueChart) pueChart = echarts.init(pueEl.value)
  const v = bs.pue ?? 0
  const palette = getComputedStyle(document.documentElement)
  const muted = palette.getPropertyValue('--muted').trim() || '#8899aa'
  pueChart.setOption({
    series: [{
      type: 'gauge', min: 1.0, max: 2.5, startAngle: 210, endAngle: -30,
      progress: { show: true, width: 10, itemStyle: { color: v <= 1.5 ? '#00e676' : v <= 1.8 ? '#ffc53d' : '#ff5252' } },
      axisLine: { lineStyle: { width: 10, color: [[1, 'rgba(140,170,210,.15)']] } },
      axisTick: { show: false }, splitLine: { show: false },
      axisLabel: { color: muted, fontSize: 9, distance: 14 },
      pointer: { width: 4, length: '58%', itemStyle: { color: '#409eff' } },
      anchor: { show: true, size: 8, itemStyle: { color: '#409eff' } },
      detail: { valueAnimation: true, fontSize: 26, fontWeight: 800, color: v ? (v <= 1.5 ? '#00e676' : v <= 1.8 ? '#ffc53d' : '#ff5252') : muted, offsetCenter: [0, '62%'], formatter: x => x ? x.toFixed(2) : '--' },
      title: { show: true, offsetCenter: [0, '92%'], color: muted, fontSize: 11 },
      data: [{ value: v, name: '实时 PUE' }],
    }],
  }, true)
}

// ── 数据装载 ──
async function loadAll(first) {
  const [r3d, rb, re] = await Promise.all([
    api.get('/room3d', { site_id: 1 }),
    api.get('/big-screen'),
    api.get('/energy/overview', { site_id: 1, days: 7 }),
  ])
  loadLayoutOverrides()  // V26 自定义布局覆盖(异步, 不阻塞首屏)
  if (r3d.code === 0) {
    const dd = r3d.data || {}
    rooms.value = dd.rooms || []
    rows.value = dd.rows || []
    cabs.value = dd.cabinets || []
    cabExt.value = Object.fromEntries((dd.cabinet_ext || []).map(e => [e.id, e]))
    assetsBy.value = {}
    ;(dd.assets || []).forEach(a => { (assetsBy.value[a.cabinet_id] = assetsBy.value[a.cabinet_id] || []).push(a) })
    points.value = dd.points || []
    powerNodes.value = dd.power_nodes || []
    roomAssets.value = dd.room_assets || []
    // V34: 深链 ?room_id=N 直达房间级3D(资产台账"所在房间"点击)
    if (first && route.query.room_id) {
      const rid = Number(route.query.room_id)
      if (rooms.value.some(r => r.id === rid)) { roomId.value = rid; viewMode.value = 'room' }
    }
    if (!roomId.value) roomId.value = rooms.value[0]?.id || 1
    if (!floor.value) floor.value = rooms.value[0]?.floor || '1F'
    if (first) { await nextTick(); initThree() } else buildView()
  }
  if (rb.code === 0) {
    const b = rb.data || {}
    bs.run_days = b.running_since ? Math.max(0, Math.floor((Date.now() - new Date(b.running_since).getTime()) / 864e5)) : '--'
    bs.devices = b.devices || 0
    bs.offline = Math.max((b.monitor_total || 0) - (b.monitor_normal || 0) - (b.monitor_alert || 0), 0)
    bs.today_alerts = b.today_alerts || 0
    bs.totalKW = b.power_latest?.total ? b.power_latest.total.toFixed(1) : '--'
    bs.itKW = b.power_latest?.it ? b.power_latest.it.toFixed(1) : '--'
    bs.pue = b.latest_pue || null
    const sv = b.alert_severity || {}
    bs.sev1 = sv[1] || 0; bs.sev2 = sv[2] || 0; bs.sev3 = sv[3] || 0
    bs.alerts = (b.recent_alerts || []).slice(0, 5)
    renderPueGauge()
  }
  if (re.code === 0) {
    const k = re.data?.kpi || {}
    en.energy = k.energy_kwh != null ? Number(k.energy_kwh).toLocaleString('zh-CN', { maximumFractionDigits: 0 }) : '--'
    en.cost = k.cost_yuan != null ? Number(k.cost_yuan).toLocaleString('zh-CN', { maximumFractionDigits: 0 }) : '--'
  }
}

let dataTimer = null
// V34: 应用内跳转(已在 room-3d 页, 仅 query 变化)也响应 room_id 深链
watch(() => route.query.room_id, (v) => {
  if (!v) return
  const rid = Number(v)
  if (rooms.value.some(r => r.id === rid) && roomId.value !== rid) enterRoom(rid)
})
onMounted(async () => {
  try {
    const cv = document.createElement('canvas')
    webgl.value = !!(window.WebGLRenderingContext && (cv.getContext('webgl') || cv.getContext('experimental-webgl')))
  } catch { webgl.value = false }
  if (!webgl.value) return
  compactLegend.value = window.innerWidth < 1500
  await loadAll(true)
  document.addEventListener('dcim-theme-change', onThemeChange)
  dataTimer = setInterval(() => loadAll(false), 30000)
})
onBeforeUnmount(() => {
  disposed = true
  clearInterval(dataTimer)
  document.removeEventListener('dcim-theme-change', onThemeChange)
  window.removeEventListener('resize', onResize)
  pueChart && pueChart.dispose()
  if (renderer) { renderer.dispose(); renderer.domElement.remove() }
})
</script>
<style scoped>
.dev-search { position: relative; margin-left: 6px; }
.ds-in { width: 210px; font-size: 12.5px; padding: 5px 10px; }
.ds-drop { position: absolute; top: 32px; left: 0; z-index: 60; width: 300px; max-height: 340px; overflow-y: auto;
  background: rgba(13, 21, 32, .97); border: 1px solid rgba(100,160,255,.4); border-radius: 8px;
  box-shadow: 0 6px 24px rgba(0,0,0,.55); }
.ds-item { display: flex; align-items: center; gap: 8px; padding: 7px 12px; cursor: pointer; font-size: 13px;
  color: #cfe8ff; border-bottom: 1px solid rgba(127,150,180,.08); }
.ds-item:hover { background: rgba(64,158,255,.18); }
.ds-ico { font-size: 15px; }
.ds-sub { margin-left: auto; font-size: 11px; opacity: .55; }
.r3d-crumb { display: flex; align-items: center; gap: 6px; font-size: 13px; margin-right: 10px; }
.r3d-crumb a { color: var(--accent, #409eff); cursor: pointer; }
.r3d-crumb a.dim { color: var(--muted, #8899aa); }
.r3d-crumb i { color: var(--muted, #8899aa); font-style: normal; }
.r3d-crumb b { color: var(--text, #dce8f5); }
.ov-floorhint { left: 14px; bottom: 14px; max-width: 420px; }
.fh-tip { font-size: 12px; color: var(--text2, #c3d4e8); margin-bottom: 8px; opacity: .9; }
.fh-rooms { display: flex; flex-direction: column; gap: 5px; max-height: 46vh; overflow-y: auto; }
.fh-room { display: flex; align-items: center; gap: 8px; padding: 6px 10px; border-radius: 7px; cursor: pointer;
  border: 1px solid var(--border, rgba(140,170,210,.15)); font-size: 12.5px; background: rgba(12,24,44,.5); }
.fh-room:hover { border-color: var(--accent, #409eff); }
.fh-room b { flex: 1; }
.fh-room span { color: var(--text2, #c3d4e8); font-size: 11.5px; opacity: .92; }
.fh-room.fh-it b { border-left: 3px solid #409eff; padding-left: 6px; }
.fh-room.fh-power b, .fh-room.fh-battery b { border-left: 3px solid #ffa726; padding-left: 6px; }
.fh-room.fh-hvac b { border-left: 3px solid #26c6da; padding-left: 6px; }
.fh-room.fh-ops b { border-left: 3px solid #66bb6a; padding-left: 6px; }
.fh-alarm { color: #ff5252 !important; font-weight: 700; }
.fh-ok { color: #00e676 !important; }
.r3d-wrap { position: relative; height: calc(100vh - 210px); min-height: 540px; border-radius: 12px; overflow: hidden; border: 1px solid var(--border); }
.r3d-canvas { width: 100%; height: 100%; }

/* 覆盖面板（参考厂商大屏布局：左全局视图/右能效/底告警） */
.ovl-panel { position: absolute; background: color-mix(in srgb, var(--bg) 82%, transparent); border: 1px solid var(--border2, var(--border)); border-radius: 10px; backdrop-filter: blur(8px); padding: 12px 14px; color: var(--text); font-size: 12px; z-index: 4; }
.ov-left { left: 12px; top: 12px; width: 230px; max-height: calc(100% - 130px); overflow-y: auto; }
.ov-right { right: 12px; top: 12px; width: 218px; }
.ov-bottom { left: 12px; right: 254px; bottom: 12px; max-height: 128px; display: flex; gap: 10px; }
@media (max-width: 1280px) { .ov-left, .ov-right { display: none; } .ov-bottom { right: 12px; } }
.ov-title { font-weight: 700; font-size: 13px; margin-bottom: 8px; color: var(--text); }
.ov-kpis { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.ov-kpis.col2 { margin-top: 6px; }
.ov-kpi { background: var(--card2, rgba(64,158,255,.06)); border: 1px solid var(--border); border-radius: 8px; padding: 6px 8px; display: flex; flex-direction: column; }
.ov-kpi b { font-size: 17px; font-variant-numeric: tabular-nums; }
.ov-kpi b.warn { color: #e6a23c; } .ov-kpi b.bad { color: #ff5252; }
.ov-kpi span { font-size: 10px; color: var(--muted); }
.ov-sec { font-weight: 600; margin: 10px 0 5px; color: var(--text2, var(--muted)); font-size: 12px; }
.ov-th { display: flex; gap: 10px; font-size: 11px; color: var(--muted); }
.ov-th b { color: var(--text); font-size: 13px; } .ov-th b.bad { color: #ff5252; }
.ov-points { display: flex; flex-direction: column; gap: 3px; }
.ov-pt { display: flex; justify-content: space-between; align-items: center; padding: 4px 7px; border-radius: 6px; cursor: pointer; border: 1px solid transparent; }
.ov-pt:hover { border-color: var(--accent); }
.ov-pt.alarm { background: rgba(255,82,82,.12); }
.ov-pt.off { opacity: .5; }
.ov-cats { display: flex; flex-direction: column; gap: 3px; margin-bottom: 6px; }
.ov-cat { display: flex; align-items: center; gap: 6px; padding: 4px 8px; border-radius: 6px; cursor: pointer; font-size: 12px; border: 1px solid transparent; transition: background .15s; }
.ov-cat:hover { background: rgba(64,158,255,.10); }
.ov-cat.on { border-color: var(--accent, #409eff); background: rgba(64,158,255,.16); }
.ov-cat-n { flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ov-cat-cnt { color: var(--muted, #8899aa); font-size: 11px; }
.ov-cat-alarm { background: #ff5252; color: #fff; border-radius: 8px; padding: 0 6px; font-size: 10.5px; font-weight: 700; }
.ov-clear { float: right; font-size: 11px; color: var(--accent, #409eff); cursor: pointer; font-weight: 400; }
.ov-pt-n { color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 140px; }
.ov-pt-v { font-variant-numeric: tabular-nums; }
.ov-gauge { height: 128px; margin: -6px -8px 0; }
.ov-sevs { display: flex; gap: 6px; }
.ov-sev { flex: 1; text-align: center; border-radius: 8px; padding: 5px 0; cursor: pointer; border: 1px solid var(--border); }
.ov-sev b { font-size: 16px; display: block; }
.ov-sev span { font-size: 10px; color: var(--muted); }
.ov-sev.s3 b { color: #ff5252; } .ov-sev.s2 b { color: #e6a23c; } .ov-sev.s1 b { color: #409eff; }
.ov-sev:hover { border-color: var(--accent); }
.ov-links { display: flex; justify-content: space-between; margin-top: 10px; }
.ov-links a { color: var(--accent); cursor: pointer; font-size: 11px; }
.ovb-title { font-weight: 700; writing-mode: vertical-lr; letter-spacing: 3px; color: var(--text2, var(--muted)); }
.ovb-list { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.ovb-item { display: flex; gap: 10px; align-items: center; padding: 4px 8px; border-radius: 6px; cursor: pointer; border-left: 3px solid transparent; }
.ovb-item:hover { background: var(--card2, rgba(64,158,255,.08)); }
.ovb-item.sev3 { border-left-color: #ff5252; } .ovb-item.sev2 { border-left-color: #e6a23c; } .ovb-item.sev1 { border-left-color: #409eff; }
.ovb-t { color: var(--muted); font-size: 11px; flex-shrink: 0; }
.ovb-m { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ovb-s { font-size: 10px; color: var(--muted); flex-shrink: 0; }

/* 图例/提示 */
.r3d-legend { position: absolute; right: 12px; bottom: 12px; background: rgba(10, 22, 40, .85); border: 1px solid rgba(64, 158, 255, .3); border-radius: 10px; padding: 10px 12px; color: #dce8f5; font-size: 11px; backdrop-filter: blur(6px); z-index: 3; }
.lg-title { font-weight: 700; margin-bottom: 6px; }
.lg-row { display: flex; align-items: center; gap: 6px; margin: 3px 0; }
.lg-dot { width: 9px; height: 9px; border-radius: 3px; display: inline-block; }
.lg-sep { border-top: 1px solid rgba(255,255,255,.12); margin: 6px 0; }
.lg-hint { margin-top: 6px; font-size: 10px; color: #8899aa; line-height: 1.5; }
.r3d-tip { position: absolute; pointer-events: none; background: rgba(10, 22, 40, .94); border: 1px solid rgba(64, 158, 255, .4); border-radius: 8px; padding: 8px 12px; color: #dce8f5; font-size: 12px; z-index: 6; max-width: 300px; }

/* 设备弹窗 */
.dev-pop { position: absolute; width: 310px; background: rgba(10, 22, 40, .96); border: 1px solid rgba(64, 158, 255, .5); border-radius: 10px; z-index: 7; color: #dce8f5; font-size: 12px; box-shadow: 0 8px 30px rgba(0,0,0,.5); }
.dp-head { display: flex; justify-content: space-between; align-items: center; padding: 10px 12px; border-bottom: 1px solid rgba(64,158,255,.25); font-size: 13px; }
.dp-x { background: none; border: 0; color: #8899aa; cursor: pointer; font-size: 13px; }
.dp-kv { width: 100%; border-collapse: collapse; }
.dp-kv td { padding: 5px 12px; border-bottom: 1px solid rgba(255,255,255,.05); }
.dp-kv td:first-child { color: #8899aa; width: 72px; }
.dp-foot { display: flex; gap: 12px; padding: 8px 12px; }
.dp-link { color: #409eff; cursor: pointer; font-size: 11px; }
.dp-link:hover { text-decoration: underline; }
.badge.st-1 { color: #67c23a; border: 1px solid rgba(103,194,58,.4); border-radius: 4px; padding: 0 6px; }
.badge.st-2 { color: #e6a23c; border: 1px solid rgba(230,162,60,.4); border-radius: 4px; padding: 0 6px; }
.mono { font-family: monospace; }

/* 概览统计 */
.r3d-stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(110px, 1fr)); gap: 10px; }
.r3d-stats .st { background: var(--card2); border: 1px solid var(--border); border-radius: 10px; padding: 10px 14px; display: flex; flex-direction: column; gap: 2px; }
.r3d-stats .st b { font-size: 20px; font-variant-numeric: tabular-nums; }
.r3d-stats .st span { font-size: 12px; color: var(--muted); }
.r3d-stats .st.warn b { color: #ff5252; }
.small-stats .st { padding: 8px 10px; }
.small-stats .st b { font-size: 16px; }
.u-map { display: flex; flex-direction: column; gap: 1px; margin-top: 10px; max-height: 300px; overflow: auto; border: 1px solid var(--border); border-radius: 8px; padding: 6px; }
.u-row { display: flex; align-items: center; gap: 8px; height: 13px; font-size: 10px; }
.u-num { width: 30px; color: var(--muted); text-align: right; font-variant-numeric: tabular-nums; }
.u-bar { flex: 1; height: 9px; border-radius: 2px; background: rgba(138, 153, 184, .15); }
.u-row.occ .u-bar { opacity: .9; }
.u-name { font-size: 10px; color: var(--text2); max-width: 150px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.seg { display: inline-flex; background: var(--card2); border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.seg-btn { border: 0; background: none; padding: 6px 14px; cursor: pointer; font-size: 13px; color: var(--muted); }
.seg-btn.on { background: var(--primary, #1677ff); color: #fff; }
.sw { cursor: pointer; font-size: 13px; display: inline-flex; align-items: center; gap: 4px; color: var(--muted); }
.muted { color: var(--muted); }
.small { font-size: 11px; }
.clickable-row { cursor: pointer; }
.clickable-row:hover { background: var(--card2); }
.drawer-links { display: flex; gap: 8px; margin-top: 14px; }
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}
.drawer{max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}
.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}
.drawer-head h3{margin:0;font-size:18px}
.drawer-body{flex:1;overflow-y:auto;padding:20px}
@media (max-width: 768px) { .r3d-wrap { height: 55vh; } .drawer { width: 100vw !important; } }
.lib-panel { position: absolute; top: 100px; right: 14px; z-index: 28; width: 210px;
  background: rgba(13, 21, 32, .94); border: 1px solid rgba(100,160,255,.3); border-radius: 10px;
  padding: 10px; color: #cfe8ff; max-height: 66vh; overflow-y: auto; box-shadow: 0 4px 20px rgba(0,0,0,.5) }
.lib-h { font-size: 14px; font-weight: 700; margin-bottom: 8px; color: #fff }
.lib-sec { font-size: 11px; opacity: .65; margin: 8px 0 6px; display: flex; justify-content: space-between; align-items: center }
.lib-up { background: rgba(64,158,255,.18); border: 1px solid rgba(64,158,255,.5); color: #9fd4ff;
  border-radius: 4px; font-size: 11px; padding: 2px 8px; cursor: pointer }
.lib-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 6px }
.lib-item { display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 7px 4px;
  background: rgba(127,150,180,.1); border: 1px solid rgba(127,150,180,.25); border-radius: 7px;
  color: #cfe8ff; cursor: pointer; font-size: 11px; transition: all .15s }
.lib-item:hover { background: rgba(64,158,255,.25); border-color: rgba(64,158,255,.6) }
.lib-ico { font-size: 17px }
.lib-nm { white-space: nowrap }
.lib-empty { font-size: 11px; opacity: .5; padding: 6px 2px }
.lib-model { display: flex; align-items: center; gap: 4px; padding: 4px 2px; font-size: 11.5px }
.lm-nm { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap }
.lm-btn { background: rgba(64,158,255,.15); border: 1px solid rgba(64,158,255,.4); color: #9fd4ff;
  border-radius: 4px; font-size: 10.5px; padding: 2px 7px; cursor: pointer }
.lm-btn.del { border-color: rgba(255,82,82,.4); color: #ff8a80; background: rgba(255,82,82,.1) }
.props-panel { position: absolute; top: 100px; left: 14px; z-index: 28; width: 200px;
  background: rgba(13, 21, 32, .94); border: 1px solid rgba(255,167,38,.45); border-radius: 10px;
  padding: 10px 12px; color: #cfe8ff; box-shadow: 0 4px 20px rgba(0,0,0,.5) }
.pp-row { display: flex; align-items: center; justify-content: space-between; margin: 6px 0; font-size: 12px }
.pp-in { width: 86px; background: rgba(127,150,180,.12); border: 1px solid rgba(127,150,180,.35);
  border-radius: 5px; color: #e8f4ff; padding: 3px 7px; font-size: 12px }
.pp-hint { font-size: 10.5px; opacity: .5; margin-top: 8px; text-align: center }
.btn.sm { padding: 3px 9px; font-size: 12px }
.btn.danger { color: #ff8a80; border-color: rgba(255,82,82,.4) }
.edit-tip { position: absolute; top: 64px; left: 50%; transform: translateX(-50%); z-index: 30;
  background: rgba(255, 167, 38, .92); color: #1a1400; padding: 6px 18px; border-radius: 20px;
  font-size: 12.5px; font-weight: 600; box-shadow: 0 2px 12px rgba(0,0,0,.3); pointer-events: none }
.plan-tip { position: absolute; top: 104px; left: 50%; transform: translateX(-50%); z-index: 30;
  background: rgba(20, 32, 48, .94); color: #cfe8ff; padding: 7px 20px; border-radius: 8px;
  font-size: 13px; border: 1px solid rgba(100,160,255,.35); cursor: pointer; max-width: 70%; text-align: center }
.sw.editing { color: #ffa726; font-weight: 700 }
.plan-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px }
.pg { background: rgba(127,150,180,.08); border-radius: 8px; padding: 12px; text-align: center }
.pg b { display: block; font-size: 20px; color: #409eff }
.pg span { font-size: 12px; opacity: .7 }
.plan-gap { background: rgba(255,167,38,.12); border: 1px solid rgba(255,167,38,.4); border-radius: 8px;
  padding: 10px 14px; font-size: 13px; margin-bottom: 14px }
.plan-gap.ok { background: rgba(0,230,118,.1); border-color: rgba(0,230,118,.4) }
.btn.block { width: 100%; padding: 11px; font-size: 14px }
</style>
