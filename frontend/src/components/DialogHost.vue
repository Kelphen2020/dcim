<template>
  <!-- V57 全局交互宿主：toast + 抽屉式 confirm/prompt（挂在 App.vue，全站单例） -->
  <div class="dh-root">
    <!-- Toast 提示（顶部居中，非阻塞，自动消失） -->
    <transition-group name="dh-toast" tag="div" class="dh-toasts">
      <div v-for="t in state.toasts" :key="t.id" class="dh-toast" :class="'dh-' + t.type">
        <SvgIcon :name="t.type === 'error' ? 'alert' : 'check'" />
        <span>{{ t.msg }}</span>
      </div>
    </transition-group>

    <!-- 确认抽屉（右侧滑入 mini drawer，替代原生 confirm 与居中弹窗） -->
    <transition name="dh-fade">
      <div v-if="state.confirm.show" class="dh-mask" @click.self="resolveConfirm(false)">
        <transition name="dh-slide" appear>
          <div class="dh-drawer" role="dialog" aria-modal="true">
            <div class="dh-head">
              <span class="dh-tone" :class="'dh-tone-' + state.confirm.tone"><SvgIcon :name="state.confirm.tone === 'danger' ? 'alert' : 'info'" /></span>
              <h3>{{ state.confirm.title }}</h3>
              <button class="dh-x" @click="resolveConfirm(false)"><SvgIcon name="x" /></button>
            </div>
            <div class="dh-body">
              <p class="dh-msg">{{ state.confirm.message }}</p>
              <div v-if="state.confirm.requirePassword" class="dh-pwd">
                <input type="password" class="input" v-model="state.confirm._pwd" placeholder="请输入当前登录密码以确认身份" @keyup.enter="resolveConfirm(true)" ref="pwdInput" />
              </div>
            </div>
            <div class="dh-foot">
              <button class="btn" @click="resolveConfirm(false)">取消</button>
              <button class="btn" :class="state.confirm.tone === 'danger' ? 'btn-danger' : 'btn-primary'"
                      :disabled="state.confirm.requirePassword && !state.confirm._pwd"
                      @click="resolveConfirm(true)">{{ state.confirm.confirmText }}</button>
            </div>
          </div>
        </transition>
      </div>
    </transition>

    <!-- 输入抽屉（替代原生 prompt） -->
    <transition name="dh-fade">
      <div v-if="state.prompt.show" class="dh-mask" @click.self="resolvePrompt(null)">
        <transition name="dh-slide" appear>
          <div class="dh-drawer" role="dialog" aria-modal="true">
            <div class="dh-head">
              <h3>{{ state.prompt.title }}</h3>
              <button class="dh-x" @click="resolvePrompt(null)"><SvgIcon name="x" /></button>
            </div>
            <div class="dh-body">
              <label class="dh-label">{{ state.prompt.label }}</label>
              <textarea v-if="state.prompt.multiline" class="input dh-input" rows="3" v-model="state.prompt._value"
                        :placeholder="state.prompt.placeholder" ref="promptInput" @keydown.ctrl.enter="resolvePrompt(state.prompt._value)"></textarea>
              <input v-else class="input dh-input" v-model="state.prompt._value"
                     :placeholder="state.prompt.placeholder" ref="promptInput" @keyup.enter="resolvePrompt(state.prompt._value)" />
            </div>
            <div class="dh-foot">
              <button class="btn" @click="resolvePrompt(null)">取消</button>
              <button class="btn btn-primary" @click="resolvePrompt(state.prompt._value)">确定</button>
            </div>
          </div>
        </transition>
      </div>
    </transition>
  </div>
</template>
<script setup>
import { nextTick, watch, ref } from 'vue'
import { dialogState as state } from '../ui-dialog'
import SvgIcon from './SvgIcon.vue'

const pwdInput = ref(null), promptInput = ref(null)

