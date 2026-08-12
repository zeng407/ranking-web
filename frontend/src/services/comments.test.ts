// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { APIClient } from '../lib/api'
import { createCommentsService } from './comments'

describe('comments service', () => {
  beforeEach(() => {
    localStorage.clear()
    localStorage.setItem('2pick:anonymous-id', 'browser-id')
  })

  it('keeps the access token in memory and sends identity plus anonymous id to Go', async () => {
    const get = vi.fn().mockResolvedValue({ items: [], page: 2 })
    const post = vi.fn().mockResolvedValue({ id: 1 })
    const client = { get, post } as unknown as APIClient
    // The token resolver is the seam now: the service asks for a bearer token rather
    // than for a Laravel session context.
    const resolveToken = vi.fn().mockResolvedValue('short-lived-token')
    const service = createCommentsService(client, resolveToken)

    await service.list('post/serial', 2, 'zh_TW')
    await service.create('post/serial', { content: 'hello', anonymous: true }, 'zh_TW')

    expect(get).toHaveBeenCalledWith(
      '/posts/post%2Fserial/comments?page=2&anonymous_id=browser-id',
      undefined,
      'include',
      { Authorization: 'Bearer short-lived-token' },
    )
    expect(post).toHaveBeenCalledWith(
      '/posts/post%2Fserial/comments',
      { content: 'hello', anonymous: true, anonymous_id: 'browser-id' },
      undefined,
      'include',
      { Authorization: 'Bearer short-lived-token' },
    )
    // The token must never be persisted: it is held in memory so that a closed tab
    // takes it with it, and any injected script gets at most its five minute life.
    expect(localStorage.getItem('short-lived-token')).toBeNull()
    expect(document.cookie).not.toContain('short-lived-token')
  })

  it('omits the Authorization header entirely when nobody is signed in', async () => {
    const get = vi.fn().mockResolvedValue({ items: [], page: 1 })
    const client = { get, post: vi.fn() } as unknown as APIClient
    const service = createCommentsService(client, async () => null)

    await service.list('post/serial', 1, 'zh_TW')

    // undefined rather than an empty Bearer: a header with no token would be rejected as
    // malformed instead of being treated as an anonymous read.
    expect(get).toHaveBeenCalledWith(
      '/posts/post%2Fserial/comments?page=1&anonymous_id=browser-id',
      undefined,
      'include',
      undefined,
    )
  })
})
