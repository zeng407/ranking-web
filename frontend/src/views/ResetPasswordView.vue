<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { localizedPath, normalizeLocale, translate, type MessageKey } from '../i18n'
import { useAuth } from '../composables/useAuth'
import { resetPassword, type AuthFieldErrors } from '../services/auth'

/**
 * Sets a new password from a mailed link, replacing Laravel's password/reset Blade page.
 *
 * The token comes from the path, which is the shape the mailed links have always had. A
 * finished reset signs the account in — the user has just proved control of the address on
 * file, and sending them to a login form to type the password they set two seconds ago
 * only loses people — so this navigates home rather than to /login.
 */

const route = useRoute()
const router = useRouter()
const locale = computed(() => normalizeLocale(route.params.locale))
const { refreshAuthState } = useAuth()

const token = computed(() => {
  const raw = route.params.token
  return Array.isArray(raw) ? raw[0] ?? '' : String(raw ?? '')
})

const form = ref({ next: '', confirm: '' })
const submitting = ref(false)
const generalError = ref('')
const fieldErrors = ref<AuthFieldErrors>({})

const limits = { passwordMin: 8 }

const validationMessages: Record<string, MessageKey> = {
  required: 'validationRequired',
  too_long: 'validationTooLong',
  too_short: 'validationTooShort',
  mismatch: 'validationMismatch',
  // One code for a link that expired, was already used, or was never issued: the server
  // does not tell them apart, because telling them apart confirms a guessed token was real.
  invalid: 'resetPasswordLinkInvalid',
}

function firstError(field: string): string {
  const code = fieldErrors.value[field]?.[0]
  if (!code) return ''
  const key = validationMessages[code]
  return key ? translate(locale.value, key) : code
}

/**
 * The confirmation is checked here, as on the account page: a mismatch is a typo the form
 * can catch without a round trip, and a round trip would spend the link.
 */
async function submit(): Promise<void> {
  if (submitting.value) return

  if (form.value.next !== form.value.confirm) {
    fieldErrors.value = { new_password: ['mismatch'] }
    return
  }

  submitting.value = true
  generalError.value = ''
  fieldErrors.value = {}

  try {
    const outcome = await resetPassword(locale.value, {
      token: token.value,
      new_password: form.value.next,
    })

    if (outcome.ok) {
      // The header draws the signed-in state, so it has to be told rather than left to a
      // reload that is not coming.
      await refreshAuthState(locale.value, true)
      await router.replace(localizedPath('/', locale.value))
      return
    }

    if (outcome.kind === 'validation') {
      fieldErrors.value = outcome.errors
      if (Object.keys(outcome.errors).length === 0) {
        generalError.value = translate(locale.value, 'authUnavailable')
      }
      return
    }

    generalError.value = translate(locale.value, 'authUnavailable')
  } finally {
    submitting.value = false
  }
}

watchEffect(() => {
  document.title = `${translate(locale.value, 'resetPasswordTitle')} · 2Pick`
})
</script>

<template>
  <div class="auth-page">
    <section class="auth-intro">
      <p class="eyebrow">{{ translate(locale, 'loginEyebrow') }}</p>
      <h1>{{ translate(locale, 'resetPasswordTitle') }}</h1>
      <p class="auth-intro-copy">{{ translate(locale, 'resetPasswordIntro') }}</p>
    </section>

    <section class="auth-card">
      <p v-if="generalError" class="auth-error" role="alert">{{ generalError }}</p>
      <p v-if="firstError('token')" class="auth-error" role="alert">{{ firstError('token') }}</p>

      <form class="auth-form" novalidate @submit.prevent="submit">
        <label class="auth-field">
          <span>{{ translate(locale, 'password') }}</span>
          <input
            v-model="form.next"
            type="password"
            name="new_password"
            autocomplete="new-password"
            required
            :minlength="limits.passwordMin"
            :aria-invalid="Boolean(firstError('new_password'))"
          >
          <small v-if="firstError('new_password')" class="auth-field-error">
            {{ firstError('new_password') }}
          </small>
        </label>

        <label class="auth-field">
          <span>{{ translate(locale, 'passwordConfirm') }}</span>
          <input
            v-model="form.confirm"
            type="password"
            name="new_password_confirmation"
            autocomplete="new-password"
            required
            :minlength="limits.passwordMin"
          >
        </label>

        <button class="auth-submit" type="submit" :disabled="submitting">
          {{ submitting ? translate(locale, 'authSubmitting') : translate(locale, 'resetPasswordSubmit') }}
        </button>
      </form>

      <p class="auth-tos">
        <RouterLink :to="localizedPath('/password/forgot', locale)">
          {{ translate(locale, 'resetPasswordAskAgain') }}
        </RouterLink>
      </p>
    </section>
  </div>
</template>
