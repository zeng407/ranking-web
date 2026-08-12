// @vitest-environment happy-dom

import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises, type DOMWrapper, type VueWrapper } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'

import AccountView from './AccountView.vue'
import type { Account, AccountService } from '../services/account'

/**
 * The settings page. The service is injected, so these tests are about what the page
 * offers and what it sends — in particular that it offers the right one of the two
 * password forms, which is a security difference rather than a cosmetic one.
 */

const account: Account = {
  name: 'the holder',
  email: 'holder@example.test',
  avatar_url: 'https://file.2pick.test/avatars/a.png',
  has_password: true,
  google_linked: true,
}

function fakeService(overrides: Partial<AccountService> = {}): AccountService {
  return {
    load: vi.fn().mockResolvedValue({ ok: true, value: account }),
    rename: vi.fn().mockResolvedValue({ ok: true, value: { ...account, name: 'after' } }),
    uploadAvatar: vi.fn().mockResolvedValue({ ok: true, value: 'https://file.2pick.test/new.png' }),
    changePassword: vi.fn().mockResolvedValue({ ok: true, value: undefined }),
    setInitialPassword: vi.fn().mockResolvedValue({ ok: true, value: undefined }),
    ...overrides,
  }
}

/** The password form is the second one on the page; the first is the name. */
function passwordForm(wrapper: VueWrapper): DOMWrapper<Element> {
  const form = wrapper.findAll('form')[1]
  if (!form) throw new Error('the password form is not on the page')
  return form
}

async function render(service: AccountService) {
  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/:locale/account', name: 'account', component: AccountView },
      { path: '/:locale/login', name: 'login', component: { template: '<p>login</p>' } },
    ],
  })
  await router.push('/zh-tw/account')
  await router.isReady()

  const wrapper = mount(AccountView, {
    props: { service },
    global: { plugins: [router] },
  })
  await flushPromises()
  return { wrapper, router }
}

describe('AccountView', () => {
  // 2,007 production accounts have no avatar. The placeholder is the header's, not
  // Laravel's storage/default-avatar.webp — the SPA is served from its own origin and that
  // path is not on it.
  it('draws a placeholder rather than a Laravel asset when there is no avatar', async () => {
    const { wrapper } = await render(fakeService({
      load: vi.fn().mockResolvedValue({ ok: true, value: { ...account, avatar_url: '' } }),
    }))

    expect(wrapper.find('.account__avatar img').exists()).toBe(false)
    expect(wrapper.find('.account__avatar-placeholder').exists()).toBe(true)
    expect(wrapper.html()).not.toContain('/storage/')
  })

  it('draws the account it loaded', async () => {
    const { wrapper } = await render(fakeService())

    expect((wrapper.find('#account-name').element as HTMLInputElement).value).toBe('the holder')
    const email = wrapper.find('#account-email').element as HTMLInputElement
    expect(email.value).toBe('holder@example.test')
    // Read-only, because the original's controller never accepted an address either.
    expect(email.disabled).toBe(true)
  })

  it('renames through the service and shows the saved notice', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)

    await wrapper.find('#account-name').setValue('after')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(service.rename).toHaveBeenCalledWith('after')
    expect(wrapper.text()).toContain('已儲存')
  })

  // The button is disabled rather than the request being sent and refused: submitting the
  // name you already have writes nothing, so offering it is offering a no-op.
  it('does not offer to save an unchanged name', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)

    const submit = wrapper.find('form button[type="submit"]')
    expect((submit.element as HTMLButtonElement).disabled).toBe(true)
  })

  it('shows the per-field code a refused rename came back with', async () => {
    const service = fakeService({
      rename: vi.fn().mockResolvedValue({
        ok: false, kind: 'validation', errors: { name: ['name_change_too_soon'] },
      }),
    })
    const { wrapper } = await render(service)

    await wrapper.find('#account-name').setValue('after')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.text()).toContain('暱稱一天只能改一次')
  })

  /**
   * WHICH PASSWORD FORM IS OFFERED IS A SECURITY DIFFERENCE. An account that has a
   * password must prove it; one that has none has nothing to prove. Offering the wrong
   * form would either ask for a password that does not exist or skip the proof.
   */
  it('asks for the current password only when the account has one', async () => {
    const withPassword = await render(fakeService())
    expect(withPassword.wrapper.find('#account-current').exists()).toBe(true)

    const without = await render(fakeService({
      load: vi.fn().mockResolvedValue({ ok: true, value: { ...account, has_password: false } }),
    }))
    expect(without.wrapper.find('#account-current').exists()).toBe(false)
    // And it explains why the option is there at all.
    expect(without.wrapper.text()).toContain('Google')
  })

  it('changes the password through the change endpoint', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)

    await wrapper.find('#account-current').setValue('the-old-password')
    await wrapper.find('#account-new').setValue('the-new-password')
    await wrapper.find('#account-confirm').setValue('the-new-password')
    await passwordForm(wrapper).trigger('submit')
    await flushPromises()

    expect(service.changePassword).toHaveBeenCalledWith('the-old-password', 'the-new-password')
    expect(service.setInitialPassword).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('其他裝置已登出')
  })

  it('sets a first password through the initial endpoint', async () => {
    const service = fakeService({
      load: vi.fn().mockResolvedValue({ ok: true, value: { ...account, has_password: false } }),
    })
    const { wrapper } = await render(service)

    await wrapper.find('#account-new').setValue('the-new-password')
    await wrapper.find('#account-confirm').setValue('the-new-password')
    await passwordForm(wrapper).trigger('submit')
    await flushPromises()

    expect(service.setInitialPassword).toHaveBeenCalledWith('the-new-password')
    expect(service.changePassword).not.toHaveBeenCalled()
  })

  // The confirmation is the form's job: the API has no message catalogue, so a mismatch
  // it reported could only come back as a code this page would have to translate anyway.
  it('catches a mismatched confirmation without spending a request', async () => {
    const service = fakeService()
    const { wrapper } = await render(service)

    await wrapper.find('#account-current').setValue('the-old-password')
    await wrapper.find('#account-new').setValue('the-new-password')
    await wrapper.find('#account-confirm').setValue('a-typo')
    await passwordForm(wrapper).trigger('submit')
    await flushPromises()

    expect(service.changePassword).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('不一致')
  })

  it('sends a signed-out visitor to the login form, remembering where they were', async () => {
    const service = fakeService({
      load: vi.fn().mockResolvedValue({ ok: false, kind: 'signed-out' }),
    })
    const { router } = await render(service)

    expect(router.currentRoute.value.path).toBe('/zh-tw/login')
    expect(router.currentRoute.value.query.redirect).toBe('/zh-tw/account')
  })

  it('says so when the account cannot be loaded', async () => {
    const { wrapper } = await render(fakeService({
      load: vi.fn().mockResolvedValue({ ok: false, kind: 'unavailable' }),
    }))

    expect(wrapper.text()).toContain('無法載入帳號資料')
  })

  it('offers the Google link only when there is none', async () => {
    const linked = await render(fakeService())
    expect(linked.wrapper.text()).toContain('已連結 Google 帳號')

    const unlinked = await render(fakeService({
      load: vi.fn().mockResolvedValue({ ok: true, value: { ...account, google_linked: false } }),
    }))
    expect(unlinked.wrapper.text()).toContain('連結 Google 帳號')
  })
})
