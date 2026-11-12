<template>
  <div>
    <div class="page-head"><h1>趋势预测预警</h1>
      <div class="toolbar" style="margin:0">
        <select class="input" style="width:130px" v-model.number="hours" @change="load">
          <option :value="12">近 12 小时</option>
          <option :value="24">近 24 小时</option>
          <option :value="48">近 48 小时</option>
        </select>
        <label class="ck"><input type="checkbox" v-model="breachOnly" @change="load" /> 只看将越限</label>
        <label class="ck"><input type="checkbox" v-model="movingOnly" @change="load" /> 只看变化中</label>
        <button class="btn" @click="load">刷新</button>
      </div>
    </div>

    <div class="stat-row">
      <div class="stat-card" :class="{warn: breaches>0}"><b>{{ breaches }}</b><span>24h内将越限</span></div>
      <div class="stat-card"><b>{{ rising }}</b><span>持续上升</span></div>
      <div class="stat-card"><b>{{ falling }}</b><span>持续下降</span></div>
      <div class="stat-card"><b>{{ list.length }}</b><span>监测测点</span></div>
    </div>

    <div v-if="breaches>0" class="banner"><SvgIcon name="alert" />有 {{ breaches }} 个测点按当前趋势将在 24 小时内越过阈值，建议提前巡检处置（系统每 10 分钟自动扫描并推送预警）</div>

    <table class="data-table">
      <thead><tr>
        <th>测点</th><th>当前值</th><th>趋势</th><th>变化率/h</th>
        <th>参照阈值</th><th>预计越限</th><th>24h预测</th><th>置信度</th><th>样本</th>
      </tr></thead>
      <tbody>
        <tr v-for="f in list" :key="f.point_id" :class="{breach: f.will_breach}">
          <td><b>{{ f.point_name }}</b><div class="dim code">{{ f.point_code }}</div></td>
          <td>{{ f.current.toFixed(2) }} {{ f.unit }}</td>
          <td>
            <span class="dir" :class="f.direction">
              {{ f.direction==='rising'?'↗ 上升':f.direction==='falling'?'↘ 下降':'→ 平稳' }}
            </span>
          </td>
          <td :class="f.direction">{{ (f.slope>=0?'+':'') + f.slope.toFixed(3) }}</td>
          <td class="dim">{{ f.threshold ? f.threshold.toFixed(2)+' '+f.unit : '—' }}</td>
          <td>
            <span v-if="f.hours_to_limit>=0" class="tag" :class="f.will_breach?'danger':'warn'">
              {{ f.hours_to_limit.toFixed(1) }}h 后
            </span>
            <span v-else class="dim">不会越限</span>
          </td>
          <td :class="{'danger': f.will_breach}"><b>{{ f.predicted_24h.toFixed(2) }}</b></td>
          <td><span class="tag" :class="'c-'+f.confidence">{{ confTxt(f.confidence) }}</span></td>
          <td class="dim">{{ f.samples }}</td>
        </tr>
        <tr v-if="!list.length"><td colspan="9" class="empty">无符合条件的测点（需有阈值且近期有读数的测点）</td></tr>
      </tbody>
    </table>

    <div class="note">
      预测方法：对近 {{ hours }} 小时读数做最小二乘线性回归，外推至阈值。置信度综合样本数与拟合优度 R²；
      R²&lt;0.3 或样本&lt;12 判为低置信，仅供参考。趋势预警不替代实时阈值告警，用于<b>提前</b>发现缓慢漂移（如冷通道温度爬升、UPS 负载增长）。
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const list = ref([]); const breaches = ref(0)
const hours = ref(24); const breachOnly = ref(false); const movingOnly = ref(false)
const rising = ref(0); const falling = ref(0)

async function load() {
  const r = await api.get('/trend-forecast', { hours: hours.value, breach_only: breachOnly.value ? 1 : '', moving_only: movingOnly.value ? 1 : '' })
  if (r.code === 0) {
    list.value = r.data.list || []
    breaches.value = r.data.breaches || 0
    rising.value = list.value.filter(f => f.direction === 'rising').length
    falling.value = list.value.filter(f => f.direction === 'falling').length
  }
}
const confTxt = c => c === 'high' ? '高' : c === 'medium' ? '中' : '低'
onMounted(load)
</script>

<style scoped>
.stat-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 14px }
.stat-card { background: var(--card, rgba(127,150,180,.07)); border-radius: 10px; padding: 14px; text-align: center }
.stat-card b { display: block; font-size: 24px; color: var(--accent, #409eff) }
.stat-card.warn b { color: #ff5252 }
.stat-card span { font-size: 12px; opacity: .7 }
.banner { background: rgba(255,82,82,.12); border: 1px solid rgba(255,82,82,.4); border-radius: 8px;
  padding: 10px 16px; margin-bottom: 14px; font-size: 13px; color: #ff8a80 }
.breach { background: rgba(255,82,82,.06) }
.dir.rising { color: #ffa726 } .dir.falling { color: #40c4ff } .dir.flat { color: #8a93a6 }
td.rising { color: #ffa726 } td.falling { color: #40c4ff }
.danger { color: #ff5252 }
.code { font-size: 11px; font-family: monospace }
.tag { font-size: 11px; padding: 1px 8px; border-radius: 8px }
.tag.danger { background: rgba(255,82,82,.15); color: #ff5252 }
.tag.warn { background: rgba(255,167,38,.14); color: #ffa726 }
.tag.c-high { background: rgba(0,230,118,.12); color: #00e676 }
.tag.c-medium { background: rgba(255,193,61,.12); color: #ffc53d }
.tag.c-low { background: rgba(138,147,166,.14); color: #8a93a6 }
.note { font-size: 12px; opacity: .6; line-height: 1.7; margin-top: 14px; padding: 10px 14px;
  background: var(--card, rgba(127,150,180,.05)); border-radius: 8px }
.toolbar { display: flex; gap: 10px; align-items: center }
.ck { font-size: 13px; display: flex; align-items: center; gap: 4px }
.empty { text-align: center; opacity: .5; padding: 30px !important }
</style>
