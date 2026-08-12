// @vitest-environment happy-dom

import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'

import MyPostsView from './MyPostsView.vue'
import type { EditorService, MyPost } from '../services/editor'

const post: MyPost = {
  serial: 'abcdefgh',
  title: 'a title',
  description: 'a description',
  access_policy: 'password',
  has_password: true,
  tags: ['cats'],
  play_count: 500,
  this_week_play_count: 20,
  last_week_play_count: 30,
}

function fakeService(overrides: Partial<EditorService> = {}): EditorService {
  return {
    posts: vi.fn().mockResolvedValue({
      ok: true, value: { posts: [post], total: 1, page: 1, per_page: 15 },
    }),
    post: vi.fn(),
    createPost: vi.fn().mockResolvedValue({ ok: true, value: 'newserial' }),
    updatePost: vi.fn(),
    deletePost: vi.fn(),
    elements: vi.fn(),
    updateElement: vi.fn(),
    deleteElement: vi.fn(),
    uploadElement: vi.fn(),
    addElementsByURL: vi.fn(),
    ...overrides,
  }
}

async function render(service: EditorService) {
  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/:locale/account/posts', name: 'posts', component: MyPostsView },
      {
        path: '/:locale/account/posts/:serial',
        name: 'editor',
        component: { template: '<p>editor</p>' },
      },
      { path: '/:locale/login', name: 'login', component: { template: '<p>login</p>' } },
    ],
  })
  await router.push('/zh-tw/account/posts')
  await router.isReady()

  const wrapper = mount(MyPostsView, { props: { service }, global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('MyPostsView', () => {
  it('lists the posts with their policy and play counts', async () => {
    const { wrapper } = await render(fakeService())

    expect(wrapper.text()).toContain('a title')
    expect(wrapper.text()).toContain('密碼存取')
    expect(wrapper.text()).toContain('500')
  })

  it('links each post to its editor', async () => {
    const { wrapper } = await render(fakeService())

    expect(wrapper.find('.posts__item-title').attributes('href')).toBe('/zh-tw/account/posts/abcdefgh')
  })

  it('says so when there is nothing yet', async () => {
    const { wrapper } = await render(fakeService({
      posts: vi.fn().mockResolvedValue({ ok: true, value: { posts: [], total: 0, page: 1, per_page: 15 } }),
    }))

    expect(wrapper.text()).toContain('無資料')
  })

  it('creates a post and opens its editor', async () => {
    const service = fakeService()
    const { wrapper, router } = await render(service)

    await wrapper.find('.posts__head button').trigger('click')
    await wrapper.find('#post-title').setValue('a new vote')
    await wrapper.find('#post-description').setValue('what it is about')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(service.createPost).toHaveBeenCalledWith({
      title: 'a new vote', description: 'what it is about', access_policy: 'public',
    })
    // Straight into the editor: a post with no media is not finished.
    expect(router.currentRoute.value.path).toBe('/zh-tw/account/posts/newserial')
  })

  it('asks for a password only when the policy needs one', async () => {
    const { wrapper } = await render(fakeService())
    await wrapper.find('.posts__head button').trigger('click')
    await flushPromises()

    expect(wrapper.find('#post-password').exists()).toBe(false)

    await wrapper.findAll('.posts__policies input')[2]!.setValue()
    await flushPromises()

    expect(wrapper.find('#post-password').exists()).toBe(true)
  })

  it('shows the per-field code a refused create came back with', async () => {
    const service = fakeService({
      createPost: vi.fn().mockResolvedValue({
        ok: false, kind: 'validation', errors: { title: ['required'] },
      }),
    })
    const { wrapper, router } = await render(service)

    await wrapper.find('.posts__head button').trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.text()).toContain('必填')
    expect(router.currentRoute.value.path).toBe('/zh-tw/account/posts')
  })

  it('pages through the list', async () => {
    const service = fakeService({
      posts: vi.fn().mockResolvedValue({
        ok: true, value: { posts: [post], total: 40, page: 1, per_page: 15 },
      }),
    })
    const { wrapper } = await render(service)

    // 40 posts at 15 a page is three pages.
    expect(wrapper.text()).toContain('1 / 3')

    await wrapper.findAll('.posts__pages button')[1]!.trigger('click')
    await flushPromises()

    expect(service.posts).toHaveBeenLastCalledWith(2)
  })

  it('sends a signed-out visitor to the login form', async () => {
    const { router } = await render(fakeService({
      posts: vi.fn().mockResolvedValue({ ok: false, kind: 'signed-out' }),
    }))

    expect(router.currentRoute.value.path).toBe('/zh-tw/login')
    expect(router.currentRoute.value.query.redirect).toBe('/zh-tw/account/posts')
  })

  it('says so when the list cannot be loaded', async () => {
    const { wrapper } = await render(fakeService({
      posts: vi.fn().mockResolvedValue({ ok: false, kind: 'unavailable' }),
    }))

    expect(wrapper.text()).toContain('無法載入投票列表')
  })
})
