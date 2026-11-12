<template>
  <div class="c3d-wrap">
    <div class="c3d-toolbar">
      <div class="c3d-modes">
        <button v-for="m in MODES" :key="m.k" class="btn btn-mini" :class="{ 'btn-primary': mode === m.k }" @click="setMode(m.k)">{{ m.label }}</button>
      </div>
      <span style="flex:1"></span>
      <button class="btn btn-mini" @click="toggleDoor">{{ doorOpen ? '关闭柜门' : '打开柜门' }}</button>
      <button class="btn btn-mini" :class="{ 'btn-primary': touring }" @click="toggleTour">
        <SvgIcon :name="touring ? 'stop' : 'play'" />{{ touring ? '停止巡检' : '自动巡检' }}
      </button>
      <button class="btn btn-mini" @click="resetView"><SvgIcon name="refresh" />复位视角</button>
    </div>
    <div class="c3d-stage" ref="stage">
      <canvas ref="cv"></canvas>
      <div v-if="!assets.length" class="c3d-empty">机柜为空 —— 3D 视图展示柜体与 U 位槽，上架设备后自动渲染</div>
      <!-- 拉出设备信息卡 -->
      <div v-if="selected" class="c3d-card">
        <div class="c3c-head">
          <span class="c3c-cat" :style="{ background: catCss(selected.category) }">{{ CATN[selected.category] || '其他' }}</span>
          <b>{{ selected.name }}</b>
          <button class="c3c-x" @click="deselect"><SvgIcon name="x" /></button>
        </div>
        <div class="c3c-row"><span class="muted">编码</span><span class="mono">{{ selected.code }}</span></div>
        <div class="c3c-row"><span class="muted">U 位</span><span>{{ selected.start_u }}~{{ selected.start_u + Math.max(selected.height_u || 1, 1) - 1 }}U（{{ selected.height_u || 1 }}U 高）</span></div>
        <div class="c3c-row"><span class="muted">功率</span><span>{{ selected.power_w || 0 }} W</span></div>
        <div class="c3c-row"><span class="muted">重量</span><span>{{ selected.weight_kg || 0 }} kg</span></div>
        <div class="c3c-row" v-if="health"><span class="muted">健康分</span>
          <span class="badge hg" :class="'hg-' + health.grade">{{ health.score }} · {{ HGN[health.grade] || health.grade }}</span>
        </div>
        <button class="btn btn-mini btn-primary c3c-go" @click="$emit('go-asset', selected)">资产详情 ›</button>
      </div>
      <div class="c3d-hint muted">拖拽旋转 · 滚轮缩放 · 点击设备拉出检查 · 双击空白复位</div>
    </div>
  </div>
</template>
<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'

const props = defineProps({
  cabinet: { type: Object, required: true },
  assets: { type: Array, default: () => [] },
  healthMap: { type: Object, default: () => ({}) },
})
defineEmits(['go-asset'])

const CAT_COLOR = { server: 0x409eff, network: 0x00b96b, storage: 0x9d7bff, power: 0xe6a23c, cooling: 0x1e88e5, security: 0xeb2f96, other: 0x8a93a6 }
const CATN = { server: '服务器', network: '网络', storage: '存储', power: '动力', cooling: '制冷', security: '安防', other: '其他' }
const HGN = { healthy: '健康', attention: '关注', sub_healthy: '亚健康', risk: '风险' }
const MODES = [{ k: 'category', label: '分类着色' }, { k: 'health', label: '健康着色' }, { k: 'power', label: '功率着色' }]

// 尺寸：真实比例 600mm 宽 × 1000mm 深，高度按 total_u 推算（1U=44.45mm）
const CAB_W = 0.6, CAB_D = 1.0, U_H = 0.0445
const totalU = computed(() => Math.max(props.cabinet?.total_u || 42, 1))
const CAB_H = computed(() => totalU.value * U_H + 0.14)
const PULL_OUT = 0.42

