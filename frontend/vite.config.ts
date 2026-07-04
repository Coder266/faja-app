import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Listen on 0.0.0.0, not just localhost. Inside the devcontainer the
    // host reaches Vite over a forwarded port, so binding to loopback only
    // makes the page unreachable from the host browser.
    host: true,
    // 5173 is used by the devcontainer itself, so run the dev server on 5174.
    // strictPort makes Vite fail loudly if 5174 is taken instead of silently
    // hopping to another port (which breaks the fixed proxy/URL assumptions).
    port: 5174,
    strictPort: true,
    proxy: {
      // Forward API calls to the Go backend during `npm run dev` so the
      // frontend can keep using relative paths (e.g. `/api/conditions`)
      // with no CORS involved — the browser only ever talks to the Vite
      // origin, and Vite proxies /api/* to the backend on :8080.
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
