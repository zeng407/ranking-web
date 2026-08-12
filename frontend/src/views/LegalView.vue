<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { useRoute } from 'vue-router'

import { legalDocumentHTML, type LegalDocument } from '../content/legal'
import { getRuntimeConfig } from '../config/runtime'
import { normalizeLocale, translate } from '../i18n'

const props = defineProps<{ document: LegalDocument }>()
const route = useRoute()
const locale = computed(() => normalizeLocale(route.params.locale))
const title = computed(() => translate(locale.value, props.document === 'tos' ? 'terms' : 'privacy'))
const content = computed(() => legalDocumentHTML(props.document, locale.value, getRuntimeConfig().contactEmail))

watchEffect(() => {
  document.title = `${title.value} · 2Pick`
})
</script>

<template>
  <div class="legal-page">
    <section class="legal-hero">
      <p class="eyebrow">{{ translate(locale, 'legalEyebrow') }}</p>
      <h1>{{ title }}</h1>
    </section>

    <aside v-if="locale === 'ja'" class="notice" role="note">
      <span aria-hidden="true">i</span>
      {{ translate(locale, 'englishFallback') }}
    </aside>

    <!-- The source is a versioned, trusted snapshot of the existing Blade legal copy. -->
    <article class="legal-copy" v-html="content" />
  </div>
</template>
