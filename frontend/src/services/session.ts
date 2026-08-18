import { getAPIClient, APIError, type APIClient } from '../lib/api'

/**
 * The Go API's session, replacing Laravel's `/session-context`.
 *
 * WHERE THE TWO HALVES LIVE. The access token is held in this module and nowhere else —
 * not localStorage, not a cookie. It is short-lived (five minutes) and has to reach an
 * Authorization header, so script must be able to read it; keeping it in memory means a
 * closed tab takes it with it. The long-lived half is the refresh token, which the
 * server sets as an httpOnly cookie that script cannot read at all. That split is the
 * point: an injected script can steal at most five minutes.
 *
 * The CSRF value is deliberately in a *readable* cookie. A cross-site request carries
 * the refresh cookie automatically but cannot read this one to copy it into the header,
 * which is what stops it.
 */

export interface SessionUser {
  user_id: string
  name: string
  avatar_url: string | null
  has_password: boolean
  linked_google: boolean
}

export interface Session {
  accessToken: string
  expiresAt: number
  userId: string
  roles: string[]
}

export interface GrantBody {
  access_token: string
  token_type: string
  expires_in: number
  csrf_token: string
  user_id: string
  roles: string[]
}

interface IdentityBody {
  subject: string
  roles: string[]
  expires_at: string
  user?: SessionUser
}

export const CSRF_COOKIE = '2pick_csrf'
export const CSRF_HEADER = 'X-CSRF-Token'

/**
 * Refreshed this many milliseconds before the token actually expires, so a request that
 * is already in flight when the clock runs out does not arrive with a dead token.
 */
const REFRESH_SKEW_MS = 30_000

let session: Session | null = null
/**
 * SINGLE FLIGHT, AND NOT AS AN OPTIMISATION.
 *
 * The server rotates refresh tokens and treats a second use of one as theft: it revokes
 * the whole family and signs the user out everywhere. Two components mounting at once
 * and each calling refresh would present the same cookie twice, and the second call
 * would look exactly like a stolen token being replayed. Sharing one promise is what
 * makes concurrent callers safe, not merely cheaper.
 */
let inFlight: Promise<Session | null> | null = null

export function readCSRFToken(): string | null {
  // document is absent under the test runner's node environment, and on a server render.
  if (typeof document === 'undefined') return null
  for (const entry of document.cookie.split(';')) {
    const [name, ...value] = entry.trim().split('=')
    if (name === CSRF_COOKIE) return decodeURIComponent(value.join('='))
  }
  return null
}

export function getCachedSession(): Session | null {
  return session
}

/** Forgets the local session without telling the server. For a 401 on any other call. */
export function clearSession(): void {
  session = null
}

/**
 * Returns a usable access token, refreshing when the cached one is missing or close to
 * expiry. Null means "not signed in" — which is an answer, not a failure.
 */
export async function getAccessToken(client: APIClient = getAPIClient()): Promise<string | null> {
  if (session && session.expiresAt - REFRESH_SKEW_MS > Date.now()) return session.accessToken
  const refreshed = await refreshSession(client)
  return refreshed?.accessToken ?? null
}

export async function refreshSession(client: APIClient = getAPIClient()): Promise<Session | null> {
  if (inFlight) return inFlight

  const csrf = readCSRFToken()
  // No CSRF cookie means no session was ever established in this browser. Calling
  // refresh anyway would be a guaranteed 401, and on the server side an unnecessary
  // "invalid token" to log.
  //
  // Answered before anything is parked in `inFlight`, and that placement is the point: a
  // body with no `await` in it runs to completion — its `finally` included — during the
  // call that creates the promise, so the assignment below would overwrite the cleanup and
  // leave a settled promise parked forever. Every later refresh in the page's life would
  // then return that stale null, and a visitor who signed in would stay anonymous to the
  // app until a full reload.
  if (!csrf) {
    session = null
    return null
  }

  inFlight = (async () => {
    try {
      const grant = await client.post<GrantBody>('/auth/refresh', {}, undefined, 'include', {
        [CSRF_HEADER]: csrf,
      })
      session = adopt(grant)
      return session
    } catch (error) {
      // A 401 here is the ordinary "the session ended" case: expired, revoked, or the
      // family was killed by a replay. Anything else is a real fault, and the answer is
      // still that there is no usable session.
      if (!(error instanceof APIError)) throw error
      session = null
      return null
    } finally {
      inFlight = null
    }
  })()

  return inFlight
}

export async function signIn(
  email: string,
  password: string,
  client: APIClient = getAPIClient(),
): Promise<Session> {
  const grant = await client.post<GrantBody>('/auth/login', { email, password }, undefined, 'include')
  session = adopt(grant)
  return session
}

export async function signOut(client: APIClient = getAPIClient()): Promise<void> {
  const csrf = readCSRFToken()
  try {
    // The cookies are cleared by the server's response, so a local reset alone would
    // leave a live session behind on the other side.
    await client.post('/auth/logout', {}, undefined, 'include', csrf ? { [CSRF_HEADER]: csrf } : undefined)
  } catch (error) {
    // Signing out must succeed locally whatever the server says. A user who cannot
    // clear their own session because the request failed has no way out.
    if (!(error instanceof APIError)) throw error
  } finally {
    session = null
  }
}

/** The current account. Null when not signed in. */
export async function fetchProfile(client: APIClient = getAPIClient()): Promise<SessionUser | null> {
  const token = await getAccessToken(client)
  if (!token) return null
  try {
    const identity = await client.get<IdentityBody>('/auth/me', undefined, 'include', {
      Authorization: `Bearer ${token}`,
    })
    return identity.user ?? null
  } catch (error) {
    if (!(error instanceof APIError)) throw error
    // The token was rejected despite being fresh: the account is gone or was banned.
    if (error.status === 401) session = null
    return null
  }
}

/**
 * Where to send the browser to sign in with Google.
 *
 * A full navigation rather than fetch: the flow ends at Google's consent screen, which
 * cannot be embedded, and the callback that follows has to be a navigation so the
 * server can set the session cookies on it.
 */
export function googleSignInURL(baseUrl: string, returnTo: string): string {
  const query = new URLSearchParams({ return_to: returnTo })
  return `${baseUrl.replace(/\/+$/, '')}/auth/oauth/google/start?${query}`
}

/**
 * Takes a grant the caller already has, without spending a request to fetch one.
 *
 * Registration needs this: the sign-up response IS a grant, and calling refresh
 * afterwards would rotate a token that is one millisecond old.
 */
export function adoptGrant(grant: GrantBody): Session {
  session = adopt(grant)
  return session
}

function adopt(grant: GrantBody): Session {
  return {
    accessToken: grant.access_token,
    // Derived from expires_in rather than trusting a server timestamp against a client
    // clock that may be wrong by hours.
    expiresAt: Date.now() + grant.expires_in * 1000,
    userId: grant.user_id,
    roles: grant.roles ?? [],
  }
}

/** Test seam: forget both the session and any in-flight refresh. */
export function resetSessionForTests(): void {
  session = null
  inFlight = null
}
