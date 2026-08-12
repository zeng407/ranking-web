import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const legacyTarget = process.env.FRONTEND_LEGACY_PROXY_TARGET ?? 'http://localhost:80'

// Laravel is still the session authority. These paths must be proxied rather
// than pointed at another origin: the session cookie and CSRF token are only
// valid on the origin that issued them, and Google OAuth must return to the
// same origin for the new session to be visible to the SPA.
const legacyAuthPaths = ['/session-context', '/register', '/logout', '/password', '/auth']

const legacyProxy = Object.fromEntries(
  legacyAuthPaths.map((path) => [path, { target: legacyTarget, changeOrigin: false }]),
)

const apiProxy = {
  '/api/v1': {
    target: process.env.FRONTEND_API_PROXY_TARGET ?? 'http://localhost:8080',
    changeOrigin: false,
  },
}

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      ...apiProxy,
      ...legacyProxy,
      // /login is shared: the SPA renders the page on GET, Laravel verifies the
      // credentials on POST. Routing has to be by method, not by path.
      '/login': {
        target: legacyTarget,
        changeOrigin: false,
        bypass: (req) => (req.method === 'GET' ? '/index.html' : undefined),
      },
    },
  },
  preview: {
    host: '0.0.0.0',
    port: 4173,
    proxy: {
      ...apiProxy,
      ...legacyProxy,
      '/login': {
        target: legacyTarget,
        changeOrigin: false,
        bypass: (req) => (req.method === 'GET' ? '/index.html' : undefined),
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    manifest: true,
  },
})
