<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { localizedPath, normalizeLocale, translate, type MessageKey } from '../i18n'
import { useAuth } from '../composables/useAuth'
import {
  getAccountService,
  nameChangeIsBlocked,
  type Account,
  type AccountFieldErrors,
  type AccountOutcome,
  type AccountService,
} from '../services/account'
import { connectGoogleURL } from '../services/auth'

/**
 * Account settings, replacing the account/profile Blade page.
 *
 * THE E-MAIL FIELD IS READ-ONLY. The old page had an editable-looking input, but its
 * controller validated name and avatar only — an address has never actually been
 * changeable. Showing a field that silently discards what is typed into it is worse than
 * showing the value.
 */

const properties = defineProps<{ service?: AccountService }>()
const service = properties.service ?? getAccountService()

const route = useRoute()
const router = useRouter()
const locale = computed(() => normalizeLocale(route.params.locale))
const { refreshAuthState } = useAuth()

const account = ref<Account | null>(null)
const loading = ref(true)
const loadFailed = ref(false)
const busy = ref(false)
const notice = ref<MessageKey | ''>('')
const generalError = ref<MessageKey | ''>('')
const fieldErrors = ref<AccountFieldErrors>({})

const nameDraft = ref('')
const passwordForm = ref({ current: '', next: '', confirm: '' })
const avatarInput = ref<HTMLInputElement | null>(null)

const nameLocked = computed(() => nameChangeIsBlocked(account.value))
const hasPassword = computed(() => account.value?.has_password ?? false)

/**
 * The server's codes, translated here. Anything unrecognised falls through as the raw
 * code rather than being hidden, so a code this build does not know about is visible.
 */
const validationMessages: Record<string, MessageKey> = {
  required: 'accountErrorRequired',
  too_long: 'accountErrorTooLong',
  too_short: 'accountErrorTooShort',
  name_change_too_soon: 'accountErrorNameTooSoon',
  incorrect: 'accountErrorIncorrect',
  mismatch: 'accountErrorMismatch',
  password_already_set: 'accountErrorPasswordAlreadySet',
  no_password_set: 'accountErrorNoPasswordSet',
  unsupported_image: 'accountErrorUnsupportedImage',
  too_large: 'accountErrorTooLarge',
}

function firstError(field: string): string {
  const code = fieldErrors.value[field]?.[0]
  if (!code) return ''
  const key = validationMessages[code]
  return key ? translate(locale.value, key) : code
}

function text(key: MessageKey | ''): string {
  return key ? translate(locale.value, key) : ''
}

onMounted(load)

async function load(): Promise<void> {
  loading.value = true
  const outcome = await service.load()
  loading.value = false

  if (outcome.ok) {
    adopt(outcome.value)
    return
  }
  if (outcome.kind === 'signed-out') {
    await sendToLogin()
    return
  }
  loadFailed.value = true
}

function adopt(next: Account): void {
  account.value = next
  nameDraft.value = next.name
}

/**
 * A signed-out answer means the refresh cookie is gone or was revoked, so there is
 * nothing this page can do but hand the browser to the login form — with where to come
 * back to, since a settings page is somewhere the user meant to be.
 */
async function sendToLogin(): Promise<void> {
  await router.replace({
    path: localizedPath('/login', locale.value),
    query: { redirect: route.fullPath },
  })
}

/** One place for the busy flag, the messages, and the signed-out case. */
async function run<T>(
  action: () => Promise<AccountOutcome<T>>,
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
      if (Object.keys(outcome.errors).length === 0) generalError.value = 'accountActionFailed'
      return
    }
    if (outcome.kind === 'signed-out') {
      await sendToLogin()
      return
    }
    generalError.value = 'accountActionFailed'
  } finally {
    busy.value = false
  }
}

function submitName(): Promise<void> {
  return run(
    () => service.rename(nameDraft.value),
    (updated) => {
      adopt(updated)
      // The header draws the name, so it has to be told rather than left to a reload.
      void refreshAuthState(locale.value, true)
    },
    'accountSaved',
  )
}

