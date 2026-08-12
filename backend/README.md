# 2pick Go backend

這是新 API 的獨立 Go service。它不會直接取代 Laravel；遷移期間由 edge 依 path 將 `/api/v1/*` 導向此服務，舊 `/api/*`、`/session-context`、登入與尚未搬遷的功能仍由 Laravel 提供。

## 已提供的 endpoints

- `GET /health/live`：process liveness，永不 cache。
- `GET /health/ready`：load balancer readiness，永不 cache。
- `GET /api/v1/system/info`：第一個 versioned public contract，示範 Cloudflare cache header。
- `GET /api/v1/auth/me`：驗證 Laravel 簽發的短效 Bearer token；未設定公鑰時回傳 `503 auth_not_configured`。
- `GET /api/v1/posts`：公開投票列表，支援 `sort_by`、`range`、`page`、`per_page`、`k`。
- `GET /api/v1/tags`、`GET /api/v1/tags/hot`：公開標籤與熱門標籤。
- `GET /api/v1/carousel-items`：首頁輪播內容。
- `GET /api/v1/champions`：最近完成的公開投票結果。
- `GET /api/v1/ranks`：指定投票的可分頁全站排行榜；`group=cumulative`（預設）或 `group=recent_1000`。
- `GET /api/v1/rank`、`GET /api/v1/rank/search`：單一選項排行榜趨勢、累積／最近一千筆名次與搜尋。
- `GET /api/v1/game-posts/{postSerial}`：公開投票的遊戲設定，可由 edge 短暫快取。
- `POST /api/v1/games`：建立匿名單人遊戲並隨機取得選項，永不快取。
- `GET /api/v1/games/{gameSerial}/elements`：恢復既有遊戲的完整候選清單，供舊版未下載完成的本地進度接續使用，永不快取。
- `POST /api/v1/games/{gameSerial}/votes/batch`：以 `expected_vote_count` 原子提交批次票；重送相同前綴具冪等性，分支不同回傳 `409`。
- `GET|POST /api/v1/posts/{postSerial}/comments`：留言列表與建立留言，支援登入身份、匿名模式、投票冠軍標籤、分頁及 200 字限制；永不快取。
- `POST /api/v1/posts/{postSerial}/comments/{commentID}/report`：檢舉指定留言；永不快取。

所有 JSON response 都使用 `{ data, meta }` 或 `{ error, meta }` envelope，並回傳 `X-Request-ID`。
完整 request/response 定義位於 [`api/openapi.yaml`](api/openapi.yaml)。

## Laravel 身份橋接

遷移期間 Laravel 繼續負責登入、註冊、Google OAuth、密碼重設與 session。登入使用者呼叫 `/session-context` 時，Laravel 可額外簽發 5 分鐘的 Ed25519 token；Go 只驗證簽章，不讀取或解密 Laravel session cookie。

先產生一組金鑰（以下命令會把私鑰顯示在本機終端，請勿貼到 log 或提交 Git）：

```bash
./vendor/bin/sail php -r '$p=sodium_crypto_sign_keypair(); echo "GO_AUTH_PRIVATE_KEY=".base64_encode(sodium_crypto_sign_secretkey($p)).PHP_EOL."GO_AUTH_PUBLIC_KEY=".base64_encode(sodium_crypto_sign_publickey($p)).PHP_EOL;'
```

設定方式：

- Laravel/AWS Secrets Manager：`GO_AUTH_PRIVATE_KEY`。
- Go service：`GO_AUTH_PUBLIC_KEY`。
- 兩邊必須一致：`GO_AUTH_ISSUER` 與 `GO_AUTH_AUDIENCE`。
- 私鑰未設定時，`/session-context` 保持原本功能並回傳 `api_token: null`。
- 前端只把 access token 放在記憶體並使用 `Authorization: Bearer ...`；不可寫入 `localStorage`、cookie 或 log。

本機的 root `.env` 可同時放置兩把 key；`compose.separated.yml` 只會把公鑰傳進 Go container。production 必須分開管理 secret，且 Cloudflare 不得 cache `/session-context` 與 `/api/v1/auth/*`。

## 本機執行

本機未安裝 Go 時可直接使用 repository root 的 compose：

```bash
docker compose -f compose.separated.yml up --build
```

若已安裝 Go 1.26：

```bash
cp .env.example .env
make test
make run
```

公開內容 endpoints 直接讀取現有 MySQL，其中首頁列表使用 Laravel 已建立的
`public_posts.data` 讀模型，不會重跑完整 Eloquent 關聯。Go service 使用獨立連線池；production
公開內容階段只需 `SELECT`；啟用新版遊戲 API 後，service account 另需對
`games`、`game_elements`、`game_1v1_rounds`、`user_game_results` 的最小必要寫入權限。
啟用留言 API 時，另需對 `comments`、`post_comments`、`reported_comments` 開放必要的
`SELECT`／`INSERT`，並對 `users`、`posts`、`post_policies` 保留 `SELECT`。
請把連線資訊存入 Secrets Manager，且不要讓前端或 CDN 取得資料庫憑證。

`compose.separated.yml` 預設透過 `host.docker.internal:3306` 連到已啟動的 Sail MySQL；先確定舊環境正在執行，再啟動分離環境：

```bash
./vendor/bin/sail up -d mysql
docker compose -f compose.separated.yml up --build -d
curl http://localhost:8080/health/ready
curl 'http://localhost:8080/api/v1/posts?sort_by=hot&range=week&page=1'
```
