<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { localizedPath, normalizeLocale, translate, type MessageKey } from '../i18n'
import {
  draftFrom,
  getEditorService,
  type AccessPolicy,
  type EditorFieldErrors,
  type EditorService,
  type MyPost,
} from '../services/editor'

/**
 * The author's own posts, replacing the account/post Blade page and IndexPost.vue.
 *
 * The create form is inline rather than a modal: it has four fields, and a modal on this
 * page existed because Bootstrap was already there, not because anything needed one.
 */

const properties = defineProps<{ service?: EditorService }>()
const service = properties.service ?? getEditorService()

const route = useRoute()
const router = useRouter()
const locale = computed(() => normalizeLocale(route.params.locale))

const posts = ref<MyPost[]>([])
const total = ref(0)
const page = ref(1)
const perPage = ref(15)
const loading = ref(true)
const loadFailed = ref(false)

const creating = ref(false)
const busy = ref(false)
const fieldErrors = ref<EditorFieldErrors>({})
const generalError = ref<MessageKey | ''>('')

const form = ref({
  title: '',
  description: '',
  access_policy: 'public' as AccessPolicy,
  password: '',
})

const limits = { title: 50, description: 300 }
const lastPage = computed(() => Math.max(1, Math.ceil(total.value / Math.max(perPage.value, 1))))

const validationMessages: Record<string, MessageKey> = {
  required: 'editorErrorRequired',
  too_long: 'editorErrorTooLong',
  too_many: 'editorErrorTooMany',
  invalid_policy: 'editorErrorInvalidPolicy',
}

function firstError(field: string): string {
  const code = fieldErrors.value[field]?.[0]
  if (!code) return ''
  const key = validationMessages[code]
  return key ? translate(locale.value, key) : code
}

const policyLabels: Record<AccessPolicy, MessageKey> = {
  public: 'policyPublic',
  private: 'policyPrivate',
  password: 'policyPassword',
}

onMounted(() => load(1))

async function load(next: number): Promise<void> {
  loading.value = true
  const outcome = await service.posts(next)
  loading.value = false

  if (outcome.ok) {
    posts.value = outcome.value.posts
    total.value = outcome.value.total
    page.value = outcome.value.page
    perPage.value = outcome.value.per_page
    loadFailed.value = false
    return
  }
  if (outcome.kind === 'signed-out') {
    await sendToLogin()
    return
  }
  loadFailed.value = true
}

async function sendToLogin(): Promise<void> {
  await router.replace({
    path: localizedPath('/login', locale.value),
    query: { redirect: route.fullPath },
  })
}

function startCreating(): void {
  creating.value = true
  fieldErrors.value = {}
  generalError.value = ''
}

function cancelCreating(): void {
  creating.value = false
  form.value = { title: '', description: '', access_policy: 'public', password: '' }
  fieldErrors.value = {}
}

