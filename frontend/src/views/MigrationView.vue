<template>
  <section class="content-page">
    <p class="eyebrow">ROUTE OWNERSHIP</p>
    <h1>漸進式前後端分離</h1>
    <p>
      瀏覽器維持同一個公開網域，由 Cloudflare 依 path 將流量送到靜態前端、Go API 或既有 Laravel。這可以保留目前 session cookie 與 CSRF 行為，也能逐頁回滾。
    </p>

    <div class="route-list">
      <div><code>/assets/*</code><span>靜態 S3／CloudFront</span></div>
      <div><code>/api/v1/*</code><span>新版 Go API</span></div>
      <div><code>/api/*</code><span>遷移期間仍由 Laravel 提供</span></div>
      <div><code>/session-context</code><span>Laravel session/private response</span></div>
      <div><code>尚未遷移的頁面</code><span>Laravel Blade</span></div>
    </div>
  </section>
</template>
