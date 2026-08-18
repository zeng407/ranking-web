# 2Pick 前端 UI 規格書

這份文件用文字描述**目前**前端畫面長什麼樣、每個網址上有哪些區塊、哪些動作、桌機與手機的差別。

## 怎麼用這份文件

1. 你直接改這個檔案：改文案、加動作、刪區塊、調整手機行為，改到你要的樣子。
2. 告訴 Claude「照 `docs/ui-spec.md` 改」，我會把這份文件跟現況比對，只動有差異的地方。
3. 想標記意圖可以用行內註記：`[改]`、`[新增]`、`[刪除]`、`[?]`（我會來問）。
   不寫也可以，我會自己比對出差異。

規則與慣例：

- 每個頁面與區塊都有一個 **ID**（例如 `HOME`、`HOME.carousel`）。**請不要改 ID**，那是我對照的依據；
  要刪掉某個區塊就在它的標題後面寫 `[刪除]`。
- 文案寫的是**目前顯示的中文（zh-TW）**，後面括號是 i18n key，例如 `熱門（popular）`。
  改文案時改中文就好；有 key 的字串我會同步改 `frontend/src/i18n.ts` 的三個語言
  （zh-TW / en / ja），並在回報時列出英日文的建議翻譯讓你確認。沒有 key 的字串是寫死在畫面上的。
- 「桌機」指視窗寬度 > 920px，「手機」指 ≤ 920px；另外 ≤ 640px 還有一層更窄的調整，
  需要區分時會寫「窄手機」。這三個斷點是目前 CSS 的實際值。
- 網址中的 `{locale}` 是 `zh-tw` / `en` / `ja`。沒有語系前綴的網址（例如 `/`、`/hot`）也存在，
  顯示繁體中文。

---

## 0. 全站共用

### SHELL — 頁面外框

- 每一頁的結構都是：頁首（`HEADER`）→ 內容 → 頁尾（`FOOTER`）。
- 主題有淺色與深色，開啟時先讀系統偏好，使用者切換後記在瀏覽器裡，下次直接套用。
  切換不會閃爍（在 HTML 載入 Vue 之前就決定好主題）。
- 內容寬度：桌機最寬 46rem 以上依頁面而定；手機是「兩側各留 1rem」。

### HEADER — 頁首

網址：全站每一頁都有。

**桌機**（由左到右）

| 元素 | 內容 | 動作 |
| --- | --- | --- |
| 品牌 | 2Pick 標誌 + 文字 | 回首頁 |
| 主選單 | 首頁（home）／熱門（popular）／最新（latest）／贊助（donate） | 切頁 |
| 語言選單 | 目前語言（地球圖示 + 名稱） | 點開下拉，選 繁體中文／English／日本語 |
| 主題切換 | 太陽／月亮圖示 | 切換深淺色 |
| 帳號 | 未登入：登入（login）連結／已登入：頭像按鈕 | 見下方帳號選單 |

**帳號選單**（已登入時點頭像展開）

- 我的投稿（myPosts）→ `/{locale}/account/posts`
- 帳號設定（accountTitle）→ `/{locale}/account`
- 後台管理（adminConsole）→ **只有管理員看得到**，點了會離開 SPA 整頁跳到 `/admin/`
- 登出（logout）
- 後台開啟失敗時，選單裡多一行：目前無法進入後台，請重新登入後再試。（adminConsoleUnavailable）

其他行為：點選單以外的地方、按 Esc、換頁，都會關閉下拉。登入狀態還在確認時，帳號位置顯示空白佔位
（避免已登入的人先看到「登入」再跳掉）。

**手機（≤920px）**

- 主選單與語言下拉**隱藏**，改成右側的漢堡按鈕。
- 展開後是一塊兩欄的選單：首頁／熱門／最新／贊助，下面整排是登入或登出，
  最下面是語言切換（三個並排的按鈕，目前語言反白）。
