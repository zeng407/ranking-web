// @vitest-environment happy-dom

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CommentSection from './CommentSection.vue'
import type { CommentItem } from '../services/comments'

const commentMocks = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  report: vi.fn(),
  remove: vi.fn(),
}))

vi.mock('../services/comments', () => ({
  createCommentsService: () => commentMocks,
}))

function comment(overrides: Partial<CommentItem> & { id: number }): CommentItem {
  return {
    parent_id: null,
    depth: 1,
    floor: null,
    content: '內容',
    created_at: '2026-08-03T10:00:00+08:00',
    edited_at: null,
    nickname: '留言者',
    avatar_url: null,
    champions: [],
    deleted: false,
    can_delete: false,
    ...overrides,
  }
}

function page(items: CommentItem[], overrides: Record<string, unknown> = {}) {
  return {
    items,
    page: 1,
    per_page: 10,
    total: items.length,
    total_pages: 1,
    profile: { nickname: '測試使用者', avatar_url: null, champions: ['本次冠軍'], is_auth: true },
    ...overrides,
  }
}

/** The replies one level under a comment, and not the ones under those. */
function repliesOf(card: Element): Element[] {
  return Array.from(card.querySelectorAll(':scope > article > .comment-replies > .comment-card'))
}

/** A comment's own text, ignoring the replies rendered inside it. */
function contentOf(card: Element): string {
  return card.querySelector(':scope > article > .comment-content')?.textContent ?? ''
}

