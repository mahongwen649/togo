import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

// No production settings injection, authentication or API proxy in this entry.
export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': resolve(__dirname, 'src') } },
  server: { host: '127.0.0.1', port: 3010, strictPort: true },
  build: {
    outDir: 'dist/exclusive-channels-demo',
    rollupOptions: { input: resolve(__dirname, 'exclusive-channels-demo.html') },
  },
})
