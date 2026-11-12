<template>
  <div class="page-bar">
    <button v-if="back" class="pb-back" @click="goBack" title="返回上一页">← 返回</button>
    <div class="pb-crumb">
      <span class="pb-home" @click="$router.push('/dashboard')" title="回到运行总览">⌂</span>
      <span class="pb-sep">/</span>
      <router-link v-if="parentTo" :to="parentTo" class="pb-link">{{ parentLabel }}</router-link>
      <span v-else class="pb-link">{{ parentLabel }}</span>
      <span v-if="label" class="pb-sep">/</span>
      <span v-if="label" class="pb-cur">{{ label }}</span>
    </div>
    <div class="pb-right"><slot /></div>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'

const props = defineProps({
  // 当前页名称（面包屑末级）
  label: { type: String, default: '' },
  // 上级页名称与路径（可选）
  parentLabel: { type: String, default: '运行总览' },
  parentTo: { type: String, default: '' },
  // 是否显示返回按钮
  back: { type: Boolean, default: true }
})

const router = useRouter()
function goBack() {
  if (window.history.length > 2) router.back()
  else if (props.parentTo) router.push(props.parentTo)
  else router.push('/dashboard')
}
</script>

<style scoped>
.page-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; flex-wrap: wrap; }
.pb-back { background: none; border: 1px solid var(--border); color: var(--text2, var(--muted)); border-radius: 8px; padding: 5px 12px; font-size: 13px; cursor: pointer; transition: all .15s; }
.pb-back:hover { color: var(--accent); border-color: var(--accent); }
.pb-crumb { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--muted); }
.pb-home { cursor: pointer; font-size: 15px; }
.pb-home:hover { color: var(--accent); }
.pb-sep { opacity: .5; }
.pb-link { color: var(--muted); text-decoration: none; cursor: pointer; }
.pb-link:hover { color: var(--accent); }
a.pb-link { cursor: pointer; }
.pb-cur { color: var(--text, #333); font-weight: 600; }
.pb-right { margin-left: auto; display: flex; align-items: center; gap: 8px; }
</style>
