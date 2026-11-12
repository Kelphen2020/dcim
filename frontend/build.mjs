import { build } from 'vite'
import { writeFileSync } from 'fs'
// V57.2: 每次构建自动生成版本标识（页面顶栏显示，用户可一眼判断拿到的是哪版）
const ver = process.env.DCIM_BUILD_VER || 'dev'
const ts = new Date().toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false })
writeFileSync(new URL('./src/build-info.js', import.meta.url), `// 构建时自动生成，勿手改
export const BUILD_VERSION = '${ver} · ${ts}'
`)
try {
  await build({ logLevel: 'warn' })
  console.log('BUILD_OK')
} catch (e) {
  console.error('BUILD_FAIL', e.message)
  process.exit(1)
}
