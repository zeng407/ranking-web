import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

/**
 * The back office bundle, built separately from the public SPA.
 *
 * WHY A SECOND BUILD RATHER THAN A ROUTE. The admin screens must not ship in the public
 * bundle: whatever lands in `dist/` is served by nginx directly, without the gate, so every
 * admin screen, endpoint path and field name would be readable by anyone who fetches a .js
 * file. The API would still refuse a non-moderator's request, but the map of the back office
 * would be public. So this build has its own entry, its own router and its own output
 * directory, and Go serves that directory only to a request carrying the pass minted by
 * POST /api/v1/admin/assets/grant — see backend/internal/httpapi/admin_assets.go.
 *
 * The output directory must therefore never be inside anything a web server serves
 * directly. `admin-dist/` is a sibling of `dist/` and is not copied into the nginx image;
 * ADMIN_ASSET_DIR overrides it for a deployment that mounts the bundle elsewhere.
 *
 * The entry file has to build to `index.html`, which is why the entry lives in its own
 * `admin/` root: Go falls back to `index.html` for any path that is not a file, and that
 * fallback is what makes the admin SPA's own deep links work.
 */

const apiProxy = {
  '/api/v1': {
    target: process.env.FRONTEND_API_PROXY_TARGET ?? 'http://localhost:8080',
    changeOrigin: false,
  },
}

export default defineConfig({
  root: 'admin',
  base: '/admin/',
  // No public directory: the bundle ships no static assets of its own, and pointing this
  // at the public SPA's would copy them into the gated output for no reason.
  publicDir: false,
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    // A port of its own, so the public dev server and the back office can run together.
    port: 5174,
    proxy: apiProxy,
  },
  preview: {
    host: '0.0.0.0',
    port: 4174,
    proxy: apiProxy,
  },
  build: {
    outDir: process.env.ADMIN_ASSET_DIR ?? '../admin-dist',
    // Relative to this config rather than to the root, and the directory is ours alone.
    emptyOutDir: true,
    // No sourcemap: the bundle is deliberately not published, and a map would hand the
    // reader of a leaked file the original sources.
    sourcemap: false,
  },
})
