import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const backend = process.env.CANVASBIN_BACKEND || 'http://localhost:8090'
const proxied = ['/api', '/v/', '/raw/', '/mcp', '/healthz']

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: Object.fromEntries(proxied.map((p) => [p, { target: backend, changeOrigin: false }])),
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1200,
  },
})
