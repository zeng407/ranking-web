import { beforeEach, describe, expect, it, vi } from 'vitest'

import { APIError, type APIClient } from '../lib/api'
import { createEditorService, draftFrom, type MyPost, type PostElement } from './editor'
import { adoptGrant, resetSessionForTests } from './session'

/**
 * The editor service. What matters is the shape on the wire — which path, which verb,
 * what is and is not sent — and that each failure becomes the right outcome instead of an
 * exception the view has to catch.
 */

const post: MyPost = {
  serial: 'abcdefgh',
  title: 'a title',
  description: 'a description',
  access_policy: 'public',
  has_password: false,
  tags: ['cats'],
  play_count: 500,
  this_week_play_count: 20,
  last_week_play_count: 30,
}

const element: PostElement = {
  id: 5,
  source_url: 'https://file.2pick.test/a.png',
  thumb_url: 'https://file.2pick.test/a-thumb.png',
  mediumthumb_url: '',
  lowthumb_url: '',
  title: 'an element',
  type: 'image',
  video_duration_second: null,
  video_start_second: null,
  video_end_second: null,
  rank: null,
}

function grantBody() {
  return {
    access_token: 'the-access-token', token_type: 'Bearer', expires_in: 300,
    csrf_token: 'the-csrf', user_id: '42', roles: [],
  }
}

function fakeClient(overrides: Partial<APIClient> = {}): APIClient {
  return {
    get: vi.fn().mockResolvedValue(post),
    post: vi.fn().mockResolvedValue({ serial: 'newserial' }),
    put: vi.fn().mockResolvedValue(post),
    delete: vi.fn().mockResolvedValue(undefined),
    postForm: vi.fn().mockResolvedValue(element),
    ...overrides,
  }
}

function apiError(status: number, data?: unknown): APIError {
  return new APIError(status, { data, error: { code: 'x', message: 'no' } } as never)
}

function signedIn(): void {
  adoptGrant(grantBody())
}

