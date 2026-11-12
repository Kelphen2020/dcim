import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    target: 'chrome86',
    cssTarget: 'chrome86',
    chunkSizeWarningLimit: 1500
  },
  server: {
    port: 3200,
    proxy: {
      '/api': { target: 'http://localhost:8095', changeOrigin: true }
    }
  }
})
