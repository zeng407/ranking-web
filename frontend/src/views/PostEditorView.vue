<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { localizedPath, normalizeLocale, translate, type MessageKey } from '../i18n'
import {
  draftFrom,
  getEditorService,
  type AccessPolicy,
  type EditorFieldErrors,
  type EditorOutcome,
  type EditorService,
  type MyPost,
  type PostElement,
} from '../services/editor'

/**
 * One post's editor, replacing account/post/{serial}/edit and EditPost.vue.
 *
 * Two tabs, not three: the rank tab drew the same table the public rank page already
 * draws, so this links to it instead of keeping a second copy.
 *
 * ADDING MEDIA IS NOT HERE. The upload and URL-batch endpoints are still Laravel's, and
 * the pipeline behind them — seven source handlers, embed parsing, rate limits — has not
 * been ported. The page says so and links to the old editor rather than hiding a button
 * that would not work.
 */

const properties = defineProps<{ service?: EditorService }>()
const service = properties.service ?? getEditorService()

const route = useRoute()
const router = useRouter()
const locale = computed(() => normalizeLocale(route.params.locale))
const serial = computed(() => String(route.params.serial ?? ''))

type Tab = 'info' | 'elements'
const tab = ref<Tab>('info')

const post = ref<MyPost | null>(null)
const loading = ref(true)
const loadError = ref<MessageKey | ''>('')
const busy = ref(false)
const notice = ref<MessageKey | ''>('')
const generalError = ref<MessageKey | ''>('')
const fieldErrors = ref<EditorFieldErrors>({})

const form = ref({
  title: '',
  description: '',
  access_policy: 'public' as AccessPolicy,
  password: '',
})
const tags = ref<string[]>([])
const tagDraft = ref('')

const deleting = ref(false)
const deletePassword = ref('')
const deleteNeedsPassword = ref(false)

const elements = ref<PostElement[]>([])
const elementTotal = ref(0)
const elementPage = ref(1)
const elementsLoading = ref(false)
const elementSearch = ref('')
const elementSort = ref<'id' | 'title'>('id')
const editingElement = ref<number | null>(null)
const confirmingElement = ref<number | null>(null)
const elementDraft = ref({ title: '', start: '', end: '' })

const uploadInput = ref<HTMLInputElement | null>(null)
const dragging = ref(false)
const uploading = ref(false)
const uploadDone = ref(0)
const uploadTotal = ref(0)
const uploadAdded = ref(0)
/** One line per file that did not make it, so a failed batch says which and why. */
const uploadFailures = ref<{ name: string; reason: string }[]>([])

const ELEMENTS_PER_PAGE = 24
const limits = { title: 50, description: 300, tag: 15, tags: 5, elementTitle: 100 }

const elementLastPage = computed(() =>
  Math.max(1, Math.ceil(elementTotal.value / ELEMENTS_PER_PAGE)))

const policyLabels: Record<AccessPolicy, MessageKey> = {
  public: 'policyPublic',
  private: 'policyPrivate',
  password: 'policyPassword',
}

const validationMessages: Record<string, MessageKey> = {
  required: 'editorErrorRequired',
  too_long: 'editorErrorTooLong',
  too_many: 'editorErrorTooMany',
  invalid_policy: 'editorErrorInvalidPolicy',
  incorrect: 'editorErrorIncorrect',
  invalid_range: 'editorErrorInvalidRange',
}

/**
 * The URL box's own errors.
 *
 * `too_many` is the code for both the five-tag limit and the hundred-URL one, and "數量超過上限"
 * next to a textarea does not say which limit or what it is. This field gets the sentence
 * that names the number.
 */
function urlsError(): string {
  if (fieldErrors.value.urls?.[0] === 'too_many') {
    return translate(locale.value, 'editorErrorTooManyURLs')
  }
  return firstError('urls')
}

function firstError(field: string): string {
  const code = fieldErrors.value[field]?.[0]
  if (!code) return ''
  const key = validationMessages[code]
  return key ? translate(locale.value, key) : code
}

