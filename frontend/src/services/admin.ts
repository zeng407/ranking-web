import { APIError, getAPIClient, type APIClient } from '../lib/api'
import { getAccessToken } from './session'
import type { AccessPolicy, ElementPage, ElementQuery, PostElement } from './editor'

/**
 * The moderation back office, against the Go API.
 *
 * Replaces the Blade admin screens. Every call carries the same bearer token the rest of
 * the SPA uses, and the server checks the admin role on each one — the bundle being
 * loadable is not authorization for anything, only for reading the files.
 */

export interface AdminPost {
  serial: string
  title: string
  description: string
  access_policy: AccessPolicy
  is_censored: boolean
  play_count: number
  owner: { id: number; name: string; email: string }
  created_at?: string
}

export interface AdminPostPage {
  posts: AdminPost[]
  total: number
  page: number
  per_page: number
}

/** What GET /admin/posts/{serial} answers: the authoring view of a post. */
export interface AdminPostDetail {
  serial: string
  title: string
  description: string
  access_policy: AccessPolicy
  has_password: boolean
  tags: string[]
  play_count: number
  this_week_play_count: number
  last_week_play_count: number
  created_at?: string
}

export interface AdminPostEdit {
  title: string
  description: string
  access_policy: AccessPolicy
  /** Omitted rather than sent empty: an empty string would clear a stored password. */
  password?: string
  tags?: string[]
  /** Omitted leaves the flag alone. */
  is_censored?: boolean
}

export interface AdminUser {
  id: number
  name: string
  email: string
  avatar_url: string
  roles: string[]
  post_count: number
  created_at?: string
}

export interface AdminUserPage {
  users: AdminUser[]
  total: number
  page: number
  per_page: number
}

export type CarouselType = 'image' | 'video'

export interface CarouselItem {
  id: number
  position: number
  type: CarouselType
  title: string
  description: string
  image_url: string
  video_url: string
  video_source?: string
  video_id?: string
  video_start_second: number | null
  video_end_second: number | null
  is_active: boolean
}

export interface CarouselDraft {
  type: CarouselType
  title: string
  description: string
  image_url: string
  video_url: string
  video_start_second?: number | null
  video_end_second?: number | null
  is_active?: boolean
}

/** Every field is optional: an absent one is left as it is. */
export interface CarouselEdit {
  title?: string
  description?: string
  video_start_second?: number | null
  video_end_second?: number | null
  is_active?: boolean
}

export interface Announcement {
  id: string
  content: string
  image_url: string
  created_at: string
  keep_minutes: number
}

export interface AnnouncementDraft {
  content: string
  image_url: string
  keep_minutes: number
}

export interface AdminFieldErrors {
  [field: string]: string[]
}

/**
 * `forbidden` is its own kind rather than folded into `unavailable`: an account that lost
 * the moderator role has to be told that, not that the server is down.
 *
 * `conflict` is the refusal that is neither a bad field nor a fault — banning an
 * administrator is the one the API sends today — so it carries the machine code rather
 * than losing it. `unavailable` carries one too when the response had one, which is how a
 * 503 for an unconfigured announcement store can say so.
 */
export type AdminOutcome<T = void> =
  | { ok: true; value: T }
  | { ok: false; kind: 'validation'; errors: AdminFieldErrors }
  | { ok: false; kind: 'not-found' }
  | { ok: false; kind: 'signed-out' }
  | { ok: false; kind: 'forbidden' }
  | { ok: false; kind: 'conflict'; code: string }
  | { ok: false; kind: 'unavailable'; code?: string }

export interface AdminService {
  posts(page?: number, signal?: AbortSignal): Promise<AdminOutcome<AdminPostPage>>
  post(serial: string, signal?: AbortSignal): Promise<AdminOutcome<AdminPostDetail>>
  updatePost(serial: string, edit: AdminPostEdit): Promise<AdminOutcome<AdminPostDetail>>
  deletePost(serial: string): Promise<AdminOutcome>
  elements(serial: string, query?: ElementQuery, signal?: AbortSignal): Promise<AdminOutcome<ElementPage>>
  updateElement(id: number, edit: { title?: string; video_start_second?: number; video_end_second?: number }): Promise<AdminOutcome<PostElement>>
  deleteElement(id: number): Promise<AdminOutcome>

  users(keyword?: string, page?: number, signal?: AbortSignal): Promise<AdminOutcome<AdminUserPage>>
  banUser(id: number): Promise<AdminOutcome>
  unbanUser(id: number): Promise<AdminOutcome>

