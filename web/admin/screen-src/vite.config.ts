import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  root: __dirname,
  server: {
    port: 3001,
    proxy: { '/api': 'http://localhost:9090' }
  },
  resolve: {
    alias: { '@': path.resolve(__dirname, '../src') }
  }
})
