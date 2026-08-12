import { APIError, getAPIClient, type APIClient } from '../lib/api'
import { adoptGrant, getAccessToken, type GrantBody } from './session'

/**
 * The account settings, against the Go API.
 *
 * Laravel served these as a Blade page posting a form and reading flash messages back
 * out of the session. There is no session to flash through here, so every operation
 * answers with either the new account or per-field codes.
 *
 * WHY THE ADDRESS IS NOT HERE AS SOMETHING EDITABLE. The old settings form had an e-mail
 * input, but its controller never accepted one — `update()` validated name and avatar
 * only. An address has therefore never been changeable, and adding it now would be a new
 * feature rather than a port.
 */

export interface Account {
  name: string
  email: string
  avatar_url: string
  /** False for an account created through Google, which has no password column value. */
  has_password: boolean
  google_linked: boolean
  /** Present only while the once-a-day name change is in force. RFC 3339. */
  name_change_allowed_at?: string
}

export interface AccountFieldErrors {
  [field: string]: string[]
}

/**
 * Every operation reports one of these. `validation` carries the machine codes the API
 * answers with — required, too_long, too_short, name_change_too_soon, incorrect,
 * unsupported_image, too_large, password_already_set — and the view translates them.
 */
export type AccountOutcome<T = void> =
  | { ok: true; value: T }
  | { ok: false; kind: 'validation'; errors: AccountFieldErrors }
  | { ok: false; kind: 'signed-out' }
  | { ok: false; kind: 'unavailable' }

export interface AccountService {
  load(signal?: AbortSignal): Promise<AccountOutcome<Account>>
  rename(name: string): Promise<AccountOutcome<Account>>
  uploadAvatar(file: File): Promise<AccountOutcome<string>>
  changePassword(currentPassword: string, newPassword: string): Promise<AccountOutcome>
  setInitialPassword(newPassword: string): Promise<AccountOutcome>
}

export function createAccountService(client: APIClient = getAPIClient()): AccountService {
  return {
    async load(signal?: AbortSignal): Promise<AccountOutcome<Account>> {
      return attempt(async (headers) =>
        client.get<Account>('/account/profile', signal, 'include', headers))
    },

    async rename(name: string): Promise<AccountOutcome<Account>> {
      return attempt(async (headers) =>
        client.put<Account>('/account/profile', { name }, undefined, 'include', headers))
    },

    async uploadAvatar(file: File): Promise<AccountOutcome<string>> {
      const form = new FormData()
      form.append('avatar', file)
      const outcome = await attempt(async (headers) =>
        client.postForm<{ avatar_url: string }>('/account/avatar', form, undefined, 'include', headers))
      return outcome.ok ? { ok: true, value: outcome.value.avatar_url } : outcome
    },

    async changePassword(currentPassword: string, newPassword: string): Promise<AccountOutcome> {
      // The response is a grant, because the change ends every session the account holds
      // — this one included. Adopting it is what keeps the page signed in.
      const outcome = await attempt(async (headers) =>
        client.put<GrantBody>('/account/password',
          { current_password: currentPassword, new_password: newPassword },
          undefined, 'include', headers))
      if (!outcome.ok) return outcome
      adoptGrant(outcome.value)
      return { ok: true, value: undefined }
    },

    async setInitialPassword(newPassword: string): Promise<AccountOutcome> {
      const outcome = await attempt(async (headers) =>
        client.post<GrantBody>('/account/password', { new_password: newPassword },
          undefined, 'include', headers))
      if (!outcome.ok) return outcome
      adoptGrant(outcome.value)
      return { ok: true, value: undefined }
    },
  }

  /**
   * Runs one request with a bearer token, and turns every failure into an outcome.
   *
   * The token is fetched per call rather than once for the service: a settings page stays
   * open for minutes and an access token lasts five, so a cached one would be stale by
   * the time the form is submitted.
   */
  async function attempt<T>(
    request: (headers: HeadersInit) => Promise<T>,
  ): Promise<AccountOutcome<T>> {
    const token = await getAccessToken(client)
    if (!token) return { ok: false, kind: 'signed-out' }
    try {
      return { ok: true, value: await request({ Authorization: `Bearer ${token}` }) }
    } catch (error) {
      if (!(error instanceof APIError)) return { ok: false, kind: 'unavailable' }
      if (error.status === 422) {
        return { ok: false, kind: 'validation', errors: fieldErrorsFrom(error.data) }
      }
      if (error.status === 401) return { ok: false, kind: 'signed-out' }
      return { ok: false, kind: 'unavailable' }
    }
  }
}

/** Reads `data.errors` off a 422 body, the same shape registration answers with. */
function fieldErrorsFrom(data: unknown): AccountFieldErrors {
  if (!data || typeof data !== 'object') return {}
  const errors = (data as { errors?: unknown }).errors
  if (!errors || typeof errors !== 'object') return {}

  const result: AccountFieldErrors = {}
  for (const [field, codes] of Object.entries(errors as Record<string, unknown>)) {
    if (Array.isArray(codes)) {
      result[field] = codes.filter((code): code is string => typeof code === 'string')
    }
  }
  return result
}

/**
 * Whether the once-a-day name change is in force, read from the account.
 *
 * The rule itself is the server's — this only reads the moment it reports, so a client
 * clock that is wrong by hours shows the wrong hint but cannot change what is allowed.
 */
export function nameChangeIsBlocked(account: Account | null, now: number = Date.now()): boolean {
  if (!account?.name_change_allowed_at) return false
  const allowedAt = Date.parse(account.name_change_allowed_at)
  return Number.isFinite(allowedAt) && allowedAt > now
}

let service: AccountService | null = null

export function getAccountService(): AccountService {
  if (!service) service = createAccountService()
  return service
}

/** Test seam. */
export function resetAccountServiceForTests(): void {
  service = null
}
