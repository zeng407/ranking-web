<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { describeFailure } from '../failures'
import { getAdminService, type AdminService, type AdminUser } from '../../services/admin'

/**
 * Accounts, and the ban.
 *
 * A ban adds the `banned` role, ends every session the account holds and clears the role
 * cache Laravel still reads; an unban removes it. Administrators cannot be banned, which
 * the server refuses with 409 rather than the button hiding it — the role list on a row can
 * be stale by the time it is clicked.
 */

const properties = defineProps<{ service?: AdminService }>()
const admin = properties.service ?? getAdminService()

const users = ref<AdminUser[]>([])
const total = ref(0)
const page = ref(1)
const perPage = ref(20)
const keyword = ref('')
const loading = ref(false)
const busyId = ref<number | null>(null)
const error = ref('')
const notice = ref('')

async function load(target = page.value): Promise<void> {
  loading.value = true
  error.value = ''
  const outcome = await admin.users(keyword.value.trim(), target)
  loading.value = false
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  users.value = outcome.value.users
  total.value = outcome.value.total
  page.value = outcome.value.page
  perPage.value = outcome.value.per_page
}

onMounted(() => void load(1))

function isBanned(user: AdminUser): boolean {
  return user.roles.includes('banned')
}

function isAdministrator(user: AdminUser): boolean {
  return user.roles.includes('admin')
}

async function toggleBan(user: AdminUser): Promise<void> {
  const banned = isBanned(user)
  if (!banned && !window.confirm(`確定要封鎖「${user.name}」？該帳號會被強制登出。`)) return

  error.value = ''
  notice.value = ''
  busyId.value = user.id
  const outcome = banned ? await admin.unbanUser(user.id) : await admin.banUser(user.id)
  busyId.value = null
  if (!outcome.ok) {
    error.value = describeFailure(outcome)
    return
  }
  notice.value = banned ? `已解除封鎖「${user.name}」。` : `已封鎖「${user.name}」。`
  await load()
}

function lastPage(): number {
  return Math.max(1, Math.ceil(total.value / Math.max(1, perPage.value)))
}
</script>

<template>
  <section>
    <h1 class="admin-title">使用者管理</h1>

    <p v-if="error" class="admin-alert error">{{ error }}</p>
    <p v-else-if="notice" class="admin-alert ok">{{ notice }}</p>

    <div class="admin-toolbar">
      <input v-model="keyword" type="search" placeholder="以名稱或 Email 搜尋" style="max-width: 20rem" />
      <button class="admin-button" :disabled="loading" @click="load(1)">搜尋</button>
      <button class="admin-button" :disabled="loading" @click="keyword = ''; load(1)">清除</button>
    </div>

    <div class="admin-table-scroll">
      <table class="admin-table">
        <thead>
          <tr>
            <th>帳號</th>
            <th>Email</th>
            <th>角色</th>
            <th>投票數</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id">
            <td>
              <div>{{ user.name }}</div>
              <span class="admin-note">#{{ user.id }}</span>
            </td>
            <td>{{ user.email }}</td>
            <td>
              <span v-for="role in user.roles" :key="role" class="admin-badge">{{ role }}</span>
              <span v-if="!user.roles.length" class="admin-note">—</span>
            </td>
            <td>{{ user.post_count }}</td>
            <td>
              <button
                class="admin-button"
                :class="{ danger: !isBanned(user) }"
                :disabled="busyId === user.id || isAdministrator(user)"
                @click="toggleBan(user)"
              >
                {{ isBanned(user) ? '解除封鎖' : '封鎖' }}
              </button>
            </td>
          </tr>
          <tr v-if="!users.length && !loading">
            <td colspan="5" class="admin-note">沒有資料。</td>
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