  carouselItems(signal?: AbortSignal): Promise<AdminOutcome<CarouselItem[]>>
  createCarouselItem(draft: CarouselDraft): Promise<AdminOutcome<CarouselItem>>
  updateCarouselItem(id: number, edit: CarouselEdit): Promise<AdminOutcome<CarouselItem>>
  deleteCarouselItem(id: number): Promise<AdminOutcome>
  /**
   * Writes the whole order in one request.
   *
   * The endpoint takes the entire list and answers with it, so a drag sends one call and
   * a failure leaves the stored order untouched rather than half applied.
   */
  reorderCarouselItems(items: { id: number; position: number }[]): Promise<AdminOutcome<CarouselItem[]>>

  /** Null is the ordinary "nothing is published" state, not a failure. */
  announcement(signal?: AbortSignal): Promise<AdminOutcome<Announcement | null>>
  publishAnnouncement(draft: AnnouncementDraft): Promise<AdminOutcome<Announcement>>

  /**
   * Mints the cookie that lets the browser load the back office bundle, then reports
   * whether it worked. The caller navigates with a full page load — the bundle is a
   * separate application, not a route of this one.
   */
  grantAssetPass(): Promise<AdminOutcome>
  /** Drops that cookie. Best effort: a moderator signing out should not wait on it. */
  revokeAssetPass(): Promise<AdminOutcome>
}

export function createAdminService(client: APIClient = getAPIClient()): AdminService {
  return {
    posts(page = 1, signal?: AbortSignal) {
      return attempt((headers) =>
        client.get<AdminPostPage>(`/admin/posts?page=${page}`, signal, 'include', headers))
    },

    post(serial: string, signal?: AbortSignal) {
      return attempt((headers) =>
        client.get<AdminPostDetail>(`/admin/posts/${encodeURIComponent(serial)}`,
          signal, 'include', headers))
    },

    updatePost(serial: string, edit: AdminPostEdit) {
      return attempt((headers) =>
        client.put<AdminPostDetail>(`/admin/posts/${encodeURIComponent(serial)}`, edit,
          undefined, 'include', headers))
    },

    async deletePost(serial: string) {
      const outcome = await attempt((headers) =>
        client.delete<void>(`/admin/posts/${encodeURIComponent(serial)}`,
          undefined, undefined, 'include', headers))
      return outcome.ok ? { ok: true as const, value: undefined } : outcome
    },

    elements(serial: string, query: ElementQuery = {}, signal?: AbortSignal) {
      const search = elementSearch(query)
      return attempt((headers) =>
        client.get<ElementPage>(
          `/admin/posts/${encodeURIComponent(serial)}/elements${search ? `?${search}` : ''}`,
          signal, 'include', headers))
    },

    updateElement(id: number, edit) {
      return attempt((headers) =>
        client.put<PostElement>(`/admin/elements/${id}`, edit, undefined, 'include', headers))
    },

    async deleteElement(id: number) {
      const outcome = await attempt((headers) =>
        client.delete<void>(`/admin/elements/${id}`, undefined, undefined, 'include', headers))
      return outcome.ok ? { ok: true as const, value: undefined } : outcome
    },

    users(keyword = '', page = 1, signal?: AbortSignal) {
      const parameters = new URLSearchParams({ page: String(page) })
      if (keyword) parameters.set('q', keyword)
      return attempt((headers) =>
        client.get<AdminUserPage>(`/admin/users?${parameters}`, signal, 'include', headers))
    },

    banUser(id: number) {
      return changeBan(id, 'ban')
    },

    unbanUser(id: number) {
      return changeBan(id, 'unban')
    },

    async carouselItems(signal?: AbortSignal) {
      const outcome = await attempt((headers) =>
        client.get<{ items: CarouselItem[] }>('/admin/carousel-items', signal, 'include', headers))
      return outcome.ok ? { ok: true as const, value: outcome.value.items } : outcome
    },

    createCarouselItem(draft: CarouselDraft) {
      return attempt((headers) =>
        client.post<CarouselItem>('/admin/carousel-items', draft, undefined, 'include', headers))
    },

    updateCarouselItem(id: number, edit: CarouselEdit) {
      return attempt((headers) =>
        client.put<CarouselItem>(`/admin/carousel-items/${id}`, edit, undefined, 'include', headers))
    },

    async deleteCarouselItem(id: number) {
      const outcome = await attempt((headers) =>
        client.delete<void>(`/admin/carousel-items/${id}`, undefined, undefined, 'include', headers))
      return outcome.ok ? { ok: true as const, value: undefined } : outcome
    },

    async reorderCarouselItems(items: { id: number; position: number }[]) {
      const outcome = await attempt((headers) =>
        client.put<{ items: CarouselItem[] }>('/admin/carousel-items/reorder', { items },
          undefined, 'include', headers))
      return outcome.ok ? { ok: true as const, value: outcome.value.items } : outcome
    },

    async announcement(signal?: AbortSignal) {
      const outcome = await attempt((headers) =>
        client.get<{ announcement: Announcement | null }>('/admin/announcement',
          signal, 'include', headers))
      return outcome.ok ? { ok: true as const, value: outcome.value.announcement } : outcome
    },

    publishAnnouncement(draft: AnnouncementDraft) {
      return attempt((headers) =>
        client.put<Announcement>('/admin/announcement', draft, undefined, 'include', headers))
    },

    async grantAssetPass() {
      const outcome = await attempt((headers) =>
        client.post<void>('/admin/assets/grant', {}, undefined, 'include', headers))
      return outcome.ok ? { ok: true as const, value: undefined } : outcome
    },

    async revokeAssetPass() {
      const outcome = await attempt((headers) =>
        client.post<void>('/admin/assets/revoke', {}, undefined, 'include', headers))
      return outcome.ok ? { ok: true as const, value: undefined } : outcome
    },
  }

  async function changeBan(id: number, action: 'ban' | 'unban'): Promise<AdminOutcome> {
    const outcome = await attempt((headers) =>
      client.put<void>(`/admin/users/${id}/${action}`, {}, undefined, 'include', headers))
    return outcome.ok ? { ok: true as const, value: undefined } : outcome
  }

  /**
   * Runs one request with a bearer token, and turns every failure into an outcome.
   *
   * The token is read per call, the same as in the editor service: an admin screen stays
   * open while a moderator reads through a list, and an access token lasts five minutes.
   */
  async function attempt<T>(request: (headers: HeadersInit) => Promise<T>): Promise<AdminOutcome<T>> {
    const token = await getAccessToken(client)
    if (!token) return { ok: false, kind: 'signed-out' }
    try {
      return { ok: true, value: await request({ Authorization: `Bearer ${token}` }) }
    } catch (error) {
      if (!(error instanceof APIError)) return { ok: false, kind: 'unavailable' }
      if (error.status === 422) {
        return { ok: false, kind: 'validation', errors: fieldErrorsFrom(error.data) }
      }
      if (error.status === 401) return { ok: false, kind: 'signed-out' }
      if (error.status === 403) return { ok: false, kind: 'forbidden' }
      if (error.status === 404) return { ok: false, kind: 'not-found' }
      if (error.status === 409) return { ok: false, kind: 'conflict', code: error.code }
      return { ok: false, kind: 'unavailable', code: error.code }
    }
  }
}

