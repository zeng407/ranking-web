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
  <main class="posts">
    <header class="posts__head">
      <h1>{{ translate(locale, 'myPosts') }}</h1>
      <button v-if="!creating" type="button" @click="startCreating">
        {{ translate(locale, 'myPostsNew') }}
      </button>
    </header>

    <form v-if="creating" class="posts__create" @submit.prevent="submitCreate">
      <h2>{{ translate(locale, 'editorCreateTitle') }}</h2>

      <label for="post-title">{{ translate(locale, 'myPostsTitle') }}</label>
      <input id="post-title" v-model="form.title" type="text" :maxlength="limits.title" :disabled="busy" />
      <p v-if="firstError('title')" class="posts__field-error">{{ firstError('title') }}</p>

      <label for="post-description">{{ translate(locale, 'myPostsDescription') }}</label>
      <textarea
        id="post-description"
        v-model="form.description"
        rows="3"
        :maxlength="limits.description"
        :disabled="busy"
      ></textarea>
      <p class="posts__hint">{{ translate(locale, 'editorDescriptionHint') }}</p>
      <p v-if="firstError('description')" class="posts__field-error">{{ firstError('description') }}</p>

      <fieldset class="posts__policies">
        <legend>{{ translate(locale, 'myPostsPublishment') }}</legend>
        <label v-for="(label, value) in policyLabels" :key="value">
          <input v-model="form.access_policy" type="radio" :value="value" :disabled="busy" />
          {{ translate(locale, label) }}
        </label>
      </fieldset>

      <template v-if="form.access_policy === 'password'">
        <label for="post-password">{{ translate(locale, 'editorPassword') }}</label>
        <input id="post-password" v-model="form.password" type="text" maxlength="255" :disabled="busy" />
        <p v-if="firstError('password')" class="posts__field-error">{{ firstError('password') }}</p>
      </template>

      <p v-if="generalError" class="posts__error" role="alert">
        {{ translate(locale, generalError) }}
      </p>

      <div class="posts__actions">
        <button type="submit" :disabled="busy">{{ translate(locale, 'editorCreateSubmit') }}</button>
        <button type="button" :disabled="busy" @click="cancelCreating">
          {{ translate(locale, 'editorCancel') }}
        </button>
      </div>
    </form>

    <p v-if="loading" class="posts__status">{{ translate(locale, 'roomLoading') }}</p>
    <p v-else-if="loadFailed" class="posts__error">{{ translate(locale, 'myPostsLoadFailed') }}</p>
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
      <button type="button" :disabled="page <= 1" @click="load(page - 1)">
        {{ translate(locale, 'myPostsPrevious') }}
      </button>
      <span>{{ page }} / {{ lastPage }}</span>
      <button type="button" :disabled="page >= lastPage" @click="load(page + 1)">
        {{ translate(locale, 'myPostsNext') }}
      </button>
    </nav>
  </main>
</template>

<style scoped>
.posts {
  max-width: 52rem;
  margin: 0 auto;
  padding: 1.5rem 1rem 3rem;
}

.posts__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.posts__create {
  display: grid;
  gap: 0.4rem;
  justify-items: start;
  margin-top: 1.5rem;
  padding: 1rem;
  border: 1px solid var(--border, #d9d9d9);
  border-radius: 0.5rem;
}

.posts__create input[type='text'],
.posts__create textarea {
  width: min(28rem, 100%);
  padding: 0.4rem 0.6rem;
}

.posts__policies {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
  border: 0;
  padding: 0;
  margin: 0.5rem 0;
}

.posts__actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.posts__list {
  list-style: none;
  padding: 0;
  margin: 1.5rem 0 0;
  display: grid;
  gap: 0.75rem;
}

.posts__item {
  padding: 0.85rem 1rem;
  border: 1px solid var(--border, #d9d9d9);
  border-radius: 0.5rem;
}

.posts__item-title {
  font-weight: 600;
  font-size: 1.05rem;
}

.posts__item-description {
  margin: 0.35rem 0;
  opacity: 0.85;
  overflow-wrap: anywhere;
}

.posts__item-meta,
.posts__item-tags {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
  font-size: 0.85rem;
  opacity: 0.75;
  margin: 0.25rem 0 0;
}

.posts__pages {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  margin-top: 1.5rem;
}

.posts__hint {
  font-size: 0.85rem;
  opacity: 0.75;
  margin: 0;
}

.posts__field-error,
.posts__error {
  color: var(--danger, #c0392b);
  font-size: 0.875rem;
  margin: 0;
}
</style>