/** Where the game and the public rank live, both already in this app. */
const playPath = computed(() => localizedPath(`/g/${serial.value}`, locale.value))
const rankPath = computed(() => localizedPath(`/r/${serial.value}`, locale.value))

onMounted(load)
watch(serial, load)

async function load(): Promise<void> {
  if (!serial.value) return
  loading.value = true
  const outcome = await service.post(serial.value)
  loading.value = false

  if (outcome.ok) {
    adopt(outcome.value)
    loadError.value = ''
    void loadElements(1)
    return
  }
  if (outcome.kind === 'signed-out') {
    await sendToLogin()
    return
  }
  loadError.value = outcome.kind === 'not-found' ? 'editorNotFound' : 'editorLoadFailed'
}

function adopt(next: MyPost): void {
  post.value = next
  form.value = {
    title: next.title,
    description: next.description,
    access_policy: next.access_policy,
    // Never prefilled: the server does not serve the password, and a placeholder would
    // suggest it had.
    password: '',
  }
  tags.value = [...next.tags]
}

async function sendToLogin(): Promise<void> {
  await router.replace({
    path: localizedPath('/login', locale.value),
    query: { redirect: route.fullPath },
  })
}

async function run<T>(
  action: () => Promise<EditorOutcome<T>>,
  onSuccess: (value: T) => void,
  successNotice: MessageKey,
): Promise<void> {
  if (busy.value) return
  busy.value = true
  notice.value = ''
  generalError.value = ''
  fieldErrors.value = {}

  try {
    const outcome = await action()
    if (outcome.ok) {
      onSuccess(outcome.value)
      notice.value = successNotice
      return
    }
    if (outcome.kind === 'validation') {
      fieldErrors.value = outcome.errors
      if (Object.keys(outcome.errors).length === 0) generalError.value = 'editorActionFailed'
      return
    }
    if (outcome.kind === 'signed-out') {
      await sendToLogin()
      return
    }
    generalError.value = outcome.kind === 'not-found' ? 'editorNotFound' : 'editorActionFailed'
  } finally {
    busy.value = false
  }
}

function saveInfo(): Promise<void> {
  return run(
    () => service.updatePost(serial.value, draftFrom(form.value, tags.value)),
    (updated) => adopt(updated),
    'editorSaved',
  )
}

function addTag(): void {
  const tag = tagDraft.value.trim()
  tagDraft.value = ''
  if (!tag || tags.value.length >= limits.tags) return
  // Silently ignored rather than shown as an error: adding a tag twice is a slip, and
  // the server would refuse the whole save for it.
  if (tags.value.includes(tag)) return
  tags.value.push(tag)
}

function removeTag(tag: string): void {
  tags.value = tags.value.filter((existing) => existing !== tag)
}

function startDelete(): void {
  deleting.value = true
  deletePassword.value = ''
  deleteNeedsPassword.value = false
  fieldErrors.value = {}
  generalError.value = ''
}

/**
 * Deletes the post, asking for the account password only once the server says it needs
 * one.
 *
 * The alternative — reading has_password from the account endpoint first — would put a
 * second source of truth in front of a rule the server already enforces, and would spend
 * a request on every author whether or not they have a password. The 11,040 accounts that
 * signed in through Google have none and are never asked.
 */
async function confirmDelete(): Promise<void> {
  if (busy.value) return
  busy.value = true
  generalError.value = ''
  fieldErrors.value = {}

  try {
    const outcome = await service.deletePost(
      serial.value, deleteNeedsPassword.value ? deletePassword.value : undefined)
    if (outcome.ok) {
      await router.replace(localizedPath('/account/posts', locale.value))
      return
    }
    if (outcome.kind === 'validation') {
      fieldErrors.value = outcome.errors
      if (outcome.errors.password) deleteNeedsPassword.value = true
      return
    }
    if (outcome.kind === 'signed-out') {
      await sendToLogin()
      return
    }
    generalError.value = 'editorActionFailed'
  } finally {
    busy.value = false
  }
}

