// @vitest-environment happy-dom

import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'

const requestPasswordReset = vi.fn()
vi.mock('../services/auth', () => ({
  requestPasswordReset: (...args: unknown[]) => requestPasswordReset(...args),
}))

import ForgotPasswordView from './ForgotPasswordView.vue'

/**
 * The forgot-password form. What matters here is the copy on the success screen: the
 * server answers the same for an address it has never seen, so a page that said "we have
 * sent you a mail" would be making a claim the server refuses to make — and would turn
 * this form into a way to test which addresses are registered.
 */

async function render() {
  requestPasswordReset.mockReset()
  requestPasswordReset.mockResolvedValue({ ok: true })

  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/:locale/password/forgot', name: 'forgot', component: ForgotPasswordView },
      { path: '/:locale/login', name: 'login', component: { template: '<p>login</p>' } },
    ],
  })
  await router.push('/zh-tw/password/forgot')
  await router.isReady()

  const wrapper = mount(ForgotPasswordView, { global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

describe('ForgotPasswordView', () => {
  it('sends the address with the locale of the page', async () => {
    const wrapper = await render()

    await wrapper.find('input[name="email"]').setValue('player@example.test')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(requestPasswordReset).toHaveBeenCalledWith('zh_TW', 'player@example.test')
  })

  it('shows a conditional confirmation rather than promising a mail', async () => {
    const wrapper = await render()

    await wrapper.find('input[name="email"]').setValue('player@example.test')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const notice = wrapper.find('[role="status"]')
    expect(notice.exists()).toBe(true)
    // "If this address is registered" — the condition is the whole point.
    expect(notice.text()).toContain('如果')
    // The form is gone, so the same address cannot be submitted again on a whim.
    expect(wrapper.find('form').exists()).toBe(false)
  })

  it('shows the rejected address on the field', async () => {
    const wrapper = await render()
    requestPasswordReset.mockResolvedValue({
      ok: false, kind: 'validation', errors: { email: ['invalid_email'] },
    })

    await wrapper.find('input[name="email"]').setValue('not-an-address')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('.auth-field-error').text()).toContain('格式')
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
  })

  it('reports an unreachable service without claiming anything was sent', async () => {
    const wrapper = await render()
    requestPasswordReset.mockResolvedValue({ ok: false, kind: 'unavailable' })

    await wrapper.find('input[name="email"]').setValue('player@example.test')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
  })
})