function pickAvatar(): void {
  avatarInput.value?.click()
}

async function submitAvatar(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  // Cleared immediately so choosing the same file twice fires change again — a retry
  // after a rejected upload is otherwise silently ignored.
  input.value = ''
  if (!file) return

  await run(
    () => service.uploadAvatar(file),
    (url) => {
      if (account.value) account.value = { ...account.value, avatar_url: url }
      void refreshAuthState(locale.value, true)
    },
    'accountSaved',
  )
}

/**
 * The confirmation is checked here, not by the server.
 *
 * Laravel's `confirmed` rule lived in the validator, but the API deliberately has no
 * message catalogue: a mismatch is a typo the form can catch without a round trip, and
 * the code would have to be translated here anyway.
 */
function submitPassword(): Promise<void> {
  const { current, next, confirm } = passwordForm.value
  if (next !== confirm) {
    fieldErrors.value = { new_password: ['mismatch'] }
    return Promise.resolve()
  }

  const action = hasPassword.value
    ? () => service.changePassword(current, next)
    : () => service.setInitialPassword(next)

  return run(
    action,
    () => {
      passwordForm.value = { current: '', next: '', confirm: '' }
      if (account.value) account.value = { ...account.value, has_password: true }
    },
    hasPassword.value ? 'accountPasswordChanged' : 'accountSaved',
  )
}

/** Starts the Google link flow, which answers with a URL to navigate to. */
const googleConnectPending = ref(false)

async function connectGoogle(): Promise<void> {
  if (googleConnectPending.value) return
  googleConnectPending.value = true
  generalError.value = ''
  try {
    const url = await connectGoogleURL(window.location.href)
    if (!url) {
      generalError.value = 'accountActionFailed'
      return
    }
    window.location.assign(url)
  } finally {
    googleConnectPending.value = false
  }
}
</script>

