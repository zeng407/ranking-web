import { beforeEach, describe, expect, it, vi } from 'vitest'

import { APIError, type APIClient } from '../lib/api'
import { createAdminService, movedOrder, type AdminPost, type CarouselItem } from './admin'
import { adoptGrant, resetSessionForTests } from './session'

/**
 * The back office service. What matters is the shape on the wire, and that each refusal
 * becomes the outcome the screens can explain — a banned administrator is not "the server
 * is down", and losing the role is not "signed out".
 */

const post: AdminPost = {
  serial: 'abcdefgh',
  title: 'a title',
  description: 'a description',
  access_policy: 'public',
  is_censored: false,
  play_count: 12,
  owner: { id: 7, name: 'an author', email: 'author@example.test' },
}

const carouselItem: CarouselItem = {
  id: 3,
  position: 1,
  type: 'image',
  title: 'a slide',
  description: '',
  image_url: 'https://file.2pick.test/a.png',
  video_url: '',
  video_start_second: null,
  video_end_second: null,
  is_active: true,
}

function grantBody(roles: string[] = ['admin']) {
  return {
    access_token: 'the-access-token', token_type: 'Bearer', expires_in: 300,
    csrf_token: 'the-csrf', user_id: '42', roles,
  }
}

function fakeClient(overrides: Partial<APIClient> = {}): APIClient {
  return {
    get: vi.fn().mockResolvedValue({ posts: [post], total: 1, page: 1, per_page: 20 }),
    post: vi.fn().mockResolvedValue(undefined),
    put: vi.fn().mockResolvedValue(undefined),
    delete: vi.fn().mockResolvedValue(undefined),
    postForm: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function apiError(status: number, code = 'x', data?: unknown): APIError {
  return new APIError(status, { data, error: { code, message: 'no' } } as never)
}

function signedIn(roles: string[] = ['admin']): void {
  adoptGrant(grantBody(roles))
}

describe('createAdminService', () => {
  beforeEach(() => {
    resetSessionForTests()
  })

  it('lists posts with a bearer token', async () => {
    signedIn()
    const client = fakeClient()

    const outcome = await createAdminService(client).posts(3)

    expect(outcome).toEqual({ ok: true, value: { posts: [post], total: 1, page: 1, per_page: 20 } })
    expect(client.get).toHaveBeenCalledWith('/admin/posts?page=3', undefined, 'include', {
      Authorization: 'Bearer the-access-token',
    })
  })

  it('sends only the fields an edit carries', async () => {
    signedIn()
    const client = fakeClient({ put: vi.fn().mockResolvedValue({ ...post, is_censored: true }) })

    await createAdminService(client).updatePost('abcdefgh', {
      title: 't', description: 'd', access_policy: 'public', tags: ['cats'], is_censored: true,
    })

    expect(client.put).toHaveBeenCalledWith('/admin/posts/abcdefgh',
      { title: 't', description: 'd', access_policy: 'public', tags: ['cats'], is_censored: true },
      undefined, 'include', expect.anything())
  })

  it('searches users by keyword and page', async () => {
    signedIn()
    const client = fakeClient({
      get: vi.fn().mockResolvedValue({ users: [], total: 0, page: 2, per_page: 20 }),
    })

    await createAdminService(client).users('someone@example.test', 2)

    expect(client.get).toHaveBeenCalledWith(
      '/admin/users?page=2&q=someone%40example.test', undefined, 'include', expect.anything())
  })

  it('omits an empty keyword rather than sending q=', async () => {
    signedIn()
    const client = fakeClient({
      get: vi.fn().mockResolvedValue({ users: [], total: 0, page: 1, per_page: 20 }),
    })

    await createAdminService(client).users('', 1)

    expect(client.get).toHaveBeenCalledWith('/admin/users?page=1', undefined, 'include', expect.anything())
  })

  it('bans and unbans through their own paths', async () => {
    signedIn()
    const client = fakeClient()
    const service = createAdminService(client)

    await service.banUser(9)
    await service.unbanUser(9)

    expect(client.put).toHaveBeenNthCalledWith(1, '/admin/users/9/ban', {}, undefined, 'include', expect.anything())
    expect(client.put).toHaveBeenNthCalledWith(2, '/admin/users/9/unban', {}, undefined, 'include', expect.anything())
  })

  // 409 is the API's answer for banning an administrator. Folding it into `unavailable`
  // would tell the moderator the server is broken when the server is working exactly as
  // designed.
  it('reports a 409 as a conflict carrying the code', async () => {
    signedIn()
    const client = fakeClient({
      put: vi.fn().mockRejectedValue(apiError(409, 'cannot_ban_administrator')),
    })

    const outcome = await createAdminService(client).banUser(1)

    expect(outcome).toEqual({ ok: false, kind: 'conflict', code: 'cannot_ban_administrator' })
  })

  it('reports a 403 as forbidden, not as signed out', async () => {
    signedIn(['user'])
    const client = fakeClient({ get: vi.fn().mockRejectedValue(apiError(403, 'forbidden')) })

    const outcome = await createAdminService(client).posts()

    expect(outcome).toEqual({ ok: false, kind: 'forbidden' })
  })

  it('reports a 422 as field codes', async () => {
    signedIn()
    const client = fakeClient({
      put: vi.fn().mockRejectedValue(apiError(422, 'validation_failed', { errors: { content: ['required'] } })),
    })

    const outcome = await createAdminService(client)
      .publishAnnouncement({ content: '', image_url: '', keep_minutes: 60 })

    expect(outcome).toEqual({ ok: false, kind: 'validation', errors: { content: ['required'] } })
  })

  it('answers signed-out without calling the API when there is no session', async () => {
    const client = fakeClient()

    const outcome = await createAdminService(client).posts()

    expect(outcome).toEqual({ ok: false, kind: 'signed-out' })
    expect(client.get).not.toHaveBeenCalled()
  })

  it('unwraps the carousel list', async () => {
    signedIn()
    const client = fakeClient({ get: vi.fn().mockResolvedValue({ items: [carouselItem] }) })

    const outcome = await createAdminService(client).carouselItems()

    expect(outcome).toEqual({ ok: true, value: [carouselItem] })
  })

  // The whole order in one request, and the stored order back: a per-item burst is what
  // left the original able to save half a drag.
  it('writes the whole order in one request and returns the stored order', async () => {
    signedIn()
    const client = fakeClient({ put: vi.fn().mockResolvedValue({ items: [carouselItem] }) })

    const outcome = await createAdminService(client)
      .reorderCarouselItems([{ id: 3, position: 1 }, { id: 4, position: 2 }])

    expect(outcome).toEqual({ ok: true, value: [carouselItem] })
    expect(client.put).toHaveBeenCalledTimes(1)
    expect(client.put).toHaveBeenCalledWith('/admin/carousel-items/reorder',
      { items: [{ id: 3, position: 1 }, { id: 4, position: 2 }] },
      undefined, 'include', expect.anything())
  })

  it('reads a missing announcement as null rather than a failure', async () => {
    signedIn()
    const client = fakeClient({ get: vi.fn().mockResolvedValue({ announcement: null }) })

    const outcome = await createAdminService(client).announcement()

    expect(outcome).toEqual({ ok: true, value: null })
  })

  it('mints and drops the asset pass', async () => {
    signedIn()
    const client = fakeClient()
    const service = createAdminService(client)

    expect(await service.grantAssetPass()).toEqual({ ok: true, value: undefined })
    expect(await service.revokeAssetPass()).toEqual({ ok: true, value: undefined })
    expect(client.post).toHaveBeenNthCalledWith(1, '/admin/assets/grant', {}, undefined, 'include', expect.anything())
    expect(client.post).toHaveBeenNthCalledWith(2, '/admin/assets/revoke', {}, undefined, 'include', expect.anything())
  })
})

describe('movedOrder', () => {
  const items = [{ id: 1 }, { id: 2 }, { id: 3 }]

  it('numbers from one after moving an entry down', () => {
    expect(movedOrder(items, 0, 2)).toEqual([
      { id: 2, position: 1 }, { id: 3, position: 2 }, { id: 1, position: 3 },
    ])
  })

  it('numbers from one after moving an entry up', () => {
    expect(movedOrder(items, 2, 0)).toEqual([
      { id: 3, position: 1 }, { id: 1, position: 2 }, { id: 2, position: 3 },
    ])
  })

  // A drop on the row the drag started from, and an index from a list that changed under
  // the drag: both must produce the order as it stands rather than a rotation of it.
  it('leaves the order alone for a no-op or an out-of-range move', () => {
    const unchanged = [{ id: 1, position: 1 }, { id: 2, position: 2 }, { id: 3, position: 3 }]
    expect(movedOrder(items, 1, 1)).toEqual(unchanged)
    expect(movedOrder(items, 5, 0)).toEqual(unchanged)
    expect(movedOrder(items, 0, -1)).toEqual(unchanged)
  })
})
