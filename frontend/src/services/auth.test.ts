// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { APIError, type APIClient } from '../lib/api'
import {
  login,
  logout,
  readOAuthResult,
  register,
  requestPasswordReset,
  resetPassword,
} from './auth'
import { getCachedSession, resetSessionForTests } from './session'

function grant() {
  return {
    access_token: 'header.claims.signature',
    token_type: 'Bearer',
    expires_in: 300,
    csrf_token: 'the-csrf-token',
    user_id: '42',
    roles: [],
  }
}

function apiError(status: number, data?: unknown): APIError {
  return new APIError(status, {
    error: { code: 'validation_failed', message: 'no' },
    data,
  } as never)
}

describe('auth service', () => {
  beforeEach(() => {
    resetSessionForTests()
  })

  it('posts credentials straight to Go with no CSRF round trip first', async () => {
    const post = vi.fn().mockResolvedValue(grant())
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await login('zh_TW', { email: ' player@example.test ', password: 'secret' }, client)

    expect(outcome).toEqual({ ok: true })
    // One request. The Laravel flow needed a /session-context call first to read a CSRF
    // token, then a POST, then a retry when the token had expired.
    expect(post).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledWith(
      '/auth/login',
      // Trimmed here, because the server compares addresses with the column collation
      // and a padded value would simply miss.
      { email: 'player@example.test', password: 'secret' },
      undefined,
      'include',
    )
    expect(getCachedSession()?.accessToken).toBe('header.claims.signature')
  })

  it('reports a rejected password as credentials, not as an outage', async () => {
    const post = vi.fn().mockRejectedValue(apiError(401))
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await login('zh_TW', { email: 'player@example.test', password: 'wrong' }, client)

    // The distinction matters to the form: 'credentials' shows "check your password",
    // 'unavailable' shows "the service is down". Collapsing them would tell users to
    // wait when their password is simply wrong.
    expect(outcome).toEqual({ ok: false, kind: 'credentials' })
  })

  it('reports a server fault as unavailable', async () => {
    const post = vi.fn().mockRejectedValue(apiError(500))
    const client = { get: vi.fn(), post } as unknown as APIClient

    expect(await login('zh_TW', { email: 'a@b.test', password: 'secret' }, client))
      .toEqual({ ok: false, kind: 'unavailable' })
  })

  it('reports a network failure as unavailable rather than throwing', async () => {
    const post = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))
    const client = { get: vi.fn(), post } as unknown as APIClient

    expect(await login('zh_TW', { email: 'a@b.test', password: 'secret' }, client))
      .toEqual({ ok: false, kind: 'unavailable' })
  })

  it('rejects an empty form without spending a request', async () => {
    const post = vi.fn()
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await login('zh_TW', { email: '  ', password: '' }, client)

    expect(outcome).toEqual({
      ok: false,
      kind: 'validation',
      errors: { email: ['required'], password: ['required'] },
    })
    expect(post).not.toHaveBeenCalled()
  })

  it('signs the user in as part of registering', async () => {
    const post = vi.fn().mockResolvedValue(grant())
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await register('zh_TW', {
      name: 'New Player',
      email: 'new@example.test',
      password: 'a-good-password',
      password_confirmation: 'a-good-password',
    }, client)

    expect(outcome).toEqual({ ok: true })
    // Matching Laravel's RegistersUsers trait, which logged the user in on success.
    // Refreshing instead would rotate a token that is one millisecond old.
    expect(getCachedSession()?.accessToken).toBe('header.claims.signature')
    expect(post).toHaveBeenCalledTimes(1)
  })

  it('surfaces per-field codes from a refused sign-up', async () => {
    const post = vi.fn().mockRejectedValue(
      apiError(422, { errors: { email: ['taken'], password: ['too_short'] } }),
    )
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await register('zh_TW', {
      name: 'New Player', email: 'taken@example.test', password: 'short', password_confirmation: 'short',
    }, client)

    expect(outcome).toEqual({
      ok: false,
      kind: 'validation',
      errors: { email: ['taken'], password: ['too_short'] },
    })
  })

  it('survives a 422 whose body is not the expected shape', async () => {
    for (const data of [undefined, null, 'nonsense', { errors: 'not-an-object' }, { errors: { email: 'plain' } }]) {
      const post = vi.fn().mockRejectedValue(apiError(422, data))
      const client = { get: vi.fn(), post } as unknown as APIClient

      const outcome = await register('zh_TW', {
        name: 'x', email: 'x@y.test', password: 'password', password_confirmation: 'password',
      }, client)

      // Still a validation outcome, so the form shows its generic message instead of
      // claiming the service is down.
      expect(outcome.ok).toBe(false)
      if (!outcome.ok) expect(outcome.kind).toBe('validation')
    }
  })

  it('signs out locally even when the server refuses', async () => {
    const post = vi.fn().mockRejectedValue(apiError(500))
    const client = { get: vi.fn(), post } as unknown as APIClient

    await logout('zh_TW', client)

    expect(getCachedSession()).toBeNull()
  })

  it('asks for a reset mail with the locale the mail should be written in', async () => {
    const post = vi.fn().mockResolvedValue({ status: 'sent' })
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await requestPasswordReset('ja', ' player@example.test ', client)

    expect(outcome).toEqual({ ok: true })
    // The mail is read outside the browser, so its language cannot be chosen here.
    expect(post).toHaveBeenCalledWith(
      '/auth/password/forgot', { email: 'player@example.test', locale: 'ja' })
  })

  it('does not spend a request on an empty address', async () => {
    const post = vi.fn()
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await requestPasswordReset('zh_TW', '   ', client)

    expect(outcome).toEqual({ ok: false, kind: 'validation', errors: { email: ['required'] } })
    expect(post).not.toHaveBeenCalled()
  })

  // A server with no mail transport answers 503. There is nothing field-specific in that
  // and nothing the user can do, so it must not surface as a rejected address.
  it('reports an unconfigured mail transport as unavailable', async () => {
    const post = vi.fn().mockRejectedValue(apiError(503))
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await requestPasswordReset('zh_TW', 'player@example.test', client)

    expect(outcome).toEqual({ ok: false, kind: 'unavailable' })
  })

  it('signs the account in when the reset succeeds', async () => {
    const post = vi.fn().mockResolvedValue(grant())
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await resetPassword(
      'zh_TW', { token: 'the-mailed-token', new_password: 'a-brand-new-password' }, client)

    expect(outcome).toEqual({ ok: true })
    expect(post).toHaveBeenCalledWith(
      '/auth/password/reset',
      { token: 'the-mailed-token', new_password: 'a-brand-new-password' },
      undefined,
      // The refresh cookie only arrives if the request carries credentials.
      'include',
    )
    // Without this the user would be sent home as a guest, having just set a password.
    expect(getCachedSession()?.accessToken).toBe('header.claims.signature')
  })

  it('reports a spent link on the token field', async () => {
    const post = vi.fn().mockRejectedValue(apiError(422, { errors: { token: ['invalid'] } }))
    const client = { get: vi.fn(), post } as unknown as APIClient

    const outcome = await resetPassword(
      'zh_TW', { token: 'a-spent-token', new_password: 'a-brand-new-password' }, client)

    expect(outcome).toEqual({ ok: false, kind: 'validation', errors: { token: ['invalid'] } })
    expect(getCachedSession()).toBeNull()
  })

  it('reads the outcome marker the OAuth callback appends', () => {
    expect(readOAuthResult('?auth=signed-in')).toEqual({ kind: 'signed-in' })
    expect(readOAuthResult('?auth=registered')).toEqual({ kind: 'registered' })
    expect(readOAuthResult('?auth=linked')).toEqual({ kind: 'linked' })
    expect(readOAuthResult('?auth=failed&reason=email-taken'))
      .toEqual({ kind: 'failed', reason: 'email-taken' })
    // A failure with no reason still reports one, so the caller always has a key to look
    // up rather than an empty message.
    expect(readOAuthResult('?auth=failed')).toEqual({ kind: 'failed', reason: 'failed' })
  })

  it('ignores a query that carries no marker or an unknown one', () => {
    for (const search of ['', '?', '?other=1', '?auth=', '?auth=something-else']) {
      expect(readOAuthResult(search)).toBeNull()
    }
  })
})
