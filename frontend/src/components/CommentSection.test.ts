// @vitest-environment happy-dom

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CommentSection from './CommentSection.vue'

const commentMocks = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  report: vi.fn(),
}))

vi.mock('../services/comments', () => ({
  createCommentsService: () => commentMocks,
}))

describe('CommentSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    commentMocks.list.mockResolvedValue({
      items: [{
        id: 7,
        content: '推薦新增這個選項\n第二行',
        created_at: '2026-08-03T10:00:00+08:00',
        edited_at: null,
        nickname: '留言者',
        avatar_url: 'https://example.test/avatar.webp',
        champions: ['本次冠軍'],
      }],
      page: 1,
      per_page: 10,
      total: 11,
      total_pages: 2,
      profile: {
        nickname: '測試使用者',
        avatar_url: null,
        champions: ['本次冠軍'],
        is_auth: true,
      },
    })
    commentMocks.create.mockResolvedValue({ id: 8 })
    commentMocks.report.mockResolvedValue({ reported: true })
  })

  it('restores comment identity, champion labels, submission, pagination, and reporting', async () => {
    const wrapper = mount(CommentSection, {
      props: { postSerial: 'post-1', locale: 'zh_TW' },
    })
    await flushPromises()

    expect(wrapper.get('.comments-title').text()).toContain('11')
    expect(wrapper.get('.comment-author').text()).toContain('留言者')
    expect(wrapper.get('.comment-content').text()).toContain('推薦新增這個選項')
    expect(wrapper.get('.comment-champion').text()).toContain('本次冠軍')
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(true)

    await wrapper.get('.comment-input').setValue('新的留言')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    expect(wrapper.get('.comment-submit').attributes('disabled')).toBeUndefined()
    await wrapper.get('.comment-submit').trigger('click')
    await flushPromises()

    expect(commentMocks.create).toHaveBeenCalledWith('post-1', {
      content: '新的留言',
      anonymous: true,
    }, 'zh_TW')

    await wrapper.get('.comment-report-button').trigger('click')
    expect((wrapper.get('.comment-report-dialog').element as HTMLDialogElement).open).toBe(true)
    await wrapper.get('.comment-report-reason').setValue('Other')
    await wrapper.get('.comment-report-other').setValue('自訂原因')
    await wrapper.get('.comment-report-submit').trigger('click')
    await flushPromises()

    expect(commentMocks.report).toHaveBeenCalledWith('post-1', 7, '自訂原因', 'zh_TW')

    await wrapper.get('.comment-pagination button:last-child').trigger('click')
    await flushPromises()
    expect(commentMocks.list).toHaveBeenLastCalledWith('post-1', 2, 'zh_TW')

    wrapper.unmount()
  })
})
