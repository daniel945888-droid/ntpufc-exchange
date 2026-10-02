import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    allowedHosts: true, 
    // ✨ 新增反向代理規則：讓前端開發伺服器幫忙打 API 給 Go
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true, // 開啟 WebSocket 支援
      }
    }
  }
})