function elementSearch(query: ElementQuery): string {
  const parameters = new URLSearchParams()
  if (query.page) parameters.set('page', String(query.page))
  if (query.per_page) parameters.set('per_page', String(query.per_page))
  if (query.title) parameters.set('title', query.title)
  if (query.sort_by) parameters.set('sort_by', query.sort_by)
  if (query.sort_dir) parameters.set('sort_dir', query.sort_dir)
  return parameters.toString()
}

function fieldErrorsFrom(data: unknown): AdminFieldErrors {
  if (!data || typeof data !== 'object') return {}
  const errors = (data as { errors?: unknown }).errors
  if (!errors || typeof errors !== 'object') return {}

  const result: AdminFieldErrors = {}
  for (const [field, codes] of Object.entries(errors as Record<string, unknown>)) {
    if (Array.isArray(codes)) {
      result[field] = codes.filter((code): code is string => typeof code === 'string')
    }
  }
  return result
}

/**
 * Reorders a list by moving one entry, and numbers the result from 1.
 *
 * Kept out of the view so the drag handler stays a call rather than index arithmetic, and
 * so the numbering the API receives is testable on its own.
 */
export function movedOrder<T extends { id: number }>(
  items: readonly T[],
  from: number,
  to: number,
): { id: number; position: number }[] {
  const next = [...items]
  if (from < 0 || from >= next.length || to < 0 || to >= next.length || from === to) {
    return next.map((item, index) => ({ id: item.id, position: index + 1 }))
  }
  next.splice(to, 0, ...next.splice(from, 1))
  return next.map((item, index) => ({ id: item.id, position: index + 1 }))
}

let service: AdminService | null = null

export function getAdminService(): AdminService {
  if (!service) service = createAdminService()
  return service
}

/** Test seam. */
export function resetAdminServiceForTests(): void {
  service = null
}
