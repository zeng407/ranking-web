<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { localizedPath, normalizeLocale, translate, type MessageKey } from '../i18n'
import { useAuth } from '../composables/useAuth'
import {
  PASSWORD_RESET_PATH,
  googleRedirectURL,
  login,
  readOAuthResult,
  register,
  type AuthFieldErrors,
  type AuthOutcome,
} from '../services/auth'

const route = useRoute()
const router = useRouter()
const locale = computed(() => normalizeLocale(route.params.locale))
const { refreshAuthState } = useAuth()

type Mode = 'login' | 'register'

const mode = ref<Mode>('login')
const submitting = ref(false)
const generalError = ref('')
const fieldErrors = ref<AuthFieldErrors>({})

const loginForm = ref({ email: '', password: '', remember: false })
const registerForm = ref({ name: '', email: '', password: '', password_confirmation: '' })

// Mirrors the Laravel validators so the browser catches the obvious cases first.
const limits = { nameMax: 20, emailMax: 50, passwordMin: 8 }

/**
 * Only same-site absolute paths are honoured. Accepting an arbitrary value here
 * would turn the login page into an open redirect.
 */
const redirectTarget = computed(() => {
  const raw = route.query.redirect
  const value = Array.isArray(raw) ? raw[0] : raw

  if (typeof value === 'string' && /^\/(?!\/)/.test(value)) return value

  return localizedPath('/', locale.value)
})

/** Splits the notice so the two policy links stay real links, without v-html. */
const tosParts = computed(() => translate(locale.value, 'loginTosNotice').split(/(\{tos\}|\{privacy\})/))

function setMode(next: Mode): void {
  mode.value = next
  generalError.value = ''
  fieldErrors.value = {}
}

/**
 * The server sends machine codes, not sentences — it has no message catalogue and could
 * only ever pick one of the three languages. Anything unrecognised falls through as-is
 * rather than being hidden, so a new server code is visible instead of silent.
 */
const validationMessages: Record<string, MessageKey> = {
  required: 'validationRequired',
  too_long: 'validationTooLong',
  too_short: 'validationTooShort',
  invalid_email: 'validationInvalidEmail',
  taken: 'validationTaken',
  mismatch: 'validationMismatch',
}

function firstError(field: string): string {
  const code = fieldErrors.value[field]?.[0]
  if (!code) return ''
  const key = validationMessages[code]
  return key ? translate(locale.value, key) : code
}

/**
 * Where the Google button goes. Built from the API base rather than a Laravel path, and
 * carrying where to return to so the callback can send the browser back here.
 */
const googleHref = computed(() => googleRedirectURL(window.location.origin + redirectTarget.value))

/**
 * The reason the OAuth callback gave, if it sent the browser back here having failed.
 *
 * A successful sign-in never lands on this page — the callback returns to wherever the
 * flow started — so the only result worth reading here is a failure.
 */
const oauthReasons: Record<string, MessageKey> = {
  'email-taken': 'oauthEmailTaken',
  'email-unverified': 'oauthEmailUnverified',
  'already-linked': 'oauthAlreadyLinked',
  expired: 'oauthExpired',
  declined: 'oauthDeclined',
  failed: 'oauthFailed',
}

async function run(action: () => Promise<AuthOutcome>, failedKey: MessageKey): Promise<void> {
  if (submitting.value) return

  submitting.value = true
  generalError.value = ''
  fieldErrors.value = {}

  try {
    const outcome = await action()

    if (outcome.ok) {
      await refreshAuthState(locale.value, true)
      await router.replace(redirectTarget.value)
      return
    }

    if (outcome.kind === 'validation') {
      fieldErrors.value = outcome.errors
      if (Object.keys(outcome.errors).length === 0) {
        generalError.value = translate(locale.value, failedKey)
      }
      return
    }

    generalError.value = translate(locale.value, 'authUnavailable')
  } finally {
    submitting.value = false
  }
}

function submitLogin(): Promise<void> {
  return run(() => login(locale.value, { ...loginForm.value }), 'loginFailed')
}

function submitRegister(): Promise<void> {
  return run(() => register(locale.value, { ...registerForm.value }), 'registerFailed')
}

watchEffect(() => {
  document.title = `${translate(locale.value, 'login')} · 2Pick`
})

// Shown once, on arrival. The marker stays in the URL — clearing it would need a router
// replace on mount, and a reload showing the same message again is the lesser problem.
watchEffect(() => {
  const result = readOAuthResult(window.location.search)
  if (result?.kind === 'failed') {
    generalError.value = translate(locale.value, oauthReasons[result.reason] ?? 'oauthFailed')
  }
})
</script>

