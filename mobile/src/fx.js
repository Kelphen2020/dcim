// 颜色与特效工具（M5 液态玻璃 UI 升级）

// ASCII base64 编码（mp-weixin 无 btoa）
const B64C = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
export function b64(str) {
  let out = '', i = 0
  while (i < str.length) {
    const c1 = str.charCodeAt(i++), c2 = str.charCodeAt(i++), c3 = str.charCodeAt(i++)
    const e1 = c1 >> 2
    const e2 = ((c1 & 3) << 4) | (isNaN(c2) ? 0 : c2 >> 4)
    const e3 = isNaN(c2) ? 64 : ((c2 & 15) << 2) | (isNaN(c3) ? 0 : c3 >> 6)
    const e4 = isNaN(c3) ? 64 : c3 & 63
    out += B64C.charAt(e1) + B64C.charAt(e2) + (e3 === 64 ? '=' : B64C.charAt(e3)) + (e4 === 64 ? '=' : B64C.charAt(e4))
  }
  return out
}

export function hexToRgb(hex) {
  const h = (hex || '#1f4e9c').replace('#', '')
  return {
    r: parseInt(h.slice(0, 2), 16) || 0,
    g: parseInt(h.slice(2, 4), 16) || 0,
    b: parseInt(h.slice(4, 6), 16) || 0,
  }
}

// 相对亮度 0~1
export function luminance(hex) {
  const { r, g, b } = hexToRgb(hex)
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255
}

// 两色插值 t∈[0,1]
export function mixColors(a, b, t) {
  const A = hexToRgb(a), B = hexToRgb(b)
  const m = (x, y) => Math.round(x + (y - x) * t)
  return `rgb(${m(A.r, B.r)},${m(A.g, B.g)},${m(A.b, B.b)})`
}

export function rgba(hex, alpha) {
  const { r, g, b } = hexToRgb(hex)
  return `rgba(${r},${g},${b},${alpha})`
}

// 由字符串（站点名等）稳定映射色相
export function hashHue(str, base = 210) {
  let h = 0
  const s = String(str || '')
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 360
  return (base + h) % 360
}

export function hslHex(h, s, l) {
  s /= 100; l /= 100
  const k = (n) => (n + h / 30) % 12
  const a = s * Math.min(l, 1 - l)
  const f = (n) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)))
  const to2 = (x) => Math.round(255 * x).toString(16).padStart(2, '0')
  return `#${to2(f(0))}${to2(f(8))}${to2(f(4))}`
}

// 圆柱卷收列表：按条目中心与视口中心距离计算 3D 形变样式
// top: 条目相对页面顶部偏移; itemH: 条目高; scrollY: 页面滚动; winH: 视口高
export function cylStyle(top, itemH, scrollY, winH) {
  const center = top - scrollY + itemH / 2
  let t = (center - winH / 2) / (winH / 2)
  t = Math.max(-1.3, Math.min(1.3, t))
  const abs = Math.abs(t)
  const rot = t * -16 // 边缘向里卷（柔和版，保证边缘卡片仍可读）
  const scale = 1 - abs * 0.08
  const opacity = Math.max(0.45, 1 - abs * 0.5)
  const bright = 1 - abs * 0.2
  return `transform:perspective(900px) rotateX(${rot.toFixed(2)}deg) scale(${scale.toFixed(3)});opacity:${opacity.toFixed(3)};filter:brightness(${bright.toFixed(3)});`
}

// 景深分层：系数越小层越"远"（移动越慢）
export function depthStyle(scrollY, factor, extraBlur = 0) {
  const y = (scrollY * (1 - factor)).toFixed(1)
  let s = `transform:translate3d(0,${-y}px,0);`
  if (extraBlur > 0) s += `filter:blur(${extraBlur}px);`
  return s
}
