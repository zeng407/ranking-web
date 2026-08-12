<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'

import { APIError } from '../lib/api'
import { htmlLanguage, translate, type Locale, type MessageKey } from '../i18n'
import {
  createCommentsService,
  type CommentItem,
  type CommentProfile,
  type CommentsPage,
} from '../services/comments'

const props = withDefaults(defineProps<{
  postSerial: string
  locale: Locale
  localChampions?: string[]
}>(), {
  localChampions: () => [],
})

const maxLength = 200
const service = createCommentsService()
const section = ref<HTMLElement | null>(null)
const reportDialog = ref<HTMLDialogElement | null>(null)
const page = ref<CommentsPage>({
  items: [], page: 1, per_page: 10, total: 0, total_pages: 0,
  profile: { nickname: 'Anonymous', avatar_url: null, champions: [], is_auth: false },
})
const loading = ref(true)
const loadError = ref(false)
const input = ref('')
const anonymous = ref(false)
const submitting = ref(false)
const submitError = ref('')
const reportTarget = ref<CommentItem | null>(null)
const reportReason = ref('')
const reportOther = ref('')
const reporting = ref(false)
const reportError = ref('')
const reportSuccess = ref(false)

const inputLength = computed(() => Array.from(input.value).length)
const validInput = computed(() => input.value.trim().length > 0 && inputLength.value <= maxLength)
const profileChampions = computed(() => page.value.profile.champions.length
  ? page.value.profile.champions
  : props.localChampions)
const reportValue = computed(() => reportReason.value === 'Other' ? reportOther.value.trim() : reportReason.value)
const reportReasons = computed(() => [
  { value: 'Spam', label: t('commentReasonSpam') },
  { value: 'Inappropriate', label: t('commentReasonInappropriate') },
  { value: 'Hate Speech', label: t('commentReasonHate') },
  { value: 'Harassment', label: t('commentReasonHarassment') },
  { value: 'Other', label: t('commentReasonOther') },
])

onMounted(() => { void loadComments(1, false) })

function t(key: MessageKey): string {
  return translate(props.locale, key)
}

