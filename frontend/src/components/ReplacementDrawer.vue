<template>
  <div class="drawer-mask" @click.self="$emit('close')">
    <div class="drawer" style="width:720px">
      <div class="drawer-head">
        <h3><SvgIcon name="box" /> 替换建议 · {{ assetName }}</h3>
        <button class="btn btn-mini" @click="$emit('close')"><SvgIcon name="x" /></button>
      </div>
      <div class="drawer-body">
        <div v-if="err" class="rp-err">{{ err }}</div>
        <template v-else-if="d">
          <div class="rp-trigger" :class="d.trigger.recommended ? 'bad' : 'ok'">
            <SvgIcon :name="d.trigger.recommended ? 'alert' : 'info'" />
            <div>
              <b>{{ d.trigger.recommended ? '已触发替换建议' : '暂未触发替换条件' }}</b>
              <div class="small">{{ d.trigger.reason }}</div>
            </div>
          </div>

          <div class="rp-grid">
            <div><span>资产编码</span><b class="mono">{{ d.asset.code }}</b></div>
            <div><span>分类 / 品牌型号</span><b>{{ d.asset.category }} · {{ d.asset.brand || '-' }} {{ d.asset.model || '' }}</b></div>
            <div><span>当前健康</span><b :class="d.health && d.health.score < 60 ? 'rp-bad' : ''">{{ d.health ? d.health.score + ' 分' : '未评估' }}</b></div>
            <div><span>退役状态</span><b>{{ d.retirement ? (d.retirement.code + ' 已批准') : '无已批准退役' }}</b></div>
          </div>

          <div v-if="d.existing" class="rp-existing">
            该资产已有待审批的替换采购申请 <b class="mono">{{ d.existing.code }}</b>
            <button class="btn btn-mini" @click="goPR">去审批</button>
          </div>
          <div v-else-if="d.recent" class="rp-existing">
            30 天内已生成过替换申请 <b class="mono">{{ d.recent.code }}</b>，为避免重复采购，暂不能再次提交。
          </div>

          <fieldset class="fs">
            <legend><SvgIcon name="search" /> 同分类参照（在架 {{ (d.candidates || []).length }} · 在库 {{ (d.spares || []).length }}）</legend>
            <div class="table-wrap">
              <table class="data-table rp-table">
                <thead><tr><th>来源</th><th>编码</th><th>名称</th><th>规格</th><th>库存</th><th>健康度</th></tr></thead>
                <tbody>
                  <tr v-for="c in [...(d.candidates || []), ...(d.spares || [])]" :key="c.kind + '-' + c.id">
                    <td><span class="badge" :class="c.kind === 'asset' ? 'ok' : 'muted'">{{ c.kind === 'asset' ? '在架' : '在库' }}</span></td>
                    <td class="mono small">{{ c.code }}</td>
                    <td>{{ c.name }}</td>
                    <td class="small">{{ c.model || '-' }}</td>
                    <td>{{ c.kind === 'spare' ? c.stock : '—' }}</td>
                    <td><span v-if="c.health_score != null" class="badge hg" :class="'hg-' + c.health_grade">{{ c.health_score }}</span><span v-else class="muted">—</span></td>
                  </tr>
                  <tr v-if="!(d.candidates || []).length && !(d.spares || []).length"><td colspan="6" class="muted small">暂无同分类参照，按原资产规格采购</td></tr>
                </tbody>
              </table>
            </div>
          </fieldset>

          <fieldset class="fs">
            <legend><SvgIcon name="order" /> 采购申请草稿</legend>
            <div class="form-grid">
              <label>数量<input class="input" type="number" min="1" max="99" v-model.number="form.qty" /></label>
              <label>预算（元）<input class="input" type="number" min="0" step="0.01" v-model.number="form.budget_yuan" /></label>
              <label class="full">供应商<input class="input" v-model="form.supplier" placeholder="默认带出资产供应商，可修改" /></label>
              <label class="full">事由<textarea class="input" rows="3" v-model="form.reason" style="width:100%;box-sizing:border-box"></textarea></label>
            </div>
            <div class="rp-note muted small">确认后生成“待审批”采购申请，进入现有采购审批流；不会直接下单。同资产 30 天内仅允许一条替换申请。</div>
          </fieldset>
        </template>
        <div v-else class="muted" style="padding:20px 0">加载替换建议…</div>
      </div>
      <div class="drawer-foot">
        <button class="btn" @click="$emit('close')">取消</button>
        <button class="btn btn-primary" :disabled="!d || !d.trigger.recommended || !!d.existing || saving" @click="submit">
          {{ saving ? '提交中…' : '生成采购申请草稿' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import SvgIcon from './SvgIcon.vue'
import { toastAuto, showConfirm } from '../ui-dialog'

const props = defineProps({ assetId: { type: Number, required: true }, assetName: { type: String, default: '' } })
const emit = defineEmits(['close', 'created'])
const router = useRouter()
const d = ref(null), err = ref(''), saving = ref(false)
const form = reactive({ qty: 1, budget_yuan: 0, supplier: '', reason: '' })

async function load() {
  err.value = ''
  const r = await api.get(`/assets/${props.assetId}/replacement-suggestion`)
  if (r.code !== 0) { err.value = r.msg || '加载失败'; return }
  d.value = r.data
  form.budget_yuan = Number(r.data.budget_yuan || 0)
  form.supplier = r.data.supplier || ''
  form.reason = r.data.suggested_reason || ''
}
function goPR() { emit('close'); router.push('/spare-parts?tab=purchase') }
async function submit() {
  if (!d.value?.trigger?.recommended) return
  const ok = await showConfirm({ message: `为「${props.assetName}」生成替换采购申请？\n数量 ${form.qty} 台，预算 ¥${Number(form.budget_yuan || 0).toFixed(2)}。`, tone: 'primary' })
  if (!ok) return
  saving.value = true
  try {
    const r = await api.post(`/assets/${props.assetId}/replacement-request`, { ...form })
    if (r.code === 0) {
      toastAuto('替换采购申请已生成，等待审批')
      emit('created', r.data)
      emit('close')
    } else toastAuto(r.msg)
  } finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped>
.rp-err { color: var(--danger); padding: 12px 0; }
.rp-trigger { display: flex; gap: 10px; align-items: flex-start; padding: 10px 12px; border-radius: 8px; margin-bottom: 12px; }
.rp-trigger.bad { background: color-mix(in srgb, var(--danger) 8%, transparent); border: 1px solid color-mix(in srgb, var(--danger) 28%, transparent); }
.rp-trigger.ok { background: color-mix(in srgb, var(--success) 8%, transparent); border: 1px solid color-mix(in srgb, var(--success) 28%, transparent); }
.rp-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 18px; margin: 6px 0 12px; font-size: 13px; }
.rp-grid > div { display: flex; gap: 8px; }
.rp-grid span { color: var(--muted); min-width: 88px; }
.rp-bad { color: var(--danger); }
.rp-existing { margin: 8px 0 12px; padding: 9px 10px; border-radius: 8px; background: color-mix(in srgb, var(--warning) 9%, transparent); border: 1px solid color-mix(in srgb, var(--warning) 30%, transparent); font-size: 13px; display: flex; align-items: center; gap: 8px; }
.rp-table td, .rp-table th { font-size: 12.5px; padding: 7px 8px; }
.rp-note { margin-top: 8px; }
</style>