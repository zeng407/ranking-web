import { ref, shallowRef, type Ref } from 'vue'

import { getRuntimeConfig } from '../config/runtime'
import { APIError } from '../lib/api'
import { subscribe, type PusherChannel, type PusherState } from '../lib/pusher'
import {
  getGameRoomService,
  type GameRoomService,
  type Leaderboard,
  type RoomBet,
  type RoomPlayer,
  type RoomState,
  type RoomVotes,
} from '../services/gameRoom'

/**
 * Drives one game room: joins it, keeps the leaderboard current, and places wagers.
 *
 * TWO SOURCES FOR THE LEADERBOARD, ON PURPOSE. The websocket carries the worker's
 * broadcast after each settlement, which is what makes the room feel live. The poll runs
 * anyway, because a websocket that dies without closing reports nothing at all — no error,
 * no close event, just silence. A stale-but-moving leaderboard is a far better failure than
 * a frozen one, and the poll is the only thing that turns the one into the other.
 *
 * The poll re-reads the WHOLE room state rather than the leaderboard alone, because the
 * pairing on screen is the host's and there is no broadcast for it: the only event the
 * worker publishes is the leaderboard. A participant left on a leaderboard-only poll sits
 * on last round's pairing until they reload, which is exactly what the old UI avoided by
 * re-reading the room on a five-second timer.
 */

/** The broadcast event the worker publishes. Named for what Laravel's listeners emitted. */
export const LEADERBOARD_EVENT = 'GameBetRank'

/**
 * How often to re-read the room. Five seconds is what the old UI used for the same job:
 * fast enough that the host advancing a round feels immediate, slow enough to be free at
 * any plausible room size.
 */
export const POLL_INTERVAL_MS = 5_000

export type RoomStatus = 'loading' | 'ready' | 'not-found' | 'failed'

export interface UseGameRoom {
  status: Ref<RoomStatus>
  player: Ref<RoomPlayer | null>
  votes: Ref<RoomVotes | null>
  leaderboard: Ref<Leaderboard | null>
  ownBet: Ref<RoomBet | null>
  gameSerial: Ref<string>
  /** connected / connecting / disconnected, for the "live" indicator. */
  live: Ref<PusherState>
  betting: Ref<boolean>
  /** A message key for the last action that failed, or ''. */
  actionError: Ref<string>
  join(): Promise<void>
  leave(): void
  bet(winnerId: number, loserId: number): Promise<void>
  rename(nickname: string): Promise<void>
  refreshLeaderboard(): Promise<void>
}

