import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  root: __dirname,
  server: {
    port: 3001,
    proxy: { '/api': 'http://localhost:9090' }
  },
  resolve: {
    alias: { '@': path.resolve(__dirname, '../src') }
  }
})
