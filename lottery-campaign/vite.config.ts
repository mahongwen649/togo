import { fileURLToPath, URL } from 'node:url'
import { resolve } from 'node:path'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: process.env.VITE_BASE_PATH || '/',
  plugins: [vue()],
  resolve: {
    alias: {
      '@shared': fileURLToPath(new URL('./shared', import.meta.url))
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        user: resolve(__dirname, 'user-web/index.html'),
        admin: resolve(__dirname, 'admin-web/index.html')
      }
    }
  },
  server: {
    proxy: {
      // The canonical local stack exposes Portal through its web gateway on 3000.
      '/api': 'http://127.0.0.1:3000'
    }
  }
})
