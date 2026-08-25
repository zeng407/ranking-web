<script setup lang="ts">
import { computed, nextTick, onMounted, provide, reactive, ref } from 'vue'

import { APIError } from '../lib/api'
import { translate, type Locale, type MessageKey } from '../i18n'
import {
  createCommentsService,
  type CommentItem,
  type CommentsPage,
} from '../services/comments'
import CommentNode from './CommentNode.vue'
import {
  avatarInitial,
  buildThreads,
  commentThreadKey,
  type CommentThreadContext,
  type ReplyState,
} from './commentThread'

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
const deleteDialog = ref<HTMLDialogElement | null>(null)
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
const reply = reactive<ReplyState>({ targetID: null, input: '', submitting: false, error: '' })
const deleteTarget = ref<CommentItem | null>(null)
const deleting = ref(false)
const deleteError = ref('')

const threads = computed(() => buildThreads(page.value.items))
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

// Handed to every comment in the tree, however deep, instead of being threaded through
// three levels of props and events. The locale is read through a getter so a language
// change still reaches comments that were provided this object long before.
const threadContext: CommentThreadContext = {
  get locale() { return props.locale },
  reply,
  openReply,
  cancelReply,
  submitReply,
  openReport,
  confirmDelete,
}
provide(commentThreadKey, threadContext)

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

function openReply(comment: CommentItem): void {
  reply.targetID = comment.id
  reply.input = ''
  reply.error = ''
}

function cancelReply(): void {
  if (reply.submitting) return
  reply.targetID = null
  reply.input = ''
  reply.error = ''
}

async function submitReply(): Promise<void> {
  const parentID = reply.targetID
  const content = reply.input.trim()
  if (parentID === null || !content || reply.submitting) return
  reply.submitting = true
  reply.error = ''
  try {
    await service.create(props.postSerial, {
      content,
      anonymous: anonymous.value,
      parent_id: parentID,
    }, props.locale)
    reply.targetID = null
    reply.input = ''
    // The reply belongs to a floor on this page, so the page it appears on is this one.
    await loadComments(page.value.page, false)
  } catch (error) {
    reply.error = errorMessage(error, 'commentReplyError')
  } finally {
    reply.submitting = false
  }
}

function confirmDelete(comment: CommentItem): void {
  deleteTarget.value = comment
  deleteError.value = ''
  deleteDialog.value?.showModal()
}

function closeDelete(): void {
  if (deleting.value) return
  deleteDialog.value?.close()
  deleteTarget.value = null
}

async function submitDelete(): Promise<void> {
  const target = deleteTarget.value
  if (!target || deleting.value) return
  deleting.value = true
  deleteError.value = ''
  try {
    await service.remove(props.postSerial, target.id, props.locale)
    deleting.value = false
    closeDelete()
    // A deleted floor keeps its place, so the page being read does not shift under it.
    await loadComments(page.value.page, false)
  } catch (error) {
    deleteError.value = errorMessage(error, 'commentDeleteError')
  } finally {
    deleting.value = false
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
  if (error instanceof APIError && error.code === 'invalid_parent') return t('commentReplyError')
  return t(fallback)
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
      <CommentNode v-for="thread in threads" :key="thread.comment.id" :node="thread" />
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
          <span v-else>{{ avatarInitial(page.profile.nickname) }}</span>
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

  <dialog ref="deleteDialog" class="comment-report-dialog comment-delete-dialog" @cancel.prevent="closeDelete">
    <form method="dialog" @submit.prevent="submitDelete">
      <header>
        <div>
          <p class="eyebrow">2PICK · DELETE</p>
          <h2>{{ t('commentDeleteTitle') }}</h2>
        </div>
        <button type="button" :aria-label="t('commentCancel')" @click="closeDelete">×</button>
      </header>
      <blockquote v-if="deleteTarget">{{ deleteTarget.content }}</blockquote>
      <p class="comment-delete-warning">{{ t('commentDeleteWarning') }}</p>
      <p v-if="deleteError" class="comment-error" role="alert">{{ deleteError }}</p>
      <footer>
        <button class="button button-quiet" type="button" :disabled="deleting" @click="closeDelete">{{ t('commentCancel') }}</button>
        <button class="button button-primary comment-delete-submit" type="button" :disabled="deleting" @click="submitDelete">
          {{ deleting ? t('commentDeleting') : t('commentDeleteSubmit') }}
        </button>
      </footer>
    </form>
  </dialog>
</template>