describe('createEditorService', () => {
  beforeEach(() => {
    resetSessionForTests()
  })

  it('lists posts with a bearer token', async () => {
    signedIn()
    const client = fakeClient({
      get: vi.fn().mockResolvedValue({ posts: [post], total: 1, page: 2, per_page: 15 }),
    })

    const outcome = await createEditorService(client).posts(2)

    expect(outcome).toEqual({ ok: true, value: { posts: [post], total: 1, page: 2, per_page: 15 } })
    expect(client.get).toHaveBeenCalledWith('/account/posts?page=2', undefined, 'include', {
      Authorization: 'Bearer the-access-token',
    })
  })

  it('creates a post and reports the serial', async () => {
    signedIn()
    const client = fakeClient()

    const outcome = await createEditorService(client).createPost({
      title: 't', description: 'd', access_policy: 'public',
    })

    expect(outcome).toEqual({ ok: true, value: 'newserial' })
    expect(client.post).toHaveBeenCalledWith('/account/posts',
      { title: 't', description: 'd', access_policy: 'public' },
      undefined, 'include', expect.anything())
  })

  // The password confirms the deletion and belongs in the body: a query string reaches
  // the access log and the browser's history.
  it('sends the delete password in the body', async () => {
    signedIn()
    const client = fakeClient()

    await createEditorService(client).deletePost('abcdefgh', 'the-account-password')

    expect(client.delete).toHaveBeenCalledWith('/account/posts/abcdefgh',
      { password: 'the-account-password' }, undefined, 'include', expect.anything())
  })

  // An account with no password sends no body at all rather than an empty one, which the
  // server would read as "the password is the empty string".
  it('omits the body when there is no password', async () => {
    signedIn()
    const client = fakeClient()

    await createEditorService(client).deletePost('abcdefgh')

    expect(client.delete).toHaveBeenCalledWith('/account/posts/abcdefgh',
      undefined, undefined, 'include', expect.anything())
  })

  it('builds the element query from what was asked for', async () => {
    signedIn()
    const client = fakeClient({
      get: vi.fn().mockResolvedValue({ elements: [element], total: 1, page: 1, per_page: 24 }),
    })

    await createEditorService(client).elements('abcdefgh', {
      page: 3, per_page: 24, title: 'cat', sort_by: 'title', sort_dir: 'asc',
    })

    const [path] = (client.get as ReturnType<typeof vi.fn>).mock.calls[0] as [string]
    expect(path).toContain('/account/posts/abcdefgh/elements?')
    expect(path).toContain('page=3')
    expect(path).toContain('per_page=24')
    expect(path).toContain('title=cat')
    expect(path).toContain('sort_by=title')
    expect(path).toContain('sort_dir=asc')
  })

  it('asks for no query at all when nothing was specified', async () => {
    signedIn()
    const client = fakeClient({ get: vi.fn().mockResolvedValue({ elements: [], total: 0, page: 1, per_page: 100 }) })

    await createEditorService(client).elements('abcdefgh')

    expect(client.get).toHaveBeenCalledWith('/account/posts/abcdefgh/elements',
      undefined, 'include', expect.anything())
  })

  it('uploads a file as a multipart part named file', async () => {
    signedIn()
    const client = fakeClient({ postForm: vi.fn().mockResolvedValue(element) })

    const outcome = await createEditorService(client).uploadElement(
      'abcdefgh', new File(['bytes'], 'holiday.png', { type: 'image/png' }))

    expect(outcome).toEqual({ ok: true, value: element })
    const call = (client.postForm as ReturnType<typeof vi.fn>).mock.calls[0]
    if (!call) throw new Error('postForm was not called')
    const [path, form] = call
    expect(path).toBe('/account/posts/abcdefgh/elements/uploads')
    // The part name is what the server reads the file out of, so it is load-bearing.
    expect((form as FormData).get('file')).toBeInstanceOf(File)
  })

  it('posts the pasted list as one field', async () => {
    signedIn()
    const client = fakeClient({
      post: vi.fn().mockResolvedValue({ added: [element], failed: [] }),
    })

    const outcome = await createEditorService(client).addElementsByURL(
      'abcdefgh', 'https://a.test/1.png,https://b.test/2.png')

    expect(outcome).toEqual({ ok: true, value: { added: [element], failed: [] } })
    expect(client.post).toHaveBeenCalledWith('/account/posts/abcdefgh/elements/urls',
      { urls: 'https://a.test/1.png,https://b.test/2.png' }, undefined, 'include', expect.anything())
  })

  // A 207 is a success with failures inside it, not a failure: the client has to be able
  // to show what was added as well as what was not.
  it('reports a partly successful batch as ok', async () => {
    signedIn()
    const client = fakeClient({
      post: vi.fn().mockResolvedValue({
        added: [element], failed: [{ url: 'https://dead.test/x', reason: 'unavailable' }],
      }),
    })

    const outcome = await createEditorService(client).addElementsByURL('abcdefgh', 'x')

    if (!outcome.ok) throw new Error('a partial batch was reported as a failure')
    expect(outcome.value.added).toHaveLength(1)
    expect(outcome.value.failed[0]?.reason).toBe('unavailable')
  })

  it('turns a 422 into per-field codes', async () => {
    signedIn()
    const client = fakeClient({
      post: vi.fn().mockRejectedValue(apiError(422, { errors: { title: ['required'] } })),
    })

    const outcome = await createEditorService(client).createPost({
      title: '', description: 'd', access_policy: 'public',
    })

    expect(outcome).toEqual({ ok: false, kind: 'validation', errors: { title: ['required'] } })
  })

  // 404 is what someone else's post answers, deliberately — the server does not tell
  // "does not exist" from "not yours", and neither does this.
  it('reports a 404 as not-found', async () => {
    signedIn()
    const client = fakeClient({ get: vi.fn().mockRejectedValue(apiError(404)) })

    expect(await createEditorService(client).post('abcdefgh')).toEqual({ ok: false, kind: 'not-found' })
  })

  it('reports a 401 as signed out', async () => {
    signedIn()
    const client = fakeClient({ get: vi.fn().mockRejectedValue(apiError(401)) })

    expect(await createEditorService(client).posts()).toEqual({ ok: false, kind: 'signed-out' })
  })

  it('does not send a request without a session', async () => {
    const client = fakeClient({ get: vi.fn() })

    expect(await createEditorService(client).posts()).toEqual({ ok: false, kind: 'signed-out' })
    expect(client.get).not.toHaveBeenCalled()
  })

  it('reports anything else as unavailable', async () => {
    signedIn()
    for (const failure of [apiError(500), new TypeError('network down')]) {
      const client = fakeClient({ get: vi.fn().mockRejectedValue(failure) })
      expect(await createEditorService(client).posts()).toEqual({ ok: false, kind: 'unavailable' })
    }
  })

  it('escapes a serial into the path', async () => {
    signedIn()
    const client = fakeClient()

    await createEditorService(client).post('a/b?c')

    expect(client.get).toHaveBeenCalledWith('/account/posts/a%2Fb%3Fc',
      undefined, 'include', expect.anything())
  })
})

describe('draftFrom', () => {
  const form = { title: '  a title  ', description: '  a description  ', password: '' }

  it('trims the text and leaves tags out when none are given', () => {
    expect(draftFrom({ ...form, access_policy: 'public' })).toEqual({
      title: 'a title', description: 'a description', access_policy: 'public',
    })
  })

  /**
   * AN UNTOUCHED PASSWORD FIELD MUST NOT BE SENT. The server reads an empty string as
   * "clear it", so posting the whole form after editing only the title would open a
   * password-protected post to everyone.
   */
  it('omits the password when the field was left alone', () => {
    const draft = draftFrom({ ...form, access_policy: 'password' })

    expect(draft.password).toBeUndefined()
  })

  it('sends the password when one was typed', () => {
    const draft = draftFrom({ ...form, access_policy: 'password', password: 'door-code' })

    expect(draft.password).toBe('door-code')
  })

  // A password typed while the policy is not `password` would be stored for a post that
  // does not use one — and revived if the author switched back later.
  it('does not send a password for a post that is not password-protected', () => {
    const draft = draftFrom({ ...form, access_policy: 'public', password: 'left-over' })

    expect(draft.password).toBeUndefined()
  })

  it('sends an empty tag list, which is how tags are cleared', () => {
    expect(draftFrom({ ...form, access_policy: 'public' }, []).tags).toEqual([])
  })
})
