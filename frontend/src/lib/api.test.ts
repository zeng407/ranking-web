// @vitest-environment happy-dom

import { describe, expect, it, vi } from 'vitest'

import { APIError, createAPIClient } from './api'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/** A 204 exactly as Go's http server produces it: no body, no Content-Length. */
function noContentResponse(): Response {
  return new Response(null, { status: 204 })
}

describe('api client', () => {
  it('unwraps the envelope', async () => {
    const fetchStub = vi.fn().mockResolvedValue(jsonResponse(200, { data: { id: 7 } }))
    const client = createAPIClient('/api/v1', fetchStub as unknown as typeof fetch)

    expect(await client.get<{ id: number }>('/things')).toEqual({ id: 7 })
    expect(fetchStub).toHaveBeenCalledWith('/api/v1/things', expect.objectContaining({ method: 'GET' }))
  })

  /**
   * THE ONE THAT SHIPPED BROKEN. A 204 carries no body at all, so response.json() throws a
   * SyntaxError — which is not an APIError and escapes every caller that only handles one.
   * logout answers 204, so every successful sign-out reported failure.
   */
  it('treats a 204 as success with no content rather than throwing', async () => {
    const fetchStub = vi.fn().mockResolvedValue(noContentResponse())
    const client = createAPIClient('/api/v1', fetchStub as unknown as typeof fetch)

    await expect(client.post('/auth/logout', {})).resolves.toBeUndefined()
  })

  it('reports a 204-shaped failure as an APIError', async () => {
    const fetchStub = vi.fn().mockResolvedValue(new Response(null, { status: 502 }))
    const client = createAPIClient('/api/v1', fetchStub as unknown as typeof fetch)

    await expect(client.post('/things', {})).rejects.toBeInstanceOf(APIError)
  })

  // An HTML error page from a proxy in front of the API is the common case here. It must
  // surface as an APIError so callers handle it, not as a raw SyntaxError.
  it('turns a non-JSON body into an APIError', async () => {
    const fetchStub = vi.fn().mockResolvedValue(
      new Response('<html>502 Bad Gateway</html>', { status: 502 }))
    const client = createAPIClient('/api/v1', fetchStub as unknown as typeof fetch)

    const error = await client.get('/things').catch((caught: unknown) => caught)
    expect(error).toBeInstanceOf(APIError)
    expect((error as APIError).status).toBe(502)
  })

  it('carries the error code and request id off a failure envelope', async () => {
    const fetchStub = vi.fn().mockResolvedValue(jsonResponse(422, {
      error: { code: 'invalid_nickname', message: 'no' },
      meta: { request_id: 'abc123' },
    }))
    const client = createAPIClient('/api/v1', fetchStub as unknown as typeof fetch)

    const error = await client.put('/things', {}).catch((caught: unknown) => caught) as APIError
    expect(error.status).toBe(422)
    expect(error.code).toBe('invalid_nickname')
    expect(error.requestId).toBe('abc123')
  })

  it('sends a PUT with a JSON body', async () => {
    const fetchStub = vi.fn().mockResolvedValue(jsonResponse(200, { data: { name: 'x' } }))
    const client = createAPIClient('/api/v1', fetchStub as unknown as typeof fetch)

    await client.put('/game-rooms/abc/player', { nickname: 'x' })

    const [, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(init.method).toBe('PUT')
    expect(init.body).toBe(JSON.stringify({ nickname: 'x' }))
    expect(new Headers(init.headers).get('Content-Type')).toBe('application/json')
  })

  // A 200 whose envelope has no data is a contract violation, not an empty success: it
  // means the endpoint answered something other than what it promised.
  it('rejects a 200 with no data', async () => {
    const fetchStub = vi.fn().mockResolvedValue(jsonResponse(200, { meta: { request_id: 'x' } }))
    const client = createAPIClient('/api/v1', fetchStub as unknown as typeof fetch)

    await expect(client.get('/things')).rejects.toBeInstanceOf(APIError)
  })
})
