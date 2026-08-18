// @vitest-environment happy-dom

import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'

const resetPassword = vi.fn()
vi.mock('../services/auth', () => ({
  resetPassword: (...args: unknown[]) => resetPassword(...args),
}))

const refreshAuthState = vi.fn().mockResolvedValue(undefined)
vi.mock('../composables/useAuth', () => ({
  useAuth: () => ({ refreshAuthState }),
}))

import ResetPasswordView from './ResetPasswordView.vue'

/**
 * The reset form. The token comes from the path, the confirmation is checked here, and a
 * finished reset leaves the user signed in — the three things that would each strand
 * somebody mid-recovery if they were wrong.
 */

async function render(path = '/zh-tw/password/reset/the-mailed-token') {
  resetPassword.mockReset()
  resetPassword.mockResolvedValue({ ok: true })
  refreshAuthState.mockClear()

  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/:locale/password/reset/:token', name: 'reset', component: ResetPasswordView },
      { path: '/:locale/password/forgot', name: 'forgot', component: { template: '<p>forgot</p>' } },
      { path: '/:locale/', name: 'home', component: { template: '<p>home</p>' } },
    ],
  })
  await router.push(path)
  await router.isReady()

  const wrapper = mount(ResetPasswordView, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

async function fill(wrapper: Awaited<ReturnType<typeof render>>['wrapper'],
  next: string, confirm: string): Promise<void> {
  await wrapper.find('input[name="new_password"]').setValue(next)
  await wrapper.find('input[name="new_password_confirmation"]').setValue(confirm)
  await wrapper.find('form').trigger('submit')
  await flushPromises()
}

describe('ResetPasswordView', () => {
  it('sends the token from the path and lands the user signed in', async () => {
    const { wrapper, router } = await render()

    await fill(wrapper, 'a-brand-new-password', 'a-brand-new-password')

    expect(resetPassword).toHaveBeenCalledWith(
      'zh_TW', { token: 'the-mailed-token', new_password: 'a-brand-new-password' })
    // Without this the header would still draw a Login link at a signed-in user.
    expect(refreshAuthState).toHaveBeenCalled()
    expect(router.currentRoute.value.path).toBe('/zh-tw/')
  })

  // A typo must not cost the link: the server would spend it on a request the form could
  // have refused for free.
  it('catches a mistyped confirmation without sending anything', async () => {
    const { wrapper } = await render()

    await fill(wrapper, 'a-brand-new-password', 'a-different-password')

    expect(resetPassword).not.toHaveBeenCalled()
    expect(wrapper.find('.auth-field-error').text()).toContain('不一致')
  })

  it('explains a spent or expired link and offers a new one', async () => {
    const { wrapper } = await render()
    resetPassword.mockResolvedValue({
      ok: false, kind: 'validation', errors: { token: ['invalid'] },
    })

    await fill(wrapper, 'a-brand-new-password', 'a-brand-new-password')

    const alert = wrapper.find('[role="alert"]')
    expect(alert.exists()).toBe(true)
    expect(alert.text()).toContain('失效')
    // The way out of a dead link is another mail, so the link to ask for one is on the page.
    expect(wrapper.find('a[href="/zh-tw/password/forgot"]').exists()).toBe(true)
  })

  it('reports a password the server refused on the password field', async () => {
    const { wrapper } = await render()
    resetPassword.mockResolvedValue({
      ok: false, kind: 'validation', errors: { new_password: ['too_short'] },
    })

    await fill(wrapper, 'short', 'short')

    expect(wrapper.find('.auth-field-error').text()).toContain('長度')
  })
})
