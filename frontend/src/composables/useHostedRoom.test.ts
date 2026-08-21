// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

import { APIError } from '../lib/api'
import { BOARD_POLL_INTERVAL_MS, onScreenPairForBatch, useHostedRoom } from './useHostedRoom'
import type { GameRoomService, Leaderboard, RoomVotes } from '../services/gameRoom'

function fakeService(overrides: Partial<GameRoomService> = {}): GameRoomService {
  return {
    open: vi.fn().mockResolvedValue({ serial: 'abcdefgh', game_serial: 'game-1' }),
    state: vi.fn(),
    leaderboard: vi.fn().mockResolvedValue(board(0)),
    votes: vi.fn().mockResolvedValue(null),
    bet: vi.fn(),
    rename: vi.fn(),
    ...overrides,
  } as unknown as GameRoomService
}

function board(total: number): Leaderboard {
  return { total_users: total, top_10: [], bottom_10: [] }
}

function apiError(status: number): APIError {
  return new APIError(status, { error: { code: 'x', message: 'no' } } as never)
}

/** Lets the watch callback and the first reads settle. */
function flush(): Promise<void> {
  return new Promise((resolve) => { setTimeout(resolve, 0) })
}

describe('useHostedRoom', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('opens a room and remembers it across a reload', async () => {
    const service = fakeService()
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)

    expect(room.hosting.value).toBe(false)
    await room.open([11, 22])

    expect(service.open).toHaveBeenCalledWith('game-1', [11, 22])
    expect(room.serial.value).toBe('abcdefgh')
    expect(room.status.value).toBe('open')

    // A fresh composable is what a reload produces. The serial has to come back without a
    // request, or the invite link cannot be shown until one completes.
    const reloaded = useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService())
    expect(reloaded.serial.value).toBe('abcdefgh')
    expect(reloaded.hosting.value).toBe(true)
  })

  // Keyed on the game, not the post: a restart is a new game and therefore a new room, and
  // the old invite link must not silently follow the host into it.
  it('does not carry a room into a different game', async () => {
    await useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService()).open()

    const other = useHostedRoom(ref('game-2'), ref('zh-tw'), fakeService())
    expect(other.serial.value).toBe('')
    expect(other.hosting.value).toBe(false)
  })

  it('builds the localized invite link', async () => {
    const room = useHostedRoom(ref('game-1'), ref('ja'), fakeService())
    await room.open()

    expect(room.inviteURL.value).toBe(`${window.location.origin}/ja/room/abcdefgh`)
  })

  it('has no invite link before a room exists', () => {
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService())
    expect(room.inviteURL.value).toBe('')
  })

  it('reports a failure without leaving a half-open room', async () => {
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService({
      open: vi.fn().mockRejectedValue(apiError(503)),
    }))

    await room.open()

    expect(room.status.value).toBe('failed')
    expect(room.hosting.value).toBe(false)
    expect(room.inviteURL.value).toBe('')
  })

  it('ignores a second open while one is in flight', async () => {
    let release: (value: unknown) => void = () => {}
    const service = fakeService({
      open: vi.fn().mockReturnValue(new Promise((resolve) => { release = resolve })),
    })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)

    const first = room.open()
    await room.open()
    release({ serial: 'abcdefgh', game_serial: 'game-1' })
    await first

    expect(service.open).toHaveBeenCalledTimes(1)
  })

  it('refuses to open without a game', async () => {
    const service = fakeService()
    const room = useHostedRoom(ref(''), ref('zh-tw'), service)

    await room.open()
    expect(service.open).not.toHaveBeenCalled()
  })

  it('follows the room it opened', async () => {
    const service = fakeService({ leaderboard: vi.fn().mockResolvedValue(board(4)) })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)

    await room.open()
    await flush()

    expect(service.leaderboard).toHaveBeenCalledWith('abcdefgh')
    expect(room.players.value).toBe(4)
    room.stopWatching()
  })

  /*
  The game serial is not known at setup: it arrives with the snapshot. Reading the stored
  room only once would leave a host who reloaded mid-game with no room on screen — and the
  button would then re-open the same room, because opening is idempotent, hiding the fault.
  */
  it('picks up a stored room when the game serial arrives', async () => {
    await useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService()).open()

    const serial = ref('')
    const service = fakeService({ leaderboard: vi.fn().mockResolvedValue(board(2)) })
    const room = useHostedRoom(serial, ref('zh-tw'), service)
    expect(room.hosting.value).toBe(false)

    serial.value = 'game-1'
    await flush()

    expect(room.serial.value).toBe('abcdefgh')
    expect(room.players.value).toBe(2)
    room.stopWatching()
  })

  it('reads the tally only while the black box is open', async () => {
    const tally: RoomVotes = {
      first_candidate: 11,
      second_candidate: 22,
      first_candidate_votes: 3,
      second_candidate_votes: 1,
      remain_elements: 2,
      total_votes: 4,
      current_round: 1,
      of_round: 1,
    }
    const service = fakeService({ votes: vi.fn().mockResolvedValue(tally) })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    expect(service.votes).not.toHaveBeenCalled()

    room.toggleBlackBox()
    await flush()
    expect(service.votes).toHaveBeenCalledWith('abcdefgh', 'game-1')
    expect(room.votes.value).toEqual(tally)

    // Dropped on close, so reopening shows the round in play rather than an old one.
    room.toggleBlackBox()
    expect(room.blackBox.value).toBe(false)
    expect(room.votes.value).toBeNull()
    room.stopWatching()
  })

  it('keeps the last board it managed to read when a poll fails', async () => {
    const leaderboard = vi.fn()
      .mockResolvedValueOnce(board(5))
      .mockRejectedValue(apiError(503))
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService({ leaderboard }))

    await room.open()
    await flush()
    expect(room.players.value).toBe(5)

    vi.useFakeTimers()
    try {
      await vi.advanceTimersByTimeAsync(BOARD_POLL_INTERVAL_MS + 1)
    } finally {
      vi.useRealTimers()
    }
    expect(room.players.value).toBe(5)
    room.stopWatching()
  })

  it('stops following a room it has forgotten', async () => {
    const service = fakeService()
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    room.toggleBlackBox()
    await flush()

    room.forget()
    const boards = (service.leaderboard as ReturnType<typeof vi.fn>).mock.calls.length
    const tallies = (service.votes as ReturnType<typeof vi.fn>).mock.calls.length

    vi.useFakeTimers()
    try {
      await vi.advanceTimersByTimeAsync(BOARD_POLL_INTERVAL_MS * 3)
    } finally {
      vi.useRealTimers()
    }

    expect((service.leaderboard as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(boards)
    expect((service.votes as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(tallies)
    expect(room.blackBox.value).toBe(false)
    expect(room.board.value).toBeNull()
  })

  it('forgets a room locally without asking the server', async () => {
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService())
    await room.open()

    room.forget()

    expect(room.hosting.value).toBe(false)
    // A fresh composable must not resurrect it.
    expect(useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService()).serial.value).toBe('')
  })
})

describe('onScreenPairForBatch', () => {
  const displayed: [{ id: number }, { id: number }] = [{ id: 11 }, { id: 22 }]

  it('reports the pair when the batch empties the outbox', () => {
    expect(onScreenPairForBatch(3, 3, displayed)).toEqual([11, 22])
  })

  /**
   * THE CASE THAT WOULD MISLEAD A ROOM. The displayed pair is the one after every LOCAL vote.
   * If the batch is only part of the outbox — a host who was offline and is catching up — the
   * server would be told about a pair several rounds ahead of the votes it just recorded, and
   * the room would show participants a match whose wagers cannot be settled yet.
   */
  it('reports nothing when the batch is only part of the outbox', () => {
    expect(onScreenPairForBatch(128, 400, displayed)).toBeUndefined()
    expect(onScreenPairForBatch(1, 2, displayed)).toBeUndefined()
  })

  it('reports nothing when there is no pair on screen', () => {
    // A finished game has no pair; sending one would name eliminated elements.
    expect(onScreenPairForBatch(1, 1, null)).toBeUndefined()
  })
})
