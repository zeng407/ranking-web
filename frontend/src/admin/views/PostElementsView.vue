<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { describeFailure } from '../failures'
import { getAdminService } from '../../services/admin'
import type { PostElement } from '../../services/editor'

/**
 * One post's elements, with a moderator's two actions: fixing a title and removing an item.
 *
 * Replacing an element's media is deliberately absent: that endpoint is the author's
 * (PUT /account/elements/{id}/media, ownership checked in SQL), and a moderator who needs
 * a picture gone deletes the element.
 */

const route = useRoute()
const admin = getAdminService()
const serial = String(route.params.serial || '')

const elements = ref<PostElement[]>([])
const total = ref(0)
const page = ref(1)
const perPage = ref(20)
const titleFilter = ref('')
const loading = ref(false)
const error = ref('')
const notice = ref('')

/** id of the row whose title is open for editing, and the value being typed. */
const editingId = ref<number | null>(null)
const editingTitle = ref('')

async function load(target = page.value): Promise<void> {
  loading.value = true
  error.value = ''
  const outcome = await admin.elements(serial, {
    page: target,
    title: titleFilter.value.trim() || undefined,
  })
  loading.value = false
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  elements.value = outcome.value.elements
  total.value = outcome.value.total
  page.value = outcome.value.page
  perPage.value = outcome.value.per_page
}

onMounted(() => void load(1))

function startEditing(element: PostElement): void {
  editingId.value = element.id
  editingTitle.value = element.title
}

async function saveTitle(element: PostElement): Promise<void> {
  error.value = ''
  notice.value = ''
  const outcome = await admin.updateElement(element.id, { title: editingTitle.value.trim() })
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  editingId.value = null
  notice.value = '已更新名稱。'
  await load()
}

async function remove(element: PostElement): Promise<void> {
  if (!window.confirm(`確定要刪除「${element.title}」？此動作無法復原。`)) return

  error.value = ''
  notice.value = ''
  const outcome = await admin.deleteElement(element.id)
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  notice.value = '已刪除元素。'
  await load()
}

function lastPage(): number {
  return Math.max(1, Math.ceil(total.value / Math.max(1, perPage.value)))
}
</script>

<template>
  <section>
    <h1 class="admin-title">元素管理</h1>
    <p class="admin-note">投票：{{ serial }}　<RouterLink to="/posts">回投票列表</RouterLink></p>

    <p v-if="error" class="admin-alert error">{{ error }}</p>
    <p v-else-if="notice" class="admin-alert ok">{{ notice }}</p>

    <div class="admin-toolbar">
      <input v-model="titleFilter" type="search" placeholder="以名稱搜尋" style="max-width: 18rem" />
      <button class="admin-button" :disabled="loading" @click="load(1)">搜尋</button>
    </div>

    <div class="admin-table-scroll">
      <table class="admin-table">
        <thead>
          <tr>
            <th>預覽</th>
            <th>名稱</th>
            <th>類型</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="element in elements" :key="element.id">
            <td>
              <img
                class="admin-thumb"
                :src="element.lowthumb_url || element.thumb_url"
                :alt="element.title"
                loading="lazy"
              />
            </td>
            <td>
              <template v-if="editingId === element.id">
                <input v-model="editingTitle" type="text" />
              </template>
              <template v-else>
                <div>{{ element.title }}</div>
                <span class="admin-note">#{{ element.id }}</span>
              </template>
            </td>
            <td>
              {{ element.type }}
              <span v-if="element.video_source" class="admin-badge">{{ element.video_source }}</span>
            </td>
            <td>
              <div class="admin-toolbar" style="margin: 0">
                <template v-if="editingId === element.id">
                  <button class="admin-button primary" @click="saveTitle(element)">儲存</button>
                  <button class="admin-button" @click="editingId = null">取消</button>
                </template>
                <template v-else>
                  <button class="admin-button" @click="startEditing(element)">改名</button>
                  <button class="admin-button danger" @click="remove(element)">刪除</button>
                </template>
              </div>
            </td>
          </tr>
          <tr v-if="!elements.length && !loading">
            <td colspan="4" class="admin-note">沒有資料。</td>
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