async function loadElements(next: number): Promise<void> {
  if (!serial.value) return
  elementsLoading.value = true
  const outcome = await service.elements(serial.value, {
    page: next,
    per_page: ELEMENTS_PER_PAGE,
    title: elementSearch.value.trim() || undefined,
    sort_by: elementSort.value,
    sort_dir: 'desc',
  })
  elementsLoading.value = false

  if (outcome.ok) {
    elements.value = outcome.value.elements
    elementTotal.value = outcome.value.total
    elementPage.value = outcome.value.page
    return
  }
  if (outcome.kind === 'signed-out') await sendToLogin()
}

/**
 * Uploads the chosen files, one request each and one at a time.
 *
 * SEQUENTIALLY, NOT IN PARALLEL. The endpoint takes one file per request and the account
 * has a budget of 30 MiB or 50 files a minute; firing thirty at once would spend it in a
 * burst and turn the tail of the batch into rate-limit refusals. One at a time also means
 * the count below is a real progress report rather than a guess.
 */
async function uploadFiles(files: File[]): Promise<void> {
  if (uploading.value || files.length === 0) return
  uploading.value = true
  uploadDone.value = 0
  uploadTotal.value = files.length
  uploadAdded.value = 0
  uploadFailures.value = []
  notice.value = ''
  generalError.value = ''

  try {
    for (const file of files) {
      const outcome = await service.uploadElement(serial.value, file)
      uploadDone.value += 1

      if (outcome.ok) {
        uploadAdded.value += 1
        continue
      }
      if (outcome.kind === 'signed-out') {
        await sendToLogin()
        return
      }
      uploadFailures.value.push({ name: file.name, reason: uploadReason(outcome) })
      // A full post or an exhausted budget will refuse everything after this too, so the
      // rest of the batch is abandoned rather than sent to be told the same thing.
      if (outcome.kind === 'validation' &&
          (outcome.errors.file?.[0] === 'post_full' || outcome.errors.file?.[0] === 'rate_limited')) {
        break
      }
    }
  } finally {
    uploading.value = false
  }

  if (uploadAdded.value > 0) await loadElements(1)
}

/** The message for one failed file, from the code the server answered with. */
function uploadReason(outcome: EditorOutcome<PostElement>): string {
  if (outcome.ok) return ''
  if (outcome.kind !== 'validation') return translate(locale.value, 'editorActionFailed')
  const code = outcome.errors.file?.[0]
  const messages: Record<string, MessageKey> = {
    unsupported_media: 'editorErrorUnsupportedMedia',
    too_large: 'editorErrorTooLargeFile',
    post_full: 'editorErrorPostFull',
    rate_limited: 'editorErrorRateLimited',
    required: 'editorErrorRequired',
  }
  const key = code ? messages[code] : undefined
  return key ? translate(locale.value, key) : (code ?? translate(locale.value, 'editorActionFailed'))
}

const urlDraft = ref('')
const addingURLs = ref(false)
const urlsAdded = ref(0)

/**
 * Adds media from the pasted list.
 *
 * A batch normally succeeds in part, so the failures are rendered the same way the
 * uploader's are — named, with a reason — rather than collapsing the whole thing into one
 * message.
 */
async function submitURLs(): Promise<void> {
  const list = urlDraft.value.trim()
  if (addingURLs.value || !list) return
  addingURLs.value = true
  notice.value = ''
  generalError.value = ''
  fieldErrors.value = {}
  uploadFailures.value = []
  urlsAdded.value = 0

  try {
    const outcome = await service.addElementsByURL(serial.value, list)
    if (outcome.ok) {
      urlsAdded.value = outcome.value.added.length
      uploadFailures.value = outcome.value.failed.map((failure) => ({
        name: failure.url, reason: urlFailureReason(failure.reason),
      }))
      if (urlsAdded.value > 0) {
        // Only what was accepted is cleared: leaving the failures in the box lets the
        // author fix them without retyping the ones that worked.
        urlDraft.value = uploadFailures.value.map((failure) => failure.name).join('\n')
        await loadElements(1)
      }
      return
    }
    if (outcome.kind === 'validation') {
      fieldErrors.value = outcome.errors
      return
    }
    if (outcome.kind === 'signed-out') {
      await sendToLogin()
      return
    }
    generalError.value = 'editorActionFailed'
  } finally {
    addingURLs.value = false
  }
}

