<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { describeFailure } from '../failures'
import { getAdminService, type Announcement } from '../../services/admin'

/**
 * The site-wide announcement.
 *
 * There is one, and publishing replaces it — the store keeps a single key with an
 * expiry, which is why the form has a "keep for" field rather than a list of past notices.
 * Absent is the normal state, so an empty form is not an error.
 */

const admin = getAdminService()

const current = ref<Announcement | null>(null)
const form = ref({ content: '', image_url: '', keep_minutes: 60 })
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  const outcome = await admin.announcement()
  loading.value = false
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  current.value = outcome.value
  if (outcome.value) {
    form.value = {
      content: outcome.value.content,
      image_url: outcome.value.image_url,
      keep_minutes: outcome.value.keep_minutes,
    }
  }
}

onMounted(() => void load())

async function publish(): Promise<void> {
  error.value = ''
  notice.value = ''
  saving.value = true
  const outcome = await admin.publishAnnouncement({
    content: form.value.content.trim(),
    image_url: form.value.image_url.trim(),
    keep_minutes: Number(form.value.keep_minutes) || 0,
  })
  saving.value = false
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  current.value = outcome.value
  notice.value = '公告已發布。'
}
</script>

<template>
  <section>
    <h1 class="admin-title">公告</h1>

    <p v-if="error" class="admin-alert error">{{ error }}</p>
    <p v-else-if="notice" class="admin-alert ok">{{ notice }}</p>

    <div class="admin-card">
      <p v-if="current" class="admin-note">
        目前公告 {{ current.id }}，發布於 {{ current.created_at }}，保留 {{ current.keep_minutes }} 分鐘。
      </p>
      <p v-else-if="!loading" class="admin-note">目前沒有公告。</p>

      <label class="admin-field">
        <span>內容</span>
        <textarea v-model="form.content"></textarea>
      </label>
      <div class="admin-grid">
        <label class="admin-field">
          <span>圖片網址（選填）</span>
          <input v-model="form.image_url" type="url" />
        </label>
        <label class="admin-field">
          <span>保留分鐘數</span>
          <input v-model.number="form.keep_minutes" type="number" min="1" />
        </label>
      </div>
      <p class="admin-toolbar" style="margin-top: 1rem">
        <button class="admin-button primary" :disabled="saving" @click="publish">發布</button>
        <button class="admin-button" :disabled="loading" @click="load">重新載入</button>
      </p>
    </div>
  </section>
</template>
