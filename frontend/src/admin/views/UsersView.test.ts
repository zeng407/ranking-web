// @vitest-environment happy-dom

import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UsersView from './UsersView.vue'
import { adminUser, fakeAdminService, ok } from '../testDouble'
import type { AdminService } from '../../services/admin'

async function render(service: AdminService) {
  const wrapper = mount(UsersView, { props: { service } })
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

describe('UsersView', () => {
  it('lists accounts with their roles', async () => {
    const wrapper = await render(fakeAdminService({
      users: vi.fn().mockResolvedValue(ok({
        users: [{ ...adminUser, roles: ['banned'] }], total: 1, page: 1, per_page: 20,
      })),
    }))

    expect(wrapper.text()).toContain('a member')
    expect(wrapper.text()).toContain('member@example.test')
    expect(wrapper.find('.admin-badge').text()).toBe('banned')
  })

  it('searches by keyword from the first page', async () => {
    const service = fakeAdminService()
    const wrapper = await render(service)

    await wrapper.find('input[type="search"]').setValue('  member@example.test  ')
    await buttonWith(wrapper, '搜尋').trigger('click')
    await flushPromises()

    expect(service.users).toHaveBeenLastCalledWith('member@example.test', 1)
  })

  it('confirms a ban and reports it', async () => {
    const service = fakeAdminService()
    const wrapper = await render(service)
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(true))

    await buttonWith(wrapper, '封鎖').trigger('click')
    await flushPromises()

    expect(service.banUser).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('已封鎖「a member」。')
  })

  // Unbanning restores an account rather than taking anything away, so it goes through
  // without a prompt — and it must not send a ban.
  it('unbans without asking', async () => {
    const service = fakeAdminService({
      users: vi.fn().mockResolvedValue(ok({
        users: [{ ...adminUser, roles: ['banned'] }], total: 1, page: 1, per_page: 20,
      })),
    })
    const wrapper = await render(service)
    const confirm = vi.fn()
    vi.stubGlobal('confirm', confirm)

    await buttonWith(wrapper, '解除封鎖').trigger('click')
    await flushPromises()

    expect(confirm).not.toHaveBeenCalled()
    expect(service.unbanUser).toHaveBeenCalledWith(42)
    expect(service.banUser).not.toHaveBeenCalled()
  })

  it('disables the button for an administrator', async () => {
    const wrapper = await render(fakeAdminService({
      users: vi.fn().mockResolvedValue(ok({
        users: [{ ...adminUser, roles: ['admin'] }], total: 1, page: 1, per_page: 20,
      })),
    }))

    expect(buttonWith(wrapper, '封鎖').attributes('disabled')).toBeDefined()
  })

  // The row's role list can be stale by the time it is clicked, so the server is the real
  // guard and its refusal has to read as a refusal, not as a fault.
  it('says why the server refused to ban an administrator', async () => {
    const wrapper = await render(fakeAdminService({
      banUser: vi.fn().mockResolvedValue({
        ok: false, kind: 'conflict', code: 'cannot_ban_administrator',
      }),
    }))
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(true))

    await buttonWith(wrapper, '封鎖').trigger('click')
    await flushPromises()

    expect(wrapper.find('.admin-alert.error').text()).toBe('管理員帳號不能被封鎖。')
  })
})
