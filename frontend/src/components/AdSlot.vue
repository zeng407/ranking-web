<script setup lang="ts">
/**
 * One AdSense unit.
 *
 * Renders nothing at all unless the deployment configured both a publisher id and an
 * id for this slot, so an unconfigured environment has no ad markup rather than empty
 * boxes. The unit is only requested once it is close to the viewport, and its height
 * is reserved up front: an ad that grows mid-scroll pushes away the ranking the reader
 * was looking at.
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { loadAdScript, pushAdUnit } from '../ads/adsense'
import { getRuntimeConfig, type AdSlotName } from '../config/runtime'
import { translate, type Locale } from '../i18n'

const props = withDefaults(defineProps<{
  name: AdSlotName
  locale: Locale
  /**
   * The shape the position expects. Only `rectangle` and `vertical` are fixed-size
   * units; the rest follow the column they sit in.
   */
  shape?: 'horizontal' | 'rectangle' | 'vertical' | 'card'
}>(), {
  shape: 'horizontal',
})

const ads = getRuntimeConfig().ads
const slotID = computed(() => ads.slots[props.name])
const enabled = computed(() => Boolean(ads.publisherId && slotID.value))
const container = ref<HTMLElement | null>(null)
const requested = ref(false)
let observer: IntersectionObserver | null = null

function request(): void {
  if (requested.value) return
  requested.value = true
  // A blocked tag is not the reader's problem: the reserved box simply stays empty.
  loadAdScript(ads.publisherId).then(pushAdUnit).catch(() => {})
}

onMounted(() => {
  if (!enabled.value || !container.value) return
  if (typeof IntersectionObserver !== 'function') {
    request()
    return
  }
  observer = new IntersectionObserver((entries) => {
    if (!entries.some((entry) => entry.isIntersecting)) return
    observer?.disconnect()
    observer = null
    request()
  }, { rootMargin: '200px' })
  observer.observe(container.value)
})

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
})
</script>

<template>
  <aside
    v-if="enabled"
    ref="container"
    class="ad-slot"
    :class="[`ad-slot-${shape}`, { 'is-requested': requested }]"
    :aria-label="translate(locale, 'adLabel')"
  >
    <span class="ad-slot-label">{{ translate(locale, 'adLabel') }}</span>
    <ins
      v-if="requested"
      class="adsbygoogle"
      style="display: block"
      :data-ad-client="ads.publisherId"
      :data-ad-slot="slotID"
      data-ad-format="auto"
      :data-full-width-responsive="shape === 'horizontal' ? 'true' : 'false'"
    />
  </aside>
</template>
