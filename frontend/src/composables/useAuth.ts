import { readonly, ref } from 'vue'

import { getAdminService } from '../services/admin'
import { logout as submitLogout, type AuthOutcome } from '../services/auth'
import { fetchProfile, getCachedSession, refreshSession, type SessionUser } from '../services/session'

/** App\Enums\Role::ADMIN, the slug the API checks. */
const ADMIN_ROLE = 'admin'
/** Where Go mounts the gated bundle, and the Path of the pass cookie. */
const ADMIN_CONSOLE_PATH = '/admin/'

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
/**
 * Whether this account holds the moderator role, read from the token's own claims.
 *
 * Only ever used to decide whether to OFFER the back office. Every admin endpoint checks
 * the role itself, and the bundle is served only against a pass the server mints, so a
 * client that flips this gains nothing but a link that answers 403.
 */
const isAdmin = ref(false)

function readRoles(): string[] {
  return getCachedSession()?.roles ?? []
}

/**
 * The resolution in flight, so two mounts that both need the answer share one request.
 *
 * The header resolves the session on every page load, and a view that cannot decide what to
 * render until it knows — the game page of an adult post — has to wait for the same answer.
 * Without this it would ask again, which is a second `/auth/refresh` racing the first.
 *
 * A forced call always goes to the server: its whole point is to re-resolve now.
 */
let resolving: Promise<void> | null = null

export function refreshAuthState(locale?: string, force = false): Promise<void> {
  if (resolving && !force) return resolving
  const attempt = resolveAuthState(locale, force).finally(() => {
    if (resolving === attempt) resolving = null
  })
  resolving = attempt
  return attempt
}

async function resolveAuthState(_locale: string | undefined, force: boolean): Promise<void> {
  loading.value = true

  try {
    // A cached token means this has already been resolved once in this page's life.
    // Skipped unless forced, so navigating between routes does not rotate the refresh
    // token on every view.
    if (!force && getCachedSession()) {
      authenticated.value = true
      isAdmin.value = readRoles().includes(ADMIN_ROLE)
      if (!user.value) user.value = await fetchProfile()
      return
    }

    const session = await refreshSession()
    authenticated.value = session !== null
    isAdmin.value = session?.roles.includes(ADMIN_ROLE) ?? false
    user.value = session ? await fetchProfile() : null
  } finally {
    loading.value = false
  }
}

/** Called after a sign-in, when the token is already in hand. */
export async function adoptSignedInState(): Promise<void> {
  authenticated.value = true
  isAdmin.value = readRoles().includes(ADMIN_ROLE)
  user.value = await fetchProfile()
}

/**
 * Takes a moderator to the back office.
 *
 * Two steps, in this order: mint the pass cookie, then leave with a FULL page load. The
 * back office is a separate bundle served by the Go process, not a route of this router, so
 * a router push would land on this app's catch-all instead.
 */
export async function enterAdminConsole(): Promise<boolean> {
  const outcome = await getAdminService().grantAssetPass()
  if (!outcome.ok) return false
  window.location.href = ADMIN_CONSOLE_PATH
  return true
}

async function signOut(): Promise<AuthOutcome> {
  // Dropped before the sign-out, while there is still a token to authorize it. The pass
  // expires within the hour on its own, so this is hygiene on a shared machine rather than
  // what stops a revoked moderator — and a failure here must not block signing out.
  if (isAdmin.value) await getAdminService().revokeAssetPass()

  const outcome = await submitLogout()

  // Cleared whatever the server said: the local session is gone either way, and leaving
  // the header showing an account the user just signed out of is worse than a stale
  // server-side row.
  authenticated.value = false
  user.value = null
  isAdmin.value = false

  return outcome
}

export function useAuth() {
  return {
    authenticated: readonly(authenticated),
    user: readonly(user),
    loading: readonly(loading),
    isAdmin: readonly(isAdmin),
    refreshAuthState,
    adoptSignedInState,
    enterAdminConsole,
    signOut,
  }
}
