# 關閉 Laravel、前端接手 port 80

Laravel 最後一個還被依賴的功能是「忘記密碼／重設密碼」——Go API 不會寄信，所以 SPA 會把
使用者導出去交給 PHP。這件事已經由 Go 接手（見 `docs/password-reset-deployment.md`），
登入、註冊、Google 登入、帳號設定、後台、排程作業先前都已經遷移，因此 Laravel 在功能上
不再有人依賴。

這份文件記錄：port 80 的改動、停掉 Laravel 的方式、回滾方式，以及**停掉之前必須先做的事**。

## 0. 上線前必做：排程

**這一節不做，趨勢排行、公開貼文列表、縮圖、每日排名歷史、sitemap 會無聲停止。**
Laravel 的 `app/Console/Kernel.php` 現在仍然是這些工作的執行者；Go scheduler 的每一條
對應項目預設都是關的（`SCHEDULE_*=false`），因為兩邊同時跑會重複計算趨勢、重複寫入公開
貼文。

停掉 Laravel **之前**，這兩件事要一起做：

1. Go scheduler 打開旗標（`compose.separated.yml` 的 `scheduler` 服務讀 `.env`）：

   ```
   SCHEDULE_POST_TREND_ALL=true
   SCHEDULE_POST_TREND_MONTH=true
   SCHEDULE_POST_TREND_WEEK=true
   SCHEDULE_POST_TREND_DAY=true
   SCHEDULE_UPDATE_PUBLIC_POSTS=true
   SCHEDULE_MAKE_THUMBNAILS=true
   SCHEDULE_REMOVE_UNUSED_IMAGES=true
   SCHEDULE_MAKE_RANK_REPORT_HISTORY=true
   SCHEDULE_REMOVE_OUTDATE_RANK_REPORT_HISTORY=true
   SCHEDULE_GENERATE_SITEMAP=true
   ```

2. 同時停掉 `app/Console/Kernel.php` 裡對應的項目（`make:post-trend` 四行、
   `Update Public Posts`、`Make Thumbnails`、`Remove Unused Images`、
   `Make Rank Report History`、`Remove Outdate Rank Report History`、
   `Generate Sitemap`）。Laravel 整個停掉時 cron 不會執行，等於自動達成；但**回滾**把
   Laravel 開回來的時候，這些項目就會跟 Go 同時跑，所以還是要在程式碼裡停掉，不能只靠
   「反正沒在跑」。

旗標打開後要檢查依賴，否則 handler 不會註冊、或是寫到沒人讀的地方：

- `SCHEDULE_GENERATE_SITEMAP` 需要 `GO_SITEMAP_BASE_URL`，而且 sitemap 會寫進
  `AWS_BUCKET`——那個 bucket 必須就是對外服務 sitemap 的來源。
- `SCHEDULE_UPDATE_PUBLIC_POSTS`、`SCHEDULE_MAKE_RANK_REPORT_HISTORY` 需要
  `LARAVEL_CACHE_PREFIX`（Go 的 freshness 旗標與 element rank memo 用這個前綴組 key）。
  留空的話 worker 會 warn，排名歷史掃描會找不到被標記的貼文。
- worker 必須在跑：scheduler 只負責 enqueue。

### 沒有 Go 對應項目的 Laravel 排程

- `cachePosts`（每五分鐘打十次 `api.public-post.index`）：**故意不移植**。它是在暖
  Laravel 自己的 response cache，Go 端直接讀 `public_posts` 表、快取交給 edge，沒有東西
  可以暖。Laravel 關掉之後這條就該連著 PHP endpoint 一起刪。
- `refresh:token twitch`、`refresh:token imgur`：Go 沒有對應項目，也不需要——Go 的
  ingest 只做 URL 分類，不呼叫這兩家的 API。這兩個 token 只有 PHP 路徑會用，Laravel 停掉
  之後就沒有讀者。若之後 Go 要接手 imgur gallery 解析，這件事要另外處理。
- `telescope:prune`：只在 local 註冊，隨 Laravel 一起消失。

## 1. Port 80 的改動

以下四處已經在 repository 裡改好：

