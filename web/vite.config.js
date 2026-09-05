import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发模式代理到本地 Go 后端
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/open': 'http://localhost:8080'
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'static' // 旧构建用 /assets/，改名以彻底失效被污染的浏览器缓存
  }
})
