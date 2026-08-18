import { APIError, getAPIClient, type APIClient } from '../lib/api'
import { getRuntimeConfig } from '../config/runtime'
import {
  adoptGrant,
  getAccessToken,
  googleSignInURL,
  signIn,
  signOut,
  type GrantBody,
} from './session'

/**
 * Sign-in, sign-up and password reset against the Go API.
 *
 * Laravel used to be the session authority, which meant posting credentials to its web
 * routes and reading a CSRF token out of `/session-context` first. None of that is here
 * any more: the Go API issues its own sessions, so a login is one request and the token
 * arrives in the response.
 *
 * Password reset was the last thing that still left the SPA, because it needs a mail
 * sender Laravel had and the Go API did not. It is two requests here now, and both pages
 * are ordinary routes.
 */

export interface AuthFieldErrors {
  [field: string]: string[]
}

export type AuthOutcome =
  | { ok: true }
  | { ok: false; kind: 'validation'; errors: AuthFieldErrors }
  | { ok: false; kind: 'credentials' }
  | { ok: false; kind: 'unavailable' }

export interface LoginPayload {
  email: string
  password: string
  /**
   * Accepted and ignored. Every session lasts thirty days and is extended by use, so
   * there is nothing for this to switch — but the login form still offers it and
   * removing the checkbox is UI work that is deliberately deferred.
   */
  remember?: boolean
}

export async function login(
  _locale: string,
  payload: LoginPayload,
  client: APIClient = getAPIClient(),
): Promise<AuthOutcome> {
  const email = payload.email.trim()
  if (!email || !payload.password) {
    // Answered locally rather than spending a request on a form the server would
    // reject anyway. The field names match what the old Laravel validator returned, so
    // the form's error display did not have to change.
    return {
      ok: false,
      kind: 'validation',
      errors: {
        ...(email ? {} : { email: ['required'] }),
        ...(payload.password ? {} : { password: ['required'] }),
      },
    }
  }

  try {
    await signIn(email, payload.password, client)
    return { ok: true }
  } catch (error) {
    if (!(error instanceof APIError)) return { ok: false, kind: 'unavailable' }
    // The server answers every credential failure identically on purpose — unknown
    // address, wrong password, or an account that has no password at all. There is
    // nothing more specific to show, and inventing a distinction here would put back
    // the enumeration oracle the server removed.
    if (error.status === 401) return { ok: false, kind: 'credentials' }
    if (error.status === 400) return { ok: false, kind: 'validation', errors: {} }
    return { ok: false, kind: 'unavailable' }
  }
}

export interface RegisterPayload {
  name: string
  email: string
  password: string
  password_confirmation: string
}

/**
 * The server answers a refused sign-up with 422 and per-field machine codes in `data`,
 * not with sentences: it has no message catalogue, and a sentence it chose could only be
 * in one of the three languages this app serves. `validationMessage` turns a code into
 * text.
 */
export async function register(
  _locale: string,
  payload: RegisterPayload,
  client: APIClient = getAPIClient(),
): Promise<AuthOutcome> {
  try {
    const grant = await client.post<GrantBody>('/auth/register', payload, undefined, 'include')
    // Registering signs the user in, matching what Laravel's RegistersUsers trait did.
    adoptGrant(grant)
    return { ok: true }
  } catch (error) {
    if (!(error instanceof APIError)) return { ok: false, kind: 'unavailable' }
    if (error.status === 422) {
      return { ok: false, kind: 'validation', errors: fieldErrorsFrom(error.data) }
    }
    return { ok: false, kind: 'unavailable' }
  }
}

/**
 * Asks for a reset mail.
 *
 * IT ANSWERS THE SAME WHETHER OR NOT THE ADDRESS HAS AN ACCOUNT. The server reports
 * success for an address it has never seen, for one that asked a moment ago, and for one
 * whose mail its relay refused — a form that answered differently would be a way to test
 * which addresses are registered here, and this one needs no credentials to use. So the
 * page it drives must not promise that a mail is on its way, only that one has been sent
 * if the address is registered.
 *
 * `locale` picks the language of the mail, which is read outside the browser and so cannot
 * be translated by this app.
 */
export async function requestPasswordReset(
  locale: string,
  email: string,
  client: APIClient = getAPIClient(),
): Promise<AuthOutcome> {
  const address = email.trim()
  if (!address) return { ok: false, kind: 'validation', errors: { email: ['required'] } }

  try {
    await client.post('/auth/password/forgot', { email: address, locale })
    return { ok: true }
  } catch (error) {
    if (!(error instanceof APIError)) return { ok: false, kind: 'unavailable' }
    if (error.status === 422) {
      return { ok: false, kind: 'validation', errors: fieldErrorsFrom(error.data) }
    }
    // 503 is a server with no mail transport configured. There is nothing the user can do
    // about it and nothing field-specific to show, so it reads as unavailable.
    return { ok: false, kind: 'unavailable' }
  }
}

