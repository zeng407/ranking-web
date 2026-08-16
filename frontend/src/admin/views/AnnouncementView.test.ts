// @vitest-environment happy-dom

import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AnnouncementView from './AnnouncementView.vue'
import { announcement, fakeAdminService, ok } from '../testDouble'
import type { AdminService } from '../../services/admin'

async function render(service: AdminService) {
  const wrapper = mount(AnnouncementView, { props: { service } })
  await flushPromises()
  return wrapper
}

function buttonWith(wrapper: Awaited<ReturnType<typeof render>>, label: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text() === label)
  if (!button) throw new Error(`no button labelled ${label}`)
  return button
}

describe('AnnouncementView', () => {
  // Nothing published is the ordinary state of this screen, so it must not look broken.
  it('says there is no announcement rather than showing an error', async () => {
    const wrapper = await render(fakeAdminService())

    expect(wrapper.text()).toContain('目前沒有公告。')
    expect(wrapper.find('.admin-alert.error').exists()).toBe(false)
  })

  it('fills the form from the published announcement', async () => {
    const wrapper = await render(fakeAdminService({
      announcement: vi.fn().mockResolvedValue(ok(announcement)),
    }))

    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('the site is up')
    expect(wrapper.text()).toContain('保留 60 分鐘')
  })

  it('publishes what was typed', async () => {
    const service = fakeAdminService()
    const wrapper = await render(service)

    await wrapper.find('textarea').setValue('  the site is going down at nine  ')
    await wrapper.find('input[type="number"]').setValue('30')
    await buttonWith(wrapper, '發布').trigger('click')
    await flushPromises()

    expect(service.publishAnnouncement).toHaveBeenCalledWith({
      content: 'the site is going down at nine',
      image_url: '',
      keep_minutes: 30,
    })
    expect(wrapper.text()).toContain('公告已發布。')
  })

  // A server without the announcement store is a configuration fact, not a fault; the
  // 503 carries a code so the screen can say which.
  it('explains an unconfigured announcement store', async () => {
    const wrapper = await render(fakeAdminService({
      announcement: vi.fn().mockResolvedValue({
        ok: false, kind: 'unavailable', code: 'announcements_not_configured',
      }),
    }))

    expect(wrapper.find('.admin-alert.error').text()).toBe('這台伺服器沒有設定公告儲存位置，公告無法讀寫。')
  })

  it('reports the fields the server rejected', async () => {
    const wrapper = await render(fakeAdminService({
      publishAnnouncement: vi.fn().mockResolvedValue({
        ok: false, kind: 'validation', errors: { content: ['required'] },
      }),
    }))

    await buttonWith(wrapper, '發布').trigger('click')
    await flushPromises()

    expect(wrapper.find('.admin-alert.error').text()).toContain('content 必填')
  })
})