1. `compose.separated.yml`
   - `frontend` 的 ports 從 `"4173:80"` 改成 `"${FRONTEND_PORT:-80}:80"`。
   - `backend` 的 `ALLOWED_ORIGINS` 加上 `http://localhost`（CORS 是白名單，沒加就是
     瀏覽器端每個 API 請求都被擋）。
   - `backend` 加上 `GO_MAIL_TRANSPORT`、`MAIL_*` 與 `APP_URL`。
   - 注意：`GO_MAIL_TRANSPORT` 目前**沒有設**，所以忘記密碼端點回 503。本機 `.env` 的
     `MAIL_USERNAME` 與 `MAIL_PASSWORD` 也是註解掉的，要真的寄信必須補上（Gmail 要 app
     password）並設 `GO_MAIL_TRANSPORT=smtp`；只是想驗流程用 `GO_MAIL_TRANSPORT=log`。
     細節見 `docs/password-reset-deployment.md`。
2. `frontend/docker/nginx.conf`：刪掉 `^/(register|logout|password|auth)` 與 `/login`
   非 GET 兩塊 `proxy_pass http://host.docker.internal:80`。
   **這兩塊不能留**：前端一旦自己佔用 host port 80，`host.docker.internal:80` 就是它自己，
   每一個這類請求都會繞回來變成迴圈。而且 `/password/*` 現在是 SPA 路由。
3. `frontend/vite.config.ts`：移除 `legacyAuthPaths` 與 `legacyProxy`（含 `/login` 的
   method 特例），否則 `npm run dev` / `npm run preview` 下 `/password/...` 會被 proxy
   吃掉送去 Laravel。
4. `frontend/routes-manifest.json`：`frontendRoutes` 補上 `/login`、`/account*`、
   `/password/forgot`、`/password/reset/*`、`/r/*`、`/room/*` 的各語系版本；
   `fallbackOrigin` 從 `laravel` 改成 `none`。

## 2. 停掉 Laravel

Laravel 目前佔著 host port 80，所以順序是「先停 PHP，再重建 frontend」，否則 frontend
會因為 port 已被佔用而起不來：

```bash
docker compose -p ranking-web stop laravel.test
docker compose -f compose.separated.yml -p ranking-web up -d frontend backend
```

容器定義與 `docker-compose.yml` 都保留，只是停止執行。確認：

```bash
curl -sI http://localhost/                      # SPA 的 index.html
curl -sS http://localhost/api/v1/system/info    # 經 nginx proxy 到 Go
```

登入、註冊、Google 登入、忘記密碼四條路徑都不應該再出現 `host.docker.internal`。

## 3. 回滾

完整回滾（Laravel 拿回 port 80）：

```bash
# 前端讓開 port 80
FRONTEND_PORT=4173 docker compose -f compose.separated.yml -p ranking-web up -d frontend

# Laravel 起回來，APP_PORT 預設就是 80
docker compose -p ranking-web up -d laravel.test
```

只是要看一眼舊站、不動前端（兩邊並存，這是比較安全的做法）：

```bash
APP_PORT=8000 docker compose -p ranking-web up -d laravel.test   # http://localhost:8000
```

回滾時要一併還原的東西：

- `routes-manifest.json` 的 `fallbackOrigin` 改回 `laravel`，反向代理才會把 allowlist
  以外的路徑送回 PHP。
- 依 §0 停掉 Go scheduler 的 `SCHEDULE_*`，或是停掉 `Kernel.php` 對應項目，**兩邊只能有
  一邊在跑**。
- `ALLOWED_ORIGINS` 若前端搬回 4173，`http://localhost` 那一項留著無害，但 4173 必須在。

已經寄出的重設連結不受回滾影響：Go 的 token 存在 `go_password_resets`，Laravel 的存在
`password_resets`，兩張表獨立。但**同一個時間點只有其中一邊寄出的連結有效**，切換的瞬間
另一邊已寄出、還沒使用的連結會失效（TTL 60 分鐘）。

## 4. 之後可以做，這次沒做

- `GO_OAUTH_GOOGLE_REDIRECT_URL` 仍指向 `:8080`。改成
  `http://localhost/api/v1/auth/oauth/google/callback` 可以讓 callback 與站台同源，但
  **必須先在 Google Cloud console 加這個 URI**，否則會直接 `redirect_uri_mismatch`。
- 刪掉 PHP 端已經沒有讀者的程式碼（`cachePosts`、Blade 版 `/admin`、auth 相關 route）。
  停掉容器不等於刪掉，這次只做前者，回滾才有東西可用。
