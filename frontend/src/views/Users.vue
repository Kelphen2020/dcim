<template>
  <div><div class="page-head"><h1>用户与权限</h1><p class="muted">管理账号、角色、站点范围与菜单权限。密码重置独立于编辑，避免误改。</p></div>
    <div class="toolbar">
      <span class="muted" style="font-size:12px">共 {{ list.length }} 个用户</span><span style="flex:1"></span>
      <button class="btn btn-primary" @click="openForm()">＋ 新建用户</button>
    </div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>用户名</th><th>显示名</th><th>角色</th><th>站点范围</th><th>菜单权限</th><th>电话</th><th>状态</th><th>最后登录</th><th>操作</th></tr></thead>
      <tbody><tr v-for="u in list" :key="u.id">
        <td class="mono">{{ u.username }}</td>
        <td>{{ u.display_name || '-' }}</td>
        <td><span class="badge" :class="'role-'+u.role">{{ ROLES[u.role]||u.role }}</span></td>
        <td><span v-if="u.role==='global_admin'" class="badge st-1">全部站点</span><span v-else-if="u.site_id_arr&&u.site_id_arr.length" class="badge">{{ u.site_id_arr.length }} 个站点</span><span v-else class="muted">-</span></td>
        <td><span v-if="!u.allowed_groups || u.allowed_groups==='*'" class="badge st-1">全部菜单</span><span v-else class="badge">{{ groupNames(u.allowed_groups) }}</span></td>
        <td class="muted">{{ u.phone || '-' }}</td>
        <td><span class="badge" :class="u.status?'st-1':'st-0'" style="cursor:pointer" @click="toggleStatus(u)" :title="u.status?'点击停用':'点击启用'">{{ u.status?'正常':'停用' }}</span></td>
        <td class="muted">{{ fmt(u.last_login_at) }}</td>
        <td class="ops">
          <button class="btn btn-mini" @click="openForm(u)">编辑</button>
          <button class="btn btn-mini" @click="openReset(u)">重置密码</button>
          <button v-if="u.username!=='admin'" class="btn btn-mini btn-mini-danger" @click="del(u)">删除</button>
        </td></tr>
      <tr v-if="!list.length"><td colspan="9" class="empty">暂无</td></tr></tbody>
    </table></div></div>

    <!-- 新建/编辑抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer"><div class="drawer-head"><h3>{{ form.id?'编辑':'新建' }}用户</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend><div class="form-grid">
          <label>用户名 *<input class="input" v-model="form.username" :disabled="!!form.id" /></label>
          <label>显示名称<input class="input" v-model="form.display_name" /></label>
          <label>角色 *<select class="input" v-model="form.role"><option v-for="(n,k) in ROLES" :key="k" :value="k">{{ n }}</option></select></label>
          <label>电话<input class="input" v-model="form.phone" /></label>
          <label v-if="!form.id">初始密码 *<input class="input" type="password" v-model="form.password" placeholder="至少8位，含大小写字母/数字/特殊字符" /></label>
          <label>状态<select class="input" v-model.number="form.status"><option :value="1">正常</option><option :value="0">停用</option></select></label>
        </div></fieldset>
        <fieldset class="fs" v-if="form.role!=='global_admin'"><legend><SvgIcon name="building" />站点范围（可多选；不选则无权访问任何站点）</legend>
          <div class="chk-wrap">
            <label v-for="s in sites" :key="s.id" class="chk"><input type="checkbox" :value="s.id" v-model="form.site_ids_arr" />{{ s.name }}</label>
            <div v-if="!sites.length" class="muted">暂无站点</div>
          </div>
        </fieldset>
        <fieldset class="fs" v-else><p class="muted" style="margin:0">全局管理员默认可访问全部站点，无需配置站点范围。</p></fieldset>
        <fieldset class="fs"><legend><SvgIcon name="puzzle" />菜单权限（控制左侧导航可见的功能分组）</legend>
          <label class="chk" style="margin-bottom:10px"><input type="checkbox" v-model="form.allGroups" />全部菜单（默认）</label>
          <div v-if="!form.allGroups" class="chk-wrap">
            <label v-for="g in groups" :key="g.key" class="chk"><input type="checkbox" :value="g.key" v-model="form.groupKeys" />{{ g.name }}</label>
          </div>
          <p class="muted" style="margin:8px 0 0;font-size:12px">未勾选的分组对该用户隐藏；具体页面内数据仍受角色与站点范围约束。</p>
        </fieldset>
        <p v-if="form.err" class="err">{{ form.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>

    <!-- 重置密码抽屉 -->
    <div v-if="reset.show" class="drawer-mask" @click.self="reset.show=false"><div class="drawer" style="width:440px"><div class="drawer-head"><h3>重置密码 · {{ reset.username }}</h3><button class="btn btn-mini" @click="reset.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="key" />新密码</legend><div class="form-grid" style="grid-template-columns:1fr">
          <label>新密码 *<input class="input" type="password" v-model="reset.pw1" placeholder="至少8位，含大小写字母/数字/特殊字符" /></label>
          <p v-if="reset.pw1 && !pwStrong" class="err" style="margin:4px 0 0">{{ pwHint }}</p>
          <label>确认新密码 *<input class="input" type="password" v-model="reset.pw2" /></label>
          <p v-if="reset.pw2 && reset.pw1!==reset.pw2" class="err" style="margin:4px 0 0">两次输入的密码不一致</p>
        </div></fieldset>
        <p v-if="reset.ok" class="ok-msg"><SvgIcon name="check" />{{ reset.ok }}</p>
        <p class="muted" style="font-size:12px">重置后旧密码立即失效，请通过安全渠道告知用户。操作会记入审计日志。</p>
        <p v-if="reset.err" class="err">{{ reset.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="reset.show=false">关闭</button><button class="btn btn-primary" :disabled="resetting" @click="doReset">{{ resetting?'重置中...':'确认重置' }}</button></div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,computed,onMounted} from 'vue';import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
const ROLES={global_admin:'全局管理员',site_admin:'站点管理员',operator:'值班员',viewer:'只读'}
const list=ref([]),sites=ref([]),groups=ref([])
const form=reactive({show:false,id:0,username:'',display_name:'',role:'viewer',phone:'',password:'',status:1,site_ids_arr:[],allGroups:true,groupKeys:[],err:''})
const reset=reactive({show:false,id:0,username:'',pw1:'',pw2:'',err:'',ok:''})
const resetting=ref(false)
// 与后端 ValidatePasswordStrength 同口径的本地预检
function strengthCheck(pw){
  if(!pw)return[false,'请输入新密码']
  if(pw.length<8)return[false,'密码至少8位']
  const miss=[]
  if(!/[A-Z]/.test(pw))miss.push('大写字母')
  if(!/[a-z]/.test(pw))miss.push('小写字母')
  if(!/[0-9]/.test(pw))miss.push('数字')
  if(!/[^A-Za-z0-9]/.test(pw))miss.push('特殊字符')
  if(miss.length)return[false,'密码需包含：'+miss.join('、')]
  return[true,'']
}
const pwStrong=computed(()=>strengthCheck(reset.pw1)[0])
const pwHint=computed(()=>strengthCheck(reset.pw1)[1])
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
const groupNameMap=ref({})
function groupNames(s){return String(s).split(',').map(k=>groupNameMap.value[k]||k).join('、')}
async function load(){
  const r=await api.get('/users',{size:200})
  list.value=r.data?.items||[]
  groups.value=r.data?.groups||[]
  const m={};(r.data?.groups||[]).forEach(g=>{m[g.key]=g.name});groupNameMap.value=m
}
async function loadSites(){const r=await api.get('/sites',{size:200});sites.value=r.data?.items||[]}
function openForm(u){
  const ag=u?.allowed_groups||'*'
  if(u)Object.assign(form,{show:true,id:u.id,username:u.username,display_name:u.display_name||'',role:u.role,phone:u.phone||'',password:'',status:u.status??1,site_ids_arr:[...(u.site_id_arr||[])],allGroups:(ag==='*'),groupKeys:ag==='*'?[]:String(ag).split(',').map(s=>s.trim()).filter(Boolean),err:''})
  else Object.assign(form,{show:true,id:0,username:'',display_name:'',role:'viewer',phone:'',password:'',status:1,site_ids_arr:[],allGroups:true,groupKeys:[],err:''})
}
async function save(){
  form.err=''
  if(!form.id&&!form.username){form.err='用户名必填';return}
  if(!form.id&&!form.password){form.err='初始密码必填';return}
  const allowedGroups=form.allGroups?'*':form.groupKeys.join(',')
  if(!form.allGroups&&!form.groupKeys.length){form.err='请至少勾选一个菜单分组，或勾选全部菜单';return}
  const payload={display_name:form.display_name,role:form.role,phone:form.phone,status:form.status,allowed_groups:allowedGroups}
  if(form.role!=='global_admin')payload.site_ids=form.site_ids_arr.join(',')
  let r
  if(form.id)r=await api.put(`/users/${form.id}`,payload)
  else r=await api.post('/users',{...payload,username:form.username,password:form.password,site_ids:form.role==='global_admin'?'':form.site_ids_arr.join(',')})
  if(r.code===0){form.show=false;load()}else form.err=r.msg||'保存失败'
}
function openReset(u){Object.assign(reset,{show:true,id:u.id,username:u.username,pw1:'',pw2:'',err:'',ok:''})}
async function doReset(){
  reset.err='';reset.ok=''
  const [ok,hint]=strengthCheck(reset.pw1)
  if(!ok){reset.err=hint;return}
  if(reset.pw1!==reset.pw2){reset.err='两次输入的密码不一致';return}
  resetting.value=true
  try{
    const r=await api.post(`/users/${reset.id}/reset-password`,{new_password:reset.pw1})
    if(r&&r.code===0){reset.ok=`用户 ${reset.username} 的密码已重置成功`;reset.pw1='';reset.pw2=''}
    else reset.err=(r&&r.msg)||'重置失败'
  }catch(e){reset.err='请求失败：'+(e.message||e)}
  finally{resetting.value=false}
}
async function toggleStatus(u){
  const ns=u.status?0:1
  const verb=ns?'启用':'停用'
  if (!(await showConfirm({ message: `确认${verb}用户 ${u.username}？` })))return
  const r=await api.put(`/users/${u.id}`,{role:u.role,site_ids:u.role==='global_admin'?'':(u.site_id_arr||[]).join(','),status:ns})
  if(r.code===0)load();else toastAuto(r.msg||'操作失败')
}
async function del(u){if (!(await showConfirm({ message: `确认删除用户 ${u.username}？此操作不可恢复。` })))return;const r=await api.del(`/users/${u.id}`);if(r.code===0)load();else toastAuto(r.msg||'删除失败')}
onMounted(()=>{load();loadSites()})
</script>
<style scoped>
.badge.role-global_admin{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.role-site_admin{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.role-operator{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.role-viewer{color:#909399;border-color:rgba(144,147,153,.4)}
.badge.st-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.st-0{color:#909399;border-color:rgba(144,147,153,.4)}
.chk-wrap{display:flex;flex-wrap:wrap;gap:8px 16px}
.chk{display:flex;align-items:center;gap:6px;font-size:13px;cursor:pointer}
.chk input{accent-color:var(--accent)}
.err{color:#f56c6c;font-size:13px;margin:8px 0 0}
.ok-msg{color:#67c23a;font-size:13.5px;font-weight:600;background:rgba(103,194,58,.08);border:1px solid rgba(103,194,58,.3);border-radius:8px;padding:10px 12px;margin:10px 0 0}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
</style>
