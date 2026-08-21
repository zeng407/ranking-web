import { computed, ref, shallowRef, watch, type Ref } from 'vue'

import { getRuntimeConfig } from '../config/runtime'
import { APIError } from '../lib/api'
import { subscribe, type PusherChannel, type PusherState } from '../lib/pusher'
import {
  getGameRoomService,
  type GameRoomService,
  type Leaderboard,
  type RoomVotes,
} from '../services/gameRoom'
import { LEADERBOARD_EVENT } from './useGameRoom'

/**
 * The host's side of a game room: open one for the game being played, and remember it.
 *
 * The participant's side lives in useGameRoom. This one is deliberately small — a host is
 * still playing their own local game, and the room is an attachment to it rather than a
 * mode it enters.
 *
 * WHAT THE HOST SEES OF THE ROOM. The standings and the head count, live, and — when they
 * ask for it — the tally for the pairing they have up. All of it read-only: the host never
 * joins their own room, because a participant row for them would put them on the very
 * leaderboard they are running.
 *
 * WHY THE SERIAL IS PERSISTED. Opening a room is idempotent server-side (the unique index
 * on game_rooms.game_id sees to that), so a reload could simply ask again. But the host's
 * page reloads often mid-game, and re-opening on every load would mean a request before the
 * invite link could be shown. Keyed by game serial rather than by post: a new game is a new
 * room.
 */

const STORAGE_PREFIX = 'gameroom_host_'

/**
 * How often the black box re-reads the tally. Five seconds, as the old UI used: the counts
 * move on every wager rather than on every settlement, so the leaderboard's broadcast says
 * nothing about them and there is nothing to be live off.
 */
export const VOTES_POLL_INTERVAL_MS = 5_000

/**
 * How often to re-read the leaderboard. Slower than the room's own poll: the host is not
 * waiting on a pairing, and the broadcast already moves the board on every settlement.
 */
export const BOARD_POLL_INTERVAL_MS = 15_000

export type HostedRoomStatus = 'closed' | 'opening' | 'open' | 'failed'

export interface UseHostedRoom {
  serial: Ref<string>
  status: Ref<HostedRoomStatus>
  hosting: Ref<boolean>
  /** The link to hand to participants, or '' when no room is open. */
  inviteURL: Ref<string>
  /** The standings, from the room's broadcast and a slow poll behind it. */
  board: Ref<Leaderboard | null>
  /** How many people have joined. */
  players: Ref<number>
  /** The tally for the pairing on screen, or null between rounds and while closed. */
  votes: Ref<RoomVotes | null>
  /** Whether the black box is open. Only then is the tally read. */
  blackBox: Ref<boolean>
  live: Ref<PusherState>
  open(currentCandidates?: number[]): Promise<void>
  /** Starts following the room. Idempotent, so a reload with a stored room can just call it. */
  startWatching(): void
  /** Stops the socket and every timer. */
  stopWatching(): void
  toggleBlackBox(): void
  /** Forgets the room locally. The room itself keeps existing server-side. */
  forget(): void
}

