// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { APIError, type APIClient } from '../lib/api'
import {
  CSRF_COOKIE,
  adoptGrant,
  fetchProfile,
  getAccessToken,
  getCachedSession,
  readCSRFToken,
  refreshSession,
  resetSessionForTests,
  signIn,
  signOut,
} from './session'

function grant(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    access_token: 'header.claims.signature',
    token_type: 'Bearer',
    expires_in: 300,
    csrf_token: 'the-csrf-token',
    user_id: '42',
    roles: [],
    ...overrides,
  }
}

function apiError(status: number): APIError {
  return new APIError(status, { error: { code: 'session_expired', message: 'gone' } } as never)
}

function setCSRFCookie(value = 'cookie-csrf'): void {
  document.cookie = `${CSRF_COOKIE}=${value}; path=/`
}

function clearCookies(): void {
  for (const entry of document.cookie.split(';')) {
    const name = entry.trim().split('=')[0]
    if (name) document.cookie = `${name}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT`
  }
}

describe('session', () => {
  beforeEach(() => {
    resetSessionForTests()
    clearCookies()
  })

  it('signs in and caches the token in memory rather than in storage', async () => {
    const post = vi.fn().mockResolvedValue(grant())
    const client = { get: vi.fn(), post } as unknown as APIClient

    const session = await signIn('player@example.test', 'secret', client)

    expect(post).toHaveBeenCalledWith(
      '/auth/login',
      { email: 'player@example.test', password: 'secret' },
      undefined,
      'include',
    )
    expect(session.accessToken).toBe('header.claims.signature')
    expect(getCachedSession()?.accessToken).toBe('header.claims.signature')
    // The refresh token lives only in an httpOnly cookie the server set; nothing about
    // it may be written here, and the access token must not be persisted either.
    expect(localStorage.getItem('header.claims.signature')).toBeNull()
    expect(JSON.stringify(localStorage)).not.toContain('header.claims.signature')
  })

  it('reuses a cached token instead of rotating on every call', async () => {
    const post = vi.fn().mockResolvedValue(grant())
    const client = { get: vi.fn(), post } as unknown as APIClient

    await signIn('player@example.test', 'secret', client)
    post.mockClear()

    expect(await getAccessToken(client)).toBe('header.claims.signature')
    expect(await getAccessToken(client)).toBe('header.claims.signature')
    expect(post).not.toHaveBeenCalled()
  })

  it('refreshes when the cached token is inside the expiry skew', async () => {
    const post = vi.fn().mockResolvedValue(grant({ access_token: 'rotated', expires_in: 300 }))
    const client = { get: vi.fn(), post } as unknown as APIClient
    setCSRFCookie()

    // A token with 10 seconds left is inside the 30 second skew, so it must not be used:
    // a request already in flight would arrive after it died.
    adoptGrant(grant({ access_token: 'nearly-dead', expires_in: 10 }) as never)

    expect(await getAccessToken(client)).toBe('rotated')
    expect(post).toHaveBeenCalledTimes(1)
  })

  /**
   * THE ONE THAT MATTERS. The server treats a second use of a refresh token as theft and
   * revokes the whole family, so two concurrent refreshes would sign the user out
   * everywhere. Anything that makes this fire twice is a logout bug, not a performance
   * bug.
   */
  it('collapses concurrent refreshes into a single request', async () => {
    let resolve: (value: unknown) => void = () => {}
    const post = vi.fn().mockReturnValue(new Promise((r) => { resolve = r }))
    const client = { get: vi.fn(), post } as unknown as APIClient
    setCSRFCookie()

    const first = refreshSession(client)
    const second = refreshSession(client)
    const third = getAccessToken(client)

    resolve(grant({ access_token: 'rotated-once' }))
    const [a, b, c] = await Promise.all([first, second, third])

    expect(post).toHaveBeenCalledTimes(1)
    expect(a?.accessToken).toBe('rotated-once')
    expect(b?.accessToken).toBe('rotated-once')
    expect(c).toBe('rotated-once')
  })

  it('allows a later refresh after the first one settles', async () => {
    const post = vi.fn()
      .mockResolvedValueOnce(grant({ access_token: 'first' }))
      .mockResolvedValueOnce(grant({ access_token: 'second' }))
    const client = { get: vi.fn(), post } as unknown as APIClient
    setCSRFCookie()

    expect((await refreshSession(client))?.accessToken).toBe('first')
    expect((await refreshSession(client))?.accessToken).toBe('second')
    expect(post).toHaveBeenCalledTimes(2)
  })

  it('sends the CSRF value from the readable cookie as a header', async () => {
    const post = vi.fn().mockResolvedValue(grant())
    const client = { get: vi.fn(), post } as unknown as APIClient
    setCSRFCookie('value-from-cookie')

    await refreshSession(client)

    expect(post).toHaveBeenCalledWith('/auth/refresh', {}, undefined, 'include', {
      'X-CSRF-Token': 'value-from-cookie',
    })
  })

  it('does not call refresh at all when there is no CSRF cookie', async () => {
    const post = vi.fn()
    const client = { get: vi.fn(), post } as unknown as APIClient

    // No cookie means no session was ever established in this browser. The request would
    // be a guaranteed 401 and an "invalid token" in the server's log for no reason.
    expect(await refreshSession(client)).toBeNull()
    expect(await getAccessToken(client)).toBeNull()
    expect(post).not.toHaveBeenCalled()
  })

  it('treats a rejected refresh as signed out rather than as an error', async () => {
    const post = vi.fn().mockRejectedValue(apiError(401))
    const client = { get: vi.fn(), post } as unknown as APIClient
    setCSRFCookie()
    adoptGrant(grant() as never)

    expect(await refreshSession(client)).toBeNull()
    expect(getCachedSession()).toBeNull()
  })

  it('rethrows a failure that is not an API error', async () => {
    const post = vi.fn().mockRejectedValue(new TypeError('network down'))
    const client = { get: vi.fn(), post } as unknown as APIClient
    setCSRFCookie()

    await expect(refreshSession(client)).rejects.toThrow('network down')
  })

  it('clears the local session on sign out even when the request fails', async () => {
    const post = vi.fn().mockRejectedValue(apiError(500))
    const client = { get: vi.fn(), post } as unknown as APIClient
    setCSRFCookie()
    adoptGrant(grant() as never)

    await signOut(client)

    // A user who cannot clear their own session because the request failed has no way
    // out of a broken state.
    expect(getCachedSession()).toBeNull()
  })

  it('reads the profile with the bearer token', async () => {
    const get = vi.fn().mockResolvedValue({
      subject: '42',
      roles: [],
      expires_at: '2026-08-06T00:00:00Z',
      user: { user_id: '42', name: 'Player', avatar_url: null, has_password: true, linked_google: false },
    })
    const client = { get, post: vi.fn() } as unknown as APIClient
    adoptGrant(grant() as never)

    const profile = await fetchProfile(client)

    expect(get).toHaveBeenCalledWith('/auth/me', undefined, 'include', {
      Authorization: 'Bearer header.claims.signature',
    })
    expect(profile?.name).toBe('Player')
  })

  it('forgets the session when the profile call is refused', async () => {
    const get = vi.fn().mockRejectedValue(apiError(401))
    const client = { get, post: vi.fn() } as unknown as APIClient
    adoptGrant(grant() as never)

    // A token that verifies but is refused means the account is gone or banned. Holding
    // on to it would leave the header showing an account that cannot do anything.
    expect(await fetchProfile(client)).toBeNull()
    expect(getCachedSession()).toBeNull()
  })

  it('reads a CSRF cookie that sits among others', () => {
    document.cookie = 'other=1; path=/'
    setCSRFCookie('mine')
    document.cookie = 'another=2; path=/'

    expect(readCSRFToken()).toBe('mine')
  })

  it('returns null rather than a wrong value when the cookie is absent', () => {
    document.cookie = 'not_the_csrf_cookie=value; path=/'
    expect(readCSRFToken()).toBeNull()
  })
})