function urlFailureReason(reason: string): string {
  const messages: Record<string, MessageKey> = {
    unrecognised: 'editorErrorUnrecognised',
    unavailable: 'editorErrorUnavailable',
    too_large: 'editorErrorTooLargeFile',
    post_full: 'editorErrorPostFull',
  }
  const key = messages[reason]
  return key ? translate(locale.value, key) : reason
}

function chooseFiles(): void {
  uploadInput.value?.click()
}

async function onFilesChosen(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  // Cleared so choosing the same file again fires change — a retry after a failure is
  // otherwise silently ignored.
  input.value = ''
  await uploadFiles(files)
}

async function onDrop(event: DragEvent): Promise<void> {
  dragging.value = false
  await uploadFiles(Array.from(event.dataTransfer?.files ?? []))
}

function startEditingElement(element: PostElement): void {
  editingElement.value = element.id
  elementDraft.value = {
    title: element.title,
    start: element.video_start_second === null ? '' : String(element.video_start_second),
    end: element.video_end_second === null ? '' : String(element.video_end_second),
  }
  fieldErrors.value = {}
}

function cancelEditingElement(): void {
  editingElement.value = null
  fieldErrors.value = {}
}

function saveElement(element: PostElement): Promise<void> {
  const edit: { title?: string; video_start_second?: number; video_end_second?: number } = {}
  if (elementDraft.value.title !== element.title) edit.title = elementDraft.value.title
  // An empty box means "leave it", not "set it to zero" — the server tells those apart
  // and so must this.
  const start = Number(elementDraft.value.start)
  const end = Number(elementDraft.value.end)
  if (elementDraft.value.start !== '' && Number.isFinite(start)) edit.video_start_second = start
  if (elementDraft.value.end !== '' && Number.isFinite(end)) edit.video_end_second = end

  return run(
    () => service.updateElement(element.id, edit),
    (updated) => {
      elements.value = elements.value.map((existing) =>
        existing.id === updated.id ? updated : existing)
      editingElement.value = null
    },
    'editorSaved',
  )
}

/**
 * Deleting an element confirms inline, the same way deleting the post does.
 *
 * Not window.confirm: it blocks the page, cannot be styled to match anything, and on a
 * phone it is a system sheet that looks like it came from somewhere else.
 */
async function deleteElement(element: PostElement): Promise<void> {
  if (busy.value) return
  await run(
    () => service.deleteElement(element.id),
    () => {
      elements.value = elements.value.filter((existing) => existing.id !== element.id)
      elementTotal.value = Math.max(0, elementTotal.value - 1)
      confirmingElement.value = null
    },
    'editorSaved',
  )
}

function text(key: MessageKey | ''): string {
  return key ? translate(locale.value, key) : ''
}

function thumbnailFor(element: PostElement): string {
  return element.lowthumb_url || element.mediumthumb_url || element.thumb_url || element.source_url
}
</script>

