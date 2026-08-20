import { getRuntimeConfig } from '../config/runtime'

export interface APIErrorBody {
  code: string
  message: string
}

interface APIEnvelope<T> {
  data?: T
  error?: APIErrorBody
  meta?: {
    request_id?: string
  }
}

export class APIError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId?: string
  readonly data?: unknown

  constructor(status: number, body: APIEnvelope<never>) {
    super(body.error?.message || `API request failed with status ${status}`)
    this.name = 'APIError'
    this.status = status
    this.code = body.error?.code || 'unknown_error'
    this.requestId = body.meta?.request_id
    this.data = body.data
  }
}

export interface APIClient {
  get<T>(path: string, signal?: AbortSignal, credentials?: RequestCredentials, headers?: HeadersInit): Promise<T>
  post<T>(path: string, body: unknown, signal?: AbortSignal, credentials?: RequestCredentials, headers?: HeadersInit): Promise<T>
  put<T>(path: string, body: unknown, signal?: AbortSignal, credentials?: RequestCredentials, headers?: HeadersInit): Promise<T>
  /**
   * Sends a DELETE, optionally with a body.
   *
   * A body on DELETE is unusual but right here: the post editor confirms a deletion with
   * the account password, and a query string would put it in the access log and in the
   * browser's history.
   */
  delete<T>(path: string, body?: unknown, signal?: AbortSignal, credentials?: RequestCredentials, headers?: HeadersInit): Promise<T>
  /**
   * Posts a multipart body — an upload.
   *
   * Separate from post because the Content-Type must NOT be set here: the browser writes
   * it, boundary included, and a header set by hand produces a body the server cannot
   * parse.
   */
  postForm<T>(path: string, form: FormData, signal?: AbortSignal, credentials?: RequestCredentials, headers?: HeadersInit): Promise<T>
}

export function createAPIClient(
  baseUrl: string,
  fetchImplementation: typeof fetch = fetch,
): APIClient {
  return {
    async get<T>(path: string, signal?: AbortSignal, credentials: RequestCredentials = 'include', headers?: HeadersInit): Promise<T> {
      const response = await fetchImplementation(joinURL(baseUrl, path), {
        method: 'GET',
        credentials,
        headers: requestHeaders(headers),
        signal,
      })

      return unwrap<T>(response)
    },
    async post<T>(path: string, requestBody: unknown, signal?: AbortSignal, credentials: RequestCredentials = 'include', headers?: HeadersInit): Promise<T> {
      return send<T>('POST', path, requestBody, signal, credentials, headers)
    },
    async put<T>(path: string, requestBody: unknown, signal?: AbortSignal, credentials: RequestCredentials = 'include', headers?: HeadersInit): Promise<T> {
      return send<T>('PUT', path, requestBody, signal, credentials, headers)
    },
    async delete<T>(path: string, requestBody?: unknown, signal?: AbortSignal, credentials: RequestCredentials = 'include', headers?: HeadersInit): Promise<T> {
      const response = await fetchImplementation(joinURL(baseUrl, path), {
        method: 'DELETE',
        credentials,
        headers: requestHeaders(headers, requestBody !== undefined),
        body: requestBody === undefined ? undefined : JSON.stringify(requestBody),
        signal,
      })
      return unwrap<T>(response)
    },
    async postForm<T>(path: string, form: FormData, signal?: AbortSignal, credentials: RequestCredentials = 'include', headers?: HeadersInit): Promise<T> {
      const response = await fetchImplementation(joinURL(baseUrl, path), {
        method: 'POST',
        credentials,
        // requestHeaders without the json flag: see postForm on APIClient for why the
        // Content-Type has to be left to the browser.
        headers: requestHeaders(headers),
        body: form,
        signal,
      })
      return unwrap<T>(response)
    },
  }

  async function send<T>(
    method: string,
    path: string,
    requestBody: unknown,
    signal?: AbortSignal,
    credentials: RequestCredentials = 'include',
    headers?: HeadersInit,
  ): Promise<T> {
    const response = await fetchImplementation(joinURL(baseUrl, path), {
      method,
      credentials,
      headers: requestHeaders(headers, true),
      body: JSON.stringify(requestBody),
      signal,
    })
    return unwrap<T>(response)
  }
}

/**
 * Observers of every response's headers.
 *
 * ONE SEAM, DELIBERATELY NARROW. Post access tokens are reissued on the response of any
 * request that used one, so something has to see headers — and unwrap is already the
 * single point every verb funnels through. A listener may not change the response or fail
 * the request: a throw here would turn a successful call into an error for a reason the
 * caller has nothing to do with, so they are called defensively.
 */
type ResponseHeaderListener = (headers: Headers) => void

const responseHeaderListeners: ResponseHeaderListener[] = []

export function onResponseHeaders(listener: ResponseHeaderListener): void {
  responseHeaderListeners.push(listener)
}

function notifyResponseHeaders(headers: Headers): void {
  for (const listener of responseHeaderListeners) {
    try {
      listener(headers)
    } catch {
      // Deliberately swallowed. See onResponseHeaders.
    }
  }
}

/**
 * Reads the envelope, or accepts that there is nothing to read.
 *
 * A 204 CARRIES NO BODY AT ALL — no JSON, not even "{}", because Go's http server strips
 * the body for that status. Calling response.json() on it throws a SyntaxError, which is
 * not an APIError and so escapes every caller that only handles APIError. That shipped
 * once already: logout answers 204, so every SUCCESSFUL sign-out surfaced as "the account
 * service is unreachable". The unit tests missed it because a fake client resolves with a
 * value rather than reproducing an empty body.
 */
async function unwrap<T>(response: Response): Promise<T> {
  notifyResponseHeaders(response.headers)
  if (response.status === 204 || response.headers.get('Content-Length') === '0') {
    if (!response.ok) {
      throw new APIError(response.status, {} as APIEnvelope<never>)
    }
    // undefined is the honest answer for "no content". Callers that declare a return type
    // for a 204 endpoint are the ones in the wrong.
    return undefined as T
  }

  let body: APIEnvelope<T>
  try {
    body = (await response.json()) as APIEnvelope<T>
  } catch {
    // A body that is not JSON is a fault, not a value — most often an HTML error page
    // from something in front of the API.
    throw new APIError(response.status, {} as APIEnvelope<never>)
  }

  if (!response.ok || body.error || body.data === undefined) {
    throw new APIError(response.status, body as APIEnvelope<never>)
  }
  return body.data
}

function requestHeaders(additional?: HeadersInit, json = false): Headers {
  const headers = new Headers(additional)
  if (!headers.has('Accept')) headers.set('Accept', 'application/json')
  if (json && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  // The language the visitor is actually reading, which is the localized route they are on
  // rather than whatever their browser prefers. Anything the API generates for a person to
  // read — a starting nickname in a game room, for one — should be in that language.
  // Taken from the document because AppHeader already keeps it in step with the route.
  const language = typeof document === 'undefined' ? '' : document.documentElement.lang
  if (language && !headers.has('Accept-Language')) headers.set('Accept-Language', language)
  return headers
}

export function getAPIClient(): APIClient {
  return createAPIClient(getRuntimeConfig().apiBaseUrl)
}

function joinURL(baseUrl: string, path: string): string {
  return `${baseUrl.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`
}
