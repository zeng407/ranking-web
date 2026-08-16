// @vitest-environment happy-dom

import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'

import PostsView from './PostsView.vue'
import { adminPost, adminPostDetail, fakeAdminService, ok } from '../testDouble'
import type { AdminService } from '../../services/admin'

async function render(service: AdminService) {
  const router = createRouter({
    history: createWebHistory('/admin/'),
    routes: [
      { path: '/posts', component: PostsView },
      { path: '/posts/:serial/elements', component: { template: '<p>elements</p>' } },
    ],
  })
  await router.push('/posts')
  await router.isReady()

  const wrapper = mount(PostsView, { props: { service }, global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

function buttonWith(wrapper: Awaited<ReturnType<typeof render>>, label: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text() === label)
  if (!button) throw new Error(`no button labelled ${label}`)
  return button
}

afterEach(() => {
  vi.unstubAllGlobals()
})

/** happy-dom has no confirm, so the destructive paths get one they can answer. */
function answerConfirm(answer: boolean) {
  const confirm = vi.fn().mockReturnValue(answer)
  vi.stubGlobal('confirm', confirm)
  return confirm
}

describe('PostsView', () => {
  it('lists every post with its author and serial', async () => {
    const wrapper = await render(fakeAdminService())

    expect(wrapper.text()).toContain('a title')
    expect(wrapper.text()).toContain('abcdefgh')
    expect(wrapper.text()).toContain('author@example.test')
    expect(wrapper.text()).toContain('500')
  })

  it('links a row to that post\'s elements', async () => {
    const wrapper = await render(fakeAdminService())

    const link = wrapper.findAll('a').find((candidate) => candidate.text() === '元素')
    expect(link?.attributes('href')).toBe('/admin/posts/abcdefgh/elements')
  })

  // The edit form is filled from the detail endpoint, not from the row: the list has no
  // tags and no password state, and saving what the row knows would erase them.
  it('loads the detail before editing and sends the author\'s tags back', async () => {
    const service = fakeAdminService()
    const wrapper = await render(service)

    await buttonWith(wrapper, '編輯').trigger('click')
    await flushPromises()
    expect(service.post).toHaveBeenCalledWith('abcdefgh')
    expect(wrapper.text()).toContain('編輯「a title」')

    await buttonWith(wrapper, '儲存').trigger('click')
    await flushPromises()

    expect(service.updatePost).toHaveBeenCalledWith('abcdefgh', {
      title: 'a title',
      description: 'a description',
      access_policy: 'public',
      tags: ['cats', 'dogs'],
      is_censored: false,
    })
  })

  it('omits the password unless one was typed', async () => {
    const service = fakeAdminService({
      post: vi.fn().mockResolvedValue(ok({ ...adminPostDetail, access_policy: 'password', has_password: true })),
    })
    const wrapper = await render(service)

    await buttonWith(wrapper, '編輯').trigger('click')
    await flushPromises()
    await buttonWith(wrapper, '儲存').trigger('click')
    await flushPromises()

    expect(vi.mocked(service.updatePost).mock.calls[0]?.[1]).not.toHaveProperty('password')

    await buttonWith(wrapper, '編輯').trigger('click')
    await flushPromises()
    const password = wrapper.findAll('input[type="text"]')[1]
    await password?.setValue('a new password')
    await buttonWith(wrapper, '儲存').trigger('click')
    await flushPromises()

    expect(vi.mocked(service.updatePost).mock.calls[1]?.[1]).toMatchObject({ password: 'a new password' })
  })

  it('flips the censored flag without touching the rest of the post', async () => {
    const service = fakeAdminService()
    const wrapper = await render(service)

    await buttonWith(wrapper, '封鎖').trigger('click')
    await flushPromises()

    expect(service.updatePost).toHaveBeenCalledWith('abcdefgh', {
      title: 'a title',
      description: 'a description',
      access_policy: 'public',
      tags: ['cats', 'dogs'],
      is_censored: true,
    })
    expect(wrapper.text()).toContain('已封鎖「a title」。')
  })

  it('asks before deleting, and does nothing when the answer is no', async () => {
    const service = fakeAdminService()
    const wrapper = await render(service)
    const confirm = answerConfirm(false)

    await buttonWith(wrapper, '刪除').trigger('click')
    await flushPromises()
    expect(service.deletePost).not.toHaveBeenCalled()

    confirm.mockReturnValue(true)
    await buttonWith(wrapper, '刪除').trigger('click')
    await flushPromises()
    expect(service.deletePost).toHaveBeenCalledWith('abcdefgh')
  })

  it('explains a refusal instead of showing an empty table', async () => {
    const wrapper = await render(fakeAdminService({
      posts: vi.fn().mockResolvedValue({ ok: false, kind: 'forbidden' }),
    }))

    expect(wrapper.find('.admin-alert.error').text()).toBe('這個帳號沒有管理權限。')
  })

  it('pages through the list', async () => {
    const service = fakeAdminService({
      posts: vi.fn().mockResolvedValue(ok({ posts: [adminPost], total: 45, page: 1, per_page: 20 })),
    })
    const wrapper = await render(service)

    expect(wrapper.text()).toContain('第 1 / 3 頁，共 45 筆')
    await buttonWith(wrapper, '下一頁').trigger('click')
    await flushPromises()

    expect(service.posts).toHaveBeenLastCalledWith(2)
  })
})
