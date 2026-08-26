import { computed, ref, shallowRef, type Ref } from 'vue'

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
  type RoomVoting,
  type RoomVotes,
  type RoundVotes,
} from '../services/gameRoom'
import { useRoundCountdown } from './useRoundCountdown'

/**
 * Drives one game room: joins it, keeps the leaderboard current, and places wagers.
 *
 * TWO SOURCES FOR THE LEADERBOARD, ON PURPOSE. The websocket carries the worker's
 * broadcast after each settlement, which is what makes the room feel live. The poll runs
 * anyway, because a websocket that dies without closing reports nothing at all — no error,
 * no close event, just silence. A stale-but-moving leaderboard is a far better failure than
 * a frozen one, and the poll is the only thing that turns the one into the other.
 *
 * The poll re-reads the WHOLE room state rather than the leaderboard alone, because a
 * pairing can also change without any event reaching us: the frame may have been dropped
 * while the socket was down, or the deployment may have no Soketi at all.
 */

/** The broadcast event the worker publishes. Named for what Laravel's listeners emitted. */
export const LEADERBOARD_EVENT = 'GameBetRank'

/**
 * The pairing event. Published when the host settles a round and when they restart into a
 * new game, and it carries the same tally the room's REST endpoint returns — so a pushed
 * pairing and a polled one cannot disagree.
 */
export const ROUND_EVENT = 'GameRoomRound'

/** What the worker publishes on ROUND_EVENT. Every field is optional over the wire. */
interface RoundFrame {
  game_serial?: string
  votes?: RoomVotes | null
  voting?: RoomVoting | null
}

/**
 * How often to re-read the room with no live socket. Five seconds is what the old UI used:
 * fast enough that the host advancing a round feels immediate, slow enough to be free at
 * any plausible room size.
 */
export const POLL_INTERVAL_MS = 5_000

/**
 * How often to re-read the room WHILE the socket is connected.
 *
 * The poll does not go away when the socket is up, because a socket that dies without
 * closing reports nothing at all — no error, no close event, just silence — and the poll is
 * the only thing that turns a frozen room into a slightly stale one. But with the pairing
 * and the leaderboard both pushed, it no longer has to carry the room: it is a safety net,
 * so it runs four times slower and costs four times less.
 */
export const LIVE_POLL_INTERVAL_MS = 20_000

export type RoomStatus = 'loading' | 'ready' | 'not-found' | 'failed'

export interface UseGameRoom {
  status: Ref<RoomStatus>
  player: Ref<RoomPlayer | null>
  votes: Ref<RoomVotes | null>
  /** The rounds this room has already decided, newest first. */
  history: Ref<RoundVotes[]>
  /** How the room decides its rounds. Carried by the state read and by every pairing frame. */
  voting: Ref<RoomVoting | null>
  /** True while the room decides its own rounds rather than the host. */
  majority: Ref<boolean>
  /**
   * Whole seconds left in this round, or null when nothing is counting down — a
   * host-decided room, or a majority round the host ends by hand.
   */
  secondsLeft: Ref<number | null>
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
  const history = ref<RoundVotes[]>([])
  const voting = ref<RoomVoting | null>(null)
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
  // The pairing the history was last read for. Null until the first read, so a room joined
  // between rounds still gets its history.
  let historyPairing: string | null = null

  const majority = computed(() => voting.value?.mode === 'majority')
  // A participant only watches this clock: the round is settled in the host's browser,
  // which is the only place that knows which pair comes next.
  const countdown = useRoundCountdown()