const stage = ref(null), cv = ref(null)
const mode = ref('category'), doorOpen = ref(false), touring = ref(false)
const selected = ref(null)
const health = computed(() => selected.value ? props.healthMap[selected.value.id] : null)

let renderer, scene, camera, controls, raycaster, pointer, raf = 0
let cabGroup = null, doorPivot = null, devGroups = [], pickables = []
let pulledIdx = -1, tourIdx = -1, tourTimer = null
let focusTarget = new THREE.Vector3(0, CAB_H.value / 2, 0)

function catCss(c) { return '#' + (CAT_COLOR[c] ?? CAT_COLOR.other).toString(16).padStart(6, '0') }

function devColor(a) {
  if (mode.value === 'health') {
    const h = props.healthMap[a.id]
    if (!h) return 0x5a6472
    return h.grade === 'healthy' ? 0x2fbf71 : h.grade === 'attention' ? 0xd9b417 : h.grade === 'sub_healthy' ? 0xe6842e : 0xd94141
  }
  if (mode.value === 'power') {
    const w = a.power_w || 0
    return w >= 800 ? 0xd94141 : w >= 400 ? 0xe6842e : w >= 150 ? 0xd9b417 : 0x2fbf71
  }
  return CAT_COLOR[a.category] ?? CAT_COLOR.other
}

function parseDepthMM(a) {
  // dimensions 形如 "482×600×88"（宽×深×高 mm），取第二段
  const s = String(a.dimensions || '')
  const m = s.match(/(\d+(?:\.\d+)?)\s*[×xX*]\s*(\d+(?:\.\d+)?)/)
  if (m) {
    const d = parseFloat(m[2])
    if (d >= 100 && d <= 1200) return d / 1000
  }
  return CAB_D * 0.68
}

function makeLabel(text) {
  const c = document.createElement('canvas')
  c.width = 320; c.height = 80
  const x = c.getContext('2d')
  x.fillStyle = 'rgba(10,22,40,.88)'
  x.beginPath(); x.roundRect(10, 12, 300, 56, 12); x.fill()
  x.strokeStyle = 'rgba(64,158,255,.55)'; x.lineWidth = 2
  x.beginPath(); x.roundRect(10, 12, 300, 56, 12); x.stroke()
  x.fillStyle = '#dce8f5'; x.font = 'bold 30px sans-serif'
  x.textAlign = 'center'; x.textBaseline = 'middle'
  x.fillText(text, 160, 41)
  const tex = new THREE.CanvasTexture(c)
  const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, depthTest: false, transparent: true }))
  sp.scale.set(1.1, 0.28, 1)
  return sp
}

