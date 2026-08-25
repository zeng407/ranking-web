import { computed, ref, shallowRef, watch, type Ref } from 'vue'

import { getRuntimeConfig } from '../config/runtime'
import { APIError } from '../lib/api'
import { subscribe, type PusherChannel, type PusherState } from '../lib/pusher'
import {
  getGameRoomService,
  type GameRoomService,
  type Leaderboard,
  type RoomVoting,
  type RoomVotes,
} from '../services/gameRoom'
import { LEADERBOARD_EVENT } from './useGameRoom'
import { useRoundCountdown } from './useRoundCountdown'

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
 * invite link could be shown. Keyed by game serial, because that is what the server keys a
 * room on — but the key MOVES when the host restarts, see below.
 *
 * WHY THE ROOM FOLLOWS A RESTART. A restart mints a new game serial, and a room belongs to
 * a game. Left alone, the host would silently stop hosting: their votes would go to a game
 * the room cannot see, while the participants sat on a match that had already been decided.
 * Opening a room for the new game instead would mint a new room serial and break every
 * invite link and QR code already handed out. So the room is moved — same serial, new game
 * — and the server broadcasts the new pairing to everybody sitting in it.
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

/**
 * The default round length the settings offer. Fifteen seconds: long enough to read two
 * names and pick, short enough that a room of strangers does not lose interest.
 */
export const DEFAULT_ROUND_SECONDS = 15

