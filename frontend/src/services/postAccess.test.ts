import { beforeEach, describe, expect, it, vi } from 'vitest'

import { APIError, type APIClient } from '../lib/api'
import {
  POST_ACCESS_HEADER,
  absorbPostAccessHeader,
  forgetPostAccess,
  hasPostAccess,
  postAccessHeaders,
  rememberPostAccess,
  resetPostAccessForTests,
  unlockPost,
} from './postAccess'

/**
 * The door code as the browser holds it.
 *
 * What matters is not that a token round-trips. It is that an expired one is not sent, a
 * refreshed one replaces what it refreshes, and a token never arrives for a post this
 * client was not already holding one for — the last because the reissue header is the one
 * place a response can write into this store.
 */

const HOUR = 60 * 60 * 1000

function fakeClient(overrides: Partial<APIClient> = {}): APIClient {
  return {
    get: vi.fn(),
    post: vi.fn().mockResolvedValue({
      token: 'the-token', expires_at: new Date(Date.now() + 30 * 60 * 1000).toISOString(), expires_in: 1800,
    }),
    put: vi.fn(),
    delete: vi.fn(),
    postForm: vi.fn(),
    ...overrides,
  }
}

function apiError(status: number): APIError {
  return new APIError(status, { error: { code: 'x', message: 'no' } } as never)
}