function buildScene() {
  // 清理旧场景
  if (cabGroup) { scene.remove(cabGroup); cabGroup.traverse(o => { o.geometry?.dispose?.(); if (o.material) { (Array.isArray(o.material) ? o.material : [o.material]).forEach(m => m.dispose?.()) } }) }
  devGroups = []; pickables = []
  cabGroup = new THREE.Group()
  const H = CAB_H.value

  const frameMat = new THREE.MeshStandardMaterial({ color: 0x2b3038, roughness: 0.34, metalness: 0.55 })
  // 后板/侧板/顶底
  const back = new THREE.Mesh(new THREE.BoxGeometry(CAB_W, H, 0.03), frameMat)
  back.position.set(0, H / 2, -CAB_D / 2); back.castShadow = true; cabGroup.add(back)
  const sideGeo = new THREE.BoxGeometry(0.03, H, CAB_D)
  const sideL = new THREE.Mesh(sideGeo, frameMat); sideL.position.set(-CAB_W / 2, H / 2, 0); sideL.castShadow = true; cabGroup.add(sideL)
  const sideR = sideL.clone(); sideR.position.x = CAB_W / 2; cabGroup.add(sideR)
  const topB = new THREE.Mesh(new THREE.BoxGeometry(CAB_W, 0.04, CAB_D), frameMat)
  topB.position.set(0, H, 0); cabGroup.add(topB)
  const botB = topB.clone(); botB.position.y = 0.04; cabGroup.add(botB)
  const base = new THREE.Mesh(new THREE.BoxGeometry(CAB_W * 1.04, 0.06, CAB_D * 1.04),
    new THREE.MeshStandardMaterial({ color: 0x15181c, roughness: 0.6, metalness: 0.5 }))
  base.position.y = 0.03; cabGroup.add(base)

  // 四立柱（内衬，导轨感）
  const railMat = new THREE.MeshStandardMaterial({ color: 0x3a414c, roughness: 0.4, metalness: 0.6 })
  for (const sx of [-1, 1]) for (const sz of [-1, 1]) {
    const rail = new THREE.Mesh(new THREE.BoxGeometry(0.025, H * 0.94, 0.025), railMat)
    rail.position.set(sx * (CAB_W / 2 - 0.035), H / 2, sz * (CAB_D / 2 - 0.06))
    cabGroup.add(rail)
  }

  // U 刻度（每 5U 右侧立柱小标记 + 空 U 槽条）
  const slotMat = new THREE.MeshBasicMaterial({ color: 0x24334d, transparent: true, opacity: 0.7 })
  for (let u = 1; u <= totalU.value; u++) {
    const yy = 0.08 + (u - 0.5) * U_H
    if (u % 5 === 0) {
      const tick = new THREE.Mesh(new THREE.BoxGeometry(0.035, 0.004, 0.004), new THREE.MeshBasicMaterial({ color: 0x5d7089 }))
      tick.position.set(CAB_W / 2 - 0.02, yy, CAB_D / 2 - 0.03); cabGroup.add(tick)
    }
    const slot = new THREE.Mesh(new THREE.BoxGeometry(CAB_W * 0.9, 0.003, 0.008), slotMat)
    slot.position.set(0, yy - U_H / 2 + 0.002, CAB_D / 2 - 0.045); cabGroup.add(slot)
  }

  // 设备（按 U 位，每台一个 group 便于整体拉出）
  props.assets.forEach((a, idx) => {
    const hu = Math.max(a.height_u || 1, 1)
    const h = hu * U_H * 0.88
    const dep = parseDepthMM(a)
    const g = new THREE.Group()
    const yBase = 0.08 + (Math.max(a.start_u || 1, 1) - 1) * U_H + h / 2
    g.position.set(0, yBase, 0)
    g.userData = { idx, asset: a, baseZ: 0, targetZ: 0 }
    const body = new THREE.Mesh(new THREE.BoxGeometry(CAB_W * 0.86, h, dep),
      new THREE.MeshStandardMaterial({ color: devColor(a), roughness: 0.42, metalness: 0.38, transparent: true, opacity: 1 }))
    body.castShadow = true
    body.userData = { kind: 'asset', idx }
    g.add(body); pickables.push(body)
    // 前面板 + LED + 通风槽
    const face = new THREE.Mesh(new THREE.BoxGeometry(CAB_W * 0.82, h * 0.9, 0.012),
      new THREE.MeshStandardMaterial({ color: 0x1a1e24, roughness: 0.45, metalness: 0.35, transparent: true, opacity: 1 }))
    face.position.z = dep / 2 + 0.004
    face.userData = { kind: 'asset', idx }
    g.add(face); pickables.push(face)
    const seed = ((a.id || idx) * 2654435761) >>> 0
    const nLed = 2 + (seed % 3)
    const ledGeo = new THREE.SphereGeometry(0.008, 6, 6)
    for (let li = 0; li < nLed; li++) {
      const lm = new THREE.Mesh(ledGeo, new THREE.MeshBasicMaterial({ color: li === nLed - 1 ? 0xffb300 : 0x00e676 }))
      lm.position.set(CAB_W * 0.2 + li * 0.05, h * 0.26, dep / 2 + 0.012)
      g.add(lm)
    }
    const slot2 = new THREE.Mesh(new THREE.PlaneGeometry(CAB_W * 0.44, 0.007), new THREE.MeshBasicMaterial({ color: 0x0a0d11 }))
    slot2.position.set(-CAB_W * 0.1, -h * 0.2, dep / 2 + 0.008)
    g.add(slot2)
    cabGroup.add(g)
    devGroups.push(g)
  })

  // 前门（pivot 在左边缘，开合动画绕 Y 轴）
  doorPivot = new THREE.Group()
  doorPivot.position.set(-CAB_W / 2, 0, CAB_D / 2)
  const perfCv = document.createElement('canvas'); perfCv.width = perfCv.height = 128
  const px = perfCv.getContext('2d')
  px.fillStyle = '#39424c'; px.fillRect(0, 0, 128, 128)
  px.fillStyle = '#232a32'
  for (let r = 0; r < 16; r++) for (let c2 = 0; c2 < 16; c2++) { px.beginPath(); px.arc(c2 * 8 + 4, r * 8 + 4, 2.1, 0, 7); px.fill() }
  const perfTex = new THREE.CanvasTexture(perfCv)
  const doorMat = new THREE.MeshPhysicalMaterial({ map: perfTex, color: 0x59636e, transparent: true, opacity: 0.4, roughness: 0.3, metalness: 0.45, side: THREE.DoubleSide, depthWrite: false })
  const door = new THREE.Mesh(new THREE.BoxGeometry(CAB_W * 0.97, H * 0.96, 0.015), doorMat)
  door.position.set(CAB_W / 2 - 0.01, H / 2, 0)
  door.userData = { kind: 'door' }
  doorPivot.add(door); pickables.push(door)
  const handleMat = new THREE.MeshStandardMaterial({ color: 0x9aa4ad, roughness: 0.3, metalness: 0.85 })
  const handle = new THREE.Mesh(new THREE.BoxGeometry(0.022, H * 0.3, 0.028), handleMat)
  handle.position.set(CAB_W * 0.9, H * 0.52, 0.02); doorPivot.add(handle)
  cabGroup.add(doorPivot)

  // 顶部铭牌 label
  const lbl = makeLabel(props.cabinet?.code || '机柜')
  lbl.position.set(0, H + 0.22, 0)
  cabGroup.add(lbl)

  scene.add(cabGroup)
  // 测试钩子
  window.__cabinet3d = {
    get devices() { return devGroups.length },
    get pulled() { return pulledIdx },
    get doorOpen() { return doorOpen.value },
    get mode() { return mode.value },
    get touring() { return touring.value },
    totalU: totalU.value,
    focusDevice: (i) => { stopTour(); pullOut(i) },
    cabCode: props.cabinet?.code || '',
    deviceScreenPos: (i) => {
      const g = devGroups[i]
      if (!g || !renderer) return null
      const v = new THREE.Vector3()
      g.getWorldPosition(v)
      v.z += 0.16
      v.project(camera)
      const rect = renderer.domElement.getBoundingClientRect()
      return { x: rect.left + (v.x + 1) / 2 * rect.width, y: rect.top + (1 - v.y) / 2 * rect.height }
    },
  }
}