<template>
  <main class="editor">
    <p v-if="loading" class="editor__status">{{ translate(locale, 'roomLoading') }}</p>
    <p v-else-if="loadError" class="editor__error">{{ text(loadError) }}</p>

    <template v-else-if="post">
      <header class="editor__head">
        <h1>{{ post.title }}</h1>
        <nav class="editor__links">
          <RouterLink :to="playPath">{{ translate(locale, 'editorStart') }}</RouterLink>
          <RouterLink :to="rankPath">{{ translate(locale, 'editorRank') }}</RouterLink>
        </nav>
      </header>

      <div class="editor__tabs" role="tablist">
        <button
          type="button"
          role="tab"
          :aria-selected="tab === 'info'"
          :class="{ 'editor__tab--active': tab === 'info' }"
          @click="tab = 'info'"
        >
          {{ translate(locale, 'editorTabInfo') }}
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="tab === 'elements'"
          :class="{ 'editor__tab--active': tab === 'elements' }"
          @click="tab = 'elements'"
        >
          {{ translate(locale, 'editorTabElements') }} ({{ elementTotal }})
        </button>
      </div>

      <p v-if="notice" class="editor__notice" role="status">{{ text(notice) }}</p>
      <p v-if="generalError" class="editor__error" role="alert">{{ text(generalError) }}</p>

      <section v-if="tab === 'info'" class="editor__panel">
        <form @submit.prevent="saveInfo">
          <label for="editor-title">{{ translate(locale, 'myPostsTitle') }}</label>
          <input id="editor-title" v-model="form.title" type="text" :maxlength="limits.title" :disabled="busy" />
          <p v-if="firstError('title')" class="editor__field-error">{{ firstError('title') }}</p>

          <label for="editor-description">{{ translate(locale, 'myPostsDescription') }}</label>
          <textarea
            id="editor-description"
            v-model="form.description"
            rows="4"
            :maxlength="limits.description"
            :disabled="busy"
          ></textarea>
          <p v-if="firstError('description')" class="editor__field-error">{{ firstError('description') }}</p>

          <fieldset class="editor__policies">
            <legend>{{ translate(locale, 'myPostsPublishment') }}</legend>
            <label v-for="(label, value) in policyLabels" :key="value">
              <input v-model="form.access_policy" type="radio" :value="value" :disabled="busy" />
              {{ translate(locale, label) }}
            </label>
          </fieldset>

          <template v-if="form.access_policy === 'password'">
            <label for="editor-password">
              {{ post.has_password ? translate(locale, 'editorNewPassword') : translate(locale, 'editorPassword') }}
            </label>
            <input id="editor-password" v-model="form.password" type="text" maxlength="255" :disabled="busy" />
            <p v-if="post.has_password" class="editor__hint">
              {{ translate(locale, 'editorPasswordKeepHint') }}
            </p>
            <p v-if="firstError('password')" class="editor__field-error">{{ firstError('password') }}</p>
          </template>

          <div class="editor__tags">
            <span class="editor__label">{{ translate(locale, 'editorTags') }}</span>
            <ul v-if="tags.length" class="editor__tag-list">
              <li v-for="tag in tags" :key="tag">
                #{{ tag }}
                <button
                  type="button"
                  :aria-label="translate(locale, 'editorTagRemove')"
                  :disabled="busy"
                  @click="removeTag(tag)"
                >
                  ×
                </button>
              </li>
            </ul>
            <p v-else class="editor__hint">{{ translate(locale, 'editorNoTags') }}</p>
            <input
              v-if="tags.length < limits.tags"
              v-model="tagDraft"
              type="text"
              :maxlength="limits.tag"
              :disabled="busy"
              :placeholder="translate(locale, 'editorTagsHint')"
              @keydown.enter.prevent="addTag"
            />
            <p v-if="firstError('tags')" class="editor__field-error">{{ firstError('tags') }}</p>
          </div>

          <button type="submit" :disabled="busy">{{ translate(locale, 'editorSave') }}</button>
        </form>

        <dl class="editor__facts">
          <div>
            <dt>{{ translate(locale, 'editorCreatedAt') }}</dt>
            <dd>{{ post.created_at?.slice(0, 10) ?? '' }}</dd>
          </div>
          <div>
            <dt>{{ translate(locale, 'myPostsPlayedAll') }}</dt>
            <dd>{{ post.play_count }}</dd>
          </div>
          <div>
            <dt>{{ translate(locale, 'myPostsPlayedThisWeek') }}</dt>
            <dd>{{ post.this_week_play_count }}</dd>
          </div>
          <div>
            <dt>{{ translate(locale, 'myPostsPlayedLastWeek') }}</dt>
            <dd>{{ post.last_week_play_count }}</dd>
          </div>
        </dl>

        <div class="editor__danger">
          <button v-if="!deleting" type="button" :disabled="busy" @click="startDelete">
            {{ translate(locale, 'editorDelete') }}
          </button>
          <div v-else class="editor__confirm">
            <p>{{ translate(locale, 'editorDeleteConfirm').replace('{title}', post.title) }}</p>
            <template v-if="deleteNeedsPassword">
              <label for="editor-delete-password">
                {{ translate(locale, 'editorDeleteAccountPassword') }}
              </label>
              <input
                id="editor-delete-password"
                v-model="deletePassword"
                type="password"
                autocomplete="current-password"
                :disabled="busy"
              />
            </template>
            <p v-if="firstError('password')" class="editor__field-error">{{ firstError('password') }}</p>
            <div class="editor__actions">
              <button type="button" :disabled="busy" @click="confirmDelete">
                {{ translate(locale, 'editorDelete') }}
              </button>
              <button type="button" :disabled="busy" @click="deleting = false">
                {{ translate(locale, 'editorCancel') }}
              </button>
            </div>
          </div>
        </div>
      </section>

      <section v-else class="editor__panel">
        <div
          class="editor__dropzone"
          :class="{ 'editor__dropzone--over': dragging }"
          role="button"
          tabindex="0"
          @click="chooseFiles"
          @keydown.enter.prevent="chooseFiles"
          @keydown.space.prevent="chooseFiles"
          @dragover.prevent="dragging = true"
          @dragleave.prevent="dragging = false"
          @drop.prevent="onDrop"
        >
          <strong>{{ translate(locale, 'editorUpload') }}</strong>
          <span class="editor__hint">{{ translate(locale, 'editorUploadHint') }}</span>
          <input
            ref="uploadInput"
            type="file"
            multiple
            accept="image/png,image/jpeg,image/gif,image/bmp,image/webp,video/mp4,video/x-msvideo,video/mpeg"
            hidden
            @change="onFilesChosen"
          />
        </div>

        <p v-if="uploading" class="editor__status" role="status">
          {{ translate(locale, 'editorUploading')
            .replace('{done}', String(uploadDone)).replace('{total}', String(uploadTotal)) }}
        </p>
        <p v-else-if="uploadAdded > 0" class="editor__notice" role="status">
          {{ translate(locale, 'editorUploadDone').replace('{count}', String(uploadAdded)) }}
        </p>

        <div v-if="uploadFailures.length" class="editor__upload-failures">
          <p>
            {{ translate(locale, 'editorUploadSomeFailed')
              .replace('{count}', String(uploadFailures.length)) }}
          </p>
          <ul>
            <li v-for="failure in uploadFailures" :key="failure.name">
              {{ failure.name }} — {{ failure.reason }}
            </li>
          </ul>
        </div>

        <form class="editor__urls" @submit.prevent="submitURLs">
          <label for="editor-urls"><strong>{{ translate(locale, 'editorAddURLs') }}</strong></label>
          <p class="editor__hint">{{ translate(locale, 'editorAddURLsHint') }}</p>
          <textarea
            id="editor-urls"
            v-model="urlDraft"
            rows="3"
            :disabled="addingURLs"
          ></textarea>
          <p v-if="urlsError()" class="editor__field-error">{{ urlsError() }}</p>
          <button type="submit" :disabled="addingURLs || !urlDraft.trim()">
            {{ translate(locale, 'editorAddURLsSubmit') }}
          </button>
          <p v-if="urlsAdded > 0" class="editor__notice" role="status">
            {{ translate(locale, 'editorAddURLsDone').replace('{count}', String(urlsAdded)) }}
          </p>
        </form>

        <div class="editor__element-controls">
          <input
            v-model="elementSearch"
            type="search"
            :placeholder="translate(locale, 'editorElementSearch')"
            @keydown.enter.prevent="loadElements(1)"
          />
          <select v-model="elementSort" @change="loadElements(1)">
            <option value="id">{{ translate(locale, 'editorSortByCreated') }}</option>
            <option value="title">{{ translate(locale, 'editorSortByTitle') }}</option>
          </select>
        </div>

        <p v-if="elementsLoading" class="editor__status">{{ translate(locale, 'roomLoading') }}</p>
        <p v-else-if="elements.length === 0" class="editor__status">
          {{ translate(locale, 'editorNoElements') }}
        </p>

        <ul v-else class="editor__elements">
          <li v-for="element in elements" :key="element.id" class="editor__element">
            <img :src="thumbnailFor(element)" :alt="element.title" loading="lazy" width="120" height="120" />

            <div v-if="editingElement === element.id" class="editor__element-form">
              <label :for="`element-title-${element.id}`">{{ translate(locale, 'editorElementTitle') }}</label>
              <input
                :id="`element-title-${element.id}`"
                v-model="elementDraft.title"
                type="text"
                :maxlength="limits.elementTitle"
                :disabled="busy"
              />
              <template v-if="element.type === 'video'">
                <label :for="`element-start-${element.id}`">
                  {{ translate(locale, 'editorElementVideoStart') }}
                </label>
                <input
                  :id="`element-start-${element.id}`"
                  v-model="elementDraft.start"
                  type="number"
                  min="0"
                  :disabled="busy"
                />
                <label :for="`element-end-${element.id}`">
                  {{ translate(locale, 'editorElementVideoEnd') }}
                </label>
                <input
                  :id="`element-end-${element.id}`"
                  v-model="elementDraft.end"
                  type="number"
                  min="0"
                  :disabled="busy"
                />
              </template>
              <p v-if="firstError('title')" class="editor__field-error">{{ firstError('title') }}</p>
              <p v-if="firstError('video_end_second')" class="editor__field-error">
                {{ firstError('video_end_second') }}
              </p>
              <p v-if="firstError('video_start_second')" class="editor__field-error">
                {{ firstError('video_start_second') }}
              </p>
              <div class="editor__actions">
                <button type="button" :disabled="busy" @click="saveElement(element)">
                  {{ translate(locale, 'editorSave') }}
                </button>
                <button type="button" :disabled="busy" @click="cancelEditingElement">
                  {{ translate(locale, 'editorCancel') }}
                </button>
              </div>
            </div>

            <div v-else class="editor__element-body">
              <p class="editor__element-title">{{ element.title }}</p>
              <p class="editor__element-meta">
                <span>{{ element.type }}</span>
                <span v-if="element.rank">
                  {{ translate(locale, 'editorElementRank') }} {{ element.rank.rank }}
                </span>
                <span v-if="element.video_start_second !== null">
                  {{ translate(locale, 'editorElementVideoStart') }} {{ element.video_start_second }}
                </span>
              </p>
              <div v-if="confirmingElement === element.id" class="editor__actions">
                <span class="editor__hint">{{ translate(locale, 'editorElementDeleteConfirm') }}</span>
                <button type="button" :disabled="busy" @click="deleteElement(element)">
                  {{ translate(locale, 'editorElementDelete') }}
                </button>
                <button type="button" :disabled="busy" @click="confirmingElement = null">
                  {{ translate(locale, 'editorCancel') }}
                </button>
              </div>
              <div v-else class="editor__actions">
                <button type="button" :disabled="busy" @click="startEditingElement(element)">
                  {{ translate(locale, 'myPostsEdit') }}
                </button>
                <button type="button" :disabled="busy" @click="confirmingElement = element.id">
                  {{ translate(locale, 'editorElementDelete') }}
                </button>
              </div>
            </div>
          </li>
        </ul>

        <nav v-if="elementLastPage > 1" class="editor__pages">
          <button type="button" :disabled="elementPage <= 1" @click="loadElements(elementPage - 1)">
            {{ translate(locale, 'myPostsPrevious') }}
          </button>
          <span>{{ elementPage }} / {{ elementLastPage }}</span>
          <button type="button" :disabled="elementPage >= elementLastPage" @click="loadElements(elementPage + 1)">
            {{ translate(locale, 'myPostsNext') }}
          </button>
        </nav>
      </section>
    </template>
  </main>