<template>
  <div class="auth-page">
    <section class="auth-intro">
      <p class="eyebrow">{{ translate(locale, 'loginEyebrow') }}</p>
      <h1>{{ translate(locale, 'loginTitle') }}</h1>
      <p class="auth-intro-copy">{{ translate(locale, 'loginIntro') }}</p>
    </section>

    <section class="auth-card">
      <div class="auth-tabs" role="tablist" :aria-label="translate(locale, 'loginTitle')">
        <button
          id="auth-tab-login"
          type="button"
          role="tab"
          class="auth-tab"
          :class="{ active: mode === 'login' }"
          :aria-selected="mode === 'login'"
          aria-controls="auth-panel-login"
          @click="setMode('login')"
        >{{ translate(locale, 'loginTab') }}</button>
        <button
          id="auth-tab-register"
          type="button"
          role="tab"
          class="auth-tab"
          :class="{ active: mode === 'register' }"
          :aria-selected="mode === 'register'"
          aria-controls="auth-panel-register"
          @click="setMode('register')"
        >{{ translate(locale, 'registerTab') }}</button>
      </div>

      <p v-if="generalError" class="auth-error" role="alert">{{ generalError }}</p>

      <form
        v-if="mode === 'login'"
        id="auth-panel-login"
        class="auth-form"
        role="tabpanel"
        aria-labelledby="auth-tab-login"
        novalidate
        @submit.prevent="submitLogin"
      >
        <label class="auth-field">
          <span>{{ translate(locale, 'email') }}</span>
          <input
            v-model.trim="loginForm.email"
            type="email"
            name="email"
            autocomplete="email"
            required
            :maxlength="limits.emailMax"
            :aria-invalid="Boolean(firstError('email'))"
          >
          <small v-if="firstError('email')" class="auth-field-error">{{ firstError('email') }}</small>
        </label>

        <label class="auth-field">
          <span>{{ translate(locale, 'password') }}</span>
          <input
            v-model="loginForm.password"
            type="password"
            name="password"
            autocomplete="current-password"
            required
            :aria-invalid="Boolean(firstError('password'))"
          >
          <small v-if="firstError('password')" class="auth-field-error">{{ firstError('password') }}</small>
        </label>

        <div class="auth-row">
          <label class="auth-checkbox">
            <input v-model="loginForm.remember" type="checkbox" name="remember">
            <span>{{ translate(locale, 'rememberMe') }}</span>
          </label>
          <!-- Password reset still lives in Laravel, so this leaves the SPA. -->
          <a :href="PASSWORD_RESET_PATH">{{ translate(locale, 'forgotPassword') }}</a>
        </div>

        <button class="auth-submit" type="submit" :disabled="submitting">
          {{ submitting ? translate(locale, 'authSubmitting') : translate(locale, 'login') }}
        </button>
      </form>

      <form
        v-else
        id="auth-panel-register"
        class="auth-form"
        role="tabpanel"
        aria-labelledby="auth-tab-register"
        novalidate
        @submit.prevent="submitRegister"
      >
        <label class="auth-field">
          <span>{{ translate(locale, 'nickname') }}</span>
          <input
            v-model.trim="registerForm.name"
            type="text"
            name="name"
            autocomplete="nickname"
            required
            :maxlength="limits.nameMax"
            :aria-invalid="Boolean(firstError('name'))"
          >
          <small class="auth-field-hint">{{ translate(locale, 'nicknameHint') }}</small>
          <small v-if="firstError('name')" class="auth-field-error">{{ firstError('name') }}</small>
        </label>

        <label class="auth-field">
          <span>{{ translate(locale, 'email') }}</span>
          <input
            v-model.trim="registerForm.email"
            type="email"
            name="email"
            autocomplete="email"
            required
            :maxlength="limits.emailMax"
            :aria-invalid="Boolean(firstError('email'))"
          >
          <small v-if="firstError('email')" class="auth-field-error">{{ firstError('email') }}</small>
        </label>

        <label class="auth-field">
          <span>{{ translate(locale, 'password') }}</span>
          <input
            v-model="registerForm.password"
            type="password"
            name="password"
            autocomplete="new-password"
            required
            :minlength="limits.passwordMin"
            :aria-invalid="Boolean(firstError('password'))"
          >
          <small v-if="firstError('password')" class="auth-field-error">{{ firstError('password') }}</small>
        </label>

        <label class="auth-field">
          <span>{{ translate(locale, 'passwordConfirm') }}</span>
          <input
            v-model="registerForm.password_confirmation"
            type="password"
            name="password_confirmation"
            autocomplete="new-password"
            required
            :minlength="limits.passwordMin"
          >
        </label>

        <button class="auth-submit" type="submit" :disabled="submitting">
          {{ submitting ? translate(locale, 'authSubmitting') : translate(locale, 'registerSubmit') }}
        </button>
      </form>

      <div class="auth-divider"><span>{{ translate(locale, 'socialDivider') }}</span></div>

      <!-- OAuth must be a full page navigation; it cannot run over fetch. -->
      <a class="auth-google" :href="googleHref">
        <svg viewBox="0 0 24 24" aria-hidden="true" width="18" height="18">
          <path fill="#4285f4" d="M23.5 12.3c0-.8-.1-1.6-.2-2.3H12v4.5h6.4a5.5 5.5 0 0 1-2.4 3.6v3h3.9c2.3-2.1 3.6-5.2 3.6-8.8Z" />
          <path fill="#34a853" d="M12 24c3.2 0 5.9-1.1 7.9-2.9l-3.9-3a7.2 7.2 0 0 1-10.7-3.8h-4v3.1A12 12 0 0 0 12 24Z" />
          <path fill="#fbbc05" d="M5.3 14.3a7.2 7.2 0 0 1 0-4.6V6.6h-4a12 12 0 0 0 0 10.8l4-3.1Z" />
          <path fill="#ea4335" d="M12 4.8c1.8 0 3.4.6 4.6 1.8l3.4-3.4A12 12 0 0 0 1.3 6.6l4 3.1A7.2 7.2 0 0 1 12 4.8Z" />
        </svg>
        <span>{{ translate(locale, 'googleLogin') }}</span>
      </a>

      <p class="auth-tos">
        <template v-for="(part, index) in tosParts" :key="index">
          <RouterLink v-if="part === '{tos}'" :to="localizedPath('/tos', locale)">
            {{ translate(locale, 'terms') }}
          </RouterLink>
          <RouterLink v-else-if="part === '{privacy}'" :to="localizedPath('/privacy', locale)">
            {{ translate(locale, 'privacy') }}
          </RouterLink>
          <span v-else>{{ part }}</span>
        </template>
      </p>
    </section>
  </div>
</template>