- 手機選單裡**沒有**「我的投稿／帳號設定／後台管理」；那些只在頭像下拉裡。
- 窄手機（≤640px）：頁首高度縮成 4.25rem，圖示按鈕的觸控範圍會被撐大到 44px。

### FOOTER — 頁尾

- 品牌 2Pick（連回首頁）
- 連結：贊助（donate）／服務條款（terms）／隱私權政策（privacy）／意見回饋（feedback，開新分頁到 Google 表單）
- © {年份} 2Pick. All rights reserved.

---

## 1. 首頁與瀏覽

### HOME — 首頁／熱門／最新

網址：`/`、`/hot`、`/new`、`/{locale}/`、`/{locale}/hot`、`/{locale}/new`
（三個網址是同一個畫面，差別只在排序與標題）

**桌機版面**：左邊是「探索側欄」（`HOME.carousel` + `HOME.champions`），右邊是主內容
（搜尋 + 投票列表）。側欄會跟著捲動固定在畫面上。

**手機版面**：改成單欄，順序是 輪播 → 冠軍出爐 → 搜尋 → 列表。

#### HOME.carousel — 社群精選輪播

- 小標：精選（highlightsEyebrow）／標題：社群精選（communityHighlights）
- 一次顯示一張投影片：YouTube 影片（內嵌播放）或圖片（可帶外連）。
- 下方有標題與說明（說明和標題相同時不重複顯示）。
- 超過一張時：左右箭頭按鈕、下方「第幾張 / 共幾張」與圓點；點圓點直接跳那一張。
- 沒有任何輪播資料時，整個區塊不出現。

#### HOME.champions — 冠軍出爐跑馬燈

- 小標：剛結束（justFinishedEyebrow）／標題：冠軍出爐（recentChampions）
- 橫向跑馬燈，每一項是一場剛結束的比賽：投稿標題 + 冠軍（championWinner）與敗者（championLoser）
  的縮圖與名字，中間一個 `›`。
- 點任何一項 → 進入該投稿的遊戲頁 `/{locale}/g/{serial}`。
- 沒有資料時整個區塊不出現。

#### HOME.search — 搜尋列

- 放大鏡圖示 + 輸入框，提示文字：搜尋投票（searchPlaceholder），上限 255 字。
- 送出 → 網址加上 `?k=關鍵字`，列表換成搜尋結果，**畫面不會捲回頂端**。
- 有關鍵字時輸入框右側出現 `×` 清除鈕（clearSearch）。
- 右側是「搜尋」送出鈕（searchVotes）。窄手機時送出鈕縮小。

#### HOME.tabs — 排序切換與標題

- 小標：探索（discoverEyebrow）；標題（h1）依排序而定（熱門／最新／搜尋結果）。
- 右側兩個分頁：熱門（popular）→ `/hot`、最新（latest）→ `/new`，目前的那個反白。

#### HOME.tags — 熱門標籤

- 「熱門話題（hotTopics）」+ 最多 9 個標籤按鈕（`#標籤`）。
- 點一下 → 以該標籤搜尋；再點目前反白的那個 → 取消。
- 沒有標籤資料時不出現。

#### HOME.list — 投票列表

每張卡片：

- 上半：兩個候選的圖片與名稱，中間一個 `VS`；被封鎖（censored）的投稿圖片會打上遮罩。
- 下半：標題、`{n} 次遊玩（plays）`、`{n} 個選項（options）`、最多 3 個標籤。
- 卡片底部兩個動作：**開始投票（openVote）**→ `/g/{serial}`、**看排行（viewRanking）**→ `/r/{serial}`。
- 整張卡片上半也是連到遊戲頁的連結。

狀態：

- 載入中：轉圈 + 載入投票中…（loadingVotes）
- 失敗：無法載入投票（loadVotesError）+ 重試（retry）
- 空：目前沒有投票（noVotes）
- 還有更多時，列表下方一顆「載入更多（loadMore）」，載入中顯示 loadingMore，失敗顯示 retry。

版面：桌機多欄網格；窄手機改為單欄。

---

