// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

/**
 * The account menu is the only way to reach the settings page, so a page nobody can open
 * is the failure this file exists to catch.
 *
 * The session module is mocked rather than exercised: signing in for real would mean a
 * refresh request, and what is under test here is the markup the signed-in state produces.
 */
vi.mock('../services/session', () => ({
  getCachedSession: () => null,
  refreshSession: vi.fn().mockResolvedValue({
    accessToken: 'a-token', expiresAt: Date.now() + 300_000, userId: '42', roles: [],
  }),
  fetchProfile: vi.fn().mockResolvedValue({ name: 'the holder', avatar_url: '' }),
  clearSession: vi.fn(),
}))

vi.mock('../services/auth', () => ({
  logout: vi.fn().mockResolvedValue({ ok: true }),
  PASSWORD_RESET_PATH: '/password/reset',
}))

import AppHeader from './AppHeader.vue'
import { refreshAuthState } from '../composables/useAuth'

async function renderHeader() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/:locale/', name: 'home', component: { template: '<p>home</p>' } },
      { path: '/:locale/account', name: 'account', component: { template: '<p>account</p>' } },
      { path: '/:locale/account/posts', name: 'my-posts', component: { template: '<p>posts</p>' } },
      { path: '/:locale/login', name: 'login', component: { template: '<p>login</p>' } },
    ],
  })
  await router.push('/zh-tw/')
  await router.isReady()

  const wrapper = mount(AppHeader, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('AppHeader account menu', () => {
  beforeEach(async () => {
    await refreshAuthState(undefined, true)
  })

  it('offers the settings page to a signed-in visitor', async () => {
    const { wrapper } = await renderHeader()

    await wrapper.find('.account-toggle').trigger('click')

    const link = wrapper.findAll('.dropdown-item').find((item) => item.text() === '帳號設定')
    expect(link, 'the account menu has no link to the settings page').toBeDefined()
    expect(link!.attributes('href')).toBe('/zh-tw/account')
  })

  // The editor is reached from here too, so the same "a page nobody can open" failure
  // applies to it.
  it('offers the post editor', async () => {
    const { wrapper } = await renderHeader()

    await wrapper.find('.account-toggle').trigger('click')

    const link = wrapper.findAll('.dropdown-item').find((item) => item.text() === '管理我的投票')
    expect(link, 'the account menu has no link to the post list').toBeDefined()
    expect(link!.attributes('href')).toBe('/zh-tw/account/posts')
  })

  it('keeps the sign-out item alongside it', async () => {
    const { wrapper } = await renderHeader()

    await wrapper.find('.account-toggle').trigger('click')

    const items = wrapper.findAll('.dropdown-item').map((item) => item.text())
    expect(items).toContain('登出')
  })

  it('closes the menu when the settings page is opened', async () => {
    const { wrapper } = await renderHeader()
    await wrapper.find('.account-toggle').trigger('click')

    const link = wrapper.findAll('.dropdown-item').find((item) => item.text() === '帳號設定')
    await link!.trigger('click')

    expect(wrapper.find('.dropdown-panel').exists()).toBe(false)
  })
})
