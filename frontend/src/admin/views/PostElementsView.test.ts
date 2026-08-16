// @vitest-environment happy-dom

import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'

import PostElementsView from './PostElementsView.vue'
import { fakeAdminService, ok } from '../testDouble'
import type { AdminService } from '../../services/admin'
import type { PostElement } from '../../services/editor'

const element: PostElement = {
  id: 91,
  source_url: 'https://file.2pick.test/a.png',
  thumb_url: 'https://file.2pick.test/a-thumb.png',
  mediumthumb_url: 'https://file.2pick.test/a-medium.png',
  lowthumb_url: 'https://file.2pick.test/a-low.png',
  title: 'a candidate',
  rank: null,
  type: 'image',
  video_duration_second: null,
  video_start_second: null,
  video_end_second: null,
}

function onePage(overrides: Partial<PostElement> = {}) {
  return ok({ elements: [{ ...element, ...overrides }], total: 1, page: 1, per_page: 20 })
}

async function render(service: AdminService) {
  const router = createRouter({
    history: createWebHistory('/admin/'),
    routes: [
      { path: '/posts', component: { template: '<p>posts</p>' } },
      { path: '/posts/:serial/elements', component: PostElementsView },
    ],
  })
  await router.push('/posts/abcdefgh/elements')
  await router.isReady()

  const wrapper = mount(PostElementsView, { props: { service }, global: { plugins: [router] } })
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

describe('PostElementsView', () => {
  it('reads the post from the route and shows its elements', async () => {
    const service = fakeAdminService({ elements: vi.fn().mockResolvedValue(onePage()) })
    const wrapper = await render(service)

    expect(service.elements).toHaveBeenCalledWith('abcdefgh', { page: 1, title: undefined })
    expect(wrapper.text()).toContain('a candidate')
    expect(wrapper.find('img.admin-thumb').attributes('src')).toBe('https://file.2pick.test/a-low.png')
  })

  it('renames an element in place', async () => {
    const service = fakeAdminService({ elements: vi.fn().mockResolvedValue(onePage()) })
    const wrapper = await render(service)

    await buttonWith(wrapper, '改名').trigger('click')
    await wrapper.find('tbody input[type="text"]').setValue('  a better name  ')
    await buttonWith(wrapper, '儲存').trigger('click')
    await flushPromises()

    expect(service.updateElement).toHaveBeenCalledWith(91, { title: 'a better name' })
    expect(wrapper.text()).toContain('已更新名稱。')
  })

  it('searches by title', async () => {
    const service = fakeAdminService({ elements: vi.fn().mockResolvedValue(onePage()) })
    const wrapper = await render(service)

    await wrapper.find('input[type="search"]').setValue('candidate')
    await buttonWith(wrapper, '搜尋').trigger('click')
    await flushPromises()

    expect(service.elements).toHaveBeenLastCalledWith('abcdefgh', { page: 1, title: 'candidate' })
  })

  it('asks before deleting an element', async () => {
    const service = fakeAdminService({ elements: vi.fn().mockResolvedValue(onePage()) })
    const wrapper = await render(service)
    const confirm = vi.fn().mockReturnValue(false)
    vi.stubGlobal('confirm', confirm)

    await buttonWith(wrapper, '刪除').trigger('click')
    await flushPromises()
    expect(service.deleteElement).not.toHaveBeenCalled()

    confirm.mockReturnValue(true)
    await buttonWith(wrapper, '刪除').trigger('click')
    await flushPromises()
    expect(service.deleteElement).toHaveBeenCalledWith(91)
  })

  it('says when the post is gone', async () => {
    const wrapper = await render(fakeAdminService({
      elements: vi.fn().mockResolvedValue({ ok: false, kind: 'not-found' }),
    }))

    expect(wrapper.find('.admin-alert.error').text()).toBe('找不到這筆資料，可能已經被刪除。')
  })
})