function resolveConfirm(ok) {
  const c = state.confirm
  const r = c._resolve
  c.show = false
  c._resolve = null
  if (r) r(ok && (!c.requirePassword || !!c._pwd) ? (c.requirePassword ? c._pwd : true) : false)
}
function resolvePrompt(val) {
  const p = state.prompt
  const r = p._resolve
  p.show = false
  p._resolve = null
  if (r) r(val)
}
watch(() => state.confirm.show, v => { if (v) nextTick(() => pwdInput.value?.focus()) })
watch(() => state.prompt.show, v => { if (v) nextTick(() => promptInput.value?.focus()) })
// ESC 关闭
function onKey(e) {
  if (e.key !== 'Escape') return
  if (state.prompt.show) resolvePrompt(null)
  else if (state.confirm.show) resolveConfirm(false)
}
watch(() => state.confirm.show || state.prompt.show, v => {
  if (v) window.addEventListener('keydown', onKey)
  else window.removeEventListener('keydown', onKey)
})
</script>
<style scoped>
.dh-root { position: relative; z-index: 3000; }
/* ---- toast ---- */
.dh-toasts { position: fixed; top: 18px; left: 50%; transform: translateX(-50%); display: flex; flex-direction: column; gap: 8px; z-index: 3200; pointer-events: none; }
.dh-toast { display: flex; align-items: center; gap: 8px; padding: 10px 18px; border-radius: 10px; font-size: 13.5px;
  background: var(--card, #fff); color: var(--text, #1f2d3d); border: 1px solid var(--border, #e4e7ed);
  box-shadow: 0 6px 24px rgba(15, 35, 60, .16); max-width: 70vw; }
.dh-toast svg { width: 16px; height: 16px; flex-shrink: 0; }
.dh-toast.dh-success svg { color: #67c23a; }
.dh-toast.dh-error svg { color: #f56c6c; }
.dh-toast.dh-info svg { color: #409eff; }
.dh-toast-enter-active, .dh-toast-leave-active { transition: all .28s ease; }
.dh-toast-enter-from, .dh-toast-leave-to { opacity: 0; transform: translateY(-14px); }
/* ---- 抽屉式 confirm/prompt ---- */
.dh-mask { position: fixed; inset: 0; background: rgba(10, 18, 32, .45); z-index: 3100; backdrop-filter: blur(1.5px); }
.dh-drawer { position: absolute; top: 0; right: 0; bottom: 0; width: 400px; max-width: 94vw; display: flex; flex-direction: column;
  background: var(--bg, #f5f7fa); border-left: 1px solid var(--border, #e4e7ed); box-shadow: -12px 0 40px rgba(10, 25, 45, .22); }
.dh-head { display: flex; align-items: center; gap: 10px; padding: 16px 18px 12px; border-bottom: 1px solid var(--border, #e4e7ed); }
.dh-head h3 { margin: 0; font-size: 15.5px; flex: 1; color: var(--text, #1f2d3d); }
.dh-tone { display: flex; }
.dh-tone svg { width: 18px; height: 18px; }
.dh-tone-danger svg { color: #f56c6c; }
.dh-tone-warn svg { color: #e6a23c; }
.dh-tone-info svg { color: #409eff; }
.dh-x { background: none; border: none; cursor: pointer; color: var(--muted, #8a93a6); display: flex; padding: 4px; border-radius: 6px; }
.dh-x:hover { background: rgba(128, 148, 178, .12); color: var(--text, #1f2d3d); }
.dh-x svg { width: 16px; height: 16px; }
.dh-body { flex: 1; padding: 18px; overflow-y: auto; }
.dh-msg { margin: 0 0 12px; font-size: 13.5px; line-height: 1.75; color: var(--text, #1f2d3d); white-space: pre-wrap; word-break: break-word; }
.dh-pwd .input, .dh-input { width: 100%; }
.dh-label { display: block; font-size: 12.5px; color: var(--muted, #8a93a6); margin-bottom: 8px; }
.dh-foot { display: flex; justify-content: flex-end; gap: 10px; padding: 12px 18px; border-top: 1px solid var(--border, #e4e7ed); background: var(--card, #fff); }
.dh-fade-enter-active, .dh-fade-leave-active { transition: opacity .22s ease; }
.dh-fade-enter-from, .dh-fade-leave-to { opacity: 0; }
.dh-slide-enter-active, .dh-slide-leave-active { transition: transform .26s cubic-bezier(.22, .8, .36, 1); }
.dh-slide-enter-from, .dh-slide-leave-to { transform: translateX(100%); }
.btn-danger { background: #f56c6c; border-color: #f56c6c; color: #fff; }
.btn-danger:hover { background: #e05252; }
@media (max-width: 640px) { .dh-drawer { width: 100%; } }
</style>