function applyColors() {
  devGroups.forEach(g => {
    const body = g.children[0]
    if (body?.material) body.material.color.setHex(devColor(g.userData.asset))
  })
}

function setMode(m) { mode.value = m; applyColors() }

function pullOut(idx, focus = true) {
  devGroups.forEach((g, i) => {
    g.userData.targetZ = i === idx ? PULL_OUT : 0
    g.children.forEach(ch => {
      if (ch.isMesh && ch.material && 'opacity' in ch.material) {
        ch.material.opacity = (idx < 0 || i === idx) ? 1 : 0.22
      }
    })
  })
  pulledIdx = idx
  selected.value = idx >= 0 ? props.assets[idx] : null
  if (focus && idx >= 0) {
    const g = devGroups[idx]
    focusTarget.set(0, g.position.y, 0.2)
  } else {
    focusTarget.set(0, CAB_H.value / 2, 0)
  }
}

function deselect() { stopTour(); pullOut(-1, false) }

function toggleDoor() { doorOpen.value = !doorOpen.value }

function resetView() {
  camera.position.set(0.9, CAB_H.value * 0.62, 2.4)
  focusTarget.set(0, CAB_H.value / 2, 0)
}

// 自动巡检：从低 U 到高 U 逐台拉出
function toggleTour() { touring.value ? stopTour() : startTour() }
function startTour() {
  if (!devGroups.length) return
  touring.value = true
  tourIdx = -1
  tourStep()
}
function tourStep() {
  if (!touring.value) return
  tourIdx = (tourIdx + 1) % devGroups.length
  pullOut(tourIdx)
  tourTimer = setTimeout(tourStep, 2600)
}
function stopTour() {
  touring.value = false
  if (tourTimer) { clearTimeout(tourTimer); tourTimer = null }
}

