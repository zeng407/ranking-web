import { readonly, ref } from 'vue'

import { logout as submitLogout, type AuthOutcome } from '../services/auth'
import { fetchProfile, getCachedSession, refreshSession, type SessionUser } from '../services/session'

/**
 * Auth state is module-level so the header, the login page and any future account view
 * all read one source and update together after a sign-in.
 *
 * The home page HTML is publicly cacheable, so the server cannot render the logged-in
 * state. It is resolved on the client instead, which is why `loading` exists: the header
 * must not flash a Login link at a user who is already signed in.
 *
 * Resolved from the Go API now rather than Laravel's `/session-context`. The mechanism
 * changed shape: there is no endpoint that reports "are you logged in". Instead the
 * refresh cookie is exchanged for a token, and getting one back IS the answer.
 */
const authenticated = ref(false)
const user = ref<SessionUser | null>(null)
const loading = ref(true)

export async function refreshAuthState(_locale?: string, force = false): Promise<void> {
  loading.value = true

  try {
    // A cached token means this has already been resolved once in this page's life.
    // Skipped unless forced, so navigating between routes does not rotate the refresh
    // token on every view.
    if (!force && getCachedSession()) {
      authenticated.value = true
      if (!user.value) user.value = await fetchProfile()
      return
    }

    const session = await refreshSession()
    authenticated.value = session !== null
    user.value = session ? await fetchProfile() : null
  } finally {
    loading.value = false
  }
}

/** Called after a sign-in, when the token is already in hand. */
export async function adoptSignedInState(): Promise<void> {
  authenticated.value = true
  user.value = await fetchProfile()
}

async function signOut(): Promise<AuthOutcome> {
  const outcome = await submitLogout()

  // Cleared whatever the server said: the local session is gone either way, and leaving
  // the header showing an account the user just signed out of is worse than a stale
  // server-side row.
  authenticated.value = false
  user.value = null

  return outcome
}

export function useAuth() {
  return {
    authenticated: readonly(authenticated),
    user: readonly(user),
    loading: readonly(loading),
    refreshAuthState,
    adoptSignedInState,
    signOut,
  }
}