describe('postAccess', () => {
  beforeEach(() => {
    resetPostAccessForTests()
  })

  it('sends a stored token for the post being asked about', () => {
    rememberPostAccess('abcdefgh', 'the-token', Date.now() + HOUR)

    expect(postAccessHeaders('abcdefgh')).toEqual({ [POST_ACCESS_HEADER]: 'abcdefgh:the-token' })
  })

  it('sends nothing at all when there is no token', () => {
    expect(postAccessHeaders('abcdefgh')).toEqual({})
  })

  /**
   * A game serial says nothing about which post it belongs to, so a request that names no
   * post still has to carry whatever the visitor has proved.
   */
  it('sends every token when no post is named', () => {
    rememberPostAccess('abcdefgh', 'first', Date.now() + HOUR)
    rememberPostAccess('ijklmnop', 'second', Date.now() + HOUR)

    const header = postAccessHeaders()[POST_ACCESS_HEADER] ?? ''

    expect(header).toContain('abcdefgh:first')
    expect(header).toContain('ijklmnop:second')
  })

  // The named post goes first, so that if the cap trims the list it is not the one the
  // request is actually about that gets dropped.
  it('puts the named post first', () => {
    rememberPostAccess('ijklmnop', 'second', Date.now() + HOUR)
    rememberPostAccess('abcdefgh', 'first', Date.now() + HOUR)

    expect(postAccessHeaders('abcdefgh')[POST_ACCESS_HEADER]).toMatch(/^abcdefgh:first/)
  })

  it('never sends more than the cap', () => {
    for (let index = 0; index < 25; index += 1) {
      rememberPostAccess(`post-${index}`, 'token', Date.now() + HOUR)
    }

    const pairs = (postAccessHeaders()[POST_ACCESS_HEADER] ?? '').split(',')

    expect(pairs.length).toBeLessThanOrEqual(10)
  })

  // An expired token is dead weight: the server refuses it and the visitor sees the post
  // vanish rather than a prompt, so it is dropped before it is sent.
  it('drops an expired token instead of sending it', () => {
    rememberPostAccess('abcdefgh', 'stale', Date.now() - 1000)

    expect(hasPostAccess('abcdefgh')).toBe(false)
    expect(postAccessHeaders('abcdefgh')).toEqual({})
  })

  // Expiry is judged with a margin, because the server checks the token after the request
  // has crossed the network.
  it('treats a token about to expire as already gone', () => {
    rememberPostAccess('abcdefgh', 'nearly-stale', Date.now() + 5_000)

    expect(hasPostAccess('abcdefgh')).toBe(false)
  })

  it('takes in a refreshed token from a response', () => {
    rememberPostAccess('abcdefgh', 'the-old-token', Date.now() + 60_000)

    absorbPostAccessHeader(new Headers({ [POST_ACCESS_HEADER]: 'abcdefgh:the-new-token' }))

    expect(postAccessHeaders('abcdefgh')).toEqual({ [POST_ACCESS_HEADER]: 'abcdefgh:the-new-token' })
  })

  /**
   * A REFRESH MUST NOT BE ABLE TO CREATE ACCESS.
   *
   * The header is written by whatever answered the request. A response that named a post
   * this client never unlocked would otherwise plant a token for it, and every later
   * request would present it — so a refresh only ever replaces something already held.
   */
  it('ignores a refresh for a post it holds no token for', () => {
    absorbPostAccessHeader(new Headers({ [POST_ACCESS_HEADER]: 'abcdefgh:a-token-from-nowhere' }))

    expect(hasPostAccess('abcdefgh')).toBe(false)
  })

  it('ignores a malformed refresh without disturbing what it holds', () => {
    rememberPostAccess('abcdefgh', 'the-token', Date.now() + HOUR)

    absorbPostAccessHeader(new Headers({ [POST_ACCESS_HEADER]: 'no-colon' }))
    absorbPostAccessHeader(new Headers({ [POST_ACCESS_HEADER]: 'abcdefgh:' }))

    expect(postAccessHeaders('abcdefgh')).toEqual({ [POST_ACCESS_HEADER]: 'abcdefgh:the-token' })
  })

  // The token itself contains a dot and may contain anything base64url does; only the
  // first colon separates it from the serial.
  it('keeps the whole token when a refresh arrives', () => {
    rememberPostAccess('abcdefgh', 'old', Date.now() + HOUR)

    absorbPostAccessHeader(new Headers({ [POST_ACCESS_HEADER]: 'abcdefgh:1234567890.c2ln' }))

    expect(postAccessHeaders('abcdefgh')[POST_ACCESS_HEADER]).toBe('abcdefgh:1234567890.c2ln')
  })

  it('forgets a token on request', () => {
    rememberPostAccess('abcdefgh', 'the-token', Date.now() + HOUR)

    forgetPostAccess('abcdefgh')

    expect(hasPostAccess('abcdefgh')).toBe(false)
  })

  it('exchanges a password for a token and keeps it', async () => {
    const client = fakeClient()

    const outcome = await unlockPost('abcdefgh', 'door-code', client)

    expect(outcome).toEqual({ ok: true })
    expect(client.post).toHaveBeenCalledWith('/posts/abcdefgh/access', { password: 'door-code' }, undefined, 'omit')
    expect(postAccessHeaders('abcdefgh')).toEqual({ [POST_ACCESS_HEADER]: 'abcdefgh:the-token' })
  })

  // The password is what the token exists to replace. Keeping it would mean the browser
  // held the shared secret for as long as the tab was open.
  it('does not keep the password anywhere', async () => {
    await unlockPost('abcdefgh', 'door-code', fakeClient())

    expect(JSON.stringify(globalThis.sessionStorage)).not.toContain('door-code')
  })

  it('maps each refusal to its own outcome', async () => {
    const cases: Array<[number, string]> = [
      [403, 'wrong-password'], [404, 'not-found'], [429, 'too-many'], [500, 'unavailable'],
    ]
    for (const [status, kind] of cases) {
      const client = fakeClient({ post: vi.fn().mockRejectedValue(apiError(status)) })
      expect(await unlockPost('abcdefgh', 'wrong', client)).toEqual({ ok: false, kind })
    }
  })

  it('reports a network failure as unavailable', async () => {
    const client = fakeClient({ post: vi.fn().mockRejectedValue(new TypeError('network down')) })

    expect(await unlockPost('abcdefgh', 'door-code', client)).toEqual({ ok: false, kind: 'unavailable' })
  })

  // A refused unlock must not leave a token behind, or every later request would present
  // one the server has never issued.
  it('stores nothing when the password is refused', async () => {
    const client = fakeClient({ post: vi.fn().mockRejectedValue(apiError(403)) })

    await unlockPost('abcdefgh', 'wrong', client)

    expect(hasPostAccess('abcdefgh')).toBe(false)
  })

  it('escapes the serial into the path', async () => {
    const client = fakeClient()

    await unlockPost('a/b?c', 'door-code', client)

    expect(client.post).toHaveBeenCalledWith('/posts/a%2Fb%3Fc/access', expect.anything(), undefined, 'omit')
  })

  // sessionStorage, not localStorage: closing the tab is the plainest way a visitor has
  // of ending a session on a shared computer.
  it('keeps tokens in sessionStorage and not localStorage', () => {
    rememberPostAccess('abcdefgh', 'the-token', Date.now() + HOUR)

    expect(globalThis.sessionStorage.getItem('2pick.post-access.abcdefgh')).toContain('the-token')
    expect(globalThis.localStorage.getItem('2pick.post-access.abcdefgh')).toBeNull()
  })
})