function onClick(e) {
  if (!renderer) return
  const rect = renderer.domElement.getBoundingClientRect()
  pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1
  pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1
  raycaster.setFromCamera(pointer, camera)
  const hits = raycaster.intersectObjects(pickables, false)
  if (hits.length) {
    const ud = hits[0].object.userData
    if (ud.kind === 'asset') { stopTour(); pullOut(ud.idx); return }
    if (ud.kind === 'door') { toggleDoor(); return }
  }
  deselect()
}
function onDblClick(e) {
  const rect = renderer.domElement.getBoundingClientRect()
  pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1
  pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1
  raycaster.setFromCamera(pointer, camera)
  if (!raycaster.intersectObjects(pickables, false).length) resetView()
}

let ro = null
function animate() {
  raf = requestAnimationFrame(animate)
  // 设备拉出动画（lerp）
  devGroups.forEach(g => {
    const t = g.userData.targetZ
    if (Math.abs(g.position.z - t) > 0.001) g.position.z += (t - g.position.z) * 0.14
  })
  // 门开合动画
  if (doorPivot) {
    const target = doorOpen.value ? -1.92 : 0
    if (Math.abs(doorPivot.rotation.y - target) > 0.005) doorPivot.rotation.y += (target - doorPivot.rotation.y) * 0.1
  }
  // 相机焦点跟随
  controls.target.lerp(focusTarget, 0.08)
  controls.update()
  renderer.render(scene, camera)
}

function init() {
  const el = stage.value
  renderer = new THREE.WebGLRenderer({ canvas: cv.value, antialias: true, preserveDrawingBuffer: true })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.shadowMap.enabled = true
  renderer.shadowMap.type = THREE.PCFSoftShadowMap
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.05
  scene = new THREE.Scene()
  scene.background = new THREE.Color(0x0b1220)
  scene.fog = new THREE.Fog(0x0b1220, 6, 14)
  camera = new THREE.PerspectiveCamera(46, el.clientWidth / el.clientHeight, 0.05, 60)
  camera.position.set(0.9, CAB_H.value * 0.62, 2.4)
  controls = new OrbitControls(camera, renderer.domElement)
  controls.enableDamping = true
  controls.minDistance = 0.8; controls.maxDistance = 7
  controls.maxPolarAngle = Math.PI * 0.52
  controls.target.set(0, CAB_H.value / 2, 0)
  raycaster = new THREE.Raycaster(); pointer = new THREE.Vector2()

  // 灯光（深色环境，与 Room3D 一致不随主题）
  scene.add(new THREE.AmbientLight(0x8fa3bd, 0.5))
  const key = new THREE.DirectionalLight(0xdfe9f5, 1.6)
  key.position.set(2.4, 3.6, 2.2); key.castShadow = true
  key.shadow.mapSize.set(1024, 1024); scene.add(key)
  const fill = new THREE.DirectionalLight(0x5f7ea6, 0.6)
  fill.position.set(-2.2, 1.6, -1.8); scene.add(fill)
  const rim = new THREE.PointLight(0x4c7dff, 0.9, 8)
  rim.position.set(0, CAB_H.value + 0.8, -1.4); scene.add(rim)

  // 地面 + 网格
  const floor = new THREE.Mesh(new THREE.CircleGeometry(3.4, 48),
    new THREE.MeshStandardMaterial({ color: 0x111a2b, roughness: 0.85, metalness: 0.15 }))
  floor.rotation.x = -Math.PI / 2; floor.receiveShadow = true; scene.add(floor)
  const grid = new THREE.GridHelper(6.8, 34, 0x27405f, 0x1b2c44)
  grid.position.y = 0.002; scene.add(grid)

  buildScene()
  renderer.domElement.addEventListener('click', onClick)
  renderer.domElement.addEventListener('dblclick', onDblClick)
  ro = new ResizeObserver(() => {
    if (!stage.value) return
    const w = stage.value.clientWidth, hgt = stage.value.clientHeight
    renderer.setSize(w, hgt, false)
    camera.aspect = w / hgt; camera.updateProjectionMatrix()
  })
  ro.observe(el)
  renderer.setSize(el.clientWidth, el.clientHeight, false)
  animate()
}

