<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { useRoute } from 'vue-router'

import { normalizeLocale, translate } from '../i18n'

const route = useRoute()
const locale = computed(() => normalizeLocale(route.params.locale))
const catDialog = ref<HTMLDialogElement | null>(null)

// Line icons rather than emoji: emoji render as a different typeface on every
// platform, carry their own colour, and cannot pick up the accent used by the
// rest of the site's iconography.
const icons = {
  coffee: 'M4.5 8.5h10v5.5a4 4 0 0 1-4 4h-2a4 4 0 0 1-4-4V8.5Zm10 1.25h2.25a2.25 2.25 0 0 1 0 4.5H14.5M6.5 5.5c0-1 1-1.25 1-2.25M10.5 5.5c0-1 1-1.25 1-2.25',
  cat: 'M4.8 10.6 4.2 6l3.6 1.9a8 8 0 0 1 8.4 0L19.8 6l-.6 4.6a7.4 7.4 0 1 1-14.4 0Zm4.6 2.4h.01m5.18 0h.01M12 15.4l-1.1 1.1m1.1-1.1 1.1 1.1',
  lunch: 'M3.5 11h17a8.5 8.5 0 0 1-8.5 8.5A8.5 8.5 0 0 1 3.5 11Zm4.4-2.6 2.6-3.9m2.2 3.9 2.6-3.9',
  server: 'M7.2 18.2h9.3a3.4 3.4 0 0 0 .5-6.77 5 5 0 0 0-9.4-1.83 4.3 4.3 0 0 0-.4 8.6Z',
} as const

const tiers = computed(() => [
  { icon: icons.coffee, label: translate(locale.value, 'coffee'), amount: '$50' },
  { icon: icons.cat, label: translate(locale.value, 'catFood'), amount: '$100', cat: true },
  { icon: icons.lunch, label: translate(locale.value, 'lunch'), amount: '$120' },
  { icon: icons.server, label: translate(locale.value, 'server'), amount: '$300' },
])

watchEffect(() => {
  document.title = `${translate(locale.value, 'donate')} · 2Pick`
})
</script>

<template>
  <div class="donate-page">
    <section class="donate-hero">
      <div>
        <p class="eyebrow">{{ translate(locale, 'donateEyebrow') }}</p>
        <h1>{{ translate(locale, 'donateTitle') }}</h1>
      </div>
      <div class="donate-intro">
        <p>{{ translate(locale, 'donateIntro') }}</p>
        <a href="https://forms.gle/DfCfZGUjFncHJdN66" target="_blank" rel="noopener noreferrer">
          {{ translate(locale, 'suggestions') }} <span aria-hidden="true">↗</span>
        </a>
      </div>
    </section>

    <section class="payment-section" aria-labelledby="payment-title">
      <div class="section-heading">
        <h2 id="payment-title">{{ translate(locale, 'payment') }}</h2>
      </div>

      <div class="payment-grid">
        <a class="payment-card" href="https://p.ecpay.com.tw/677F5BF" target="_blank" rel="noopener noreferrer">
          <div class="qr-frame">
            <img
              src="https://payment.ecpay.com.tw/Upload/QRCode/202407/QRCode_2ff3f451-50b1-4fe0-81f2-63ea255d817f.png"
              width="180"
              height="180"
              loading="lazy"
              alt="ECPay QR code"
            >
          </div>
          <div>
            <span class="payment-kicker">{{ translate(locale, 'amountCustom') }}</span>
            <h3>{{ translate(locale, 'ecpay') }}</h3>
          </div>
          <span class="payment-arrow" aria-hidden="true">↗</span>
        </a>

        <a class="payment-card" href="https://qr.opay.tw/HTXLZ" target="_blank" rel="noopener noreferrer">
          <div class="qr-frame">
            <img
              src="https://payment.opay.tw/Upload/Broadcaster/1967663/QRcode/QRCode_EA6A62EF78EB2573D2570E271C1610B7.png"
              width="180"
              height="180"
              loading="lazy"
              alt="OPay QR code"
            >
          </div>
          <div>
            <span class="payment-kicker">{{ translate(locale, 'amountCustom') }}</span>
            <h3>{{ translate(locale, 'opay') }}</h3>
          </div>
          <span class="payment-arrow" aria-hidden="true">↗</span>
        </a>
      </div>
    </section>

    <section class="support-section" aria-labelledby="support-title">
      <div class="section-heading">
        <h2 id="support-title">{{ translate(locale, 'smallSupport') }}</h2>
      </div>

      <div class="tier-grid">
        <article v-for="tier in tiers" :key="tier.label" class="tier-card">
          <button
            v-if="tier.cat"
            class="cat-peek"
            type="button"
            :aria-label="translate(locale, 'seeCat')"
            @click="catDialog?.showModal()"
          >↗</button>
          <a href="https://p.ecpay.com.tw/677F5BF" target="_blank" rel="noopener noreferrer">
            <span class="tier-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24"><path :d="tier.icon" /></svg>
            </span>
            <span class="tier-label">{{ tier.label }}</span>
            <strong>{{ tier.amount }}</strong>
          </a>
        </article>
      </div>
    </section>

    <dialog ref="catDialog" class="cat-dialog" @click.self="catDialog?.close()">
      <div class="dialog-heading">
        <strong>{{ translate(locale, 'catDialogTitle') }}</strong>
        <button type="button" :aria-label="translate(locale, 'close')" @click="catDialog?.close()">×</button>
      </div>
      <img src="https://2pick.app/storage/cat.png" alt="2Pick developer's cat" width="720" height="720">
    </dialog>
  </div>
</template>
