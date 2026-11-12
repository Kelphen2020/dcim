// V57 全局轻量交互服务：toast 提示 + 抽屉式 confirm / prompt
// 彻底替代原生 alert/confirm/prompt（浏览器原生弹窗破坏 UI 风格且阻塞）
// 用法：
//   import { toast, showConfirm, showPrompt } from '../ui-dialog'
//   toast('保存成功')  /  toast('操作失败：xx', 'error')
//   if (!(await showConfirm({ message: '确认删除？' }))) return
//   const reason = await showPrompt({ title: '忽略告警', label: '说明（可留空）' })  // null=取消
import { reactive } from 'vue'

export const dialogState = reactive({
  toasts: [],          // {id, msg, type}
  confirm: { show: false, title: '', message: '', tone: 'danger', confirmText: '确认', requirePassword: false, _resolve: null, _pwd: '' },
  prompt: { show: false, title: '', label: '', placeholder: '', defaultValue: '', multiline: false, _resolve: null, _value: '' },
})

let toastSeq = 0
export function toast(msg, type = 'success') {
  const id = ++toastSeq
  dialogState.toasts.push({ id, msg: String(msg ?? ''), type })
  setTimeout(() => {
    const i = dialogState.toasts.findIndex(t => t.id === id)
    if (i >= 0) dialogState.toasts.splice(i, 1)
  }, type === 'error' ? 4200 : 2600)
}

// 智能 toast：按消息内容自动判 success/error（批量替换 alert 时用）
export function toastAuto(msg) {
  const s = String(msg ?? '')
  toast(s, /失败|错误|异常|不存在|无权|已存在|not found|error/i.test(s) ? 'error' : 'success')
}

export function showConfirm(opts = {}) {
  const c = dialogState.confirm
  c.title = opts.title || '操作确认'
  c.message = opts.message || ''
  c.tone = opts.tone || 'danger'
  c.confirmText = opts.confirmText || '确认'
  c.requirePassword = !!opts.requirePassword
  c._pwd = ''
  c.show = true
  return new Promise(resolve => { c._resolve = resolve })
}

export function showPrompt(opts = {}) {
  const p = dialogState.prompt
  p.title = opts.title || '请输入'
  p.label = opts.label || ''
  p.placeholder = opts.placeholder || ''
  p.defaultValue = opts.defaultValue || ''
  p.multiline = !!opts.multiline
  p._value = opts.defaultValue || ''
  p.show = true
  return new Promise(resolve => { p._resolve = resolve })
}