describe('CommentSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    commentMocks.list.mockResolvedValue(page([comment({
      id: 7,
      floor: 1,
      content: '推薦新增這個選項\n第二行',
      avatar_url: 'https://example.test/avatar.webp',
      champions: ['本次冠軍'],
    })], { total: 11, total_pages: 2 }))
    commentMocks.create.mockResolvedValue({ id: 8 })
    commentMocks.report.mockResolvedValue({ reported: true })
    commentMocks.remove.mockResolvedValue(undefined)
  })

  it('restores comment identity, champion labels, submission, pagination, and reporting', async () => {
    const wrapper = mount(CommentSection, {
      props: { postSerial: 'post-1', locale: 'zh_TW' },
    })
    await flushPromises()

    expect(wrapper.get('.comments-title').text()).toContain('11')
    expect(wrapper.get('.comment-author').text()).toContain('留言者')
    expect(wrapper.get('.comment-floor').text()).toBe('1F')
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

  it('nests replies under the floor they answer and stops offering to reply at the third level', async () => {
    commentMocks.list.mockResolvedValue(page([
      comment({ id: 7, floor: 2, content: '第二樓' }),
      comment({ id: 3, floor: 1, content: '第一樓' }),
      comment({ id: 4, parent_id: 3, depth: 2, content: '回覆一樓', nickname: '回覆者' }),
      comment({ id: 5, parent_id: 4, depth: 3, content: '回到底了', nickname: '第三層' }),
    ]))
    const wrapper = mount(CommentSection, { props: { postSerial: 'post-1', locale: 'zh_TW' } })
    await flushPromises()

    // Floors sit at the top level in the order the server sent them; replies hang off
    // the comment they answer rather than being listed beside it.
    const floors = wrapper.findAll('.comments-list > .comment-card')
    expect(floors).toHaveLength(2)
    expect(contentOf(floors[0]!.element)).toBe('第二樓')

    const replies = repliesOf(floors[1]!.element)
    expect(replies).toHaveLength(1)
    expect(contentOf(replies[0]!)).toBe('回覆一樓')
    expect(replies[0]!.classList).toContain('is-reply')

    const deepest = repliesOf(replies[0]!)
    expect(deepest).toHaveLength(1)
    // Only the deepest level names who it answers; one level down the indent says it.
    expect(contentOf(deepest[0]!)).toBe('@回覆者回到底了')
    expect(deepest[0]!.querySelector('.comment-reply-button')).toBeNull()
    expect(replies[0]!.querySelector(':scope > article > .comment-reply-button')).not.toBeNull()

    wrapper.unmount()
  })

  it('sends a reply against the comment it was opened under', async () => {
    commentMocks.list.mockResolvedValue(page([comment({ id: 3, floor: 1, content: '第一樓' })]))
    const wrapper = mount(CommentSection, { props: { postSerial: 'post-1', locale: 'zh_TW' } })
    await flushPromises()

    expect(wrapper.find('.comment-reply-form').exists()).toBe(false)
    await wrapper.get('.comment-reply-button').trigger('click')
    await wrapper.get('.comment-reply-form .comment-input').setValue('我來回覆')
    await wrapper.get('.comment-reply-submit').trigger('click')
    await flushPromises()

    expect(commentMocks.create).toHaveBeenCalledWith('post-1', {
      content: '我來回覆',
      anonymous: false,
      parent_id: 3,
    }, 'zh_TW')
    // The reply lands on the page being read, so that page is what is reloaded.
    expect(commentMocks.list).toHaveBeenLastCalledWith('post-1', 1, 'zh_TW')
    expect(wrapper.find('.comment-reply-form').exists()).toBe(false)

    wrapper.unmount()
  })

  it('keeps a deleted floor in place with nothing on it but its number', async () => {
    commentMocks.list.mockResolvedValue(page([
      comment({ id: 3, floor: 1, content: '', nickname: '', deleted: true }),
      comment({ id: 4, parent_id: 3, depth: 2, content: '回覆還在' }),
    ]))
    const wrapper = mount(CommentSection, { props: { postSerial: 'post-1', locale: 'zh_TW' } })
    await flushPromises()

    const floor = wrapper.get('.comments-list > .comment-card')
    expect(floor.classes()).toContain('is-deleted')
    expect(floor.get('.comment-floor').text()).toBe('1F')
    expect(floor.get('.comment-deleted').text()).toBe('這則留言已刪除')
    // Nothing to attribute, nothing to report, nothing to answer.
    expect(floor.element.querySelector(':scope > article > header .comment-author')).toBeNull()
    expect(floor.element.querySelector(':scope > article > header .comment-report-button')).toBeNull()
    expect(floor.element.querySelector(':scope > article > .comment-reply-button')).toBeNull()
    expect(contentOf(repliesOf(floor.element)[0]!)).toBe('回覆還在')

    wrapper.unmount()
  })

  it('offers deletion only for comments the server says are ours, and confirms first', async () => {
    commentMocks.list.mockResolvedValue(page([
      comment({ id: 3, floor: 2, content: '我的留言', can_delete: true }),
      comment({ id: 2, floor: 1, content: '別人的留言' }),
    ]))
    const wrapper = mount(CommentSection, { props: { postSerial: 'post-1', locale: 'zh_TW' } })
    await flushPromises()

    const cards = wrapper.findAll('.comments-list > .comment-card')
    expect(cards[0]?.find('.comment-delete-button').exists()).toBe(true)
    expect(cards[1]?.find('.comment-delete-button').exists()).toBe(false)

    await cards[0]!.get('.comment-delete-button').trigger('click')
    const dialog = wrapper.get('.comment-delete-dialog')
    expect((dialog.element as HTMLDialogElement).open).toBe(true)
    expect(dialog.get('blockquote').text()).toBe('我的留言')
    // Opening the dialog asks; only the confirming button deletes.
    expect(commentMocks.remove).not.toHaveBeenCalled()

    await wrapper.get('.comment-delete-submit').trigger('click')
    await flushPromises()

    expect(commentMocks.remove).toHaveBeenCalledWith('post-1', 3, 'zh_TW')
    expect(commentMocks.list).toHaveBeenLastCalledWith('post-1', 1, 'zh_TW')
    expect((dialog.element as HTMLDialogElement).open).toBe(false)

    wrapper.unmount()
  })
})
