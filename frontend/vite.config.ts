import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Forward /api/* and /_/* to the PocketBase backend
      "/api": {
        target: process.env.VITE_PB_URL ?? "http://127.0.0.1:8090",
        changeOrigin: true,
      },
      "/_": {
        target: process.env.VITE_PB_URL ?? "http://127.0.0.1:8090",
        changeOrigin: true,
      },
    },
  },
})
