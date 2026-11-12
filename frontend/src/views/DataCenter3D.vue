<template>
  <div>
    <div class="page-head"><h1>3D 机房视图</h1></div>
    <div class="toolbar">
      <select class="input" style="width:180px" v-model.number="siteId" @change="load">
        <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
      </select>
      <span style="flex:1"></span>
      <span class="muted" style="font-size:13px">机柜 {{ cabinets.length }} · 旋转拖拽查看</span>
    </div>
    <div class="card" style="overflow:hidden;padding:0">
      <div ref="container" style="width:100%;height:600px;background:#1a1a2e;position:relative">
        <canvas ref="canvas" style="display:block;width:100%;height:100%"></canvas>
        <div v-if="!cabinets.length" class="empty" style="position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);color:#aaa">暂无该站点机柜数据</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import api from '../api'

const sites=ref([]),siteId=ref(0),cabinets=ref([])
const container=ref(null),canvas=ref(null)
let ctx=null,angleY=0,angleX=0.5,dragging=false,lastX=0,lastY=0,scale=1,animId=null

const COLORS={0:'#555',1:'#2a5a2a',2:'#5a5a2a',3:'#5a2a2a'}
const rows=4,cols=5,spacing=100,floorY=300

function project(x,y,z){
  const cx=canvas.value.width/2,cy=canvas.value.height/2
  const cosY=Math.cos(angleY),sinY=Math.sin(angleY),cosX=Math.cos(angleX),sinX=Math.sin(angleX)
  let rx=x*cosY-z*sinY,rz=x*sinY+z*cosY
  let ry=y*cosX-rz*sinX,rz2=y*sinX+rz*cosX
  const d=600/(600+rz2)
  return{x:cx+rx*d,y:cy-ry*d,scale:d}
}

function draw(){
  if(!ctx)return
  const w=canvas.value.width,h=canvas.value.height
  ctx.clearRect(0,0,w,h)
  ctx.fillStyle='#1a1a2e';ctx.fillRect(0,0,w,h)
  // 地面网格
  ctx.strokeStyle='rgba(64,158,255,.1)';ctx.lineWidth=1
  for(let i=-400;i<=400;i+=spacing){
    ctx.beginPath();ctx.moveTo(project(-400,floorY,i).x,project(-400,floorY,i).y);ctx.lineTo(project(400,floorY,i).x,project(400,floorY,i).y);ctx.stroke()
    ctx.beginPath();ctx.moveTo(project(i,floorY,-400).x,project(i,floorY,-400).y);ctx.lineTo(project(i,floorY,400).x,project(i,floorY,400).y);ctx.stroke()
  }
  // 绘制机柜
  const items=cabinets.value.map((c,i)=>{
    const row=Math.floor(i/cols),col=i%cols
    const x=(col-cols/2)*spacing,y=floorY-30,z=(row-rows/2)*spacing
    return{...c,x,y,z}
  })
  items.sort((a,b)=>{const pa=project(a.x,a.y,a.z),pb=project(b.x,b.y,b.z);return pb.scale-pa.scale})
  for(const c of items){
    const p=project(c.x,c.y,c.z)
    const s=p.scale*40
    const usedPct=c.total_u>0?c.used_u/c.total_u:0
    ctx.fillStyle=COLORS[c.status]||'#555';ctx.fillRect(p.x-s,p.y-s*1.5,s*2,s*3)
    ctx.strokeStyle='#888';ctx.lineWidth=1;ctx.strokeRect(p.x-s,p.y-s*1.5,s*2,s*3)
    // 使用率颜色条
    if(usedPct>0){
      const fillH=usedPct*s*3;ctx.fillStyle='rgba(64,158,255,.5)';ctx.fillRect(p.x-s,p.y-s*1.5+s*3-fillH,s*2,fillH)
    }
    // 标签
    ctx.fillStyle='rgba(255,255,255,.8)';ctx.font=`${Math.max(10,s*0.3)}px Arial`;ctx.textAlign='center'
    ctx.fillText(c.name||`#${c.id}`,p.x,p.y-s*1.5-5)
  }
}

function animate(){animId=requestAnimationFrame(animate);draw()}

function onDown(e){e.preventDefault();dragging=true;lastX=e.clientX;lastY=e.clientY}
function onMove(e){if(!dragging)return;angleY+=(e.clientX-lastX)*0.005;angleX=Math.max(-1,Math.min(1,angleX+(e.clientY-lastY)*0.005));lastX=e.clientX;lastY=e.clientY}
function onUp(){dragging=false}
function onWheel(e){e.preventDefault();scale=Math.max(0.5,Math.min(2,scale-e.deltaY*0.001))}

async function load(){
  const params={site_id:siteId.value,size:50}
  const r=await api.get('/cabinets',params)
  cabinets.value=r.data?.items||[]
}

onMounted(async()=>{
  const r=await api.get('/sites',{size:100})
  sites.value=r.data?.items||[];siteId.value=sites.value[0]?.id||0
  await load()
  await nextTick()
  canvas.value.width=canvas.value.offsetWidth;canvas.value.height=canvas.value.offsetHeight
  ctx=canvas.value.getContext('2d')
  const el=canvas.value
  el.addEventListener('mousedown',onDown);el.addEventListener('mousemove',onMove);el.addEventListener('mouseup',onUp);el.addEventListener('mouseleave',onUp);el.addEventListener('wheel',onWheel)
  animate()
})
onUnmounted(()=>{if(animId)cancelAnimationFrame(animId)})
watch(siteId,load)
</script>