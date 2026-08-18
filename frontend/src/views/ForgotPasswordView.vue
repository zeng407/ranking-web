<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { useRoute } from 'vue-router'

import { localizedPath, normalizeLocale, translate, type MessageKey } from '../i18n'
import { requestPasswordReset, type AuthFieldErrors } from '../services/auth'

/**
 * Asks for a reset mail, replacing Laravel's password/email Blade page.
 *
 * THE SUCCESS SCREEN DOES NOT SAY A MAIL WAS SENT. The server answers the same whether or
 * not the address has an account — otherwise this form, which needs no credentials, would
 * tell anyone which addresses are registered here. So the copy is conditional ("if this
 * address is registered"), and the same screen appears for an address that has never been
 * seen and for one whose mail was throttled.
 */

const route = useRoute()
const locale = computed(() => normalizeLocale(route.params.locale))

const email = ref('')
const submitting = ref(false)
const sent = ref(false)
const generalError = ref('')
const fieldErrors = ref<AuthFieldErrors>({})

const limits = { emailMax: 50 }

const validationMessages: Record<string, MessageKey> = {
  required: 'validationRequired',
  too_long: 'validationTooLong',
  invalid_email: 'validationInvalidEmail',
}

function firstError(field: string): string {
  const code = fieldErrors.value[field]?.[0]
  if (!code) return ''
  const key = validationMessages[code]
  return key ? translate(locale.value, key) : code
}

async function submit(): Promise<void> {
  if (submitting.value) return

  submitting.value = true
  generalError.value = ''
  fieldErrors.value = {}

  try {
    const outcome = await requestPasswordReset(locale.value, email.value)
    if (outcome.ok) {
      sent.value = true
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
  document.title = `${translate(locale.value, 'forgotPasswordTitle')} · 2Pick`
})
</script>

<template>
  <div class="auth-page">
    <section class="auth-intro">
      <p class="eyebrow">{{ translate(locale, 'loginEyebrow') }}</p>
      <h1>{{ translate(locale, 'forgotPasswordTitle') }}</h1>
      <p class="auth-intro-copy">{{ translate(locale, 'forgotPasswordIntro') }}</p>
    </section>

    <section class="auth-card">
      <template v-if="sent">
        <p class="auth-notice" role="status">{{ translate(locale, 'forgotPasswordSent') }}</p>
        <p class="auth-tos">
          <RouterLink :to="localizedPath('/login', locale)">
            {{ translate(locale, 'forgotPasswordBackToLogin') }}
          </RouterLink>
        </p>
      </template>

      <template v-else>
        <p v-if="generalError" class="auth-error" role="alert">{{ generalError }}</p>

        <form class="auth-form" novalidate @submit.prevent="submit">
          <label class="auth-field">
            <span>{{ translate(locale, 'email') }}</span>
            <input
              v-model.trim="email"
              type="email"
              name="email"
              autocomplete="email"
              required
              :maxlength="limits.emailMax"
              :aria-invalid="Boolean(firstError('email'))"
            >
            <small v-if="firstError('email')" class="auth-field-error">{{ firstError('email') }}</small>
          </label>

          <button class="auth-submit" type="submit" :disabled="submitting">
            {{ submitting ? translate(locale, 'authSubmitting') : translate(locale, 'forgotPasswordSubmit') }}
          </button>
        </form>

        <p class="auth-tos">
          <RouterLink :to="localizedPath('/login', locale)">
            {{ translate(locale, 'forgotPasswordBackToLogin') }}
          </RouterLink>
        </p>
      </template>
    </section>
  </div>
</template>
