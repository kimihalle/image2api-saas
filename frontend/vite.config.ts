import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig(({ mode }) => {
  const backend = loadEnv(mode, '.', '').VITE_BACKEND || 'http://127.0.0.1:6666'
  return ({
  plugins: [vue()],
  server: {
    port: 5174,
    proxy: {
      '/admin/api': { target: backend, changeOrigin: true },
      '/health': { target: backend, changeOrigin: true },
      '/images': { target: backend, changeOrigin: true },
      '/v1': { target: backend, changeOrigin: true },
    },
  },
  })
})
