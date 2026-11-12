import { build } from 'vite'
try {
  await build({ logLevel: 'error' })
  console.log('VITE_BUILD_OK')
} catch (e) {
  console.error('VITE_BUILD_FAIL', e.message)
  process.exit(1)
}
