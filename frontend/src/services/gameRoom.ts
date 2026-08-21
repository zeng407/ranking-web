import { getAnonymousID } from '../lib/anonymousId'
import { getAPIClient, type APIClient } from '../lib/api'

/**
 * The multiplayer betting room.
 *
 * A room is playable without an account: identity inside one is the browser's anonymous
 * id, the same value the comments service sends. An access token, when the visitor happens
 * to have one, is recorded server-side for audit but is never required and never decides
 * who you are in the room.
 */

/**
 * The nickname cap, in characters. Mirrors gameroom.MaxNicknameRunes on the server, which
 * counts RUNES rather than bytes — the input's maxlength counts UTF-16 code units, which
 * agrees for everything in these names and is the closest the DOM offers.
 */
export const MaxNicknameLength = 10

export interface RoomPlayer {
  player_id: string
  name: string
  score: number
  rank: number
  /** A string, because the server sends "63.49" and a float would render as 63.489999999. */
  accuracy: string
  total_played: number
  total_correct: number
  combo: number
}

/** One entry of the broadcast payload. Carries user_id, not player_id — see the API spec. */
export interface BoardPlayer {
  user_id: string
  name: string
  score: number
  rank: number
  accuracy: string
  total_played: number
  total_correct: number
  combo: number
}

export interface Leaderboard {
  total_users: number
  /** Best first. */
  top_10: BoardPlayer[]
  /** WORST first: the server preserves the old ordering, and reversing it would flip the UI. */
  bottom_10: BoardPlayer[]
}

export interface RoomVotes {
  first_candidate: number
  second_candidate: number
  first_candidate_votes: number
  second_candidate_votes: number
  remain_elements: number
  total_votes: number
  /**
   * The match in progress, so the view can show "31 of 32". Derived server-side: the round
   * table records COMPLETED matches, so the one in play is a bracket calculation the
   * browser has no state for.
   */
  current_round: number
  of_round: number
}

export interface RoomBet {
  winner_id: number
  loser_id: number
  current_round: number
  of_round: number
  remain_elements: number
}

export interface RoomState {
  serial: string
  game_serial: string
  player: RoomPlayer | null
  /** Null when no pairing is in progress — distinct from "nobody has voted yet". */
  votes: RoomVotes | null
  /** The caller's own last wager, for rehydrating a reloaded page. */
  latest_bet: RoomBet | null
  leaderboard: Leaderboard | null
}

export interface OpenedRoom {
  serial: string
  game_serial: string
}

export function createGameRoomService(client: APIClient = getAPIClient()) {
  return {
    /**
     * Opens the room for a game, or returns the one already running.
     *
     * currentCandidates is the pair already on screen. A host opens the room mid-game, and
     * without it the first participants are shown the match that was last decided.
     */
    async open(gameSerial: string, currentCandidates?: number[]): Promise<OpenedRoom> {
      return client.post<OpenedRoom>('/game-rooms', {
        game_serial: gameSerial,
        ...(currentCandidates && currentCandidates.length === 2
          ? { current_candidates: currentCandidates }
          : {}),
      })
    },

    /**
     * Joins a room and reads everything needed to draw it in one call.
     *
     * gameSerial is optional and checked by the server when given, so a stale link cannot
     * silently join another game's room.
     */
    async state(roomSerial: string, gameSerial?: string, signal?: AbortSignal): Promise<RoomState> {
      const query = new URLSearchParams({ anonymous_id: getAnonymousID() })
      if (gameSerial) query.set('game_serial', gameSerial)
      return client.get<RoomState>(
        `/game-rooms/${encodeURIComponent(roomSerial)}?${query}`, signal)
    },

    /**
     * The tally for the pairing on screen, without joining the room.
     *
     * This is what a host's black box reads. state() would also carry it, but that call
     * joins: it would put the host on the leaderboard of their own room.
     *
     * Null means no pairing is in progress, which is a normal answer between rounds.
     */
    async votes(
      roomSerial: string, gameSerial?: string, signal?: AbortSignal,
    ): Promise<RoomVotes | null> {
      const query = new URLSearchParams()
      if (gameSerial) query.set('game_serial', gameSerial)
      const suffix = query.size > 0 ? `?${query}` : ''
      const body = await client.get<{ votes: RoomVotes | null }>(
        `/game-rooms/${encodeURIComponent(roomSerial)}/votes${suffix}`, signal)
      return body.votes
    },

    /**
     * The standings on their own.
     *
     * Used by the room's slow poll, which runs even when the websocket is connected: a
     * socket that dies without closing reports nothing, and a stale-but-moving leaderboard
     * is a far better failure than a frozen one.
     */
    async leaderboard(roomSerial: string, signal?: AbortSignal): Promise<Leaderboard> {
      return client.get<Leaderboard>(
        `/game-rooms/${encodeURIComponent(roomSerial)}/leaderboard`, signal)
    },

    /**
     * Wagers on the round in progress.
     *
     * Sends only the pick. The round numbers are the server's: it reads the match in
     * progress itself and refuses a pick that is not the pairing on screen, so a stale page
     * cannot record a wager the settlement would never resolve.
     *
     * Repeating the same wager replaces it rather than adding one, so a double-tapped
     * button is harmless — that is what the server's unique index is for.
     */
    async bet(roomSerial: string, pick: { winner_id: number; loser_id: number }): Promise<void> {
      await client.post<unknown>(`/game-rooms/${encodeURIComponent(roomSerial)}/bets`, {
        anonymous_id: getAnonymousID(),
        ...pick,
      })
    },

    /** Renames the caller. Rate limited server-side to one change per thirty seconds. */
    async rename(roomSerial: string, nickname: string): Promise<RoomPlayer> {
      return client.put<RoomPlayer>(`/game-rooms/${encodeURIComponent(roomSerial)}/player`, {
        anonymous_id: getAnonymousID(),
        nickname,
      })
    },
  }
}

export type GameRoomService = ReturnType<typeof createGameRoomService>

let cached: GameRoomService | null = null

export function getGameRoomService(): GameRoomService {
  if (!cached) cached = createGameRoomService()
  return cached
}