## 2. 遊戲與排行

`/g/{serial}` 與 `/r/{serial}` 是**同一個畫面**的兩種進入方式：前者開始玩，後者直接看排行。
舊網址 `/r/{serial}/export` 會直接導到 `/r/{serial}`。

### GAME — 遊戲頁

網址：`/g/{serial}`、`/{locale}/g/{serial}`

#### GAME.locked — 需要密碼（僅密碼保護的投稿）

- 標題：這個投票需要通行碼（gameDoorCodeTitle）+ 說明（gameDoorCodeHint）
- 一個輸入框（gameDoorCodeLabel）+ 送出鈕；錯誤時顯示紅字。

#### GAME.setup — 開始前的設定畫面

- 小標 `2PICK · NEW GAME`、投稿標題（h1）、說明。
- 預覽：兩個候選的圖片／影片（影片會靜音預覽）。
- 右側面板：`共有 {n} 個選項（gameAvailable）`、選擇要玩幾個（gameChooseCount），
  下方是數量按鈕：8 / 16 / 32 / 64 / 128 / 256 與「全部」，只列出不超過選項總數的；
  預設選 32（或選項總數，取小者）。
- 動作：**開始遊戲**（主要按鈕）、**看排行**、**分享**（複製連結）。
- 手機：改為單欄，預覽在上、設定在下。

#### GAME.play — 對戰進行中

頁首列（`GAME.play.header`）：

- 投稿標題（h1）
- 統計：目前輪次名稱（例如 8 強）+ `第幾場/共幾場`、剩餘數量
- 圖示按鈕（每個都有 tooltip）：看排行 → `/r/{serial}`、
  **接手（gameTakeOver，只有在別的分頁正在玩時出現）**、
  開房間／複製邀請連結（roomHostStart／roomInviteTitle，比賽結束後隱藏）、
  分享（gameShare，複製後變成 gameShareCopied）、
  重新開始（gameRefresh，開啟對話框）、
  操作說明（gameControls）

對戰區（`GAME.play.arena`）：

- 兩張候選卡並排，各自有圖片／影片／YouTube 內嵌與標題，點下去就是投給它。
- 鍵盤：`←` 或 `1` 選左邊、`→` 或 `2` 選右邊、`?` 開關操作說明。
  游標在輸入框裡時鍵盤快捷鍵不作用。
- 操作說明面板（`?` 或圖示開啟）：列出上述按鍵，右上角 `×` 關閉。

側欄（`GAME.play.history`）：對戰紀錄（gameHistory），每列是「勝方 › 敗方」的縮圖；
沒有紀錄時顯示 gameNoHistory。旁邊有一個廣告位（advertisement）。

房間邀請（`GAME.play.invite`，開了房間才出現）：顯示邀請標題、邀請網址、
「複製連結」與「進入房間」兩個動作；開房失敗顯示錯誤。

**手機（≤920px）**：
- 版面從「側欄 + 對戰區」變成上下堆疊，對戰紀錄移到對戰區下方。
- 1250px 以下時，對戰紀錄側欄先變窄（仍在左側），920px 以下才換成堆疊。
- 兩張候選卡改為上下排列，圖片高度縮小以塞進一個螢幕。

#### GAME.result — 結果與排行

- 結算中的過場：小標 `2PICK · RESULT`、正在整理排行（gamePreparingRanking）+ 說明 + 進度條。
- 冠軍區塊：冠軍圖片 + 小標 冠軍（gameWinner）+ 名稱，動作是**再玩一次（gameRestart）**與**回首頁（gameBackHome）**。
- 排行區塊（`GAME.result.ranking`）：
  - 小標 `2PICK · RANKING` + 投稿標題
  - 動作：分享我的結果、匯出圖片（開啟 `EXPORT` 對話框）、開始新遊戲
  - 兩個分頁：**我的排行**與**大家的排行**（有個人結果時才出現）
  - 大家的排行下面還有一排分組：累積 / 最近 1000 場等
  - 每一列：名次、縮圖、名稱、勝率（gameWinRate）
  - 點某一列 → 上方展開該選項的趨勢圖（折線 + 起訖日期）與大圖／YouTube 預覽
  - 分頁：上一頁（previousPage）／`第幾頁 / 共幾頁`／下一頁（nextPage）
  - 狀態：載入中、載入失敗 + 重試、沒有資料
