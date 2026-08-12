<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { describeFailure } from '../failures'
import {
  getAdminService,
  movedOrder,
  type CarouselDraft,
  type CarouselItem,
  type CarouselType,
} from '../../services/admin'

/**
 * The home page carousel.
 *
 * Order is written whole: dragging a row sends the entire list to
 * PUT /admin/carousel-items/reorder in one request, and the response is the stored order.
 * The original fired one request per slide, so a failure part-way left an order nobody
 * chose — here a failure leaves the previous order intact and the list is reloaded from
 * the answer rather than from what the drag produced locally.
 */

const admin = getAdminService()

const items = ref<CarouselItem[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')

const draft = ref<CarouselDraft>({
  type: 'image',
  title: '',
  description: '',
  image_url: '',
  video_url: '',
  video_start_second: null,
  video_end_second: null,
  is_active: true,
})

const dragIndex = ref<number | null>(null)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  const outcome = await admin.carouselItems()
  loading.value = false
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  items.value = outcome.value
}

onMounted(() => void load())

async function create(): Promise<void> {
  error.value = ''
  notice.value = ''
  saving.value = true
  const outcome = await admin.createCarouselItem({
    ...draft.value,
    title: draft.value.title.trim(),
    description: draft.value.description.trim(),
    image_url: draft.value.image_url.trim(),
    video_url: draft.value.video_url.trim(),
  })
  saving.value = false
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  notice.value = '已新增輪播項目。'
  draft.value = {
    type: 'image', title: '', description: '', image_url: '', video_url: '',
    video_start_second: null, video_end_second: null, is_active: true,
  }
  await load()
}

async function toggleActive(item: CarouselItem): Promise<void> {
  error.value = ''
  notice.value = ''
  const outcome = await admin.updateCarouselItem(item.id, { is_active: !item.is_active })
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  await load()
}

async function saveText(item: CarouselItem): Promise<void> {
  error.value = ''
  notice.value = ''
  const outcome = await admin.updateCarouselItem(item.id, {
    title: item.title.trim(),
    description: item.description.trim(),
    video_start_second: item.video_start_second,
    video_end_second: item.video_end_second,
  })
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  notice.value = '已儲存。'
  await load()
}

async function remove(item: CarouselItem): Promise<void> {
  if (!window.confirm(`確定要刪除「${item.title || item.id}」？`)) return

  error.value = ''
  notice.value = ''
  const outcome = await admin.deleteCarouselItem(item.id)
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  notice.value = '已刪除。'
  await load()
}

function startDrag(index: number): void {
  dragIndex.value = index
}

async function dropOn(index: number): Promise<void> {
  const from = dragIndex.value
  dragIndex.value = null
  if (from === null || from === index) return

  error.value = ''
  notice.value = ''
  saving.value = true
  const outcome = await admin.reorderCarouselItems(movedOrder(items.value, from, index))
  saving.value = false
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    // Reloaded rather than left as the drag showed it: the stored order is the truth.
    await load()
    return
  }
  items.value = outcome.value
  notice.value = '已更新順序。'
}

function typeLabel(type: CarouselType): string {
  return type === 'video' ? '影片' : '圖片'
}
</script>

<template>
  <section>
    <h1 class="admin-title">輪播管理</h1>

    <p v-if="error" class="admin-alert error">{{ error }}</p>
    <p v-else-if="notice" class="admin-alert ok">{{ notice }}</p>

    <div class="admin-card">
      <h2 class="admin-title">新增項目</h2>
      <div class="admin-grid">
        <label class="admin-field">
          <span>類型</span>
          <select v-model="draft.type">
            <option value="image">圖片</option>
            <option value="video">影片</option>
          </select>
        </label>
        <label class="admin-field">
          <span>標題</span>
          <input v-model="draft.title" type="text" />
        </label>
        <label class="admin-field">
          <span>說明</span>
          <input v-model="draft.description" type="text" />
        </label>
        <label class="admin-field">
          <span>圖片網址</span>
          <input v-model="draft.image_url" type="url" />
        </label>
        <label class="admin-field">
          <span>影片網址</span>
          <input v-model="draft.video_url" type="url" :disabled="draft.type !== 'video'" />
        </label>
        <label class="admin-field">
          <span>開始秒數</span>
          <input v-model.number="draft.video_start_second" type="number" min="0" :disabled="draft.type !== 'video'" />
        </label>
        <label class="admin-field">
          <span>結束秒數</span>
          <input v-model.number="draft.video_end_second" type="number" min="0" :disabled="draft.type !== 'video'" />
        </label>
      </div>
      <label class="admin-check">
        <input v-model="draft.is_active" type="checkbox" />
        <span>啟用</span>
      </label>
      <p class="admin-toolbar" style="margin-top: 1rem">
        <button class="admin-button primary" :disabled="saving" @click="create">新增</button>
      </p>
    </div>

    <p class="admin-note" style="margin-top: 1rem">拖曳最左邊的握把可以調整順序，放下後整份順序會一次送出。</p>

    <div class="admin-table-scroll">
      <table class="admin-table">
        <thead>
          <tr>
            <th></th>
            <th>順序</th>
            <th>類型</th>
            <th>標題／說明</th>
            <th>秒數</th>
            <th>狀態</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(item, index) in items"
            :key="item.id"
            :class="{ dragging: dragIndex === index }"
            @dragover.prevent
            @drop.prevent="dropOn(index)"
          >
            <td
              class="admin-drag-handle"
              draggable="true"
              aria-label="拖曳排序"
              @dragstart="startDrag(index)"
              @dragend="dragIndex = null"
            >⠿</td>
            <td>{{ item.position }}</td>
            <td>{{ typeLabel(item.type) }}</td>
            <td>
              <input v-model="item.title" type="text" />
              <input v-model="item.description" type="text" style="margin-top: 0.35rem" />
            </td>
            <td style="min-width: 9rem">
              <div class="admin-toolbar" style="margin: 0">
                <input v-model.number="item.video_start_second" type="number" min="0" style="width: 4rem" />
                <input v-model.number="item.video_end_second" type="number" min="0" style="width: 4rem" />
              </div>
            </td>
            <td>
              <span class="admin-badge">{{ item.is_active ? '啟用' : '停用' }}</span>
            </td>
            <td>
              <div class="admin-toolbar" style="margin: 0">
                <button class="admin-button" :disabled="saving" @click="saveText(item)">儲存</button>
                <button class="admin-button" :disabled="saving" @click="toggleActive(item)">
                  {{ item.is_active ? '停用' : '啟用' }}
                </button>
                <button class="admin-button danger" :disabled="saving" @click="remove(item)">刪除</button>
              </div>
            </td>
          </tr>
          <tr v-if="!items.length && !loading">
            <td colspan="7" class="admin-note">沒有輪播項目。</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
