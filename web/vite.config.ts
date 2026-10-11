import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// In development, `make dev/serve` runs the Go API on :7878 and Vite proxies to it.
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    proxy: { '/api': 'http://localhost:7878' },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
