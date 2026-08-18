import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// There is no legacy proxy any more. /register, /logout, /login, /password and /auth
// used to be forwarded to Laravel, which owned the session cookie and the OAuth
// handshake; Go owns both now under /api/v1/auth, so all five are plain SPA routes.
//
// Keeping the /password entry in particular would have broken this build: the reset
// pages are routes here now, and the proxy would have swallowed them.
const goTarget = process.env.FRONTEND_API_PROXY_TARGET ?? 'http://localhost:8080'

const apiProxy = {
  '/api/v1': {
    target: goTarget,
    changeOrigin: false,
  },
  // The back office bundle, which Go serves from ADMIN_ASSET_DIR behind the pass cookie.
  // Proxied rather than served here so a full-page navigation from this dev server lands on
  // the real gate — a 403 for a browser without a pass, exactly as in production. The admin
  // sources themselves are developed against `npm run dev:admin` on port 5174.
  '/admin': {
    target: goTarget,
    changeOrigin: false,
  },
}

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: apiProxy,
  },
  preview: {
    host: '0.0.0.0',
    port: 4173,
    proxy: apiProxy,
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    manifest: true,
  },
})