async function submitCreate(): Promise<void> {
  if (busy.value) return
  busy.value = true
  fieldErrors.value = {}
  generalError.value = ''

  try {
    const outcome = await service.createPost(draftFrom(form.value))
    if (outcome.ok) {
      // Straight into the editor for it: a post with no media is not finished, and the
      // next thing the author wants is the page that adds some.
      await router.push(localizedPath(`/account/posts/${outcome.value}`, locale.value))
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
    generalError.value = 'editorActionFailed'
  } finally {
    busy.value = false
  }
}

function editorPath(serial: string): string {
  return localizedPath(`/account/posts/${serial}`, locale.value)
}
</script>

<template>
  <div class="posts">
    <header class="posts__head">
      <h1>{{ translate(locale, 'myPosts') }}</h1>
      <button v-if="!creating" type="button" class="form-button" @click="startCreating">
        {{ translate(locale, 'myPostsNew') }}
      </button>
    </header>

    <form v-if="creating" class="posts__create" @submit.prevent="submitCreate">
      <h2>{{ translate(locale, 'editorCreateTitle') }}</h2>

      <div class="form-field">
        <label for="post-title">{{ translate(locale, 'myPostsTitle') }}</label>
        <input id="post-title" v-model="form.title" type="text" :maxlength="limits.title" :disabled="busy" />
        <p v-if="firstError('title')" class="form-error">{{ firstError('title') }}</p>
      </div>

      <div class="form-field">
        <label for="post-description">{{ translate(locale, 'myPostsDescription') }}</label>
        <textarea
          id="post-description"
          v-model="form.description"
          rows="3"
          :maxlength="limits.description"
          :disabled="busy"
        ></textarea>
        <p class="form-hint">{{ translate(locale, 'editorDescriptionHint') }}</p>
        <p v-if="firstError('description')" class="form-error">{{ firstError('description') }}</p>
      </div>

      <fieldset class="posts__policies">
        <legend>{{ translate(locale, 'myPostsPublishment') }}</legend>
        <label v-for="(label, value) in policyLabels" :key="value" class="posts__policy">
          <input v-model="form.access_policy" type="radio" :value="value" :disabled="busy" />
          {{ translate(locale, label) }}
        </label>
      </fieldset>

      <div v-if="form.access_policy === 'password'" class="form-field">
        <label for="post-password">{{ translate(locale, 'editorPassword') }}</label>
        <input id="post-password" v-model="form.password" type="text" maxlength="255" :disabled="busy" />
        <p v-if="firstError('password')" class="form-error">{{ firstError('password') }}</p>
      </div>

      <p v-if="generalError" class="form-error" role="alert">
        {{ translate(locale, generalError) }}
      </p>

      <div class="posts__actions form-actions">
        <button type="submit" class="form-submit" :disabled="busy">
          {{ translate(locale, 'editorCreateSubmit') }}
        </button>
        <button type="button" class="form-button" :disabled="busy" @click="cancelCreating">
          {{ translate(locale, 'editorCancel') }}
        </button>
      </div>
    </form>

    <p v-if="loading" class="posts__status">{{ translate(locale, 'roomLoading') }}</p>
    <p v-else-if="loadFailed" class="form-error">{{ translate(locale, 'myPostsLoadFailed') }}</p>
    <p v-else-if="posts.length === 0" class="posts__status">{{ translate(locale, 'myPostsEmpty') }}</p>

    <ul v-else class="posts__list">
      <li v-for="post in posts" :key="post.serial" class="posts__item">
        <RouterLink class="posts__item-title" :to="editorPath(post.serial)">{{ post.title }}</RouterLink>
        <p class="posts__item-description">{{ post.description }}</p>
        <p class="posts__item-meta">
          <span>{{ translate(locale, policyLabels[post.access_policy]) }}</span>
          <span>{{ translate(locale, 'myPostsPlayedAll') }} {{ post.play_count }}</span>
          <span>{{ translate(locale, 'myPostsPlayedThisWeek') }} {{ post.this_week_play_count }}</span>
          <span>{{ translate(locale, 'myPostsPlayedLastWeek') }} {{ post.last_week_play_count }}</span>
        </p>
        <p v-if="post.tags.length" class="posts__item-tags">
          <span v-for="tag in post.tags" :key="tag">#{{ tag }}</span>
        </p>
      </li>
    </ul>

    <nav v-if="!loading && lastPage > 1" class="posts__pages">
      <button type="button" class="form-button" :disabled="page <= 1" @click="load(page - 1)">
        {{ translate(locale, 'myPostsPrevious') }}
      </button>
      <span>{{ page }} / {{ lastPage }}</span>
      <button type="button" class="form-button" :disabled="page >= lastPage" @click="load(page + 1)">
        {{ translate(locale, 'myPostsNext') }}
      </button>
    </nav>
  </div>
</template>

<style scoped>
/* Fields, hints, errors and buttons come from the shared .form-* classes in
   main.css. Only the list and the page's own layout live here. */
.posts {
  display: grid;
  width: min(100%, 46rem);
  margin-inline: auto;
  gap: 1.25rem;
}

.posts__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 1rem;
}

.posts__head h1 {
  margin: 0;
}

.posts__status {
  margin: 0;
  color: var(--text-soft);
}

.posts__create {
  display: grid;
  padding: 1.5rem;
  border: 1px solid var(--border);
  border-radius: 1.1rem;
  background: var(--bg-elevated);
  gap: 0.95rem;
}

.posts__create h2 {
  margin: 0;
  font-size: 1rem;
}

.posts__policies {
  display: flex;
  flex-wrap: wrap;
  padding: 0;
  border: 0;
  margin: 0;
  gap: 0.4rem 1rem;
}

.posts__policies legend {
  padding: 0;
  margin-bottom: 0.35rem;
  font-size: 0.78rem;
  font-weight: 700;
}

.posts__policies input {
  width: 1.05rem;
  height: 1.05rem;
  accent-color: var(--accent);
}

/* The label carries the radio's touch target: the input itself is ~17px. */
.posts__policy {
  display: flex;
  align-items: center;
  min-height: 2.75rem;
  font-size: 0.85rem;
  gap: 0.4rem;
  cursor: pointer;
}

.posts__list {
  display: grid;
  padding: 0;
  margin: 0;
  list-style: none;
  gap: 0.75rem;
}

.posts__item {
  padding: 1.1rem 1.25rem;
  border: 1px solid var(--border);
  border-radius: 1.1rem;
  background: var(--bg-elevated);
}

.posts__item-title {
  color: var(--text);
  font-size: 1rem;
  font-weight: 700;
  text-decoration: none;
}

.posts__item-title:hover {
  color: var(--accent);
}

.posts__item-description {
  margin: 0.35rem 0 0;
  color: var(--text-soft);
  font-size: 0.85rem;
  overflow-wrap: anywhere;
}

.posts__item-meta,
.posts__item-tags {
  display: flex;
  flex-wrap: wrap;
  margin: 0.5rem 0 0;
  color: var(--text-faint);
  font-size: 0.75rem;
  gap: 0.35rem 0.75rem;
}

.posts__pages {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-soft);
  font-size: 0.85rem;
  gap: 1rem;
}
</style>