- 手機：分組分頁與列表改為單欄，趨勢圖寬度貼齊螢幕。

#### GAME.restart — 重新開始對話框

- 小標 `2PICK · GAME`，右上 `×` 關閉。
- 若有未完成的遊戲，第一個選項是「繼續上一場」。
- 下半是「開新的一輪（gameNewRound）」+ 數量按鈕（同 `GAME.setup`）。

### EXPORT — 排行匯出對話框

由排行頁的「匯出圖片」開啟。

- 小標 `2PICK · EXPORT` + 標題（rankingExportTitle），右上 `×`。
- 左欄：產生出來的排行圖片預覽 + **下載（rankingExportDownload）**。
  手機上不能直接下載時，改顯示提示：長按圖片儲存（rankingExportMobileHint）。
- 右欄：可複製的文字版排行（唯讀），下面是**複製（rankingExportCopy）**，
  複製成功變成 rankingExportCopied，失敗顯示錯誤。
- 狀態：產生中（rankingExportPreparing）、產生失敗 + 重試。
- 窄手機：對話框幾乎滿版，兩欄改成上下。

### COMMENTS — 留言區

出現在排行頁下方。

- 小標 `2PICK · COMMENTS`、標題 留言（commentTitle）+ 則數。
- 每則留言：頭像（沒有就顯示暱稱首字）、暱稱、相對時間、內容、
  該使用者在這個投稿選出的冠軍（獎盃圖示），右上角 `⋯` 是檢舉。
- 分頁：上一頁／`第幾頁 / 共幾頁`／下一頁。
- 留言框：頭像 + 暱稱（勾選匿名時顯示 `****`）、我的投票結果、
  多行輸入框（提示 commentLeave）、字數 `{現在} / {上限}`、送出鈕（commentSubmit／commentSubmitting）。
- 檢舉對話框：小標 `2PICK · REPORT`、被檢舉的留言內容、原因下拉、
  選「其他」時多一個自由輸入框、取消與送出；成功顯示已檢舉（commentReported）。
- 狀態：載入中、載入失敗 + 重試、沒有留言（commentEmpty）。

### ROOM — 觀戰／同樂房間

網址：`/room/{serial}`、`/{locale}/room/{serial}`

- 標題：遊戲房間（gameRoom）+ 房號；右上角是連線狀態燈：
  即時連線（roomLive）或輪詢中（roomPolling）——兩種都能用，只是後者慢一點。
- 本輪對戰：`第幾輪 / 共幾輪` + 總票數，兩個候選按鈕（圖片、名稱、目前票數），
  點下去下注；自己已選的那個會反白。送出中兩個都不能點。
- 我的成績：分數、名次、命中率、連勝，下面是可以改暱稱的表單（roomNickname + roomSave）。
- 排行榜：標題 + 參與人數，每列是 名次 / 暱稱 / 分數 / 命中率，自己那列會highlight。
  沒有人時顯示 roomNoPlayers。
- 狀態：載入中（roomLoading）、找不到房間（roomNotFound）、載入失敗 + 重試、
  目前沒有進行中的回合（roomNoRound）。

---

## 3. 帳號與投稿管理

### LOGIN — 登入／註冊

網址：`/login`、`/{locale}/login`

- 左上小標（loginEyebrow）+ 標題（loginTitle）+ 說明（loginIntro）。
- 卡片上方兩個分頁：登入（loginTab）／註冊（registerTab）。
- 登入表單：Email、密碼、記住我（rememberMe）、忘記密碼（forgotPassword，連到 SPA 內的
  `/{locale}/password/forgot`）、送出鈕（送出中顯示 authSubmitting）。
