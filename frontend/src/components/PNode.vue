<template>
  <div class="pn" :style="{ marginLeft: depth ? '26px' : '0' }">
    <div class="pn-card" :class="{ warn: node.load_pct >= 80 || node.status > 1 || node.active_alerts > 0 }">
      <span class="pn-kind" :style="{ background: KIND_COLOR[node.kind] || '#909399' }">{{ KIND[node.kind] || node.kind }}</span>
      <span class="pn-name">{{ node.name }}</span>
      <span v-if="node.model" class="pn-model">{{ node.model }}</span>
      <span class="pn-load">
        <b :class="pctClass(node.load_pct)">{{ node.effective_kw.toFixed(1) }} kW</b>
        <span v-if="node.rated_kva" class="pn-pct">/ {{ node.rated_kva }}kVA · {{ node.load_pct.toFixed(0) }}%</span>
        <span class="badge mini" :class="node.load_source === 'point' ? 'src-live' : 'src-manual'">{{ node.load_source === 'point' ? '实时' : '手工' }}</span>
      </span>
      <span v-if="node.children_kw > 0" class="pn-sub">下级合计 {{ node.children_kw.toFixed(1) }} kW</span>
      <span v-if="node.asset_name" class="pn-rel" :title="'关联资产: ' + node.asset_name"><SvgIcon name="box" />{{ node.asset_name }}</span>
      <span v-if="node.cabinet_code" class="pn-rel" :title="'供电机柜'"><SvgIcon name="rack" />{{ node.cabinet_code }}</span>
      <span v-if="node.point_name" class="pn-rel" :title="'绑定测点: ' + node.point_name + (node.point_value != null ? ' 当前 ' + node.point_value : '')"><SvgIcon name="trend" />{{ node.point_name }}</span>
      <span v-if="node.active_alerts > 0" class="pn-alert"><SvgIcon name="bell" />{{ node.active_alerts }}</span>
      <span class="pn-st" :class="'st-' + node.status"></span>
      <span class="pn-ops">
        <button class="btn btn-mini" @click="$emit('impact', node)">影响分析</button>
        <button class="btn btn-mini" @click="$emit('edit', node)">编辑</button>
        <button class="btn btn-mini btn-mini-danger" @click="$emit('del', node)">删除</button>
      </span>
    </div>
    <div v-if="kids.length" class="pn-kids">
      <PNode v-for="ch in kids" :key="ch.id" :node="ch" :children-map="childrenMap" :depth="depth + 1"
             @edit="$emit('edit', $event)" @del="$emit('del', $event)" @impact="$emit('impact', $event)" />
    </div>
  </div>
</template>
<script setup>
import { computed } from 'vue'
const props = defineProps({ node: Object, childrenMap: Object, depth: Number })
defineEmits(['edit', 'del', 'impact'])
defineOptions({ name: 'PNode' })
const KIND = { utility: '市电', incoming: '总进线', transformer: '变压器', ups: 'UPS', pdu: 'PDU', busbar: '母线', branch: '支路', other: '其他' }
const KIND_COLOR = { utility: '#8b5cf6', incoming: '#0ea5e9', transformer: '#f59e0b', ups: '#10b981', pdu: '#3b82f6', busbar: '#eab308', branch: '#94a3b8', other: '#909399' }
const kids = computed(() => props.childrenMap[props.node.id] || [])
function pctClass(p) { return p >= 90 ? 'red' : p >= 70 ? 'yellow' : 'green' }
</script>
<style scoped>
.pn { position: relative; }
.pn-kids { border-left: 2px dashed var(--border); margin-left: 14px; padding-left: 12px; margin-top: 6px; }
.pn-card { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 10px 14px; margin-bottom: 6px; box-shadow: var(--shadow-sm); }
.pn-card.warn { border-color: rgba(245, 108, 108, .55); background: rgba(245, 108, 108, .04); }
.pn-kind { color: #fff; font-size: 11px; font-weight: 700; padding: 3px 9px; border-radius: 5px; flex-shrink: 0; }
.pn-name { font-weight: 700; font-size: 14.5px; }
.pn-model { font-size: 12px; color: var(--muted); }
.pn-load { display: flex; align-items: center; gap: 5px; margin-left: auto; }
.pn-load b { font-size: 15px; }
.pn-load b.green { color: #67c23a; } .pn-load b.yellow { color: #e6a23c; } .pn-load b.red { color: #f56c6c; }
.pn-pct { font-size: 11.5px; color: var(--muted); }
.badge.mini { font-size: 10.5px; padding: 1px 6px; }
.badge.src-live { color: #409eff; border-color: rgba(64, 158, 255, .4); } .badge.src-manual { color: #909399; border-color: rgba(144, 147, 153, .4); }
.pn-sub { font-size: 11px; color: var(--muted); background: var(--bg2); border-radius: 5px; padding: 2px 8px; }
.pn-rel { font-size: 11.5px; color: var(--text2, var(--muted)); background: var(--bg2); border-radius: 5px; padding: 2px 8px; }
.pn-alert { font-size: 12px; color: #f56c6c; font-weight: 700; }
.pn-st { width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0; }
.pn-st.st-1 { background: #67c23a; } .pn-st.st-2 { background: #e6a23c; } .pn-st.st-3 { background: #f56c6c; animation: blink 1s infinite; } .pn-st.st-0 { background: #909399; }
@keyframes blink { 50% { opacity: .3; } }
.pn-ops { display: flex; gap: 5px; }
</style>
