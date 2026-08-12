<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { translate, type Locale } from '../i18n'
import {
  createPersonalRankingExport,
  disposePersonalRankingExport,
  downloadPersonalRankingExport,
  type PersonalRankingExport,
  type RankingExportItem,
} from '../rank/exportRanking'

const props = defineProps<{
  open: boolean
  title: string
  items: RankingExportItem[]
  locale: Locale
}>()

const emit = defineEmits<{
  close: []
}>()

const dialog = ref<HTMLDialogElement | null>(null)
const result = ref<PersonalRankingExport | null>(null)
const generating = ref(false)
const generationFailed = ref(false)
const copyState = ref<'idle' | 'copied' | 'failed'>('idle')
const mobileSaveOnly = ref(false)
let generationVersion = 0
let copyResetTimer: number | undefined
let mobileMedia: MediaQueryList | undefined

function t(key: Parameters<typeof translate>[1]): string {
  return translate(props.locale, key)
}

function updateMobileSaveMode(): void {
  mobileSaveOnly.value = mobileMedia?.matches ?? window.innerWidth <= 767
}

async function generatePreview(): Promise<void> {
  const version = ++generationVersion
  releaseResult()
  generating.value = true
  generationFailed.value = false
  copyState.value = 'idle'
  try {
    const generated = await createPersonalRankingExport(props.title, props.items)
    if (version !== generationVersion || !props.open) {
      disposePersonalRankingExport(generated)
      return
    }
    result.value = generated
  } catch {
    if (version === generationVersion) generationFailed.value = true
  } finally {
    if (version === generationVersion) generating.value = false
  }
}

function releaseResult(): void {
  if (!result.value) return
  disposePersonalRankingExport(result.value)
  result.value = null
}

function close(): void {
  generationVersion += 1
  generating.value = false
  releaseResult()
  if (dialog.value?.open) dialog.value.close()
  emit('close')
}

function download(): void {
  if (!result.value || mobileSaveOnly.value) return
  downloadPersonalRankingExport(result.value)
}

async function copyText(): Promise<void> {
  if (!result.value?.text) return
  let copied = false
  try {
    if (!navigator.clipboard?.writeText) throw new Error('clipboard unavailable')
    await navigator.clipboard.writeText(result.value.text)
    copied = true
  } catch {
    const textarea = dialog.value?.querySelector<HTMLTextAreaElement>('.ranking-export-text')
    if (textarea) {
      textarea.select()
      textarea.setSelectionRange(0, textarea.value.length)
      copied = document.execCommand?.('copy') ?? false
      textarea.blur()
    }
  }
  copyState.value = copied ? 'copied' : 'failed'
  if (copyResetTimer) window.clearTimeout(copyResetTimer)
  copyResetTimer = window.setTimeout(() => { copyState.value = 'idle' }, 2_000)
}

watch(() => props.open, async (open) => {
  if (!open) {
    if (dialog.value?.open) dialog.value.close()
    generationVersion += 1
    releaseResult()
    return
  }
  if (!dialog.value?.open) dialog.value?.showModal()
  await generatePreview()
}, { immediate: true, flush: 'post' })

onMounted(() => {
  mobileMedia = window.matchMedia?.('(max-width: 767px)')
  updateMobileSaveMode()
  mobileMedia?.addEventListener?.('change', updateMobileSaveMode)
})

onBeforeUnmount(() => {
  generationVersion += 1
  mobileMedia?.removeEventListener?.('change', updateMobileSaveMode)
  if (copyResetTimer) window.clearTimeout(copyResetTimer)
  releaseResult()
})
</script>

<template>
  <dialog ref="dialog" class="ranking-export-dialog" aria-labelledby="ranking-export-title" @cancel.prevent="close">
    <section class="ranking-export-shell">
      <header>
        <div>
          <p class="eyebrow">2PICK · EXPORT</p>
          <h2 id="ranking-export-title">{{ t('rankingExportTitle') }}</h2>
        </div>
        <button class="ranking-export-close" type="button" :aria-label="t('close')" @click="close">×</button>
      </header>

      <div v-if="generating" class="ranking-export-state" role="status">
        <span aria-hidden="true"></span>
        <p>{{ t('rankingExportPreparing') }}</p>
      </div>

      <div v-else-if="generationFailed" class="ranking-export-state" role="alert">
        <p>{{ t('rankingExportGenerateError') }}</p>
        <button class="button button-primary" type="button" @click="generatePreview">{{ t('retry') }}</button>
      </div>

      <div v-else-if="result" class="ranking-export-content">
        <div class="ranking-export-image-column">
          <div class="ranking-export-image-frame">
            <img
              class="ranking-export-preview"
              :src="result.imageUrl"
              :alt="t('rankingExportPreviewAlt')"
              draggable="false"
            >
          </div>
          <p v-if="mobileSaveOnly" class="ranking-export-mobile-hint">
            {{ t('rankingExportMobileHint') }}
          </p>
          <button
            v-else
            class="button button-primary ranking-export-download"
            type="button"
            @click="download"
          >{{ t('rankingExportDownload') }}</button>
        </div>

        <div class="ranking-export-text-column">
          <label for="ranking-export-text">{{ t('rankingExportTextLabel') }}</label>
          <textarea
            id="ranking-export-text"
            class="ranking-export-text"
            :value="result.text"
            rows="10"
            readonly
          ></textarea>
          <button class="button button-quiet ranking-export-copy" type="button" @click="copyText">
            {{ copyState === 'copied' ? t('rankingExportCopied') : t('rankingExportCopy') }}
          </button>
          <p v-if="copyState === 'failed'" class="ranking-export-copy-error" role="alert">
            {{ t('rankingExportCopyError') }}
          </p>
        </div>
      </div>
    </section>
  </dialog>
</template>
