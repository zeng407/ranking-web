// @vitest-environment happy-dom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { APIError } from '../lib/api'
import {
  LIVE_POLL_INTERVAL_MS,
  POLL_INTERVAL_MS,
  ROUND_EVENT,
  boardRows,
  useGameRoom,
} from './useGameRoom'
import type { GameRoomService, Leaderboard, RoomState } from '../services/gameRoom'
import type { PusherState } from '../lib/pusher'

/**
 * The socket, captured rather than opened. The composable only ever subscribes when a key is
 * configured, so the config is faked too — otherwise the live half of this file is dead code
 * that no test can reach.
 */
const socket = {
  handlers: {} as Record<string, (payload: unknown) => void>,
  setState: undefined as ((state: PusherState) => void) | undefined,
  leave: vi.fn(),
}

/** Fires one frame at whatever the composable subscribed for the pairing event. */
function pushRound(payload: unknown): void {
  const handler = socket.handlers[ROUND_EVENT]
  if (!handler) throw new Error('the room never subscribed to the pairing event')
  handler(payload)
}

vi.mock('../lib/pusher', () => ({
  subscribe: (
    _options: unknown,
    _channel: string,
    handlers: Record<string, (payload: unknown) => void>,
    onState?: (state: PusherState) => void,
  ) => {
    socket.handlers = handlers
    socket.setState = onState
    return { leave: socket.leave }
  },
}))

vi.mock('../config/runtime', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../config/runtime')>()
  return {
    ...actual,
    getRuntimeConfig: () => ({
      ...actual.getRuntimeConfig(),
      realtime: { key: 'test-key', host: 'soketi', port: 6001, secure: false, cluster: '' },
    }),
  }
})

function board(overrides: Partial<Leaderboard> = {}): Leaderboard {
  return { total_users: 2, top_10: [], bottom_10: [], ...overrides }
}

function player(id: string, rank: number, score: number) {
  return {
    user_id: id, name: `player-${id}`, score, rank,
    accuracy: '50.00', total_played: 2, total_correct: 1, combo: 0,
  }
}

function state(overrides: Partial<RoomState> = {}): RoomState {
  return {
    serial: 'abcdefgh',
    game_serial: 'game-serial',
    player: {
      player_id: 'me', name: '路人', score: 1000, rank: 3,
      accuracy: '63.49', total_played: 10, total_correct: 6, combo: 2,
    },
    votes: {
      first_candidate: 11, second_candidate: 22,
      first_candidate_votes: 2, second_candidate_votes: 1,
      remain_elements: 34, total_votes: 3, current_round: 31, of_round: 32,
    },
    latest_bet: null,
    leaderboard: board(),
    ...overrides,
  }
}

function apiError(status: number, code = 'x'): APIError {
  return new APIError(status, { error: { code, message: 'no' } } as never)
}

function fakeService(overrides: Partial<GameRoomService> = {}): GameRoomService {
  return {
    open: vi.fn(),
    state: vi.fn().mockResolvedValue(state()),
    leaderboard: vi.fn().mockResolvedValue(board()),
    bet: vi.fn().mockResolvedValue(undefined),
    rename: vi.fn(),
    ...overrides,
  } as unknown as GameRoomService
}

