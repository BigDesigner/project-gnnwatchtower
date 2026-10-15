import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import fs from 'fs'

let appVersion = '0.4.2'
for (const p of [path.resolve(__dirname, '../VERSION'), path.resolve(__dirname, '../VERSIYON')]) {
  try {
    if (fs.existsSync(p)) {
      const v = fs.readFileSync(p, 'utf-8').trim()
      if (v) {
        appVersion = v
        break
      }
    }
  } catch {}
}

export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify(appVersion)
  },
  base: './',
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src')
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:3800',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: '../backend/cmd/server/dist',
    emptyOutDir: true
  }
})
