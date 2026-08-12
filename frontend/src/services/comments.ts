import { getAnonymousID } from '../lib/anonymousId'
import { getAPIClient, type APIClient } from '../lib/api'
import { getAccessToken } from './session'

export interface CommentItem {
  id: number
  content: string
  created_at: string
  edited_at: string | null
  nickname: string
  avatar_url: string | null
  champions: string[]
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
    async create(postSerial: string, input: { content: string; anonymous: boolean }, locale: string): Promise<CommentItem> {
      return client.post(
        `/posts/${encodeURIComponent(postSerial)}/comments`,
        { ...input, anonymous_id: getAnonymousID() },
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
