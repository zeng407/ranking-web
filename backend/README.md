# 2pick Go backend

這是新 API 的獨立 Go service。它不會直接取代 Laravel；遷移期間由 edge 依 path 將 `/api/v1/*` 導向此服務，舊 `/api/*` 與尚未搬遷的功能仍由 Laravel 提供；session 與登入已由此服務接手（`/api/v1/auth/*`）。

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

## Session 金鑰

登入、註冊、Google OAuth 與 session 由此服務負責（`/api/v1/auth/*`）。Laravel 已不再簽發身份 token，也不參與 session；Go 用同一組 Ed25519 金鑰簽發並驗證自己的 access token。

先產生一組金鑰（以下命令會把私鑰顯示在本機終端，請勿貼到 log 或提交 Git）：

```bash
docker run --rm golang:1.26.5-alpine sh -c 'cd "$(mktemp -d)" && cat > main.go <<"EOF" && go mod init keygen >/dev/null && go run .
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

func main() {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	fmt.Println("GO_AUTH_PRIVATE_KEY=" + base64.StdEncoding.EncodeToString(privateKey))
	fmt.Println("GO_AUTH_PUBLIC_KEY=" + base64.StdEncoding.EncodeToString(publicKey))
}
EOF
'
```

設定方式：

- Go service：`GO_AUTH_PRIVATE_KEY` 與 `GO_AUTH_PUBLIC_KEY`（production 由 AWS Secrets Manager 提供）。
- 兩邊必須一致：`GO_AUTH_ISSUER` 與 `GO_AUTH_AUDIENCE`。
- 私鑰未設定時，此服務無法簽發 session，登入會失敗；受密碼保護的貼文也維持不可見（fail closed）。
- 前端只把 access token 放在記憶體並使用 `Authorization: Bearer ...`；不可寫入 `localStorage`、cookie 或 log。

本機的 root `.env` 可同時放置兩把 key。production 必須以 secret 管理私鑰，且 Cloudflare 不得 cache `/api/v1/auth/*`。

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

## 後台靜態檔

`ADMIN_ASSET_DIR` 指到的目錄由這個 process 自己 serve，而且只 serve 給帶著簽章 pass cookie 的請求（`POST /api/v1/admin/assets/grant` 發出，來源金鑰由 `GO_AUTH_PRIVATE_KEY` 推導，所以輪替私鑰會讓已發出的 pass 失效）。

**這個目錄不能在任何 web server 直接對外的 document root 底下** —— web server 讀得到的檔案就是不帶 pass 也拿得到的檔案。沒設定時 `/admin/` 與 grant 端點都回 404。

完整部署與驗收步驟見 [`docs/admin-console-deployment.md`](../docs/admin-console-deployment.md)。