describe('useGameRoom', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    socket.handlers = {}
    socket.setState = undefined
    socket.leave.mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('joins and exposes the room', async () => {
    const room = useGameRoom('abcdefgh', fakeService())
    await room.join()

    expect(room.status.value).toBe('ready')
    expect(room.player.value?.name).toBe('路人')
    expect(room.votes.value?.current_round).toBe(31)
    expect(room.gameSerial.value).toBe('game-serial')
    room.leave()
  })

  // 404 is its own state because retrying cannot help: the link is wrong or the room is
  // gone. Everything else gets a retry affordance.
  it('separates a missing room from a failure', async () => {
    const notFound = useGameRoom('nope', fakeService({
      state: vi.fn().mockRejectedValue(apiError(404)),
    }))
    await notFound.join()
    expect(notFound.status.value).toBe('not-found')

    const broken = useGameRoom('abcdefgh', fakeService({
      state: vi.fn().mockRejectedValue(apiError(500)),
    }))
    await broken.join()
    expect(broken.status.value).toBe('failed')
  })

  /**
   * THE POLL RUNS EVEN THOUGH THE WEBSOCKET EXISTS. A socket that dies without closing
   * reports nothing — no error, no close event — so the poll is the only thing standing
   * between "slightly stale" and "frozen forever".
   */
  it('polls the room state on an interval', async () => {
    const service = fakeService()
    const room = useGameRoom('abcdefgh', service)
    await room.join()

    expect(service.state).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(service.state).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(service.state).toHaveBeenCalledTimes(3)

    room.leave()
  })

  /**
   * THE POLL CARRIES THE PAIRING TOO, not just the board. The pairing is pushed as well, but
   * a frame dropped while the socket was down is never redelivered — and a deployment with
   * no Soketi has only this.
   */
  it('follows the host onto the next pairing', async () => {
    const next = state({
      votes: {
        first_candidate: 33, second_candidate: 44,
        first_candidate_votes: 0, second_candidate_votes: 0,
        remain_elements: 17, total_votes: 0, current_round: 32, of_round: 32,
      },
    })
    const service = fakeService({
      state: vi.fn().mockResolvedValueOnce(state()).mockResolvedValue(next),
    })
    const room = useGameRoom('abcdefgh', service)
    await room.join()
    expect(room.votes.value?.first_candidate).toBe(11)

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(room.votes.value?.first_candidate).toBe(33)
    expect(room.votes.value?.current_round).toBe(32)

    room.leave()
  })

  it('stops polling on leave', async () => {
    const service = fakeService()
    const room = useGameRoom('abcdefgh', service)
    await room.join()
    room.leave()

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 3)
    expect(service.state).toHaveBeenCalledTimes(1)
  })

  // A failed poll leaves the room that is on screen: it is stale, but it is still the best
  // answer available, and blanking it would be worse.
  it('keeps the current state when a poll fails', async () => {
    const service = fakeService({
      state: vi.fn().mockResolvedValueOnce(state()).mockRejectedValue(apiError(503)),
    })
    const room = useGameRoom('abcdefgh', service)
    await room.join()
    const before = room.leaderboard.value
    const votes = room.votes.value

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(room.leaderboard.value).toBe(before)
    expect(room.votes.value).toBe(votes)
  })

  /**
   * THE PAIRING IS PUSHED. The worker publishes it when the host settles a round and when
   * they restart, carrying the same tally the REST endpoint returns — so the room moves at
   * socket speed and the poll behind it never has to disagree.
   */
  it('applies a pushed pairing without re-reading the room', async () => {
    const service = fakeService()
    const room = useGameRoom('abcdefgh', service)
    await room.join()
    expect(service.state).toHaveBeenCalledTimes(1)

    pushRound({
      game_serial: 'game-serial',
      votes: {
        first_candidate: 33, second_candidate: 44,
        first_candidate_votes: 1, second_candidate_votes: 0,
        remain_elements: 17, total_votes: 1, current_round: 32, of_round: 32,
      },
    })

    expect(room.votes.value?.first_candidate).toBe(33)
    expect(room.votes.value?.current_round).toBe(32)
    expect(service.state).toHaveBeenCalledTimes(1)
    room.leave()
  })

  // The restart. The room followed the host onto a new game, and the view needs the new
  // serial to load that game's elements — without it the pairing draws as two bare ids.
  it('follows the room onto the game a restart created', async () => {
    const room = useGameRoom('abcdefgh', fakeService())
    await room.join()
    expect(room.gameSerial.value).toBe('game-serial')

    pushRound({ game_serial: 'restarted-game', votes: null })

    expect(room.gameSerial.value).toBe('restarted-game')
    // Null is a real answer, not a malformed frame: between rounds there is no pairing.
    expect(room.votes.value).toBeNull()
    room.leave()
  })

  // The frame the host's settlement produced can land between our POST and the read that
  // wager does itself, and would put the pre-wager counts back on screen.
  it('ignores a pushed pairing while a wager is in flight', async () => {
    let release: () => void = () => {}
    const service = fakeService({
      bet: vi.fn().mockReturnValue(new Promise<void>((resolve) => { release = resolve })),
    })
    const room = useGameRoom('abcdefgh', service)
    await room.join()

    const wager = room.bet(11, 22)
    pushRound({ game_serial: 'game-serial', votes: null })
    expect(room.votes.value?.first_candidate).toBe(11)

    release()
    await wager
    room.leave()
  })

  /**
   * THE POLL SLOWS DOWN, IT DOES NOT STOP. With the pairing and the board both pushed it is
   * a safety net rather than the room's source — but a socket that dies without closing
   * reports nothing at all, and the poll is still the only thing that notices.
   */
  it('slows the poll while the socket is connected', async () => {
    const service = fakeService()
    const room = useGameRoom('abcdefgh', service)
    await room.join()

    socket.setState?.('connected')
    await vi.advanceTimersByTimeAsync(0)
    // A connect owes a fresh read: whatever happened while the socket was down was never
    // delivered to it.
    const afterConnect = vi.mocked(service.state).mock.calls.length
    expect(afterConnect).toBe(2)

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(service.state).toHaveBeenCalledTimes(afterConnect)

    await vi.advanceTimersByTimeAsync(LIVE_POLL_INTERVAL_MS - POLL_INTERVAL_MS)
    expect(service.state).toHaveBeenCalledTimes(afterConnect + 1)
    room.leave()
  })

  // And back to full speed when it drops: the poll is the whole story again.
  it('speeds the poll back up when the socket drops', async () => {
    const service = fakeService()
    const room = useGameRoom('abcdefgh', service)
    await room.join()

    socket.setState?.('connected')
    await vi.advanceTimersByTimeAsync(0)
    socket.setState?.('disconnected')
    const before = vi.mocked(service.state).mock.calls.length

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(service.state).toHaveBeenCalledTimes(before + 1)
    room.leave()
  })

  it('sends only the pick, and the server confirms it', async () => {
    // Realistic: once a wager is recorded, the next state read reports it.
    const placed = { winner_id: 22, loser_id: 11, current_round: 31, of_round: 32, remain_elements: 34 }
    const service = fakeService({
      state: vi.fn()
        .mockResolvedValueOnce(state())
        .mockResolvedValue(state({ latest_bet: placed })),
    })
    const room = useGameRoom('abcdefgh', service)
    await room.join()

    await room.bet(22, 11)

    // No round counters: the server derives them, and a participant has no way to know
    // which match is in play.
    expect(service.bet).toHaveBeenCalledWith('abcdefgh', { winner_id: 22, loser_id: 11 })
    expect(room.ownBet.value).toEqual(placed)
    // Re-read so the vote counts include this wager; the broadcast carries only the board.
    expect(service.state).toHaveBeenCalledTimes(2)
    room.leave()
  })

  /**
   * The optimistic write exists so the pick highlights during the round trip rather than
   * after it. Asserted while the confirming read is still pending, which is the only window
   * in which it is observable.
   */
  it('highlights the pick before the server confirms', async () => {
    let confirm: (value: RoomState) => void = () => {}
    const service = fakeService({
      state: vi.fn()
        .mockResolvedValueOnce(state())
        .mockImplementation(() => new Promise<RoomState>((resolve) => { confirm = resolve })),
    })
    const room = useGameRoom('abcdefgh', service)
    await room.join()

    const pending = room.bet(22, 11)
    await vi.waitFor(() => expect(room.ownBet.value?.winner_id).toBe(22))
    // The counters come from the votes payload, so the highlight matches the round on screen.
    expect(room.ownBet.value?.current_round).toBe(31)

    confirm(state({ latest_bet: { winner_id: 22, loser_id: 11, current_round: 31, of_round: 32, remain_elements: 34 } }))
    await pending
    room.leave()
  })

  /**
   * 409 is not the player's fault: the host advanced the round while they were deciding. It
   * has to read as "the round moved on" and pull the new pairing, rather than leaving a
   * stale board whose buttons keep failing.
   */
  it('refreshes and explains itself when the round moved on', async () => {
    const service = fakeService({
      bet: vi.fn().mockRejectedValue(apiError(409, 'stale_pairing')),
    })
    const room = useGameRoom('abcdefgh', service)
    await room.join()

    await room.bet(22, 11)

    expect(room.actionError.value).toBe('roomRoundMoved')
    expect(service.state).toHaveBeenCalledTimes(2)
    room.leave()
  })

  it('distinguishes betting between rounds', async () => {
    const service = fakeService({
      bet: vi.fn().mockRejectedValue(apiError(409, 'no_round_in_progress')),
    })
    const room = useGameRoom('abcdefgh', service)
    await room.join()

    await room.bet(22, 11)
    expect(room.actionError.value).toBe('roomNoRound')
    room.leave()
  })

  it('ignores a second wager while one is in flight', async () => {
    let release: () => void = () => {}
    const service = fakeService({
      bet: vi.fn().mockImplementation(() => new Promise<void>((resolve) => { release = () => resolve() })),
    })
    const room = useGameRoom('abcdefgh', service)
    await room.join()

    const first = room.bet(22, 11)
    await room.bet(11, 22)
    release()
    await first

    expect(service.bet).toHaveBeenCalledTimes(1)
    room.leave()
  })

  it('maps a rename refusal to its own message', async () => {
    for (const [status, key] of [[429, 'roomRenameTooSoon'], [422, 'roomNicknameInvalid'], [500, 'roomActionFailed']] as const) {
      const room = useGameRoom('abcdefgh', fakeService({
        rename: vi.fn().mockRejectedValue(apiError(status)),
      }))
      await room.join()
      await room.rename('新名字')
      expect(room.actionError.value).toBe(key)
      room.leave()
    }
  })

  it('adopts the renamed player on success', async () => {
    const renamed = { ...state().player!, name: '新名字' }
    const room = useGameRoom('abcdefgh', fakeService({
      rename: vi.fn().mockResolvedValue(renamed),
    }))
    await room.join()

    await room.rename('新名字')
    expect(room.player.value?.name).toBe('新名字')
    room.leave()
  })
})

describe('boardRows', () => {
  it('merges both halves best first, without duplicates', () => {
    // bottom_10 arrives WORST first, so it is reversed before merging. In a small room the
    // two halves overlap entirely.
    const rows = boardRows(board({
      top_10: [player('a', 1, 300), player('b', 2, 200)],
      bottom_10: [player('c', 3, 100), player('b', 2, 200)],
    }))

    expect(rows.map((row) => row.user_id)).toEqual(['a', 'b', 'c'])
  })

  it('is empty for a missing board', () => {
    expect(boardRows(null)).toEqual([])
  })
})
