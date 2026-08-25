import { getAnonymousID } from '../lib/anonymousId'
import { getAPIClient, type APIClient } from '../lib/api'
import { getAccessToken } from './session'

export interface CommentItem {
  id: number
  /** The comment this one replies to, or null for a comment on a floor of its own. */
  parent_id: number | null
  /** 1 for a floor, 2 or 3 for a reply. Replies stop being offered at 3. */
  depth: number
  /** The floor number, counted from the oldest comment. Null for a reply. */
  floor: number | null
  content: string
  created_at: string
  edited_at: string | null
  nickname: string
  avatar_url: string | null
  champions: string[]
  /**
   * A deleted comment kept in place so its floor number and the replies under it
   * survive. The server strips the author and the text; the client shows a notice.
   */
  deleted: boolean
  /** Whether this browser or account may delete it. Decided by the server. */
  can_delete: boolean
}

export interface CommentProfile {
  nickname: string
  avatar_url: string | null
  champions: string[]
  is_auth: boolean
}

export interface CommentsPage {
  items: CommentItem[]
  page: number
  per_page: number
  total: number
  total_pages: number
  profile: CommentProfile
}

/**
 * Resolves the bearer token for a request. Returns null when nobody is signed in, which
 * is normal: comments are readable anonymously and are posted under an anonymous id.
 */
type TokenResolver = () => Promise<string | null>

export function createCommentsService(
  client: APIClient = getAPIClient(),
  resolveToken: TokenResolver = () => getAccessToken(),
) {
  async function authorization(_locale?: string): Promise<HeadersInit | undefined> {
    const token = await resolveToken()
    return token ? { Authorization: `Bearer ${token}` } : undefined
  }

  return {
    async list(postSerial: string, page: number, locale: string): Promise<CommentsPage> {
      const query = new URLSearchParams({ page: String(page), anonymous_id: getAnonymousID() })
      return client.get(
        `/posts/${encodeURIComponent(postSerial)}/comments?${query}`,
        undefined,
        'include',
        await authorization(locale),
      )
    },
    async create(
      postSerial: string,
      input: { content: string; anonymous: boolean; parent_id?: number },
      locale: string,
    ): Promise<CommentItem> {
      return client.post(
        `/posts/${encodeURIComponent(postSerial)}/comments`,
        { ...input, anonymous_id: getAnonymousID() },
        undefined,
        'include',
        await authorization(locale),
      )
    },
    /**
     * Deletes a comment. The request carries no proof of ownership of its own: a signed-in
     * caller is known by its bearer token, and a signed-out one by the httpOnly delete-key
     * cookie the browser attaches, which is why credentials are included.
     */
    async remove(postSerial: string, commentID: number, locale: string): Promise<void> {
      return client.delete(
        `/posts/${encodeURIComponent(postSerial)}/comments/${commentID}`,
        undefined,
        undefined,
        'include',
        await authorization(locale),
      )
    },
    async report(postSerial: string, commentID: number, reason: string, locale: string): Promise<{ reported: boolean }> {
      return client.post(
        `/posts/${encodeURIComponent(postSerial)}/comments/${commentID}/report`,
        { reason, anonymous_id: getAnonymousID() },
        undefined,
        'include',
        await authorization(locale),
      )
    },
  }
}