<template>
  <div class="account">
    <h1>{{ translate(locale, 'accountTitle') }}</h1>

    <p v-if="loading" class="account__status">{{ translate(locale, 'roomLoading') }}</p>
    <p v-else-if="loadFailed" class="form-error">{{ translate(locale, 'accountLoadFailed') }}</p>

    <template v-else-if="account">
      <p v-if="notice" class="form-notice" role="status">{{ text(notice) }}</p>
      <p v-if="generalError" class="form-error" role="alert">{{ text(generalError) }}</p>

      <section class="account__section">
        <h2>{{ translate(locale, 'accountProfile') }}</h2>

        <div class="account__avatar">
          <!--
            The same placeholder the header draws, not Laravel's storage/default-avatar.webp:
            the SPA is served from its own origin and that path is not on it. 2,007 accounts
            have no avatar, so this is the common case rather than an edge one.
          -->
          <img
            v-if="account.avatar_url"
            :src="account.avatar_url"
            :alt="translate(locale, 'accountAvatar')"
            width="96"
            height="96"
          />
          <svg v-else class="account__avatar-placeholder" viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="12" cy="8.5" r="3.5" />
            <path d="M5 20a7 7 0 0 1 14 0" />
          </svg>
          <button type="button" class="form-button" :disabled="busy" @click="pickAvatar">
            {{ translate(locale, 'accountAvatarChange') }}
          </button>
          <input
            ref="avatarInput"
            type="file"
            accept="image/png,image/jpeg,image/gif,image/webp"
            hidden
            @change="submitAvatar"
          />
          <p v-if="firstError('avatar')" class="form-error">{{ firstError('avatar') }}</p>
        </div>

        <form class="account__form" @submit.prevent="submitName">
          <div class="form-field">
            <label for="account-name">{{ translate(locale, 'accountName') }}</label>
            <input id="account-name" v-model="nameDraft" type="text" maxlength="20" :disabled="busy" />
            <p v-if="nameLocked" class="form-hint">{{ translate(locale, 'accountErrorNameTooSoon') }}</p>
            <p v-if="firstError('name')" class="form-error">{{ firstError('name') }}</p>
          </div>
          <button type="submit" class="form-submit" :disabled="busy || nameDraft === account.name">
            {{ translate(locale, 'accountSave') }}
          </button>
        </form>

        <div class="account__form">
          <div class="form-field">
            <label for="account-email">{{ translate(locale, 'accountEmail') }}</label>
            <input id="account-email" :value="account.email" type="email" disabled />
            <p class="form-hint">{{ translate(locale, 'accountEmailFixed') }}</p>
          </div>
        </div>
      </section>

      <section class="account__section">
        <h2>Google</h2>
        <p v-if="account.google_linked" class="account__copy">
          {{ translate(locale, 'accountGoogleLinked') }}
        </p>
        <template v-else>
          <p class="account__copy">{{ translate(locale, 'accountGoogleUnlinked') }}</p>
          <button
            type="button"
            class="form-button account__google"
            :disabled="googleConnectPending"
            @click="connectGoogle"
          >
            {{ translate(locale, 'accountGoogleConnect') }}
          </button>
        </template>
      </section>

      <section class="account__section">
        <h2>{{ translate(locale, 'accountPassword') }}</h2>
        <p v-if="!hasPassword" class="form-hint">
          {{ translate(locale, 'accountPasswordInitHint') }}
        </p>

        <form class="account__form" @submit.prevent="submitPassword">
          <div v-if="hasPassword" class="form-field">
            <label for="account-current">{{ translate(locale, 'accountPasswordCurrent') }}</label>
            <input
              id="account-current"
              v-model="passwordForm.current"
              type="password"
              autocomplete="current-password"
              :disabled="busy"
            />
            <p v-if="firstError('current_password')" class="form-error">
              {{ firstError('current_password') }}
            </p>
          </div>

          <div class="form-field">
            <label for="account-new">{{ translate(locale, 'accountPasswordNew') }}</label>
            <input
              id="account-new"
              v-model="passwordForm.next"
              type="password"
              autocomplete="new-password"
              minlength="8"
              :disabled="busy"
            />
          </div>

          <div class="form-field">
            <label for="account-confirm">{{ translate(locale, 'accountPasswordConfirm') }}</label>
            <input
              id="account-confirm"
              v-model="passwordForm.confirm"
              type="password"
              autocomplete="new-password"
              :disabled="busy"
            />
            <p v-if="firstError('new_password')" class="form-error">
              {{ firstError('new_password') }}
            </p>
          </div>

          <button type="submit" class="form-submit" :disabled="busy">
            {{ translate(locale, hasPassword ? 'accountPasswordSet' : 'accountPasswordInit') }}
          </button>
        </form>
      </section>
    </template>
  </div>
</template>

<style scoped>
/* Fields, hints, errors and buttons come from the shared .form-* classes in
   main.css, the same shapes the login page draws. Only the page's own layout
   lives here. */
.account {
  display: grid;
  width: min(100%, 32rem);
  margin-inline: auto;
  gap: 1.25rem;
}

.account h1 {
  margin: 0;
}

.account__status {
  margin: 0;
  color: var(--text-soft);
}

.account__section {
  display: grid;
  padding: 1.5rem;
  border: 1px solid var(--border);
  border-radius: 1.1rem;
  background: var(--bg-elevated);
  gap: 1.1rem;
}

.account__section h2 {
  margin: 0;
  font-size: 1rem;
}

.account__copy {
  margin: 0;
  color: var(--text-soft);
  font-size: 0.85rem;
}

.account__form {
  display: grid;
  gap: 0.95rem;
}

.account__avatar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 1rem;
}

.account__avatar img,
.account__avatar-placeholder {
  width: 96px;
  height: 96px;
  border: 1px solid var(--border);
  border-radius: 50%;
  background: var(--surface);
  object-fit: cover;
}

.account__avatar-placeholder {
  fill: none;
  stroke: currentColor;
  stroke-width: 1.6;
  opacity: 0.55;
}

/* The connect button sizes to its label instead of the card: it is one action,
   not the section's submit. */
.account__google {
  justify-self: start;
}
</style>
