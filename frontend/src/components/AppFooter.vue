<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { localizedPath, normalizeLocale, translate } from '../i18n'

const route = useRoute()
const locale = computed(() => normalizeLocale(route.params.locale))
const year = new Date().getFullYear()
const link = (path: string) => computed(() => localizedPath(path, locale.value))

const donatePath = link('/donate')
const termsPath = link('/tos')
const privacyPath = link('/privacy')
</script>

<template>
  <footer class="site-footer">
    <div class="footer-main">
      <RouterLink class="footer-brand" :to="localizedPath('/', locale)" aria-label="2Pick">
        <span class="brand-mark" aria-hidden="true"><img src="/brand-mark.svg" alt="" width="42" height="42"></span>
        <span>2Pick</span>
      </RouterLink>
    </div>

    <nav class="footer-links" :aria-label="translate(locale, 'legalEyebrow')">
      <RouterLink :to="donatePath">{{ translate(locale, 'donate') }}</RouterLink>
      <RouterLink :to="termsPath">{{ translate(locale, 'terms') }}</RouterLink>
      <RouterLink :to="privacyPath">{{ translate(locale, 'privacy') }}</RouterLink>
      <a href="https://forms.gle/DfCfZGUjFncHJdN66" target="_blank" rel="noopener noreferrer">
        {{ translate(locale, 'feedback') }}
      </a>
    </nav>

    <p class="footer-copyright">© {{ year }} 2Pick. All rights reserved.</p>
  </footer>
</template>
