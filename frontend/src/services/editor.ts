import { APIError, getAPIClient, type APIClient } from '../lib/api'
import { getAccessToken } from './session'

/**
 * The post editor, against the Go API.
 *
 * Replaces what IndexPost.vue and EditPost.vue talked to. Adding media is not here: that
 * endpoint is still Laravel's, and the pipeline behind it has not been ported.
 */

export type AccessPolicy = 'private' | 'public' | 'password'

export interface MyPost {
  serial: string
  title: string
  description: string
  access_policy: AccessPolicy
  /** Whether a password is set — never what it is. */
  has_password: boolean
  tags: string[]
  play_count: number
  this_week_play_count: number
  last_week_play_count: number
  created_at?: string
}

export interface MyPostPage {
  posts: MyPost[]
  total: number
  page: number
  per_page: number
}

export interface PostDraft {
  title: string
  description: string
  access_policy: AccessPolicy
  /** Omitted rather than sent empty: an empty string would clear a stored password. */
  password?: string
  /** Omitted leaves the tags alone; an empty array clears them. */
  tags?: string[]
}

export interface ElementRank {
  rank: number
  win_rate: number
  final_win_rate: number
}

export interface PostElement {
  id: number
  source_url: string
  thumb_url: string
  mediumthumb_url: string
  lowthumb_url: string
  title: string
  type: 'image' | 'video'
  video_source?: string
  video_id?: string
  video_duration_second: number | null
  video_start_second: number | null
  video_end_second: number | null
  created_at?: string
  rank: ElementRank | null
}

export interface ElementPage {
  elements: PostElement[]
  total: number
  page: number
  per_page: number
}

export interface ElementQuery {
  page?: number
  per_page?: number
  title?: string
  sort_by?: 'id' | 'title'
  sort_dir?: 'asc' | 'desc'
}

export interface ElementEdit {
  title?: string
  video_start_second?: number
  video_end_second?: number
}

export interface EditorFieldErrors {
  [field: string]: string[]
}

export type EditorOutcome<T = void> =
  | { ok: true; value: T }
  | { ok: false; kind: 'validation'; errors: EditorFieldErrors }
  | { ok: false; kind: 'not-found' }
  | { ok: false; kind: 'signed-out' }
  | { ok: false; kind: 'unavailable' }

/** The four limits the upload endpoint enforces, mirrored so the form can say them. */
export const UPLOAD_LIMITS = {
  maxFileBytes: 4 * 1024 * 1024,
  maxElements: 1024,
  /** Per account, per minute. */
  bytesAMinute: 30 * 1024 * 1024,
  filesAMinute: 50,
}

export interface EditorService {
  posts(page?: number, signal?: AbortSignal): Promise<EditorOutcome<MyPostPage>>
  post(serial: string, signal?: AbortSignal): Promise<EditorOutcome<MyPost>>
  createPost(draft: PostDraft): Promise<EditorOutcome<string>>
  updatePost(serial: string, draft: PostDraft): Promise<EditorOutcome<MyPost>>
  /** password is omitted for an account that has none; the server says when it needs one. */
  deletePost(serial: string, password?: string): Promise<EditorOutcome>
  elements(serial: string, query?: ElementQuery, signal?: AbortSignal): Promise<EditorOutcome<ElementPage>>
  updateElement(id: number, edit: ElementEdit): Promise<EditorOutcome<PostElement>>
  deleteElement(id: number): Promise<EditorOutcome>
  /** Adds one uploaded file to a post. The endpoint takes one file per request. */
  uploadElement(serial: string, file: File): Promise<EditorOutcome<PostElement>>
  /**
   * Adds media from a pasted list of URLs.
   *
   * A batch normally succeeds in part, so this resolves ok even when some URLs failed —
   * the failures are in the value, not in the outcome.
   */
  addElementsByURL(serial: string, urls: string): Promise<EditorOutcome<AddedElements>>
}

export interface AddedElements {
  added: PostElement[]
  failed: { url: string; reason: string }[]
}