export function useGameRoom(
  roomSerial: string,
  service: GameRoomService = getGameRoomService(),
): UseGameRoom {
  const status = ref<RoomStatus>('loading')
  const player = ref<RoomPlayer | null>(null)
  const votes = ref<RoomVotes | null>(null)
  const leaderboard = ref<Leaderboard | null>(null)
  const ownBet = ref<RoomBet | null>(null)
  const gameSerial = ref('')
  const live = ref<PusherState>('disconnected')
  const betting = ref(false)
  const actionError = ref('')

  // shallowRef: these are handles, not reactive data, and making their internals
  // reactive would have Vue walk a WebSocket.
  const channel = shallowRef<PusherChannel | null>(null)
  let pollTimer: ReturnType<typeof setInterval> | undefined
  let controller: AbortController | null = null

  function applyState(state: RoomState): void {
    player.value = state.player
    votes.value = state.votes
    ownBet.value = state.latest_bet
    leaderboard.value = state.leaderboard
    gameSerial.value = state.game_serial
  }

  async function join(): Promise<void> {
    status.value = 'loading'
    controller?.abort()
    controller = new AbortController()

    try {
      applyState(await service.state(roomSerial, undefined, controller.signal))
      status.value = 'ready'
    } catch (error) {
      if (isAbort(error)) return
      // 404 is its own state: the link is wrong or the room is gone, and retrying will not
      // help. Everything else is worth an explicit retry affordance.
      status.value = error instanceof APIError && error.status === 404 ? 'not-found' : 'failed'
      return
    }

    startLive()
    startPolling()
  }

  function startLive(): void {
    const { realtime } = getRuntimeConfig()
    // No key configured means no websocket. The room stays playable on the poll alone,
    // which is the right degradation for a deployment without Soketi.
    if (!realtime.key) return
    channel.value?.leave()
    channel.value = subscribe(
      realtime,
      `game-room.${roomSerial}`,
      {
        [LEADERBOARD_EVENT]: (payload) => {
          const board = payload as Leaderboard | null
          // Guarded rather than assigned blindly: this is data from the network, and a
          // malformed frame must not blank the board that is already on screen.
          if (board && Array.isArray(board.top_10) && Array.isArray(board.bottom_10)) {
            leaderboard.value = board
          }
        },
      },
      (state) => {
        live.value = state
      },
    )
  }

  function startPolling(): void {
    if (pollTimer) clearInterval(pollTimer)
    pollTimer = setInterval(() => {
      // Not while a wager is in flight: the read would land between the POST and the
      // refresh that wager does itself, and put the pre-wager counts back on screen.
      if (betting.value) return
      void refreshState()
    }, POLL_INTERVAL_MS)
  }

  async function refreshLeaderboard(): Promise<void> {
    try {
      leaderboard.value = await service.leaderboard(roomSerial)
    } catch {
      // A failed read is not worth surfacing: the board already on screen is still the best
      // answer available, and the poll comes round again in seconds.
    }
  }

  async function bet(winnerId: number, loserId: number): Promise<void> {
    if (betting.value) return
    betting.value = true
    actionError.value = ''
    try {
      await service.bet(roomSerial, { winner_id: winnerId, loser_id: loserId })
      // Reflected locally at once so the pick highlights without waiting for a round trip.
      // The authoritative copy arrives with the next state read or broadcast.
      if (votes.value) {
        ownBet.value = {
          winner_id: winnerId,
          loser_id: loserId,
          current_round: votes.value.current_round,
          of_round: votes.value.of_round,
          remain_elements: votes.value.remain_elements,
        }
      }
      // Re-read so the vote counts include this wager. The broadcast only carries the
      // leaderboard, and the counts move on every wager rather than on every settlement.
      await refreshState()
    } catch (error) {
      actionError.value = betErrorKey(error)
      // The pairing moved under us, so pull the new one rather than leaving a stale board
      // with a button that will keep failing.
      if (error instanceof APIError && error.status === 409) await refreshState()
    } finally {
      betting.value = false
    }
  }

  async function rename(nickname: string): Promise<void> {
    actionError.value = ''
    try {
      player.value = await service.rename(roomSerial, nickname)
    } catch (error) {
      actionError.value = renameErrorKey(error)
    }
  }

  async function refreshState(): Promise<void> {
    try {
      applyState(await service.state(roomSerial))
    } catch (error) {
      if (isAbort(error)) return
      // Leaves the last good state on screen: a failed refresh should not empty the room.
    }
  }

  function leave(): void {
    controller?.abort()
    controller = null
    if (pollTimer) clearInterval(pollTimer)
    pollTimer = undefined
    channel.value?.leave()
    channel.value = null
  }

  return {
    status,
    player,
    votes,
    leaderboard,
    ownBet,
    gameSerial,
    live,
    betting,
    actionError,
    join,
    leave,
    bet,
    rename,
    refreshLeaderboard,
  }
}

/**
 * Maps a wager failure to a message key.
 *
 * 409 is the interesting one and is NOT an error the player caused: the host advanced the
 * round while they were deciding. It has to read as "the round moved on", not as "your
 * vote was rejected".
 */
function betErrorKey(error: unknown): string {
  if (!(error instanceof APIError)) return 'roomActionFailed'
  if (error.status === 409) {
    return error.code === 'no_round_in_progress' ? 'roomNoRound' : 'roomRoundMoved'
  }
  return 'roomActionFailed'
}

function renameErrorKey(error: unknown): string {
  if (!(error instanceof APIError)) return 'roomActionFailed'
  if (error.status === 429) return 'roomRenameTooSoon'
  if (error.status === 422) return 'roomNicknameInvalid'
  return 'roomActionFailed'
}

function isAbort(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

/** Merges the two halves of the board into one list, best first, for rendering. */
export function boardRows(board: Leaderboard | null) {
  if (!board) return []
  const seen = new Set<string>()
  const rows = [...board.top_10, ...[...board.bottom_10].reverse()]
  // top_10 and bottom_10 overlap in a small room, and bottom_10 arrives worst first — so
  // it is reversed before merging and duplicates are dropped by player id.
  return rows.filter((row) => {
    if (seen.has(row.user_id)) return false
    seen.add(row.user_id)
    return true
  })
}

/** True when the caller's own row is the one being rendered. */
export function isOwnRow(playerId: string | undefined, rowId: string): boolean {
  return Boolean(playerId) && playerId === rowId
}