- 註冊表單：暱稱（含提示 nicknameHint）、Email、密碼、確認密碼、送出鈕。
- 分隔線（socialDivider）下方是 **使用 Google 登入（googleLogin）**，這是整頁跳轉。
- 最下面一行是服務條款與隱私權政策的連結。
- 錯誤：整體錯誤顯示在卡片上方紅字；欄位錯誤顯示在該欄位下方。

### PASSWORD — 忘記密碼／重設密碼

網址：`/{locale}/password/forgot`、`/{locale}/password/reset/{token}`
（沒有語系前綴的舊格式會轉到 `zh-tw`，因為 Laravel 寄出的信是那個形狀）。

- 兩頁都用 LOGIN 的 `.auth-page` / `.auth-card` 版型，小標沿用 loginEyebrow。
- 忘記密碼：一個 Email 欄位 + 送出鈕（forgotPasswordSubmit）。送出成功後表單換成
  `role="status"` 的說明（forgotPasswordSent）。
  **文案必須是有條件的**（「如果這個電子郵件已經註冊過…」）：伺服器對沒有帳號的地址、
  被 throttle 擋下的請求、寄信失敗都一律回 200，改寫成「信已寄出」會把這個表單變成
  查詢某個信箱有沒有註冊的工具。
- 重設密碼：新密碼 + 確認密碼（不一致由前端擋，不浪費一次連結）、送出鈕
  （resetPasswordSubmit）。成功後直接登入並回首頁。
- 連結失效、用過、亂填一律顯示同一句 resetPasswordLinkInvalid，並提供回到忘記密碼頁的連結。

### ACCOUNT — 帳號設定

網址：`/{locale}/account`（只有語系版本）

- 標題：帳號設定（accountTitle）
- **個人資料**：頭像（沒有就顯示預設人形）+ 更換頭像（accountAvatarChange，開檔案選擇；
  接受 png/jpeg/gif/webp）；暱稱輸入框（上限 20 字）+ 儲存；暱稱剛改過時顯示
  「太快了」提示（accountErrorNameTooSoon）。Email 只顯示不可改（accountEmailFixed）。
- **Google**：已綁定顯示 accountGoogleLinked；未綁定顯示說明 + 連結 Google（accountGoogleConnect）。
- **密碼**：已有密碼時要填目前密碼；沒有密碼（Google 註冊）時顯示 accountPasswordInitHint，
  只要填新密碼與確認；送出鈕文字依情況是 accountPasswordSet 或 accountPasswordInit。
- 狀態：載入中、載入失敗（accountLoadFailed）、成功或失敗的橫幅。

### MYPOSTS — 我的投稿

網址：`/{locale}/account/posts`

- 標題：我的投稿（myPosts）+ 右上「新增（myPostsNew）」。
- 新增表單（點新增後展開）：標題、說明（含 editorDescriptionHint）、
  公開範圍（公開／不公開／密碼）單選，選密碼時多一個密碼欄，最後是建立與取消。
- 列表：每一項是標題（連到編輯頁）、說明、公開範圍、
  總遊玩數 / 本週 / 上週，以及標籤。
- 分頁：上一頁 / `第幾頁 / 共幾頁` / 下一頁。
- 狀態：載入中、載入失敗、目前沒有投稿（myPostsEmpty）。

### EDITOR — 投稿編輯

網址：`/{locale}/account/posts/{serial}`

- 頁首：投稿標題 + 兩個分頁：**基本資料**與**選項**，以及回列表的連結。
- **基本資料**分頁：標題、說明、公開範圍（同上，含密碼欄）、
  標籤（已有的標籤各自可刪，另有新增輸入框）、儲存。
  下方是刪除區：先點刪除，密碼保護的投稿要再輸入密碼，然後確認或取消。
