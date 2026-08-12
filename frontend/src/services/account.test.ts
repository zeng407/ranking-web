import { beforeEach, describe, expect, it, vi } from 'vitest'

import { APIError, type APIClient } from '../lib/api'
import { createAccountService, nameChangeIsBlocked, type Account } from './account'
import { adoptGrant, resetSessionForTests } from './session'

/**
 * The account service. What matters here is the shape of the requests — which verb, which
 * path, and that the bearer token is attached — plus that each failure becomes the right
 * outcome rather than an exception the view has to catch.
 */

const account: Account = {
  name: 'the holder',
  email: 'holder@example.test',
  avatar_url: 'https://file.2pick.test/avatars/a.png',
  has_password: false,
  google_linked: true,
}

function fakeClient(overrides: Partial<APIClient> = {}): APIClient {
  return {
    get: vi.fn().mockResolvedValue(account),
    post: vi.fn().mockResolvedValue(grantBody()),
    put: vi.fn().mockResolvedValue(account),
    postForm: vi.fn().mockResolvedValue({ avatar_url: 'https://file.2pick.test/avatars/new.png' }),
    delete: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function grantBody() {
  return {
    access_token: 'the-new-access-token',
    token_type: 'Bearer',
    expires_in: 300,
    csrf_token: 'the-new-csrf',
    user_id: '42',
    roles: [],
  }
}

function apiError(status: number, data?: unknown): APIError {
  return new APIError(status, { data, error: { code: 'x', message: 'no' } } as never)
}

/** A session in hand, so getAccessToken does not spend a refresh request. */
function signedIn(): void {
  adoptGrant(grantBody())
}

describe('createAccountService', () => {
  beforeEach(() => {
    resetSessionForTests()
  })

  it('loads the account with a bearer token', async () => {
    signedIn()
    const client = fakeClient()

    const outcome = await createAccountService(client).load()

    expect(outcome).toEqual({ ok: true, value: account })
    expect(client.get).toHaveBeenCalledWith('/account/profile', undefined, 'include', {
      Authorization: 'Bearer the-new-access-token',
    })
  })

  it('renames with PUT and reports the account it gets back', async () => {
    signedIn()
    const client = fakeClient({ put: vi.fn().mockResolvedValue({ ...account, name: 'after' }) })

    const outcome = await createAccountService(client).rename('after')

    expect(outcome).toEqual({ ok: true, value: { ...account, name: 'after' } })
    expect(client.put).toHaveBeenCalledWith('/account/profile', { name: 'after' },
      undefined, 'include', { Authorization: 'Bearer the-new-access-token' })
  })

  it('uploads the avatar as a multipart part named avatar', async () => {
    signedIn()
    const client = fakeClient()

    const outcome = await createAccountService(client).uploadAvatar(
      new File(['bytes'], 'me.png', { type: 'image/png' }))

    expect(outcome).toEqual({ ok: true, value: 'https://file.2pick.test/avatars/new.png' })
    const call = (client.postForm as ReturnType<typeof vi.fn>).mock.calls[0]
    if (!call) throw new Error('postForm was not called')
    const [path, form] = call
    expect(path).toBe('/account/avatar')
    // The field name is what the server reads the part out of, so it is load-bearing.
    expect((form as FormData).get('avatar')).toBeInstanceOf(File)
  })

  /**
   * THE CHANGE ENDS EVERY SESSION, INCLUDING THIS ONE. The response is a fresh grant, and
   * adopting it is what keeps the page usable — without it the next request would 401 and
   * the user would be thrown to the login form by their own password change.
   */
  it('adopts the grant a password change answers with', async () => {
    signedIn()
    const client = fakeClient({
      put: vi.fn().mockResolvedValue({ ...grantBody(), access_token: 'issued-by-the-change' }),
    })
    const service = createAccountService(client)

    const outcome = await service.changePassword('the-old-password', 'the-new-password')
    expect(outcome.ok).toBe(true)

    // The next call carries the token the change issued, not the one from before it.
    await service.load()
    expect(client.get).toHaveBeenCalledWith('/account/profile', undefined, 'include', {
      Authorization: 'Bearer issued-by-the-change',
    })
  })

  it('sends the current password on a change and none on a first-time set', async () => {
    signedIn()
    // Both password endpoints answer with a grant, not with an account: the fake has to
    // match that, or adopting the response leaves a session with no token in it and the
    // next call reports itself signed out.
    const client = fakeClient({ put: vi.fn().mockResolvedValue(grantBody()) })
    const service = createAccountService(client)

    await service.changePassword('the-old-password', 'the-new-password')
    expect(client.put).toHaveBeenCalledWith('/account/password',
      { current_password: 'the-old-password', new_password: 'the-new-password' },
      undefined, 'include', expect.anything())

    await service.setInitialPassword('the-new-password')
    expect(client.post).toHaveBeenCalledWith('/account/password',
      { new_password: 'the-new-password' }, undefined, 'include', expect.anything())
  })

  it('turns a 422 into per-field codes', async () => {
    signedIn()
    const client = fakeClient({
      put: vi.fn().mockRejectedValue(apiError(422, { errors: { name: ['name_change_too_soon'] } })),
    })

    const outcome = await createAccountService(client).rename('after')

    expect(outcome).toEqual({
      ok: false, kind: 'validation', errors: { name: ['name_change_too_soon'] },
    })
  })

  it('reports a 401 as signed out rather than as a failure', async () => {
    signedIn()
    const client = fakeClient({ get: vi.fn().mockRejectedValue(apiError(401)) })

    const outcome = await createAccountService(client).load()

    expect(outcome).toEqual({ ok: false, kind: 'signed-out' })
  })

  // No session at all: nothing is sent, because every one of these endpoints acts on the
  // account the token names and there is no token to name one.
  it('does not send a request without a session', async () => {
    const client = fakeClient({ get: vi.fn(), put: vi.fn() })

    expect(await createAccountService(client).load()).toEqual({ ok: false, kind: 'signed-out' })
    expect(client.get).not.toHaveBeenCalled()
  })

  it('reports anything else as unavailable', async () => {
    signedIn()
    for (const failure of [apiError(500), new TypeError('network down')]) {
      const client = fakeClient({ get: vi.fn().mockRejectedValue(failure) })
      expect(await createAccountService(client).load()).toEqual({ ok: false, kind: 'unavailable' })
    }
  })

  // A stale token would be one the page picked up when it opened; an access token lasts
  // five minutes and a settings page stays open longer than that.
  it('reads the token again for every request', async () => {
    signedIn()
    const client = fakeClient()
    const service = createAccountService(client)

    await service.load()
    adoptGrant({ ...grantBody(), access_token: 'rotated' })
    await service.load()

    const calls = (client.get as ReturnType<typeof vi.fn>).mock.calls
    expect(calls).toHaveLength(2)
    expect(calls[0]?.[3]).toEqual({ Authorization: 'Bearer the-new-access-token' })
    expect(calls[1]?.[3]).toEqual({ Authorization: 'Bearer rotated' })
  })
})

describe('nameChangeIsBlocked', () => {
  const now = Date.parse('2026-08-06T12:00:00Z')

  it('is false when the server reported no limit', () => {
    expect(nameChangeIsBlocked(account, now)).toBe(false)
    expect(nameChangeIsBlocked(null, now)).toBe(false)
  })

  it('is true while the moment the server gave is still ahead', () => {
    expect(nameChangeIsBlocked(
      { ...account, name_change_allowed_at: '2026-08-07T00:00:00Z' }, now)).toBe(true)
  })

  it('is false once that moment has passed', () => {
    expect(nameChangeIsBlocked(
      { ...account, name_change_allowed_at: '2026-08-06T00:00:00Z' }, now)).toBe(false)
  })

  // A value this build cannot parse must not lock the field: the server is the one that
  // enforces the rule, and a stuck form would be a worse failure than an extra 422.
  it('is false for an unparseable value', () => {
    expect(nameChangeIsBlocked({ ...account, name_change_allowed_at: 'tomorrow' }, now)).toBe(false)
  })
})