export function useHostedRoom(
  gameSerial: Ref<string>,
  locale: Ref<string>,
  service: GameRoomService = getGameRoomService(),
): UseHostedRoom {
  const serial = ref('')
  const status = ref<HostedRoomStatus>('closed')
  const board = ref<Leaderboard | null>(null)
  const votes = ref<RoomVotes | null>(null)
  const blackBox = ref(false)
  const live = ref<PusherState>('disconnected')
  const players = computed(() => board.value?.total_users ?? 0)

  // shallowRef: a channel is a handle, and making its internals reactive would have Vue
  // walk a WebSocket.
  const channel = shallowRef<PusherChannel | null>(null)
  let boardTimer: ReturnType<typeof setInterval> | undefined
  let votesTimer: ReturnType<typeof setInterval> | undefined

  const hosting = computed(() => Boolean(serial.value))
  const inviteURL = computed(() => {
    if (!serial.value || typeof window === 'undefined') return ''
    // The localized route, because the recipient lands on a page and reads it. The room
    // view is at /{locale}/room/{serial}.
    return new URL(`/${locale.value}/room/${encodeURIComponent(serial.value)}`,
      window.location.origin).toString()
  })

  // The game serial is not known at setup: the snapshot is fetched, or restored from
  // storage, after the view has mounted. Reading the stored room only once would mean a
  // host who reloads mid-game sees no room until they press the button again — and pressing
  // it would then re-open the same room, because opening is idempotent, hiding the fault.
  watch(gameSerial, (value) => {
    const stored = readStored(value)
    if (stored === serial.value) return
    stopWatching()
    serial.value = stored
    status.value = stored ? 'open' : 'closed'
    if (stored) startWatching()
  }, { immediate: true })

  async function open(currentCandidates?: number[]): Promise<void> {
    if (!gameSerial.value || status.value === 'opening') return
    status.value = 'opening'
    try {
      const room = await service.open(gameSerial.value, currentCandidates)
      serial.value = room.serial
      store(gameSerial.value, room.serial)
      status.value = 'open'
      startWatching()
    } catch (error) {
      // Nothing partial to undo: the room either exists server-side or it does not, and a
      // retry is idempotent.
      status.value = 'failed'
      if (!(error instanceof APIError)) throw error
    }
  }

  function startWatching(): void {
    if (!serial.value) return
    void refreshBoard()
    startLive()
    if (!boardTimer) boardTimer = setInterval(() => void refreshBoard(), BOARD_POLL_INTERVAL_MS)
  }

  function startLive(): void {
    const { realtime } = getRuntimeConfig()
    // No key configured means no websocket, and the poll above is then the whole story —
    // the right degradation for a deployment without Soketi.
    if (!realtime.key || channel.value) return
    channel.value = subscribe(
      realtime,
      `game-room.${serial.value}`,
      {
        [LEADERBOARD_EVENT]: (payload) => {
          const next = payload as Leaderboard | null
          // Guarded rather than assigned blindly: a malformed frame must not blank the
          // board already on screen.
          if (next && Array.isArray(next.top_10) && Array.isArray(next.bottom_10)) {
            board.value = next
          }
        },
      },
      (state) => {
        live.value = state
      },
    )
  }

  function toggleBlackBox(): void {
    blackBox.value = !blackBox.value
    if (!blackBox.value) {
      stopVotesPoll()
      // Dropped rather than left on screen: reopening it should show the round in play,
      // not whatever was up when it was last closed.
      votes.value = null
      return
    }
    void refreshVotes()
    votesTimer ??= setInterval(() => void refreshVotes(), VOTES_POLL_INTERVAL_MS)
  }

  function stopWatching(): void {
    if (boardTimer) clearInterval(boardTimer)
    boardTimer = undefined
    stopVotesPoll()
    channel.value?.leave()
    channel.value = null
    live.value = 'disconnected'
  }

  function stopVotesPoll(): void {
    if (votesTimer) clearInterval(votesTimer)
    votesTimer = undefined
  }

  async function refreshBoard(): Promise<void> {
    if (!serial.value) return
    try {
      board.value = await service.leaderboard(serial.value)
    } catch {
      // A failed read leaves the last good board up: it is still the best answer
      // available, and the next tick is fifteen seconds away.
    }
  }

  async function refreshVotes(): Promise<void> {
    if (!serial.value) return
    try {
      votes.value = await service.votes(serial.value, gameSerial.value)
    } catch {
      // See refreshBoard. The panel keeps showing the last tally it managed to read.
    }
  }

  function forget(): void {
    stopWatching()
    blackBox.value = false
    board.value = null
    votes.value = null
    serial.value = ''
    status.value = 'closed'
    clearStored(gameSerial.value)
  }

  return {
    serial,
    status,
    hosting,
    inviteURL,
    board,
    players,
    votes,
    blackBox,
    live,
    open,
    startWatching,
    stopWatching,
    toggleBlackBox,
    forget,
  }
}

/**
 * The pair to report as on screen, or undefined.
 *
 * ONLY WHEN THIS BATCH EMPTIES THE OUTBOX. games.candidates means "the pair the host is
 * looking at", and the displayed pair is the one after every LOCAL vote. If the batch is
 * only part of the outbox — a host who was offline and is catching up — the server would be
 * told about a pair that is several rounds ahead of the votes it has just recorded, and the
 * room would show its participants a match whose wagers cannot be settled yet.
 *
 * Omitting it in that case leaves the column holding the last decided pair, which is stale
 * but coherent, and the next flush corrects it.
 */
export function onScreenPairForBatch(
  batchLength: number,
  outboxLength: number,
  displayed: readonly [{ id: number }, { id: number }] | null,
): number[] | undefined {
  if (!displayed || batchLength !== outboxLength) return undefined
  return [displayed[0].id, displayed[1].id]
}

function storageKey(gameSerial: string): string {
  return STORAGE_PREFIX + gameSerial
}

function readStored(gameSerial: string): string {
  if (!gameSerial || typeof localStorage === 'undefined') return ''
  try {
    return localStorage.getItem(storageKey(gameSerial)) || ''
  } catch {
    // Private browsing can deny storage entirely; the room still works, it just has to be
    // reopened after a reload.
    return ''
  }
}

function store(gameSerial: string, roomSerial: string): void {
  if (typeof localStorage === 'undefined') return
  try {
    localStorage.setItem(storageKey(gameSerial), roomSerial)
  } catch {
    // See readStored.
  }
}

function clearStored(gameSerial: string): void {
  if (typeof localStorage === 'undefined') return
  try {
    localStorage.removeItem(storageKey(gameSerial))
  } catch {
    // See readStored.
  }
}