export interface ResetPasswordPayload {
  token: string
  new_password: string
}

/**
 * Sets the new password and signs the account in.
 *
 * Signing in is the server's doing: the user has just proved control of the address on
 * file, which is the same proof a login gives. A `token: ['invalid']` error covers a link
 * that expired, was already used, or was never issued — the server does not distinguish
 * them, and neither should the page.
 */
export async function resetPassword(
  _locale: string,
  payload: ResetPasswordPayload,
  client: APIClient = getAPIClient(),
): Promise<AuthOutcome> {
  try {
    const grant = await client.post<GrantBody>(
      '/auth/password/reset', payload, undefined, 'include')
    adoptGrant(grant)
    return { ok: true }
  } catch (error) {
    if (!(error instanceof APIError)) return { ok: false, kind: 'unavailable' }
    if (error.status === 422) {
      return { ok: false, kind: 'validation', errors: fieldErrorsFrom(error.data) }
    }
    return { ok: false, kind: 'unavailable' }
  }
}

/** Reads `data.errors` off a 422 body. */
function fieldErrorsFrom(data: unknown): AuthFieldErrors {
  if (!data || typeof data !== 'object') return {}
  const errors = (data as { errors?: unknown }).errors
  if (!errors || typeof errors !== 'object') return {}

  const result: AuthFieldErrors = {}
  for (const [field, codes] of Object.entries(errors as Record<string, unknown>)) {
    if (Array.isArray(codes)) {
      result[field] = codes.filter((code): code is string => typeof code === 'string')
    }
  }
  return result
}

export async function logout(
  _locale?: string,
  client: APIClient = getAPIClient(),
): Promise<AuthOutcome> {
  try {
    await signOut(client)
    return { ok: true }
  } catch {
    // signOut already clears the local session whatever happens, so the user is signed
    // out here either way.
    return { ok: false, kind: 'unavailable' }
  }
}

/**
 * The URL that starts the Google flow, including where to come back to.
 *
 * The return target is validated against the API's allowed origins when the flow starts
 * and remembered server-side, so a tampered value cannot redirect anyone anywhere.
 */
export function googleRedirectURL(returnTo: string = currentLocation()): string {
  return googleSignInURL(getRuntimeConfig().apiBaseUrl, returnTo)
}

/**
 * Starts linking a Google account to the one already signed in, and answers with the URL
 * to navigate to.
 *
 * A POST that returns a URL rather than a link that redirects: the request has to carry
 * the bearer token, which a navigation cannot, and the server needs to know which account
 * the link is for before it builds the state.
 *
 * Null means the request failed. There is nothing field-specific to report — the caller
 * shows one message.
 */
export async function connectGoogleURL(
  returnTo: string = currentLocation(),
  client: APIClient = getAPIClient(),
): Promise<string | null> {
  const token = await getAccessToken(client)
  if (!token) return null
  try {
    // return_to goes in the query, which is where the server reads it from; the body is
    // empty. It is validated against the API's allowed origins and then kept server-side
    // with the one-shot state, so a tampered value cannot redirect anyone anywhere.
    const query = new URLSearchParams({ return_to: returnTo })
    const started = await client.post<{ authorization_url: string }>(
      `/auth/oauth/google/connect?${query}`, {}, undefined, 'include',
      { Authorization: `Bearer ${token}` })
    return started.authorization_url || null
  } catch {
    return null
  }
}

/** The result the OAuth callback reports back in the query string. */
export type OAuthResult =
  | { kind: 'signed-in' }
  | { kind: 'registered' }
  | { kind: 'linked' }
  | { kind: 'failed'; reason: string }

/**
 * Reads the `?auth=` marker the callback appends.
 *
 * Only a marker: there is never a token in the URL, because a URL reaches the browser's
 * history and the next request's Referer.
 */
export function readOAuthResult(search: string): OAuthResult | null {
  const parameters = new URLSearchParams(search)
  const outcome = parameters.get('auth')
  switch (outcome) {
    case 'signed-in':
    case 'registered':
    case 'linked':
      return { kind: outcome }
    case 'failed':
      return { kind: 'failed', reason: parameters.get('reason') || 'failed' }
    default:
      return null
  }
}

function currentLocation(): string {
  if (typeof window === 'undefined') return '/'
  return window.location.href
}