</template>

<style scoped>
.editor {
  max-width: 56rem;
  margin: 0 auto;
  padding: 1.5rem 1rem 3rem;
}

.editor__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.editor__links {
  display: flex;
  gap: 1rem;
}

.editor__tabs {
  display: flex;
  gap: 0.5rem;
  margin: 1rem 0;
  border-bottom: 1px solid var(--border, #d9d9d9);
}

.editor__tabs button {
  border: 0;
  background: none;
  padding: 0.5rem 0.75rem;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  color: inherit;
}

.editor__tab--active {
  border-bottom-color: currentColor;
  font-weight: 600;
}

.editor__panel form {
  display: grid;
  gap: 0.4rem;
  justify-items: start;
}

.editor__panel input[type='text'],
.editor__panel textarea {
  width: min(32rem, 100%);
  padding: 0.4rem 0.6rem;
}

.editor__policies {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
  border: 0;
  padding: 0;
  margin: 0.5rem 0;
}

.editor__tags {
  margin: 0.75rem 0;
  display: grid;
  gap: 0.4rem;
  justify-items: start;
}

.editor__label {
  font-weight: 600;
}

.editor__tag-list {
  list-style: none;
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  padding: 0;
  margin: 0;
}

.editor__tag-list li {
  border: 1px solid var(--border, #d9d9d9);
  border-radius: 999px;
  padding: 0.15rem 0.6rem;
  font-size: 0.85rem;
}

.editor__tag-list button {
  border: 0;
  background: none;
  cursor: pointer;
  color: inherit;
}

.editor__facts {
  display: flex;
  gap: 1.5rem;
  flex-wrap: wrap;
  margin: 1.5rem 0;
  padding-top: 1rem;
  border-top: 1px solid var(--border, #d9d9d9);
}

.editor__facts dt {
  font-size: 0.8rem;
  opacity: 0.7;
}

.editor__danger {
  margin-top: 2rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border, #d9d9d9);
}

.editor__confirm {
  display: grid;
  gap: 0.5rem;
  justify-items: start;
}

.editor__dropzone {
  display: grid;
  gap: 0.35rem;
  justify-items: center;
  text-align: center;
  padding: 1.5rem 1rem;
  border: 2px dashed var(--border, #d9d9d9);
  border-radius: 0.5rem;
  cursor: pointer;
}

.editor__dropzone--over {
  border-color: currentColor;
  opacity: 0.85;
}

.editor__upload-failures {
  margin: 0.75rem 0;
  font-size: 0.875rem;
  color: var(--danger, #c0392b);
}

.editor__upload-failures ul {
  margin: 0.25rem 0 0;
  padding-left: 1.25rem;
}

.editor__urls {
  display: grid;
  gap: 0.4rem;
  justify-items: start;
  margin: 1.25rem 0;
}

.editor__urls textarea {
  width: 100%;
  padding: 0.5rem 0.6rem;
  font-family: inherit;
}

.editor__element-controls {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  margin: 1rem 0;
}

.editor__elements {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 0.75rem;
}

.editor__element {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
  padding: 0.75rem;
  border: 1px solid var(--border, #d9d9d9);
  border-radius: 0.5rem;
}

.editor__element img {
  object-fit: cover;
  border-radius: 0.35rem;
  flex: none;
}

.editor__element-body,
.editor__element-form {
  display: grid;
  gap: 0.35rem;
  justify-items: start;
  min-width: 0;
}

.editor__element-title {
  font-weight: 600;
  margin: 0;
  overflow-wrap: anywhere;
}

.editor__element-meta {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
  font-size: 0.85rem;
  opacity: 0.75;
  margin: 0;
}

.editor__actions {
  display: flex;
  gap: 0.5rem;
}

.editor__pages {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  margin-top: 1.5rem;
}

.editor__hint {
  font-size: 0.85rem;
  opacity: 0.75;
  margin: 0;
}

.editor__field-error,
.editor__error {
  color: var(--danger, #c0392b);
  font-size: 0.875rem;
  margin: 0;
}

.editor__notice {
  color: var(--success, #1e7e34);
  font-size: 0.875rem;
}
</style>
