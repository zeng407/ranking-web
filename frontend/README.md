# 2pick separated frontend

Vue 3 + TypeScript + Vite 靜態前端。`npm run build` 產生的 `dist/` 可上傳至 private S3，再由 CloudFront OAC 或既有 Cloudflare edge 發布。

## 本機開發

```bash
npm install
npm run dev
```

建議使用 Node 24 LTS；最低需求為 Node 22.18。

Vite 會將 `/api/v1/*` proxy 到 `http://localhost:8080`。可用 `FRONTEND_API_PROXY_TARGET` 改變本機 backend。

也可以從 repository root 一次啟動：

```bash
docker compose -f compose.separated.yml up --build
```

瀏覽 `http://localhost:4173`。

## 已遷移頁面

- `/`：目前維持繁體中文首頁
- `/{zh-tw|en|ja}/`
- `/{zh-tw|en|ja}/donate`
- `/{zh-tw|en|ja}/tos`
- `/{zh-tw|en|ja}/privacy`
- `/g/{postSerial}`、`/{zh-tw|en|ja}/g/{postSerial}`：新版單人二選一遊戲

這些頁面完全由靜態前端提供，不建立 Laravel session。日文的服務條款與隱私權政策和舊站相同，暫時回退至英文，畫面會顯示提示。

舊 `/lang/{locale}/*` 以及無語系的 `/donate`、`/tos`、`/privacy` 由 Nginx 永久 `301` 到 canonical locale URL。`/` 暫時提供繁中，但內部繁中語言連結使用 `/zh-tw/`；等 `/zh-tw/` 收錄與流量穩定後，再決定是否把 `/` 改為 `x-default` 國際入口。

`routes-manifest.json` 是這一批可切流的 allowlist 與 redirect contract。Cloudflare／反向代理可把首頁、`/hot`、`/new`、公開遊戲頁與多語系對應路徑送到新版 frontend；登入與尚未遷移的管理頁仍交給 Laravel。切流時保留 Laravel 同名 route 作為立即回滾來源。

公開遊戲採 local-first：每票先保存至 `localStorage`，再送到 Go API。重新整理會保留已完成的票並重新抽本輪尚未對決的候選；雙分頁由可接管的 lease 保證只有一頁寫入。Go 回傳分支衝突時，前端只停止雲端同步，不覆蓋或回溯本地進度。

`app-config.js` 可填入公開聯絡信箱：

```js
window.__APP_CONFIG__ = Object.freeze({
  apiBaseUrl: '/api/v1',
  contactEmail: 'support@example.com',
})
```

新版 UI 支援系統 dark/light preference、手動切換、`localStorage` 偏好保存及手機導覽。主題初始化放在 `index.html`，避免載入 Vue 前出現錯誤主題閃爍。

## Runtime config 與 CDN

`public/app-config.js` 不進 hashed bundle，部署時必須使用 `Cache-Control: no-store`。它讓同一份前端 artifact 在不同環境指定 API base URL。

- `index.html`：`public, max-age=0`，由 edge 設短 TTL / stale-while-revalidate。
- `/assets/*`：`public, max-age=31536000, immutable`。
- `/app-config.js`：`no-store`。

正式環境建議讓 `apiBaseUrl` 保持 `/api/v1`，由 Cloudflare path routing 導向 Go API。這能維持同源 cookie 與 CSRF，不需要把 credentialed CORS 設成 `*`。
