import type { InjectionKey } from 'vue'

import { htmlLanguage, type Locale } from '../i18n'
import type { CommentItem } from '../services/comments'

/**
 * How deep a reply may go, mirroring the server's limit. The reply affordance disappears
 * at the last level rather than the request being refused there.
 */
export const maxCommentDepth = 3

/** A comment together with the replies hanging off it. */
export interface CommentThread {
  comment: CommentItem
  /**
   * The nickname a reply is answering, shown in front of its text. Only set at the
   * deepest level: one level down, the comment being answered is the line directly
   * above, and naming it there would say what the indent already says.
   */
  addressee: string | null
  replies: CommentThread[]
}

/** The reply composer's state. One is open at a time, under the comment it answers. */
export interface ReplyState {
  targetID: number | null
  input: string
  submitting: boolean
  error: string
}

/**
 * What a comment needs from the section it belongs to.
 *
 * Passed by injection rather than by props: a thread renders itself recursively, and
 * threading the same handlers through every level as props and re-emitting every event
 * back up would be three levels of plumbing for a fixed set of actions.
 */
export interface CommentThreadContext {
  locale: Locale
  reply: ReplyState
  openReply(comment: CommentItem): void
  cancelReply(): void
  submitReply(): void
  openReport(comment: CommentItem): void
  confirmDelete(comment: CommentItem): void
}

export const commentThreadKey: InjectionKey<CommentThreadContext> = Symbol('comment-thread')

/**
 * Rebuilds the tree from the flat page the API returns.
 *
 * The server sends the comments already in display order — newest floor first, each
 * followed by its own replies oldest first — so nothing here sorts anything; the order
 * items arrive in is the order they are nested in. A reply whose parent is not on this
 * page cannot happen (a page carries whole threads), but if one ever did it is treated as
 * a floor rather than dropped, because a comment nobody can see is worse than one shown
 * at the wrong indent.
 */
export function buildThreads(items: CommentItem[]): CommentThread[] {
  const threads = new Map<number, CommentThread>()
  const roots: CommentThread[] = []
  for (const comment of items) {
    threads.set(comment.id, { comment, addressee: null, replies: [] })
  }
  for (const comment of items) {
    const thread = threads.get(comment.id)
    if (!thread) continue
    const parent = comment.parent_id === null ? undefined : threads.get(comment.parent_id)
    if (!parent) {
      roots.push(thread)
      continue
    }
    if (comment.depth >= maxCommentDepth && !parent.comment.deleted) {
      thread.addressee = parent.comment.nickname
    }
    parent.replies.push(thread)
  }
  return roots
}

/** The letter shown when someone has no avatar image. */
export function avatarInitial(nickname: string): string {
  return Array.from(nickname.trim())[0]?.toUpperCase() || '2'
}

/** "3 minutes ago", in the reader's language. */
export function relativeTime(value: string, locale: Locale): string {
  const timestamp = Date.parse(value)
  if (!Number.isFinite(timestamp)) return value
  const seconds = Math.round((timestamp - Date.now()) / 1_000)
  const absolute = Math.abs(seconds)
  const formatter = new Intl.RelativeTimeFormat(htmlLanguage(locale), { numeric: 'auto' })
  if (absolute < 60) return formatter.format(seconds, 'second')
  if (absolute < 3_600) return formatter.format(Math.round(seconds / 60), 'minute')
  if (absolute < 86_400) return formatter.format(Math.round(seconds / 3_600), 'hour')
  if (absolute < 2_592_000) return formatter.format(Math.round(seconds / 86_400), 'day')
  if (absolute < 31_536_000) return formatter.format(Math.round(seconds / 2_592_000), 'month')
  return formatter.format(Math.round(seconds / 31_536_000), 'year')
}