watch(() => [props.cabinet?.id, props.assets.length, totalU.value], () => { if (scene) { stopTour(); selected.value = null; pulledIdx = -1; buildScene() } })
watch(() => props.healthMap, () => { if (mode.value === 'health') applyColors() }, { deep: true })

onMounted(() => nextTick(init))
onBeforeUnmount(() => {
  stopTour()
  cancelAnimationFrame(raf)
  ro?.disconnect()
  renderer?.domElement.removeEventListener('click', onClick)
  renderer?.domElement.removeEventListener('dblclick', onDblClick)
  renderer?.dispose()
  delete window.__cabinet3d
})
</script>
<style scoped>
.c3d-wrap { border: 1px solid var(--border); border-radius: 10px; overflow: hidden; background: #0b1220; }
.c3d-toolbar { display: flex; align-items: center; gap: 8px; padding: 10px 14px; background: var(--card); border-bottom: 1px solid var(--border); flex-wrap: wrap; }
.c3d-modes { display: flex; gap: 6px; }
.c3d-stage { position: relative; height: 540px; }
.c3d-stage canvas { width: 100%; height: 100%; display: block; }
.c3d-empty { position: absolute; top: 14px; left: 50%; transform: translateX(-50%); background: rgba(10, 22, 40, .8); border: 1px solid rgba(64, 158, 255, .35); color: #dce8f5; padding: 8px 16px; border-radius: 8px; font-size: 13px; pointer-events: none; }
.c3d-card { position: absolute; top: 14px; right: 14px; width: 250px; background: rgba(13, 24, 42, .94); border: 1px solid rgba(64, 158, 255, .4); border-radius: 10px; padding: 12px 14px; color: #dce8f5; box-shadow: 0 6px 24px rgba(0, 0, 0, .4); }
.c3c-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.c3c-head b { font-size: 14px; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.c3c-cat { font-size: 11px; padding: 2px 8px; border-radius: 6px; color: #0b1220; font-weight: 700; flex-shrink: 0; }
.c3c-x { background: none; border: none; color: #8fa3bd; cursor: pointer; padding: 2px; display: flex; }
.c3c-row { display: flex; justify-content: space-between; font-size: 12.5px; padding: 3px 0; color: #dce8f5; }
.c3c-row .muted { color: #8fa3bd; }
.c3c-go { margin-top: 10px; width: 100%; }
.badge.hg { font-weight: 700; }
.badge.hg-healthy { color: #67c23a; border-color: rgba(103, 194, 58, .45); }
.badge.hg-attention { color: #d9b417; border-color: rgba(217, 180, 23, .45); }
.badge.hg-sub_healthy { color: #e6842e; border-color: rgba(230, 132, 46, .45); }
.badge.hg-risk { color: #f56c6c; border-color: rgba(245, 108, 108, .45); }
.c3d-hint { position: absolute; bottom: 10px; left: 50%; transform: translateX(-50%); font-size: 12px; color: #6d8099; background: rgba(10, 18, 32, .7); padding: 4px 12px; border-radius: 6px; pointer-events: none; }
@media (max-width: 900px) { .c3d-stage { height: 420px; } .c3d-card { width: 200px; } }
</style>