  function applyState(state: RoomState): void {
    player.value = state.player
    applyVotes(state.votes)
    applyVoting(state.voting)
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
        [ROUND_EVENT]: (payload) => {
          applyRound(payload as RoundFrame | null)
        },
      },
      (state) => {
        const wasConnected = live.value === 'connected'
        live.value = state
        if (state === 'connected') {
          // Whatever happened while the socket was down was never delivered, so a fresh
          // read is owed on every connect — including a reconnect the poll has not reached.
          if (!wasConnected) void refreshState()
          startPolling()
          return
        }
        // Off the socket, the poll is the room's only source again and goes back to full
        // speed. A wager in flight is still skipped, see startPolling.
        if (wasConnected) startPolling()
      },
    )
  }

  /**
   * Applies a pushed pairing.
   *
   * Not while a wager is in flight: the frame the host's settlement produced can land
   * between our POST and the read that wager does itself, and would put the pre-wager
   * counts back on screen.
   */
  function applyRound(payload: RoundFrame | null): void {
    if (!payload || betting.value) return
    // A game serial we have not seen means the host restarted: the room followed them onto
    // a new game, and the view has to reload that game's elements before it can draw the
    // pairing. Assigned first for that reason.
    if (typeof payload.game_serial === 'string' && payload.game_serial) {
      gameSerial.value = payload.game_serial
    }
    // votes is null between rounds, which is a real answer and not a malformed frame.
    applyVotes(payload.votes ?? null)
    applyVoting(payload.voting ?? null)
  }

  /**
   * Puts a tally on screen, and re-reads the history when the round has moved.
   *
   * The pairing is the trigger because it is the only observable one: a round can be
   * decided by the clock, by the host, or by a restart, and all three reach this page as a
   * different pair to vote on. Between rounds there is no tally at all, which is also a
   * move — the round that just ended is the row the history has gained.
   */
  function applyVotes(next: RoomVotes | null): void {
    votes.value = next
    const pairing = next ? `${next.first_candidate}:${next.second_candidate}` : ''
    if (pairing === historyPairing) return
    historyPairing = pairing
    void refreshHistory()
  }

  /**
   * Takes the room's settings and re-seeds the clock from them.
   *
   * The remainder is the server's, measured by the clock that armed the deadline, because
   * this device's own clock is not comparable to the host's. Between reads it is counted
   * down locally, so every read corrects the drift rather than letting it accumulate.
   */
  function applyVoting(next: RoomVoting | null | undefined): void {
    voting.value = next ?? null
    countdown.seed(next?.seconds_left ?? null)
  }

  function startPolling(): void {
    if (pollTimer) clearInterval(pollTimer)
    const interval = live.value === 'connected' ? LIVE_POLL_INTERVAL_MS : POLL_INTERVAL_MS
    pollTimer = setInterval(() => {
      // Not while a wager is in flight: the read would land between the POST and the
      // refresh that wager does itself, and put the pre-wager counts back on screen.
      if (betting.value) return
      void refreshState()
    }, interval)
  }

  async function refreshHistory(): Promise<void> {
    try {
      history.value = await service.history(roomSerial)
    } catch {
      // See refreshLeaderboard: the rounds already listed are still true, and the next
      // settled round asks again.
    }
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
    history,
    voting,
    majority,
    secondsLeft: countdown.display,
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

/**
 * The board read from the top: the players who most often sided with the room.
 *
 * top_10 already arrives best first, and the rank each row carries is its place on this
 * board, so both are used as they come.
 */
export function popularRows(board: Leaderboard | null) {
  return board ? board.top_10 : []
}

/**
 * The same column read from the bottom: the players who most often went their own way.
 *
 * bottom_10 arrives worst first, and worst by score is exactly most unique, so a row's
 * position in this list is its unique rank. The rank it carries is its place on the
 * popular board and would read here as a large number, so it is replaced. It cannot be
 * inverted from total_users either: that count includes players no refresh has ranked yet,
 * and every one of them would shift the whole board by one.
 */
export function uniqueRows(board: Leaderboard | null) {
  if (!board) return []
  return board.bottom_10.map((row, index) => ({ ...row, rank: index + 1 }))
}

/** True when the caller's own row is the one being rendered. */
export function isOwnRow(playerId: string | undefined, rowId: string): boolean {
  return Boolean(playerId) && playerId === rowId
}