- **選項**分頁：
  - 上傳區：可選檔上傳，失敗的檔案會逐一列出原因。
  - 用網址新增（editorAddURLs）：多行輸入框，一行一個網址，加入鈕。
  - 搜尋標題 + 排序下拉。
  - 每個選項一列：縮圖、標題、（影片的）起訖秒數；
    動作是編輯（就地改標題與秒數，然後儲存／取消）與刪除（要再確認一次）。
  - 分頁：上一頁 / 下一頁。

### LEGAL — 服務條款／隱私權政策

網址：`/{locale}/tos`、`/{locale}/privacy`（`/tos`、`/privacy` 會導到 `/zh-tw/...`）

- 小標（legalEyebrow）+ 標題 + 內文。
- 日文版目前是英文內容，頁面上會顯示一則提示（englishFallback）。

### DONATE — 贊助

網址：`/{locale}/donate`（`/donate` 會導到 `/zh-tw/donate`）

- 標題區：小標 + 標題 + 說明 + 「意見回饋」外連。
- **付款方式**：兩張卡片（綠界 ECPay、歐付寶 OPay），各自有 QR code 圖與「自訂金額」，點了開新分頁。
- **小額支持**：幾個金額級距的卡片，點了到綠界；其中一張旁邊有個 `↗` 會打開一張貓的圖片對話框。
- 手機：標題區與卡片改為單欄。

### MIGRATION — 內部說明頁

網址：`/migration`。給開發者看的路由分流說明，沒有從任何地方連過去。

---

## 4. 後台（獨立的 build）

後台是另一份程式，網址在 `/admin/`，**只有管理員拿得到檔案**（沒有通行證是 403）。
進入方式是前台帳號選單的「後台管理」。介面目前只有桌機版排版（表格橫向捲動），沒有做手機版。

### ADMIN.shell — 後台外框

- 頁首：品牌 + 四個分頁：投稿／使用者／輪播／公告，右邊一個「回前台」（整頁跳回 `/`）。
- 每頁的錯誤都會翻成中文說明；伺服器沒設定公告儲存時會明講是設定問題，不是壞掉。

### ADMIN.posts — 投稿管理

網址：`/admin/posts`（後台首頁會導到這裡）

- 表格：標題與序號、作者（名字 + Email）、公開範圍、遊玩次數、是否封鎖的標記。
- 每列動作：編輯、元素、封鎖／解除封鎖、刪除（刪除要確認）。
- 編輯表單：標題、說明、公開範圍、密碼（留空表示不更動）、封鎖開關。
- 分頁。

### ADMIN.elements — 投稿的元素

網址：`/admin/posts/{serial}/elements`

- 表格：縮圖、標題（可就地改名）、動作：儲存、刪除（要確認）。
- 標題搜尋 + 分頁。
- 換圖片目前**不在後台**，仍由作者自己在前台編輯頁換。

### ADMIN.users — 使用者

網址：`/admin/users`

- 搜尋（名稱或 Email）+ 表格：名稱、Email、角色標記、投稿數。
- 動作：封鎖／解除封鎖。管理員不能被封鎖（按鈕會停用，伺服器也會擋）。
- 分頁。

### ADMIN.carousel — 首頁輪播

網址：`/admin/carousel`

- 新增表單：類型（圖片／影片）、標題、說明、圖片網址、影片網址、影片起訖秒數、是否啟用。
- 列表：每列可就地改標題／說明／秒數、切換啟用、刪除。
- **拖曳排序**：拖左側把手改變順序，放開後一次送出整份順序；失敗時會重新讀取伺服器上的順序。

### ADMIN.announcement — 公告

網址：`/admin/announcement`

- 顯示目前公告（沒有公告是正常狀態，不是錯誤）。
- 發佈表單：內容、圖片網址、保留分鐘數。

---

## 5. 目前刻意「沒有」的東西

寫在這裡是為了避免被當成漏掉：

- 後台沒有手機版排版。
- 後台沒有操作紀錄（誰在什麼時候封鎖了誰）。
- 手機版的漢堡選單裡沒有帳號相關項目。
- 首頁沒有無限捲動，是「載入更多」按鈕。
