// @vitest-environment happy-dom

import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'

import PostEditorView from './PostEditorView.vue'
import type { EditorService, MyPost, PostElement } from '../services/editor'

/**
 * The editor page. The service is injected, so these are about what the page sends and
 * what it offers — most of all the password rules, where sending the wrong thing opens a
 * post to everyone.
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
  created_at: '2026-08-01T10:00:00Z',
}

const element: PostElement = {
  id: 5,
  source_url: 'https://file.2pick.test/a.png',
  thumb_url: 'https://file.2pick.test/a-thumb.png',
  mediumthumb_url: '',
  lowthumb_url: '',
  title: 'an element',
  type: 'video',
  video_duration_second: 120,
  video_start_second: null,
  video_end_second: null,
  rank: { rank: 3, win_rate: 62.5, final_win_rate: 75 },
}

function fakeService(overrides: Partial<EditorService> = {}): EditorService {
  return {
    posts: vi.fn(),
    post: vi.fn().mockResolvedValue({ ok: true, value: post }),
    createPost: vi.fn(),
    updatePost: vi.fn().mockImplementation((_serial, draft) =>
      Promise.resolve({ ok: true, value: { ...post, ...draft } })),
    deletePost: vi.fn().mockResolvedValue({ ok: true, value: undefined }),
    elements: vi.fn().mockResolvedValue({
      ok: true, value: { elements: [element], total: 1, page: 1, per_page: 24 },
    }),
    updateElement: vi.fn().mockResolvedValue({ ok: true, value: element }),
    deleteElement: vi.fn().mockResolvedValue({ ok: true, value: undefined }),
    uploadElement: vi.fn().mockResolvedValue({ ok: true, value: element }),
    addElementsByURL: vi.fn().mockResolvedValue({ ok: true, value: { added: [element], failed: [] } }),
    ...overrides,
  }
}

async function render(service: EditorService) {
  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/:locale/account/posts/:serial', name: 'editor', component: PostEditorView },
      { path: '/:locale/account/posts', name: 'posts', component: { template: '<p>posts</p>' } },
      { path: '/:locale/login', name: 'login', component: { template: '<p>login</p>' } },
      { path: '/:locale/g/:serial', name: 'game', component: { template: '<p>game</p>' } },
      { path: '/:locale/r/:serial', name: 'rank', component: { template: '<p>rank</p>' } },
    ],
  })
  await router.push('/zh-tw/account/posts/abcdefgh')
  await router.isReady()

  const wrapper = mount(PostEditorView, { props: { service }, global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

async function openElementsTab(wrapper: VueWrapper): Promise<void> {
  const tabs = wrapper.findAll('[role="tab"]')
  await tabs[1]!.trigger('click')
  await flushPromises()
}

describe('PostEditorView', () => {
  it('draws the post it loaded', async () => {
    const { wrapper } = await render(fakeService())

    expect((wrapper.find('#editor-title').element as HTMLInputElement).value).toBe('a title')
    expect(wrapper.text()).toContain('cats')
  })

  it('saves the metadata through the service', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)

    await wrapper.find('#editor-title').setValue('a new title')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(service.updatePost).toHaveBeenCalledWith('abcdefgh', expect.objectContaining({
      title: 'a new title', description: 'a description', access_policy: 'public',
    }))
    expect(wrapper.text()).toContain('已儲存')
  })

  /**
   * THE PASSWORD FIELD IS NEVER PREFILLED AND NEVER SENT UNTOUCHED. The server reads an
   * empty password as "clear it", so a save that posted the blank field would open a
   * password-protected post to everyone who has the link.
   */
  it('does not send a password the author did not type', async () => {
    const service = fakeService({
      post: vi.fn().mockResolvedValue({
        ok: true, value: { ...post, access_policy: 'password', has_password: true },
      }),
    })
    const { wrapper } = await render(service)

    expect((wrapper.find('#editor-password').element as HTMLInputElement).value).toBe('')

    await wrapper.find('#editor-title').setValue('a new title')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const draft = (service.updatePost as ReturnType<typeof vi.fn>).mock.calls[0]![1]
    expect(draft).not.toHaveProperty('password')
  })

  it('sends a password that was typed', async () => {
    const service = fakeService({
      post: vi.fn().mockResolvedValue({
        ok: true, value: { ...post, access_policy: 'password', has_password: true },
      }),
    })
    const { wrapper } = await render(service)

    await wrapper.find('#editor-password').setValue('a-new-door-code')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const draft = (service.updatePost as ReturnType<typeof vi.fn>).mock.calls[0]![1]
    expect(draft.password).toBe('a-new-door-code')
  })

  it('offers the password field only for a password-protected post', async () => {
    const publicPost = await render(fakeService())
    expect(publicPost.wrapper.find('#editor-password').exists()).toBe(false)

    const { wrapper } = await render(fakeService())
    await wrapper.findAll('.editor__policies input')[2]!.setValue()
    await flushPromises()
    expect(wrapper.find('#editor-password').exists()).toBe(true)
  })

  it('adds and removes tags, and sends them with the save', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)

    const tagInput = wrapper.find('.editor__tags input')
    await tagInput.setValue('dogs')
    await tagInput.trigger('keydown.enter')
    await flushPromises()
    expect(wrapper.text()).toContain('dogs')

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const draft = (service.updatePost as ReturnType<typeof vi.fn>).mock.calls[0]![1]
    expect(draft.tags).toEqual(['cats', 'dogs'])
  })

  // The pivot's composite key turns a repeat into a constraint error, which would fail
  // the whole save over a slip.
  it('ignores a tag that is already on the post', async () => {
    const { wrapper } = await render(fakeService())

    const tagInput = wrapper.find('.editor__tags input')
    await tagInput.setValue('cats')
    await tagInput.trigger('keydown.enter')
    await flushPromises()

    expect(wrapper.findAll('.editor__tag-list li')).toHaveLength(1)
  })

  it('stops offering the tag box at the limit of five', async () => {
    const { wrapper } = await render(fakeService({
      post: vi.fn().mockResolvedValue({
        ok: true, value: { ...post, tags: ['a', 'b', 'c', 'd', 'e'] },
      }),
    }))

    expect(wrapper.find('.editor__tags input').exists()).toBe(false)
  })

  /**
   * The account password is asked for only once the server says it needs one. The 11,040
   * accounts that signed in through Google have none and must not be shown a field they
   * cannot fill.
   */
  it('deletes without a password until the server asks for one', async () => {
    const service = fakeService()
    const { wrapper, router } = await render(service)

    await wrapper.find('.editor__danger button').trigger('click')
    await flushPromises()
    expect(wrapper.find('#editor-delete-password').exists()).toBe(false)

    await wrapper.find('.editor__confirm .editor__actions button').trigger('click')
    await flushPromises()

    expect(service.deletePost).toHaveBeenCalledWith('abcdefgh', undefined)
    expect(router.currentRoute.value.path).toBe('/zh-tw/account/posts')
  })

  it('reveals the password field when the server refuses without one', async () => {
    const service = fakeService({
      deletePost: vi.fn()
        .mockResolvedValueOnce({ ok: false, kind: 'validation', errors: { password: ['required'] } })
        .mockResolvedValueOnce({ ok: true, value: undefined }),
    })
    const { wrapper, router } = await render(service)

    await wrapper.find('.editor__danger button').trigger('click')
    await wrapper.find('.editor__confirm .editor__actions button').trigger('click')
    await flushPromises()

    const field = wrapper.find('#editor-delete-password')
    expect(field.exists(), 'the password field was not revealed').toBe(true)

    await field.setValue('the-account-password')
    await wrapper.find('.editor__confirm .editor__actions button').trigger('click')
    await flushPromises()

    expect(service.deletePost).toHaveBeenLastCalledWith('abcdefgh', 'the-account-password')
    expect(router.currentRoute.value.path).toBe('/zh-tw/account/posts')
  })

  it('shows the code a wrong delete password came back with', async () => {
    const service = fakeService({
      deletePost: vi.fn()
        .mockResolvedValueOnce({ ok: false, kind: 'validation', errors: { password: ['required'] } })
        .mockResolvedValueOnce({ ok: false, kind: 'validation', errors: { password: ['incorrect'] } }),
    })
    const { wrapper } = await render(service)

    await wrapper.find('.editor__danger button').trigger('click')
    await wrapper.find('.editor__confirm .editor__actions button').trigger('click')
    await flushPromises()
    await wrapper.find('#editor-delete-password').setValue('nope')
    await wrapper.find('.editor__confirm .editor__actions button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('密碼不正確')
  })

  it('lists the elements and their rank in this post', async () => {
    const { wrapper } = await render(fakeService())
    await openElementsTab(wrapper)

    expect(wrapper.text()).toContain('an element')
    expect(wrapper.text()).toContain('3')
  })

  it('renames an element without touching its trim', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.find('.editor__element-body .editor__actions button').trigger('click')
    await flushPromises()
    await wrapper.find('#element-title-5').setValue('renamed')
    await wrapper.find('.editor__element-form .editor__actions button').trigger('click')
    await flushPromises()

    expect(service.updateElement).toHaveBeenCalledWith(5, { title: 'renamed' })
  })

  // An empty box means "leave it", not "set it to zero" — the API tells those apart.
  it('does not send an empty trim box as a zero', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.find('.editor__element-body .editor__actions button').trigger('click')
    await flushPromises()
    await wrapper.find('#element-start-5').setValue('15')
    await wrapper.find('.editor__element-form .editor__actions button').trigger('click')
    await flushPromises()

    const edit = (service.updateElement as ReturnType<typeof vi.fn>).mock.calls[0]![1]
    expect(edit.video_start_second).toBe(15)
    expect(edit).not.toHaveProperty('video_end_second')
  })

  it('offers the trim boxes only for a video', async () => {
    const { wrapper } = await render(fakeService({
      elements: vi.fn().mockResolvedValue({
        ok: true,
        value: { elements: [{ ...element, type: 'image' }], total: 1, page: 1, per_page: 24 },
      }),
    }))
    await openElementsTab(wrapper)
    await wrapper.find('.editor__element-body .editor__actions button').trigger('click')
    await flushPromises()

    expect(wrapper.find('#element-start-5').exists()).toBe(false)
    expect(wrapper.find('#element-title-5').exists()).toBe(true)
  })

  // Inline, like the post's own delete — not window.confirm, which blocks the page and
  // cannot be styled to match anything around it.
  it('deletes an element only after the inline confirmation', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.findAll('.editor__element-body .editor__actions button')[1]!.trigger('click')
    await flushPromises()
    expect(service.deleteElement, 'the first click deleted without confirming').not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('確定要刪除這個素材嗎')

    await wrapper.findAll('.editor__element-body .editor__actions button')[0]!.trigger('click')
    await flushPromises()

    expect(service.deleteElement).toHaveBeenCalledWith(5)
    expect(wrapper.text()).toContain('這個投票還沒有素材')
  })

  it('leaves the element alone when the confirmation is cancelled', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.findAll('.editor__element-body .editor__actions button')[1]!.trigger('click')
    await flushPromises()
    await wrapper.findAll('.editor__element-body .editor__actions button')[1]!.trigger('click')
    await flushPromises()

    expect(service.deleteElement).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('an element')
  })

  // Both ways of adding media are here now, so nothing on this page points at Laravel.
  it('offers both the dropzone and the URL box, and links nowhere else', async () => {
    const { wrapper } = await render(fakeService())
    await openElementsTab(wrapper)

    expect(wrapper.find('.editor__dropzone').exists()).toBe(true)
    expect(wrapper.find('#editor-urls').exists()).toBe(true)
    const stragglers = wrapper.findAll('a')
      .filter((anchor) => anchor.attributes('href')?.includes('/account/post/'))
    expect(stragglers, 'the old editor is still linked').toHaveLength(0)
  })

  it('adds media from the pasted URLs', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.find('#editor-urls').setValue('https://www.youtube.com/watch?v=jNQXAC9IVRw')
    await wrapper.find('.editor__urls').trigger('submit')
    await flushPromises()

    expect(service.addElementsByURL).toHaveBeenCalledWith(
      'abcdefgh', 'https://www.youtube.com/watch?v=jNQXAC9IVRw')
    expect(wrapper.text()).toContain('已從網址新增 1 個素材')
  })

  /**
   * WHAT FAILED STAYS IN THE BOX. A batch normally succeeds in part, and clearing the
   * whole field would make the author retype the links that worked to retry the one that
   * did not.
   */
  it('keeps the failed URLs in the box and names why each one failed', async () => {
    const service = fakeService({
      addElementsByURL: vi.fn().mockResolvedValue({
        ok: true,
        value: {
          added: [element],
          failed: [{ url: 'https://dead.test/x', reason: 'unavailable' }],
        },
      }),
    })
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.find('#editor-urls').setValue('https://good.test/a.png,https://dead.test/x')
    await wrapper.find('.editor__urls').trigger('submit')
    await flushPromises()

    expect((wrapper.find('#editor-urls').element as HTMLTextAreaElement).value)
      .toBe('https://dead.test/x')
    expect(wrapper.text()).toContain('讀不到這個網址的資料')
  })

  it('shows the code a refused batch came back with', async () => {
    const service = fakeService({
      addElementsByURL: vi.fn().mockResolvedValue({
        ok: false, kind: 'validation', errors: { urls: ['too_many'] },
      }),
    })
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.find('#editor-urls').setValue('a,b,c')
    await wrapper.find('.editor__urls').trigger('submit')
    await flushPromises()

    expect(wrapper.text()).toContain('一次最多 100 個網址')
  })

  it('uploads the files that were dropped on it', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    const files = [
      new File(['a'], 'one.png', { type: 'image/png' }),
      new File(['b'], 'two.png', { type: 'image/png' }),
    ]
    await wrapper.find('.editor__dropzone').trigger('drop', { dataTransfer: { files } })
    await flushPromises()

    expect(service.uploadElement).toHaveBeenCalledTimes(2)
    expect(service.uploadElement).toHaveBeenNthCalledWith(1, 'abcdefgh', files[0])
    expect(service.uploadElement).toHaveBeenNthCalledWith(2, 'abcdefgh', files[1])
    expect(wrapper.text()).toContain('已新增 2 個素材')
  })

  // The list has to be re-read: the new elements are only on the server.
  it('reloads the elements after an upload', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)
    const before = (service.elements as ReturnType<typeof vi.fn>).mock.calls.length

    await wrapper.find('.editor__dropzone').trigger('drop', {
      dataTransfer: { files: [new File(['a'], 'one.png', { type: 'image/png' })] },
    })
    await flushPromises()

    expect((service.elements as ReturnType<typeof vi.fn>).mock.calls.length)
      .toBeGreaterThan(before)
  })

  /**
   * A refused file names itself and says why. A batch that reported only "something went
   * wrong" would leave the author guessing which of thirty files to look at.
   */
  it('names each file that failed and the reason', async () => {
    const service = fakeService({
      uploadElement: vi.fn()
        .mockResolvedValueOnce({ ok: true, value: element })
        .mockResolvedValueOnce({
          ok: false, kind: 'validation', errors: { file: ['unsupported_media'] },
        }),
    })
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.find('.editor__dropzone').trigger('drop', {
      dataTransfer: {
        files: [
          new File(['a'], 'good.png', { type: 'image/png' }),
          new File(['b'], 'notes.txt', { type: 'text/plain' }),
        ],
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('1 個檔案沒有上傳成功')
    expect(wrapper.text()).toContain('notes.txt')
    expect(wrapper.text()).toContain('不支援這種檔案格式')
    // The one that worked still counts.
    expect(wrapper.text()).toContain('已新增 1 個素材')
  })

  /**
   * A full post or an exhausted minute refuses everything after it too, so the rest of
   * the batch is abandoned rather than sent thirty times to be told the same thing.
   */
  it('stops the batch when the post is full or the budget is gone', async () => {
    for (const code of ['post_full', 'rate_limited']) {
      const service = fakeService({
        uploadElement: vi.fn().mockResolvedValue({
          ok: false, kind: 'validation', errors: { file: [code] },
        }),
      })
      const { wrapper } = await render(service)
      await openElementsTab(wrapper)

      await wrapper.find('.editor__dropzone').trigger('drop', {
        dataTransfer: {
          files: [
            new File(['a'], '1.png', { type: 'image/png' }),
            new File(['b'], '2.png', { type: 'image/png' }),
            new File(['c'], '3.png', { type: 'image/png' }),
          ],
        },
      })
      await flushPromises()

      expect(service.uploadElement, code).toHaveBeenCalledTimes(1)
    }
  })

  // One request per file and one at a time: the account's budget is 30 MiB or 50 files a
  // minute, and a burst would spend it on the first few and rate-limit the rest.
  it('uploads one file at a time', async () => {
    let inFlight = 0
    let overlapped = false
    const service = fakeService({
      // Yields on a microtask rather than a timer: flushPromises drains microtasks, so
      // the whole batch finishes before the assertion — and a parallel loop would still
      // have all three in flight across the yield.
      uploadElement: vi.fn().mockImplementation(async () => {
        inFlight += 1
        if (inFlight > 1) overlapped = true
        await Promise.resolve()
        await Promise.resolve()
        inFlight -= 1
        return { ok: true, value: element }
      }),
    })
    const { wrapper } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.find('.editor__dropzone').trigger('drop', {
      dataTransfer: {
        files: [
          new File(['a'], '1.png', { type: 'image/png' }),
          new File(['b'], '2.png', { type: 'image/png' }),
          new File(['c'], '3.png', { type: 'image/png' }),
        ],
      },
    })
    await flushPromises()

    expect(overlapped, 'two uploads were in flight at once').toBe(false)
    expect(service.uploadElement).toHaveBeenCalledTimes(3)
  })

  it('sends a signed-out uploader to the login form', async () => {
    const service = fakeService({
      uploadElement: vi.fn().mockResolvedValue({ ok: false, kind: 'signed-out' }),
    })
    const { wrapper, router } = await render(service)
    await openElementsTab(wrapper)

    await wrapper.find('.editor__dropzone').trigger('drop', {
      dataTransfer: { files: [new File(['a'], '1.png', { type: 'image/png' })] },
    })
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/zh-tw/login')
  })

  it('says so when the post is not the callers', async () => {
    const { wrapper } = await render(fakeService({
      post: vi.fn().mockResolvedValue({ ok: false, kind: 'not-found' }),
    }))

    expect(wrapper.text()).toContain('找不到這個投票')
  })

  it('sends a signed-out visitor to the login form', async () => {
    const { router } = await render(fakeService({
      post: vi.fn().mockResolvedValue({ ok: false, kind: 'signed-out' }),
    }))

    expect(router.currentRoute.value.path).toBe('/zh-tw/login')
    expect(router.currentRoute.value.query.redirect).toBe('/zh-tw/account/posts/abcdefgh')
  })
})