/** The range the server accepts. Mirrors gameroom.MinRoundSeconds / MaxRoundSeconds. */
export const MIN_ROUND_SECONDS = 5
export const MAX_ROUND_SECONDS = 300

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
  /** How the room decides its rounds, as the server last reported it. */
  voting: Ref<RoomVoting | null>
  /** True while the room is deciding its own rounds. */
  majority: Ref<boolean>
  /** Whole seconds left in this round, or null when nothing is counting down. */
  secondsLeft: Ref<number | null>
  /**
   * True once a running countdown has reached zero and until the next round is armed.
   *
   * The host's view watches this and settles, because the bracket is played in their
   * browser: the server holds the clock, but it does not know which pair comes next and
   * cannot decide the round itself.
   */
  roundExpired: Ref<boolean>
  /** Whether the black box is open. Only then is the tally read. */
  blackBox: Ref<boolean>
  live: Ref<PusherState>
  open(currentCandidates?: number[]): Promise<void>
  /**
   * Hands the decision to the room, or takes it back. Also arms the clock for the round
   * already on screen, so the setting takes effect now rather than next round.
   */
  setVoting(mode: RoomVoting['mode'], roundSeconds: number): Promise<void>
  /**
   * Who wins the round on screen, decided by the room.
   *
   * Reads the tally fresh rather than trusting the polled copy, and breaks a tie — 0-0
   * included — at random, so an unwatched room still advances.
   */
  majorityWinner(pair: readonly [number, number], random?: () => number): Promise<number>
  /**
   * Tells the room which pair is on screen now. Does nothing when no room is open, so the
   * game view can call it on every re-pick without asking whether it is hosting.
   */
  reportPair(): Promise<void>
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
  /**
   * The pair on screen in the game the room is being moved onto, for a restart.
   *
   * The server cannot work it out: the client plays its bracket locally, and a game nobody
   * has voted in yet has no pairing recorded at all. Without it the participants would be
   * shown an empty column until the host's first vote of the new game lands.
   */
  onScreen?: () => number[] | undefined,
): UseHostedRoom {
  const serial = ref('')
  const status = ref<HostedRoomStatus>('closed')
  const board = ref<Leaderboard | null>(null)
  const votes = ref<RoomVotes | null>(null)
  const voting = ref<RoomVoting | null>(null)
  const blackBox = ref(false)
  const live = ref<PusherState>('disconnected')
  const players = computed(() => board.value?.total_users ?? 0)

  // shallowRef: a channel is a handle, and making its internals reactive would have Vue
  // walk a WebSocket.
  const channel = shallowRef<PusherChannel | null>(null)
  let boardTimer: ReturnType<typeof setInterval> | undefined
  let votesTimer: ReturnType<typeof setInterval> | undefined

  const majority = computed(() => voting.value?.mode === 'majority')
  // The countdown is seeded from every read; onExpire is the view's business, so this one
  // only keeps the number. GameView watches `expired` and settles.
  const countdown = useRoundCountdown()

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
  watch(gameSerial, (value, previous) => {
    const stored = storedRoomSerial(value)
    if (stored) {
      if (stored === serial.value) return
      stopWatching()
      serial.value = stored
      status.value = 'open'
      startWatching()
      void reportPair()
      return
    }
    // No room stored for this game. If one is open on the game we just left, the host
    // restarted and the room follows them; see the note at the top of the file.
    if (previous && value && serial.value) {
      void follow(previous, value)
      return
    }
    if (serial.value) {
      stopWatching()
      serial.value = ''
      status.value = 'closed'
    }
  }, { immediate: true })

  /**
   * Tells the room which pair the host has up, when nothing else would.
   *
   * A RELOAD RE-PICKS THE PAIRING. The match on screen has not been voted on, so the game
   * view is free to choose another of the stage's ready candidates when it resumes, and it
   * does. The room would never hear about it: the vote sync carries the pair only when a
   * vote is cast, so the people seated would keep looking at the pre-reload match — and
   * their poll would keep confirming it, because the server's own record is what went
   * stale.
   *
   * Opening is idempotent, so it doubles as the report; the server broadcasts the pairing
   * to the room when a pair comes with it. Best effort: the host is mid-game, and their
   * next vote records the pair anyway.
   *
   * No room open means nothing to tell, and the guard matters: opening would CREATE one, so
   * without it a solo host re-picking a match would find themselves hosting.
   */
  async function reportPair(): Promise<void> {
    const pair = onScreen?.()
    if (!serial.value || !gameSerial.value || !pair || pair.length !== 2) return
    try {
      await service.open(gameSerial.value, pair)
    } catch (error) {
      if (!(error instanceof APIError)) throw error
    }
  }

  /**
   * Moves the open room onto the game the host has just restarted into.
   *
   * The room serial does not change, so the socket, the timers and the invite link all keep
   * working; only the stored key and the tally on screen belong to the old game.
   */
  async function follow(fromGameSerial: string, toGameSerial: string): Promise<void> {
    const room = serial.value
    // The old game's tally describes a match nobody is voting on any more.
    votes.value = null
    // The clock too: the round it was counting down belongs to the game just left.
    countdown.reset()
    try {
      await service.rebind(room, fromGameSerial, toGameSerial, onScreen?.())
    } catch (error) {
      if (!(error instanceof APIError)) throw error
      // Refused: the room has already moved, or the new game belongs to another post. Stop
      // claiming a room this page does not drive rather than keep a link that lies.
      stopWatching()
      serial.value = ''
      status.value = 'closed'
      clearStored(fromGameSerial)
      return
    }
    // A further restart may have overtaken this one while the request was in flight; that
    // later call owns the room now, and this one must not write its key back.
    if (gameSerial.value !== toGameSerial || serial.value !== room) return
    store(toGameSerial, room)
    clearStored(fromGameSerial)
    status.value = 'open'
    if (blackBox.value || majority.value) void refreshVotes()
  }

  async function open(currentCandidates?: number[]): Promise<void> {
    if (!gameSerial.value || status.value === 'opening') return
    status.value = 'opening'
    try {
      const room = await service.open(gameSerial.value, currentCandidates)
      serial.value = room.serial
      store(gameSerial.value, room.serial)
      status.value = 'open'
      startWatching()
      // The room may already have a mode: opening is idempotent, so this is also the path
      // a host takes back into a room they set up before reloading.
      void refreshVotes()
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
    void refreshVotes()
    startLive()
    if (!boardTimer) boardTimer = setInterval(() => void refreshBoard(), BOARD_POLL_INTERVAL_MS)
  }

  /**
   * Keeps the tally poll running for as long as anything needs it.
   *
   * The black box needs it to show counts. A majority room needs it whether or not the box
   * is open, because the countdown is re-seeded from the same read — without it the host's
   * clock would free-run from whenever the mode was set and drift away from the room's.
   */
  function syncVotesPoll(): void {
    if (!serial.value || (!blackBox.value && !majority.value)) {
      stopVotesPoll()
      return
    }
    votesTimer ??= setInterval(() => void refreshVotes(), VOTES_POLL_INTERVAL_MS)
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
    if (!blackBox.value && !majority.value) {
      stopVotesPoll()
      // Dropped rather than left on screen: reopening it should show the round in play,
      // not whatever was up when it was last closed. Kept in a majority room, where the
      // same read drives the clock.
      votes.value = null
      return
    }
    void refreshVotes()
    syncVotesPoll()
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
      const read = await service.votes(serial.value, gameSerial.value)
      // The counts only go on screen when the black box is open. A majority room reads
      // them every tick regardless, because the same call carries the clock — but the host
      // asked not to see the votes, and this is where that is honoured.
      votes.value = blackBox.value ? read.votes : null
      voting.value = read.voting
      // Re-seeded on every read, so local drift lasts at most one poll interval and is
      // corrected rather than accumulated.
      countdown.seed(read.voting?.seconds_left ?? null)
      syncVotesPoll()
    } catch {
      // See refreshBoard. The panel keeps showing the last tally it managed to read, and
      // the clock keeps ticking down from the last remainder the server gave it.
    }
  }

  async function setVoting(mode: RoomVoting['mode'], roundSeconds: number): Promise<void> {
    if (!serial.value || !gameSerial.value) return
    voting.value = await service.setVoting(serial.value, gameSerial.value, mode, roundSeconds)
    // Not from the response: SetVoting clears the deadline and arms a fresh one, and the
    // remainder it returns was measured before the round trip home. Reading again costs one
    // request and starts the host's clock from the same number everybody else will get.
    await refreshVotes()
  }

  /**
   * The room's verdict on the pair currently on screen.
   *
   * Read fresh rather than taken from the polled copy: up to five seconds of wagers can be
   * sitting between the last poll and this moment, and settling on a stale tally would
   * discard exactly the votes cast while the countdown was running out.
   *
   * A tie goes to chance — including 0-0, which is a tie like any other and is what an
   * unwatched room produces every round. Deciding it any other way (always the left, always
   * the incumbent) would quietly bias every bracket a quiet room ever played.
   */
  async function majorityWinner(
    pair: readonly [number, number], random: () => number = Math.random,
  ): Promise<number> {
    const [first, second] = pair
    if (!serial.value) return random() < 0.5 ? first : second
    let tally: RoomVotes | null = null
    try {
      const read = await service.votes(serial.value, gameSerial.value)
      if (blackBox.value) votes.value = read.votes
      voting.value = read.voting
      tally = read.votes
    } catch {
      // Unreadable: the round still has to end, and a coin toss is the honest answer when
      // the votes cannot be counted at all. Stalling the game would be worse.
    }
    if (!tally) return random() < 0.5 ? first : second
    // The tally names its own candidates, and they are what the counts belong to. A pair
    // the server has not caught up with yet must not have its votes read off the wrong
    // side, so anything that does not match the pair on screen is treated as no votes.
    const forFirst = candidateVotes(tally, first)
    const forSecond = candidateVotes(tally, second)
    if (forFirst > forSecond) return first
    if (forSecond > forFirst) return second
    return random() < 0.5 ? first : second
  }

  function forget(): void {
    stopWatching()
    countdown.reset()
    blackBox.value = false
    board.value = null
    votes.value = null
    voting.value = null
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
    voting,
    majority,
    secondsLeft: countdown.display,
    roundExpired: countdown.expired,
    blackBox,
    live,
    open,
    setVoting,
    majorityWinner,
    reportPair,
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

/**
 * How many votes a tally records for one candidate, or 0 when the tally is about some other
 * pair. See majorityWinner for why a mismatch counts as no votes rather than as a guess.
 */
function candidateVotes(tally: RoomVotes, candidateID: number): number {
  if (tally.first_candidate === candidateID) return tally.first_candidate_votes
  if (tally.second_candidate === candidateID) return tally.second_candidate_votes
  return 0
}

function storageKey(gameSerial: string): string {
  return STORAGE_PREFIX + gameSerial
}

function storedRoomSerial(gameSerial: string): string {
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
    // See storedRoomSerial.
  }
}

function clearStored(gameSerial: string): void {
  if (typeof localStorage === 'undefined') return
  try {
    localStorage.removeItem(storageKey(gameSerial))
  } catch {
    // See storedRoomSerial.
  }
}
