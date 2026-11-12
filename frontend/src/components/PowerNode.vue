<template>
  <div class="chain-node">
    <div class="node-card" :style="{ marginLeft: depth * 36 + 'px' }">
      <span class="node-kind">{{ KINDS[node.kind] || node.kind }}</span>
      <span class="node-name">{{ node.name }}</span>
      <span v-if="node.model" class="muted">{{ node.model }}</span>
      <span class="badge" :class="st[1]">{{ st[0] }}</span>
      <span class="muted">{{ node.rated_kva ? `${node.load_kw}/${node.rated_kva}kVA` : `${node.load_kw}kW` }}</span>
      <span v-if="loadPct !== null" class="pct" :class="{ 'pct-high': loadPct > 80 }">{{ loadPct }}%</span>
      <span class="ops">
        <button class="btn btn-mini" @click="$emit('add-child', node.id)">＋子级</button>
        <button class="btn btn-mini" @click="$emit('edit', node)">编辑</button>
        <button class="btn btn-mini btn-mini-danger" @click="$emit('del', node)">删除</button>
      </span>
    </div>
    <PowerNode v-for="k in children" :key="k.id" :node="k" :nodes="nodes" :depth="depth + 1"
               @add-child="id => $emit('add-child', id)" @edit="n => $emit('edit', n)" @del="n => $emit('del', n)" />
  </div>
</template>

<script setup>
import { computed } from 'vue'

const KINDS = { utility: '市电', transformer: '变压器', ups: 'UPS', ats: 'ATS', rack_pdu: '列头柜', pdu: 'PDU', other: '其他' }
const ST = { 1: ['正常', 'st-1'], 2: ['告警', 'st-warn'], 3: ['故障', 'st-err'], 0: ['停用', 'st-0'] }

const props = defineProps({ node: Object, nodes: Array, depth: Number })
defineEmits(['add-child', 'edit', 'del'])

const st = computed(() => ST[props.node.status] || ST[0])
const loadPct = computed(() => props.node.rated_kva > 0 ? Math.round(props.node.load_kw / props.node.rated_kva * 100) : null)
const children = computed(() => props.nodes.filter(n => n.parent_id === props.node.id))
</script>

<style scoped>
.node-card { display: flex; align-items: center; gap: 12px; padding: 8px 12px; margin: 4px 0; background: rgba(64, 158, 255, .05); border: 1px solid rgba(64, 158, 255, .15); border-radius: 6px; font-size: 13px; }
.node-kind { font-size: 11px; padding: 2px 8px; border-radius: 3px; background: rgba(64, 158, 255, .15); color: #409eff; white-space: nowrap; }
.node-name { font-weight: 600; }
.pct { font-weight: 600; color: #67c23a; }
.pct-high { color: #f56c6c; }
.badge.st-1 { color: #67c23a; border-color: rgba(103, 194, 58, .4); }
.badge.st-warn { color: #e6a23c; border-color: rgba(230, 162, 60, .4); }
.badge.st-err { color: #f56c6c; border-color: rgba(245, 108, 108, .4); }
.badge.st-0 { color: #909399; border-color: rgba(144, 147, 153, .4); }
</style>