export function createEditorService(client: APIClient = getAPIClient()): EditorService {
  return {
    posts(page = 1, signal?: AbortSignal) {
      return attempt((headers) =>
        client.get<MyPostPage>(`/account/posts?page=${page}`, signal, 'include', headers))
    },

    post(serial: string, signal?: AbortSignal) {
      return attempt((headers) =>
        client.get<MyPost>(`/account/posts/${encodeURIComponent(serial)}`, signal, 'include', headers))
    },

    async createPost(draft: PostDraft) {
      const outcome = await attempt((headers) =>
        client.post<{ serial: string }>('/account/posts', draft, undefined, 'include', headers))
      return outcome.ok ? { ok: true as const, value: outcome.value.serial } : outcome
    },

    updatePost(serial: string, draft: PostDraft) {
      return attempt((headers) =>
        client.put<MyPost>(`/account/posts/${encodeURIComponent(serial)}`, draft,
          undefined, 'include', headers))
    },

    async deletePost(serial: string, password?: string) {
      const outcome = await attempt((headers) =>
        client.delete<void>(`/account/posts/${encodeURIComponent(serial)}`,
          password === undefined ? undefined : { password }, undefined, 'include', headers))
      return outcome.ok ? { ok: true as const, value: undefined } : outcome
    },

    elements(serial: string, query: ElementQuery = {}, signal?: AbortSignal) {
      const parameters = new URLSearchParams()
      if (query.page) parameters.set('page', String(query.page))
      if (query.per_page) parameters.set('per_page', String(query.per_page))
      if (query.title) parameters.set('title', query.title)
      if (query.sort_by) parameters.set('sort_by', query.sort_by)
      if (query.sort_dir) parameters.set('sort_dir', query.sort_dir)
      const search = parameters.toString()
      return attempt((headers) =>
        client.get<ElementPage>(
          `/account/posts/${encodeURIComponent(serial)}/elements${search ? `?${search}` : ''}`,
          signal, 'include', headers))
    },

    updateElement(id: number, edit: ElementEdit) {
      return attempt((headers) =>
        client.put<PostElement>(`/account/elements/${id}`, edit, undefined, 'include', headers))
    },

    async uploadElement(serial: string, file: File) {
      const form = new FormData()
      form.append('file', file)
      return attempt((headers) =>
        client.postForm<PostElement>(
          `/account/posts/${encodeURIComponent(serial)}/elements/uploads`,
          form, undefined, 'include', headers))
    },

    addElementsByURL(serial: string, urls: string) {
      return attempt((headers) =>
        client.post<AddedElements>(
          `/account/posts/${encodeURIComponent(serial)}/elements/urls`,
          { urls }, undefined, 'include', headers))
    },

    async deleteElement(id: number) {
      const outcome = await attempt((headers) =>
        client.delete<void>(`/account/elements/${id}`, undefined, undefined, 'include', headers))
      return outcome.ok ? { ok: true as const, value: undefined } : outcome
    },
  }

  /**
   * Runs one request with a bearer token, and turns every failure into an outcome.
   *
   * The token is read per call: an editor page stays open for as long as it takes to
   * write a description, and an access token lasts five minutes.
   */
  async function attempt<T>(request: (headers: HeadersInit) => Promise<T>): Promise<EditorOutcome<T>> {
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
      // 404 is also what a post belonging to someone else answers, deliberately: the
      // server does not distinguish, and neither should this.
      if (error.status === 404) return { ok: false, kind: 'not-found' }
      return { ok: false, kind: 'unavailable' }
    }
  }
}

function fieldErrorsFrom(data: unknown): EditorFieldErrors {
  if (!data || typeof data !== 'object') return {}
  const errors = (data as { errors?: unknown }).errors
  if (!errors || typeof errors !== 'object') return {}

  const result: EditorFieldErrors = {}
  for (const [field, codes] of Object.entries(errors as Record<string, unknown>)) {
    if (Array.isArray(codes)) {
      result[field] = codes.filter((code): code is string => typeof code === 'string')
    }
  }
  return result
}

/**
 * Builds the draft to send.
 *
 * A password is sent only when one was typed. An empty string would clear the stored one
 * — which is exactly what the author did NOT ask for by leaving the field alone.
 */
export function draftFrom(
  form: { title: string; description: string; access_policy: AccessPolicy; password: string },
  tags?: string[],
): PostDraft {
  const draft: PostDraft = {
    title: form.title.trim(),
    description: form.description.trim(),
    access_policy: form.access_policy,
  }
  if (form.access_policy === 'password' && form.password) draft.password = form.password
  if (tags) draft.tags = tags
  return draft
}

let service: EditorService | null = null

export function getEditorService(): EditorService {
  if (!service) service = createEditorService()
  return service
}

/** Test seam. */
export function resetEditorServiceForTests(): void {
  service = null
}
