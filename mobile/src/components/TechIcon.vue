<template>
  <image class="tech-icon" :style="st" :src="src" mode="aspectFit" />
</template>

<script>
// 科技感线性图标库（stroke 风格 SVG，替代 emoji）
import { b64 } from '../fx.js'

// 图标定义：viewBox 24，stroke 线性
const PATHS = {
  // 导航/通用
  grid: 'M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z',
  bell: 'M18 9a6 6 0 1 0-12 0c0 6-2 7-2 7h16s-2-1-2-7M13.7 20a2 2 0 0 1-3.4 0',
  alert: 'M12 3 2 20h20L12 3zM12 10v4M12 17.5v.5',
  ticket: 'M4 8a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v2a2 2 0 0 0 0 4v2a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-2a2 2 0 0 0 0-4V8zM13 6v12',
  scan: 'M4 8V5.5A1.5 1.5 0 0 1 5.5 4H8M16 4h2.5A1.5 1.5 0 0 1 20 5.5V8M20 16v2.5a1.5 1.5 0 0 1-1.5 1.5H16M8 20H5.5A1.5 1.5 0 0 1 4 18.5V16M4 12h16',
  user: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 20c0-4 3.6-6 8-6s8 2 8 6',
  clock: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7v5l3.5 2',
  box: 'M12 3 3 7.5v9L12 21l9-4.5v-9L12 3zM3 7.5 12 12l9-4.5M12 12v9',
  counter: 'M4 19h16M6 19V9M11 19V5M16 19v-7M20 19v-4',
  chart: 'M4 20V10M9.3 20V4M14.7 20v-8M20 20V7',
  shield: 'M12 3 5 6v5c0 4.4 3 8.5 7 10 4-1.5 7-5.6 7-10V6l-7-3zM9.2 12.2 11.3 14.3 15 10.6',
  doc: 'M6 3h8l4 4v14H6V3zM14 3v4h4M9 12h6M9 16h6',
  bolt: 'M13 2 4 14h6l-1 8 9-12h-6l1-8z',
  temp: 'M10 13.6V5a2 2 0 1 1 4 0v8.6a4.5 4.5 0 1 1-4 0z',
  eye: 'M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
  'eye-off': 'M4 4l16 16M9.9 5.9A9.9 9.9 0 0 1 12 5.5c6 0 9.5 6.5 9.5 6.5a17 17 0 0 1-3.3 4.1M6.4 7.9A16.8 16.8 0 0 0 2.5 12S6 18.5 12 18.5c1.4 0 2.6-.3 3.7-.8M9.9 9.9a3 3 0 0 0 4.2 4.2',
  drop: 'M12 3s6 6.4 6 10.5a6 6 0 1 1-12 0C6 9.4 12 3 12 3z',
  server: 'M4 5h16v5H4zM4 14h16v5H4zM7.5 7.5h.5M7.5 16.5h.5',
  globe: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM3.5 9h17M3.5 15h17M12 3c2.5 2.6 3.8 5.7 3.8 9S14.5 18.4 12 21c-2.5-2.6-3.8-5.7-3.8-9S9.5 5.6 12 3z',
  check: 'M4.5 12.5 9.5 17.5 19.5 6.5',
  'check-circle': 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM8 12.5l2.8 2.8L16.5 9.5',
  x: 'M6 6l12 12M18 6 6 18',
  pin: 'M12 21s7-6.2 7-11a7 7 0 1 0-14 0c0 4.8 7 11 7 11zM12 12.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5z',
  wrench: 'M14.5 6a3.5 3.5 0 0 1 4.9 3.2L21 10.7l-2.8 2.8-1.5-1.5A3.5 3.5 0 0 1 12 8.4L4.6 15.8a2 2 0 1 0 2.8 2.8L14.8 11',
  search: 'M10.5 18a7.5 7.5 0 1 0 0-15 7.5 7.5 0 0 0 0 15zM16 16l4.5 4.5',
  cal: 'M4 6h16v15H4V6zM4 10h16M8 3v4M16 3v4',
  layers: 'M12 3 3 8l9 5 9-5-9-5zM3 13l9 5 9-5M3 17l9 5 9-5',
  swap: 'M7 4 4 7.5 7 11M4 7.5h11a4 4 0 0 1 0 8H9M17 20l3-3.5-3-3.5',
  lock: 'M6 11h12v9H6v-9zM9 11V7.5a3 3 0 0 1 6 0V11',
  pen: 'M4 20h4L20 8l-4-4L4 16v4zM14.5 5.5l4 4',
  inbox: 'M4 13h4l1.5 3h5L16 13h4M4 13 6.5 5h11L20 13v6H4v-6z',
  gauge: 'M12 21a9 9 0 1 1 9-9M12 12l4.5-4.5M21 12h-3M18.4 5.6l-2.1 2.1',
  cpu: 'M7 7h10v10H7V7zM4 10h3M4 14h3M17 10h3M17 14h3M10 4v3M14 4v3M10 17v3M14 17v3',
  'arrow-up': 'M12 19V5M5.5 11.5 12 5l6.5 6.5',
  'arrow-down': 'M12 5v14M5.5 12.5 12 19l6.5-6.5',
  camera: 'M4 8h3l1.5-2h7L17 8h3a1 1 0 0 1 1 1v9a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V9a1 1 0 0 1 1-1zM12 16.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z',
}

const cache = {}

function makeSrc(name, color) {
  const key = name + '|' + color
  if (cache[key]) return cache[key]
  const d = PATHS[name] || PATHS.grid
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="${color}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="${d}"/></svg>`
  const src = 'data:image/svg+xml;base64,' + b64(svg)
  cache[key] = src
  return src
}

export default {
  name: 'TechIcon',
  props: {
    name: { type: String, default: 'grid' },
    color: { type: String, default: '#1F4E9C' },
    size: { type: [Number, String], default: 36 }, // rpx
  },
  computed: {
    src() { return makeSrc(this.name, this.color) },
    st() {
      const s = typeof this.size === 'number' ? this.size + 'rpx' : this.size
      return `width:${s};height:${s};`
    },
  },
}
</script>

<style scoped>
.tech-icon { display: inline-block; vertical-align: middle; flex-shrink: 0; }
</style>
