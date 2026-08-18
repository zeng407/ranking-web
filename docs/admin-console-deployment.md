# 後台（admin console）部署須知

後台是**第二份、獨立的前端 build**，不在公開的 `dist/` 裡，也不是公開 SPA 的一條 route。
它的檔案由 Go api 這個 process 自己 serve，而且只 serve 給帶著簽章 pass cookie 的請求。

這份文件只講部署；程式在 `frontend/src/admin/`、`frontend/vite.admin.config.ts`，
閘門在 `backend/internal/httpapi/admin_assets.go`。

## 1. 建置

```bash
cd frontend
npm run build:admin      # vue-tsc --noEmit && vite build --config vite.admin.config.ts
```

輸出預設在 `frontend/admin-dist/`（已列入 `.gitignore`），可用 `ADMIN_ASSET_DIR` 環境變數改 `outDir`。
入口固定是 `index.html`：Go 對「不是檔案」的路徑會 fallback 到 `index.html`，換成別的檔名會讓
`/admin/posts` 這類深層網址壞掉。

`npm run build`（公開站）**不會**也**不可以**產生後台的任何 chunk。

## 2. 這些檔案要放在哪裡

> **這是整個機制唯一會被部署方式弄失效的地方。**
> `ADMIN_ASSET_DIR` 指到的目錄**絕對不能**在 nginx / CloudFront / S3 這些直接對外的 document root
> 底下。web server 讀得到的檔案，就是不帶 pass 也拿得到的檔案 —— Go 的閘門擋不到不是 Go serve 的檔案。

正確做法是把 build 放進只有 Go 這個 process 讀得到的路徑，例如容器內的 `/srv/admin`
（唯讀掛載即可），再設定：

```
ADMIN_ASSET_DIR=/srv/admin
```

`compose.separated.yml` 的 `backend` 已經把 `./frontend/admin-dist` 唯讀掛到 `/srv/admin`；
本機在 root `.env` 設 `GO_ADMIN_ASSET_DIR=/srv/admin` 就會生效。

沒設 `ADMIN_ASSET_DIR` 時 `/admin/` 與 grant 端點都回 404 —— 這正是「這個環境不放後台」該有的行為。

## 3. Edge 必須把 `/admin/*` 送到 Go

`frontend/routes-manifest.json` 的 `fallbackOrigin` 現在是 `none`，`/admin/*` 列在
`backendRoutes`。但只要環境裡還有一個仍在執行的 Laravel（回滾用的那一份也算），它同樣會
serve Blade 版的 `/admin`；沒有調整路由的環境，使用者會拿到舊後台而不是新的 bundle。

部署新後台時，反向代理要把這兩組路徑送到 Go api：

- `/admin/*` —— 後台的靜態檔（Go 直接處理，不進 manifest 的 fallback 清單）
- `/api/v1/admin/*` —— 後台的 JSON API 與 pass 端點

同時 edge **不可以 cache** 這兩組路徑。Go 已經回 `Cache-Control: private, no-store`
與 `Cloudflare-Cdn-Cache-Control: no-store`，但 edge 端仍應明確排除，避免 pass 背後的內容
被共用快取吐給沒有 pass 的人。

回滾方式：把 `/admin/*` 的路由改回 Laravel，Blade 後台仍然可用。

## 4. 金鑰

pass cookie 的簽章金鑰是從 `GO_AUTH_PRIVATE_KEY` 推導出來的，**沒有另外一個 secret 要管**。

- `ADMIN_ASSET_DIR` 有設、但 `GO_AUTH_PRIVATE_KEY` 沒設：所有 `/admin/*` 一律 403，
  啟動時會記一筆 `admin_asset_key_unset`。
- **輪替 `GO_AUTH_PRIVATE_KEY` 會讓所有已發出的 pass 立刻失效**，正在用後台的人要重新登入一次。
  這是刻意的行為。

## 5. 使用流程（部署後確認用）

1. 以有 admin role 的帳號登入公開站。
2. 帳號選單出現「後台管理」，點下去會先 `POST /api/v1/admin/assets/grant`（Bearer token），
   Go 回 204 並種下 httpOnly、`Path=/admin/`、`SameSite=Lax`、1 小時的 cookie `2pick_admin`。
3. 前端接著用整頁跳轉到 `/admin/` —— 那是另一份 document，vue-router 導不過去。
4. 登出時前端會先打 `POST /api/v1/admin/assets/revoke`。

cookie 在剩下不到一半 TTL 時會自動續期，所以一直在用後台的人不會被中途踢出。

## 6. 上線後要驗的事

在**對外的網址**上跑一次（不是本機、也不要帶 cookie）：

```bash
# 沒有 pass：必須是 403，不是 200，也不是 404
curl -s -o /dev/null -w '%{http_code}\n' https://<host>/admin/
curl -s -o /dev/null -w '%{http_code}\n' https://<host>/admin/index.html
curl -s -o /dev/null -w '%{http_code}\n' https://<host>/admin/assets/<實際檔名>.js

# 沒有登入：401
curl -s -o /dev/null -w '%{http_code}\n' -X POST https://<host>/api/v1/admin/assets/grant
```

**只要其中任何一條回 200，就是靜態檔被公開 serve 了**，`ADMIN_ASSET_DIR` 或 edge 路由設錯，
必須先修好再開放。

拿到 403（而不是 404）是刻意的：pass 過期的人要能分辨「重新登入」和「這個 build 沒有這個檔」。

登入後再確認一次：grant 回 204、`/admin/` 與深層路徑（例如 `/admin/carousel`）回 200、
回應標頭有 `Cache-Control: private, no-store` 與 `X-Frame-Options: DENY`、
revoke 之後 `/admin/` 回到 403。

另外確認一個**沒有** admin role 的一般帳號：grant 與 `/api/v1/admin/*` 都必須是 403。
bundle 載得進來從來就不等於有權限做事 —— 每一條 admin API 在伺服器端都會再檢查一次 role。
