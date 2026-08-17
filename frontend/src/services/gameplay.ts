import { getAPIClient, type APIClient } from '../lib/api'
import { postAccessHeaders } from './postAccess'
import { getCachedSession } from './session'

/**
 * One of the two options previewed before a game starts. `url` is the display
 * thumbnail and `url2` a larger fallback for when the first fails to load.
 */
export interface PreviewElement {
  id: number | null
  url: string | null
  url2: string | null
  title: string | null
  type: string | null
  video_source: string | null
  previewable: boolean
}

export interface GameDefinition {
  title: string
  serial: string
  description: string
  is_censored: boolean
  elements_count: number
  max_elements: number
  // Optional so the page still works against an API that predates previews.
  element1?: PreviewElement | null
  element2?: PreviewElement | null
}

export interface GameElement {
  id: number
  source_url: string | null
  thumb_url: string | null
  mediumthumb_url: string | null
  lowthumb_url: string | null
  title: string
  type: string
  video_start_second: number | null
  video_end_second: number | null
  video_source: string | null
  video_id: string | null
  video_duration_second: number | null
}

export interface GameSession {
  game_serial: string
  server_vote_count: number
  post: GameDefinition
  elements: GameElement[]
}

export interface GameVote {
  winner_id: number
  loser_id: number
}

export interface BatchVoteResult {
  status: 'processing' | 'end_game'
  server_vote_count: number
  complete: boolean
}

export interface GameResultItem {
  rank: number
  win_count: number
  global_rank: number | null
  element: GameElement
}

export interface GameResult {
  game_serial: string
  post_serial: string
  items: GameResultItem[]
}

/**
 * What a request says about who is asking.
 *
 * Two independent claims, and a protected post accepts either: the door code proves
 * knowledge of a shared secret, the bearer token proves an account — and an author reads
 * their own posts without ever being given a code. GamePolicy::play checked them in the
 * same order.
 *
 * The cached session is used rather than a refreshed one: these calls run on every vote,
 * and a token refresh on each is a cost paid by the overwhelming majority of players, who
 * are playing a public post while signed out. A signed-in author whose token has lapsed
 * sees the post reload, not a failure.
 */
function callerHeaders(postSerial?: string): Record<string, string> {
  const headers: Record<string, string> = { ...postAccessHeaders(postSerial) }
  const session = getCachedSession()
  if (session && session.expiresAt > Date.now()) {
    headers.Authorization = `Bearer ${session.accessToken}`
  }
  return headers
}

export function createGameplayService(client: APIClient = getAPIClient()) {
  return {
    definition(postSerial: string, signal?: AbortSignal): Promise<GameDefinition> {
      return client.get(`/game-posts/${encodeURIComponent(postSerial)}`, signal, 'omit',
        callerHeaders(postSerial))
    },
    create(postSerial: string, elementCount: number, signal?: AbortSignal): Promise<GameSession> {
      return client.post('/games', {
        post_serial: postSerial,
        element_count: elementCount,
      }, signal, 'omit', callerHeaders(postSerial))
    },
    resume(gameSerial: string, signal?: AbortSignal): Promise<GameSession> {
      // No post serial to name: a game serial says nothing about which post it belongs to
      // until the server looks, so every token this client holds rides along.
      return client.get(`/games/${encodeURIComponent(gameSerial)}/elements`, signal, 'omit',
        callerHeaders())
    },
    result(gameSerial: string, signal?: AbortSignal): Promise<GameResult> {
      return client.get(`/games/${encodeURIComponent(gameSerial)}/result`, signal, 'omit',
        callerHeaders())
    },
    /**
     * Submits a batch of local votes.
     *
     * currentCandidates is the pair the client is showing AFTER these votes, and is sent
     * only while hosting a game room. games.candidates means "the pair on screen": in
     * Laravel the server knew it because the host asked for each next pair, but this client
     * plays its bracket locally, so it has to say. Without it the column ends up holding the
     * pair just eliminated and the room shows its participants an already-decided match.
     */
    submitVotes(
      gameSerial: string,
      expectedVoteCount: number,
      votes: GameVote[],
      anonymousId: string,
      signal?: AbortSignal,
      currentCandidates?: number[],
    ): Promise<BatchVoteResult> {
      return client.post(`/games/${encodeURIComponent(gameSerial)}/votes/batch`, {
        expected_vote_count: expectedVoteCount,
        votes,
        anonymous_id: anonymousId,
        // Omitted rather than sent empty: an absent hint leaves the column alone, which is
        // right for a solo game.
        ...(currentCandidates && currentCandidates.length === 2
          ? { current_candidates: currentCandidates }
          : {}),
      }, signal, 'omit', callerHeaders())
    },
  }
}

export function preferredGameImage(element: GameElement): string | null {
  return element.lowthumb_url || element.mediumthumb_url || element.thumb_url || element.source_url
}

/**
 * The largest picture the API offers, falling back down the thumbnail sizes. A
 * video has no full-size frame, so its thumbnail is as large as it gets.
 */
export function fullSizeImage(element: Pick<GameElement, 'type' | 'source_url' | 'thumb_url' | 'mediumthumb_url' | 'lowthumb_url'>): string | null {
  if (element.type === 'video') return element.thumb_url ?? element.mediumthumb_url ?? null
  return element.source_url ?? element.thumb_url ?? element.mediumthumb_url ?? element.lowthumb_url ?? null
}

export function gamePreviewImage(element: GameElement): string | null {
  const thumbnail = element.lowthumb_url || element.mediumthumb_url || element.thumb_url
  if (thumbnail) return thumbnail
  if (element.video_id && ['youtube', 'youtube_embed'].includes((element.video_source || '').toLowerCase())) {
    return `https://i.ytimg.com/vi/${encodeURIComponent(element.video_id)}/hqdefault.jpg`
  }
  return element.type === 'image' ? element.source_url : null
}

/**
 * The subset of an element this needs. Ranking rows carry the same video columns
 * as gameplay elements but none of the gameplay-only fields, so the parameter is
 * structural rather than a full GameElement.
 */
export interface EmbeddableVideo {
  video_id: string | null
  video_source: string | null
  video_start_second?: number | null
  video_end_second?: number | null
}

export function youtubeEmbedURL(element: EmbeddableVideo): string | null {
  if (!element.video_id || !['youtube', 'youtube_embed'].includes((element.video_source || '').toLowerCase())) return null
  const parameters = new URLSearchParams({
    autoplay: '1',
    mute: '1',
    controls: '1',
    enablejsapi: '1',
    playsinline: '1',
    rel: '0',
    loop: '1',
    playlist: element.video_id,
  })
  if (element.video_start_second) parameters.set('start', String(element.video_start_second))
  if (element.video_end_second) parameters.set('end', String(element.video_end_second))
  return `https://www.youtube-nocookie.com/embed/${encodeURIComponent(element.video_id)}?${parameters}`
}
