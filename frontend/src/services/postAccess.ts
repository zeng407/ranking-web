import { APIError, getAPIClient, onResponseHeaders, type APIClient } from '../lib/api'

/**
 * The door code on a password-protected post.
 *
 * The server does not remember who has entered what — Laravel kept it in the session, this
 * API has none — so the proof lives here as a signed token and travels on every request
 * that touches the post. What this module owns is where that token is kept, when it is
 * sent, and when it is thrown away.
 */

export const POST_ACCESS_HEADER = 'X-Post-Access'

/** How many tokens are ever sent at once. The server caps this too; see maxPostAccessTokens. */
const MAX_TOKENS = 10

const STORAGE_PREFIX = '2pick.post-access.'

export interface PostAccessGrant {
  token: string
  expires_at: string
  expires_in: number
}

interface StoredToken {
  token: string
  expiresAt: number
}

/**
 * sessionStorage, not localStorage: a door code shared for one sitting should not outlive
 * the tab. Closing it is the plainest way a visitor has of saying "I'm done here", and on
 * a shared computer it is the only one they will think of.
 */
function storage(): Storage | null {
  try {
    // globalThis rather than window: the setup in src/test/webStorage.ts installs the
    // storages globally, and a module that insists on window works in the browser and
    // silently degrades to memory under test — which would hide exactly the persistence
    // this is here to provide.
    return globalThis.sessionStorage ?? null
  } catch {
    // Blocked by the browser's privacy settings. Tokens then last as long as the page,
    // which still gets someone through one game.
    return null
  }
}

const memory = new Map<string, StoredToken>()

function keyFor(serial: string): string {
  return STORAGE_PREFIX + serial
}

export function rememberPostAccess(serial: string, token: string, expiresAt: number): void {
  const entry: StoredToken = { token, expiresAt }
  memory.set(serial, entry)
  storage()?.setItem(keyFor(serial), JSON.stringify(entry))
}

export function forgetPostAccess(serial: string): void {
  memory.delete(serial)
  storage()?.removeItem(keyFor(serial))
}

function readToken(serial: string): StoredToken | null {
  const cached = memory.get(serial)
  if (cached) return liveOrNull(serial, cached)

  const raw = storage()?.getItem(keyFor(serial))
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as StoredToken
    if (typeof parsed?.token !== 'string' || typeof parsed?.expiresAt !== 'number') return null
    memory.set(serial, parsed)
    return liveOrNull(serial, parsed)
  } catch {
    return null
  }
}

/**
 * Drops a token that has expired.
 *
 * Ten seconds of margin, because the token is checked by the server after the request
 * crosses the network: one that expires in flight is refused, and the visitor sees a post
 * vanish rather than a prompt.
 */
function liveOrNull(serial: string, entry: StoredToken): StoredToken | null {
  if (entry.expiresAt - 10_000 > Date.now()) return entry
  forgetPostAccess(serial)
  return null
}

export function hasPostAccess(serial: string): boolean {
  return readToken(serial) !== null
}

/**
 * Builds the header for a request.
 *
 * The serial being asked about goes first, so that if there are more tokens than the cap
 * allows it is the one that survives. The others ride along because one request can touch
 * a post the caller did not name — a game serial says nothing about which post it belongs
 * to until the server looks.
 */
export function postAccessHeaders(serial?: string): Record<string, string> {
  const pairs: string[] = []
  const seen = new Set<string>()

  if (serial) {
    const entry = readToken(serial)
    if (entry) {
      pairs.push(`${serial}:${entry.token}`)
      seen.add(serial)
    }
  }
  for (const key of knownSerials()) {
    if (pairs.length >= MAX_TOKENS) break
    if (seen.has(key)) continue
    const entry = readToken(key)
    if (entry) pairs.push(`${key}:${entry.token}`)
  }

  return pairs.length > 0 ? { [POST_ACCESS_HEADER]: pairs.join(', ') } : {}
}

function knownSerials(): string[] {
  const serials = new Set<string>(memory.keys())
  const store = storage()
  if (store) {
    for (let index = 0; index < store.length; index += 1) {
      const key = store.key(index)
      if (key?.startsWith(STORAGE_PREFIX)) serials.add(key.slice(STORAGE_PREFIX.length))
    }
  }
  return [...serials]
}

/**
 * Takes in a refreshed token from a response.
 *
 * The server reissues on every use, which is what stops a visitor being locked out of a
 * game that runs past the half hour. AccessTokenService::extendPostAccessToken did the
 * same thing by pushing the session entry's expiry forward.
 *
 * A refresh only ever replaces a token this client already had: it cannot be the first
 * time a serial is heard of, because the server would not have issued it.
 */
export function absorbPostAccessHeader(headers: Headers): void {
  const value = headers.get(POST_ACCESS_HEADER)
  if (!value) return
  for (const entry of value.split(',')) {
    const trimmed = entry.trim()
    const separator = trimmed.indexOf(':')
    if (separator <= 0) continue
    const serial = trimmed.slice(0, separator)
    const token = trimmed.slice(separator + 1)
    if (!token) continue
    const previous = memory.get(serial)
    if (!previous) continue
    rememberPostAccess(serial, token, Date.now() + TOKEN_TTL_MS)
  }
}

/** Thirty minutes, matching the server's postaccess.TTL. */
const TOKEN_TTL_MS = 30 * 60 * 1000

export type UnlockOutcome =
  | { ok: true }
  | { ok: false; kind: 'wrong-password' | 'not-found' | 'too-many' | 'unavailable' }

/**
 * Exchanges a password for a token and keeps it.
 *
 * The password itself is never stored — only what the server hands back for it. That is
 * the whole reason the endpoint returns a token instead of the client re-sending the
 * password on each request.
 */
export async function unlockPost(
  serial: string,
  password: string,
  client: APIClient = getAPIClient(),
  signal?: AbortSignal,
): Promise<UnlockOutcome> {
  try {
    const grant = await client.post<PostAccessGrant>(
      `/posts/${encodeURIComponent(serial)}/access`, { password }, signal, 'omit')
    const expiresAt = Date.parse(grant.expires_at)
    rememberPostAccess(serial, grant.token,
      Number.isNaN(expiresAt) ? Date.now() + grant.expires_in * 1000 : expiresAt)
    return { ok: true }
  } catch (failure) {
    if (failure instanceof APIError) {
      if (failure.status === 403) return { ok: false, kind: 'wrong-password' }
      if (failure.status === 404) return { ok: false, kind: 'not-found' }
      if (failure.status === 429) return { ok: false, kind: 'too-many' }
    }
    return { ok: false, kind: 'unavailable' }
  }
}

/**
 * Every response is watched for a reissued token, not only the ones a call site remembers
 * to check. A refresh that is dropped costs the visitor their game half an hour in, and
 * the only way to be sure none is dropped is to not rely on anyone remembering.
 */
onResponseHeaders(absorbPostAccessHeader)

export function resetPostAccessForTests(): void {
  for (const serial of knownSerials()) forgetPostAccess(serial)
  memory.clear()
}