async function loadComments(targetPage = 1, moveToComments = true): Promise<void> {
  if (loading.value && targetPage !== 1) return
  loading.value = true
  loadError.value = false
  try {
    page.value = await service.list(props.postSerial, targetPage, props.locale)
    if (moveToComments) {
      await nextTick()
      section.value?.scrollIntoView?.({ block: 'start', behavior: 'smooth' })
    }
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

async function submitComment(): Promise<void> {
  if (!validInput.value || submitting.value) return
  submitting.value = true
  submitError.value = ''
  try {
    await service.create(props.postSerial, {
      content: input.value.trim(),
      anonymous: anonymous.value,
    }, props.locale)
    input.value = ''
    await loadComments(1)
  } catch (error) {
    submitError.value = errorMessage(error, 'commentSubmitError')
  } finally {
    submitting.value = false
  }
}

function openReport(comment: CommentItem): void {
  reportTarget.value = comment
  reportReason.value = ''
  reportOther.value = ''
  reportError.value = ''
  reportSuccess.value = false
  reportDialog.value?.showModal()
}

function closeReport(): void {
  if (reporting.value) return
  reportDialog.value?.close()
  reportTarget.value = null
}

async function submitReport(): Promise<void> {
  if (!reportTarget.value || !reportValue.value || reporting.value) return
  reporting.value = true
  reportError.value = ''
  try {
    await service.report(props.postSerial, reportTarget.value.id, reportValue.value, props.locale)
    reportSuccess.value = true
    window.setTimeout(closeReport, 700)
  } catch (error) {
    reportError.value = errorMessage(error, 'commentReportError')
  } finally {
    reporting.value = false
  }
}

function errorMessage(error: unknown, fallback: MessageKey): string {
  if (error instanceof APIError && error.code === 'rate_limited') return t('commentRateLimited')
  return t(fallback)
}

function avatarInitial(profile: Pick<CommentProfile, 'nickname'> | Pick<CommentItem, 'nickname'>): string {
  return Array.from(profile.nickname.trim())[0]?.toUpperCase() || '2'
}

function relativeTime(value: string): string {
  const timestamp = Date.parse(value)
  if (!Number.isFinite(timestamp)) return value
  const seconds = Math.round((timestamp - Date.now()) / 1_000)
  const absolute = Math.abs(seconds)
  const formatter = new Intl.RelativeTimeFormat(htmlLanguage(props.locale), { numeric: 'auto' })
  if (absolute < 60) return formatter.format(seconds, 'second')
  if (absolute < 3_600) return formatter.format(Math.round(seconds / 60), 'minute')
  if (absolute < 86_400) return formatter.format(Math.round(seconds / 3_600), 'hour')
  if (absolute < 2_592_000) return formatter.format(Math.round(seconds / 86_400), 'day')
  if (absolute < 31_536_000) return formatter.format(Math.round(seconds / 2_592_000), 'month')
  return formatter.format(Math.round(seconds / 31_536_000), 'year')
}
</script>

<template>
  <section ref="section" class="comments-section" aria-labelledby="comments-title">
    <header class="comments-heading">
      <div>
        <p class="eyebrow">2PICK · COMMENTS</p>
        <h2 id="comments-title" class="comments-title">{{ t('commentTitle') }} <span>{{ page.total }}</span></h2>
      </div>
      <span v-if="loading" class="comments-loading-dot" role="status" :aria-label="t('commentLoading')"></span>
    </header>

    <div v-if="loadError && !page.items.length" class="comments-state">
      <p>{{ t('commentLoadError') }}</p>
      <button class="button button-quiet" type="button" @click="loadComments(page.page, false)">{{ t('retry') }}</button>
    </div>
    <p v-else-if="!loading && !page.items.length" class="comments-state">{{ t('commentEmpty') }}</p>
    <ol v-else class="comments-list" :aria-busy="loading">
      <li v-for="comment in page.items" :key="comment.id" class="comment-card">
        <div class="comment-avatar" aria-hidden="true">
          <img v-if="comment.avatar_url" :src="comment.avatar_url" alt="" loading="lazy">
          <span v-else>{{ avatarInitial(comment) }}</span>
        </div>
        <article>
          <header>
            <div>
              <strong class="comment-author">{{ comment.nickname }}</strong>
              <time :datetime="comment.created_at" :title="comment.created_at">{{ relativeTime(comment.created_at) }}</time>
            </div>
            <button class="comment-report-button" type="button" :aria-label="t('commentReport')" :title="t('commentReport')" @click="openReport(comment)">
              <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/></svg>
            </button>
          </header>
          <div v-if="comment.champions.length" class="comment-champions">
            <span v-for="champion in comment.champions" :key="champion" class="comment-champion">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 4h8v4a4 4 0 0 1-8 0V4ZM6 6H3v1a4 4 0 0 0 5 4M18 6h3v1a4 4 0 0 1-5 4M12 12v4M8 20h8M9 16h6"/></svg>
              {{ champion }}
            </span>
          </div>
          <p class="comment-content">{{ comment.content }}</p>
        </article>
      </li>
    </ol>

    <nav v-if="page.total_pages > 1" class="comment-pagination" :aria-label="t('commentTitle')">
      <button type="button" :disabled="loading || page.page <= 1" @click="loadComments(page.page - 1)">{{ t('previousPage') }}</button>
      <span>{{ page.page }} / {{ page.total_pages }}</span>
      <button type="button" :disabled="loading || page.page >= page.total_pages" @click="loadComments(page.page + 1)">{{ t('nextPage') }}</button>
    </nav>

    <form class="comment-composer" @submit.prevent="submitComment">
      <div class="comment-profile">
        <div class="comment-avatar" aria-hidden="true">
          <img v-if="page.profile.avatar_url" :src="page.profile.avatar_url" alt="">
          <span v-else>{{ avatarInitial(page.profile) }}</span>
        </div>
        <div>
          <small>{{ t('commentNickname') }}</small>
          <strong>{{ anonymous ? '****' : page.profile.nickname }}</strong>
        </div>
        <label v-if="page.profile.is_auth" class="comment-anonymous-toggle">
          <input v-model="anonymous" type="checkbox">
          <span>{{ t('commentAnonymous') }}</span>
        </label>
      </div>
      <div v-if="profileChampions.length" class="comment-profile-result">
        <span>{{ t('commentVoteResult') }}</span>
        <strong v-for="champion in profileChampions" :key="champion">{{ champion }}</strong>
      </div>
      <label class="sr-only" for="comment-input">{{ t('commentLeave') }}</label>
      <textarea
        id="comment-input"
        v-model="input"
        class="comment-input"
        rows="4"
        :maxlength="maxLength"
        :placeholder="t('commentLeave')"
      ></textarea>
      <div class="comment-composer-footer">
        <span :class="{ invalid: inputLength > maxLength }">{{ inputLength }} / {{ maxLength }}</span>
        <button class="button button-primary comment-submit" type="button" :disabled="!validInput || submitting" @click="submitComment">
          {{ submitting ? t('commentSubmitting') : t('commentSubmit') }}
        </button>
      </div>
      <p v-if="submitError" class="comment-error" role="alert">{{ submitError }}</p>
    </form>
  </section>

  <dialog ref="reportDialog" class="comment-report-dialog" @cancel.prevent="closeReport">
    <form method="dialog" @submit.prevent="submitReport">
      <header>
        <div>
          <p class="eyebrow">2PICK · REPORT</p>
          <h2>{{ t('commentReportTitle') }}</h2>
        </div>
        <button type="button" :aria-label="t('commentCancel')" @click="closeReport">×</button>
      </header>
      <blockquote v-if="reportTarget">{{ reportTarget.content }}</blockquote>
      <label>
        <span>{{ t('commentReportReason') }}</span>
        <select v-model="reportReason" class="comment-report-reason">
          <option value="" disabled>{{ t('commentReportChoose') }}</option>
          <option v-for="reason in reportReasons" :key="reason.value" :value="reason.value">{{ reason.label }}</option>
        </select>
      </label>
      <label v-if="reportReason === 'Other'">
        <span>{{ t('commentReportOther') }}</span>
        <input v-model="reportOther" class="comment-report-other" type="text" maxlength="200">
      </label>
      <p v-if="reportError" class="comment-error" role="alert">{{ reportError }}</p>
      <p v-if="reportSuccess" class="comment-success" role="status">{{ t('commentReported') }}</p>
      <footer>
        <button class="button button-quiet" type="button" :disabled="reporting" @click="closeReport">{{ t('commentCancel') }}</button>
        <button class="button button-primary comment-report-submit" type="button" :disabled="!reportValue || reporting || reportSuccess" @click="submitReport">
          {{ reporting ? t('commentReporting') : t('commentReportSubmit') }}
        </button>
      </footer>
    </form>
  </dialog>
</template>
