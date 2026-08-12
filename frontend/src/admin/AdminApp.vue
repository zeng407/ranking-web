<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
</script>

<template>
  <div class="admin-shell">
    <header class="admin-header">
      <span class="admin-brand">2pick 後台</span>
      <nav class="admin-nav">
        <RouterLink to="/posts">投票</RouterLink>
        <RouterLink to="/users">使用者</RouterLink>
        <RouterLink to="/carousel">輪播</RouterLink>
        <RouterLink to="/announcement">公告</RouterLink>
      </nav>
      <!--
        A plain link, not a RouterLink: the public site is a different application on the
        same origin, so leaving here has to be a full page load.
      -->
      <a href="/">回前台</a>
    </header>
    <main class="admin-main">
      <RouterView />
    </main>
  </div>
</template>
