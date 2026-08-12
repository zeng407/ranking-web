<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute } from 'vue-router'

import AppFooter from './components/AppFooter.vue'
import AppHeader from './components/AppHeader.vue'

const route = useRoute()
const isGameRoute = computed(() => String(route.name || '').startsWith('game'))
const isHomeRoute = computed(() => route.meta.viewKey === 'home')
</script>

<template>
  <div class="app-shell">
    <AppHeader />
    <main
      id="main-content"
      class="page-shell"
      :class="{ 'game-page-shell': isGameRoute, 'home-page-shell': isHomeRoute }"
    >
      <RouterView v-slot="{ Component, route: currentRoute }">
        <!-- Home, hot and new share a viewKey so the listing view stays
             mounted while its watcher replaces posts through the API.
             Do not wrap GameView in Transition: it intentionally renders a
             page and a dialog as sibling roots, which cannot be transitioned
             and can leave /g -> /r navigation blank. -->
        <KeepAlive include="HomeView">
          <component :is="Component" :key="currentRoute.meta.viewKey || currentRoute.path" />
        </KeepAlive>
      </RouterView>
    </main>
    <AppFooter v-if="!isGameRoute" />
  </div>
</template>
