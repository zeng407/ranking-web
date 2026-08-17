import { getAPIClient, type APIClient } from '../lib/api'
import { postAccessHeaders } from './postAccess'
import { getCachedSession } from './session'

export interface Tag {
  name: string
  count: number
}

export interface CarouselItem {
  title: string | null
  description: string | null
  image_url: string | null
  video_url: string | null
  position: number
  type: string
  video_source: string | null
  video_id: string | null
  video_start_second: string | null
}

export interface PostElement {
  id: number | null
  url: string | null
  url2: string | null
  title: string | null
  type: string | null
  video_source: string | null
  previewable: boolean
}

export interface PublicPost {
  title: string
  serial: string
  is_private: boolean
  description: string
  element1: PostElement
  element2: PostElement
  created_at: string
  updated_at: string
  play_count: number
  elements_count: number
  tags: string[]
  is_censored: number
}

export interface PostsPage {
  items: PublicPost[]
  page: number
  per_page: number
  total: number
  total_pages: number
}

export interface ChampionElement {
  name: string
  thumb_url: string | null
  is_winner: boolean
}

export interface Champion {
  post_title: string
  post_serial: string
  left: ChampionElement | null
  right: ChampionElement | null
  datetime: string
  thumb_url: string | null
  key: string
}

export interface RankElement {
  title: string | null
  type: string
  id: number
  video_id: string | null
  source_url: string | null
  video_source: string | null
  thumb_url: string | null
  lowthumb_url: string | null
  mediumthumb_url: string | null
}

/** One element's standing in a past ranking run. */
export interface RankSnapshot {
  rank: number
  win_rate: string
  date: string
}

export interface RankReport {
  rank: number | null
  win_rate: string
  date: string
  element: RankElement
  /**
   * The same element's place over the last thousand votes, sent with a cumulative
   * listing so one table holds both standings. Absent when the latest snapshot did
   * not place this element.
   */
  recent?: RankSnapshot | null
}

export type RankGroup = 'cumulative' | 'recent_1000'

export interface RankHistoryPoint {
  rank: number
  win_rate: string
  date: string
}

export interface RankDetails {
  current: RankReport | null
  groups?: {
    cumulative: RankReport | null
    recent_1000: RankReport | null
  }
  history: Record<string, RankHistoryPoint[]>
}

export interface RanksPage {
  items: RankReport[]
  group?: RankGroup
  page: number
  per_page: number
  total: number
  total_pages: number
}

export interface PostsParameters {
  sortBy?: 'hot' | 'new'
  range?: 'all' | 'year' | 'month' | 'week' | 'day'
  keyword?: string
  page?: number
  perPage?: number
}

export function createPublicContentService(client: APIClient = getAPIClient()) {
  const publicGet = <T>(path: string): Promise<T> => client.get<T>(path, undefined, 'omit')

  /**
   * A rank read, which a protected post's ranks are as protected as. Laravel had a whole
   * second set of endpoints for these behind PostPolicy::readRank; here the same request
   * carries whatever the caller has proved and the server decides.
   */
  const rankGet = <T>(path: string, postSerial: string): Promise<T> =>
    client.get<T>(path, undefined, 'omit', rankHeaders(postSerial))

  return {
    posts(parameters: PostsParameters = {}): Promise<PostsPage> {
      const sortBy = parameters.sortBy ?? 'hot'
      return publicGet(`/posts?${new URLSearchParams({
        sort_by: sortBy,
        ...(sortBy === 'hot' ? { range: parameters.range ?? 'week' } : {}),
        page: String(parameters.page ?? 1),
        per_page: String(parameters.perPage ?? 15),
        ...(parameters.keyword ? { k: parameters.keyword } : {}),
      })}`)
    },
    tags(keyword = ''): Promise<Tag[]> {
      const query = keyword ? `?${new URLSearchParams({ keyword })}` : ''
      return publicGet(`/tags${query}`)
    },
    hotTags(): Promise<Record<string, number>> {
      return publicGet('/tags/hot')
    },
    carouselItems(): Promise<CarouselItem[]> {
      return publicGet('/carousel-items')
    },
    champions(): Promise<Champion[]> {
      return publicGet('/champions')
    },
    /**
     * The one ranking table. Each cumulative row carries its own thousand-vote
     * standing, so the recent group is not a second list to fetch and page through.
     */
    ranks(postSerial: string, page = 1, perPage = 20): Promise<RanksPage> {
      return rankGet(`/ranks?${new URLSearchParams({
        post_serial: postSerial,
        group: 'cumulative',
        page: String(page),
        per_page: String(perPage),
      })}`, postSerial)
    },
    searchRanks(postSerial: string, keyword: string): Promise<RankReport[]> {
      return rankGet(
        `/rank/search?${new URLSearchParams({ post_serial: postSerial, keyword })}`, postSerial)
    },
    rank(postSerial: string, elementId: number, ranges: string[]): Promise<RankDetails> {
      const query = new URLSearchParams({ post_serial: postSerial, element_id: String(elementId) })
      ranges.forEach((range) => query.append('time', range))
      return rankGet(`/rank?${query}`, postSerial)
    },
  }
}

/** The same two claims the gameplay service sends. See callerHeaders there. */
function rankHeaders(postSerial: string): Record<string, string> {
  const headers: Record<string, string> = { ...postAccessHeaders(postSerial) }
  const session = getCachedSession()
  if (session && session.expiresAt > Date.now()) {
    headers.Authorization = `Bearer ${session.accessToken}`
  }
  return headers
}

export function preferredElementImage(element: PostElement): string | null {
  return element.url2 || element.url
}

export function carouselYoutubeEmbedURL(item: CarouselItem): string | null {
  if (!item.video_id || !['youtube', 'youtube_embed'].includes((item.video_source || '').toLowerCase())) {
    return null
  }
  const parameters = new URLSearchParams({
    controls: '1',
    playsinline: '1',
    rel: '0',
  })
  const startSecond = Number.parseInt(item.video_start_second || '', 10)
  if (Number.isFinite(startSecond) && startSecond > 0) parameters.set('start', String(startSecond))
  return `https://www.youtube-nocookie.com/embed/${encodeURIComponent(item.video_id)}?${parameters}`
}
