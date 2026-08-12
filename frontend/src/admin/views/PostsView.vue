<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { describeFailure } from '../failures'
import {
  getAdminService,
  type AdminPost,
  type AdminPostDetail,
  type AdminPostEdit,
} from '../../services/admin'
import type { AccessPolicy } from '../../services/editor'

/**
 * Every post on the site, with the two moderation actions: censoring one and deleting it.
 *
 * `is_censored` is not editable on its own endpoint, so flipping it reads the post first
 * and sends it back whole. That costs one extra request and keeps the title and tags the
 * author wrote — sending an edit with empty fields would erase them.
 */

const admin = getAdminService()

const posts = ref<AdminPost[]>([])
const total = ref(0)
const page = ref(1)
const perPage = ref(20)
const loading = ref(false)
const error = ref('')
const notice = ref('')

/** The post being edited, once its detail has loaded. Null closes the form. */
const editing = ref<AdminPostDetail | null>(null)
const form = ref({ title: '', description: '', access_policy: 'public' as AccessPolicy, password: '', tags: '', censored: false })
const saving = ref(false)

async function load(target = page.value): Promise<void> {
  loading.value = true
  error.value = ''
  const outcome = await admin.posts(target)
  loading.value = false
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  posts.value = outcome.value.posts
  total.value = outcome.value.total
  page.value = outcome.value.page
  perPage.value = outcome.value.per_page
}

onMounted(() => void load(1))

async function openEditor(post: AdminPost): Promise<void> {
  error.value = ''
  notice.value = ''
  const outcome = await admin.post(post.serial)
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  editing.value = outcome.value
  form.value = {
    title: outcome.value.title,
    description: outcome.value.description,
    access_policy: outcome.value.access_policy,
    password: '',
    tags: outcome.value.tags.join(', '),
    censored: post.is_censored,
  }
}

function closeEditor(): void {
  editing.value = null
}

async function save(): Promise<void> {
  const current = editing.value
  if (!current) return

  const edit: AdminPostEdit = {
    title: form.value.title.trim(),
    description: form.value.description.trim(),
    access_policy: form.value.access_policy,
    tags: form.value.tags.split(',').map((tag) => tag.trim()).filter(Boolean),
    is_censored: form.value.censored,
  }
  // Only when one was typed: an empty string would clear the stored password.
  if (form.value.access_policy === 'password' && form.value.password) {
    edit.password = form.value.password
  }

  saving.value = true
  const outcome = await admin.updatePost(current.serial, edit)
  saving.value = false
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  notice.value = `已更新「${outcome.value.title}」。`
  editing.value = null
  await load()
}

async function toggleCensored(post: AdminPost): Promise<void> {
  error.value = ''
  notice.value = ''
  const detail = await admin.post(post.serial)
  if (!detail.ok) {
    error.value = describeFailure(detail)
    return
  }
  const outcome = await admin.updatePost(post.serial, {
    title: detail.value.title,
    description: detail.value.description,
    access_policy: detail.value.access_policy,
    tags: detail.value.tags,
    is_censored: !post.is_censored,
  })
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  notice.value = post.is_censored ? `已解除封鎖「${post.title}」。` : `已封鎖「${post.title}」。`
  await load()
}

async function remove(post: AdminPost): Promise<void> {
  // Deleting a post takes the whole ranking with it, so it is confirmed here rather than
  // by a password the way an author's own deletion is.
  if (!window.confirm(`確定要刪除「${post.title}」？此動作無法復原。`)) return

  error.value = ''
  notice.value = ''
  const outcome = await admin.deletePost(post.serial)
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  notice.value = `已刪除「${post.title}」。`
  await load()
}

function lastPage(): number {
  return Math.max(1, Math.ceil(total.value / Math.max(1, perPage.value)))
}
</script>

<template>
  <section>
    <h1 class="admin-title">投票管理</h1>

    <p v-if="error" class="admin-alert error">{{ error }}</p>
    <p v-else-if="notice" class="admin-alert ok">{{ notice }}</p>

    <div v-if="editing" class="admin-card">
      <h2 class="admin-title">編輯「{{ editing.title }}」</h2>
      <label class="admin-field">
        <span>標題</span>
        <input v-model="form.title" type="text" />
      </label>
      <label class="admin-field">
        <span>說明</span>
        <textarea v-model="form.description"></textarea>
      </label>
      <div class="admin-grid">
        <label class="admin-field">
          <span>公開設定</span>
          <select v-model="form.access_policy">
            <option value="public">公開</option>
            <option value="private">私人</option>
            <option value="password">密碼</option>
          </select>
        </label>
        <label class="admin-field">
          <span>密碼（留空不變更）</span>
          <input v-model="form.password" type="text" :disabled="form.access_policy !== 'password'" />
        </label>
        <label class="admin-field">
          <span>標籤（以逗號分隔）</span>
          <input v-model="form.tags" type="text" />
        </label>
      </div>
      <label class="admin-check">
        <input v-model="form.censored" type="checkbox" />
        <span>封鎖（不出現在公開列表）</span>
      </label>
      <p class="admin-toolbar" style="margin-top: 1rem">
        <button class="admin-button primary" :disabled="saving" @click="save">儲存</button>
        <button class="admin-button" :disabled="saving" @click="closeEditor">取消</button>
      </p>
    </div>

    <div class="admin-table-scroll">
      <table class="admin-table">
        <thead>
          <tr>
            <th>標題</th>
            <th>作者</th>
            <th>公開</th>
            <th>遊玩</th>
            <th>狀態</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="post in posts" :key="post.serial">
            <td>
              <div>{{ post.title }}</div>
              <span class="admin-note">{{ post.serial }}</span>
            </td>
            <td>
              <div>{{ post.owner.name }}</div>
              <span class="admin-note">{{ post.owner.email }}</span>
            </td>
            <td>{{ post.access_policy }}</td>
            <td>{{ post.play_count }}</td>
            <td>
              <span v-if="post.is_censored" class="admin-badge">已封鎖</span>
              <span v-else class="admin-note">正常</span>
            </td>
            <td>
              <div class="admin-toolbar" style="margin: 0">
                <button class="admin-button" @click="openEditor(post)">編輯</button>
                <RouterLink class="admin-button" :to="`/posts/${post.serial}/elements`">元素</RouterLink>
                <button class="admin-button" @click="toggleCensored(post)">
                  {{ post.is_censored ? '解除封鎖' : '封鎖' }}
                </button>
                <button class="admin-button danger" @click="remove(post)">刪除</button>
              </div>
            </td>
          </tr>
          <tr v-if="!posts.length && !loading">
            <td colspan="6" class="admin-note">沒有資料。</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="admin-pager">
      <button class="admin-button" :disabled="loading || page <= 1" @click="load(page - 1)">上一頁</button>
      <span class="admin-note">第 {{ page }} / {{ lastPage() }} 頁，共 {{ total }} 筆</span>
      <button class="admin-button" :disabled="loading || page >= lastPage()" @click="load(page + 1)">下一頁</button>
    </div>
  </section>
</template>
