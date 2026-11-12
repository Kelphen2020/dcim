<template>
  <div>
    <div class="page-head"><h1>开放 API</h1></div>
    <div class="card">
      <div class="card-title">{{ info.title }} {{ info.version }}</div>
      <p class="muted" style="margin-bottom:12px">Phase: {{ info.phase }} · {{ info.note }}</p>
      <div class="table-wrap"><table class="data-table">
        <thead><tr><th>方法</th><th>路径</th><th>说明</th></tr></thead>
        <tbody>
          <tr v-for="ep in info.endpoints" :key="ep.path">
            <td><span class="badge" :class="'method-'+ep.method.toLowerCase()">{{ ep.method }}</span></td>
            <td class="mono">{{ ep.path }}</td>
            <td>{{ ep.desc }}</td>
          </tr>
        </tbody>
      </table></div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
const info=ref({endpoints:[]})
onMounted(async()=>{const r=await api.get('/api-docs');info.value=r.data||{endpoints:[]}})
</script>

<style scoped>
.badge.method-get{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.method-post{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.method-put{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.method-delete{color:#f56c6c;border-color:rgba(245,108,108,.4)}
</style>