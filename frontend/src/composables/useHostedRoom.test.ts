// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

import { APIError } from '../lib/api'
import { BOARD_POLL_INTERVAL_MS, onScreenPairForBatch, useHostedRoom } from './useHostedRoom'
import type { GameRoomService, Leaderboard, RoomVotes } from '../services/gameRoom'

function fakeService(overrides: Partial<GameRoomService> = {}): GameRoomService {
  return {
    open: vi.fn().mockResolvedValue({ serial: 'abcdefgh', game_serial: 'game-1' }),
    rebind: vi.fn().mockResolvedValue({ serial: 'abcdefgh', game_serial: 'game-2' }),
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

  // A page that opens on another game hosts nothing: the stored room belongs to the game it
  // was opened for, and nothing here says this host ever ran it.
  it('does not carry a room into a different game', async () => {
    await useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService()).open()

    const other = useHostedRoom(ref('game-2'), ref('zh-tw'), fakeService())
    expect(other.serial.value).toBe('')
    expect(other.hosting.value).toBe(false)
  })

  /*
  The restart. The game serial changes under an open room, and the room has to follow it:
  abandoning it would leave the participants voting on a decided match while the host's votes
  went to a game the room cannot see, and re-opening would mint a serial that invalidates
  every invite link already handed out.
  */
  it('moves the open room onto the game a restart created', async () => {
    const service = fakeService()
    const gameSerial = ref('game-1')
    const room = useHostedRoom(gameSerial, ref('zh-tw'), service, () => [11, 22])

    await room.open()
    gameSerial.value = 'game-2'
    await flush()

    expect(service.rebind).toHaveBeenCalledWith('abcdefgh', 'game-1', 'game-2', [11, 22])
    expect(room.serial.value).toBe('abcdefgh')
    expect(room.status.value).toBe('open')

    // The stored key moved with it, so the host's next reload finds the room on the new
    // game rather than opening a second one.
    const reloaded = useHostedRoom(ref('game-2'), ref('zh-tw'), fakeService())
    expect(reloaded.serial.value).toBe('abcdefgh')
    reloaded.stopWatching()
    room.stopWatching()
  })

  // A refusal means this page does not drive that room — it has already moved, or the new
  // game belongs to another post. Better to stop hosting than to show a link that lies.
  it('stops hosting when the room refuses to follow', async () => {
    const service = fakeService({ rebind: vi.fn().mockRejectedValue(apiError(409)) })
    const gameSerial = ref('game-1')
    const room = useHostedRoom(gameSerial, ref('zh-tw'), service)

    await room.open()
    gameSerial.value = 'game-2'
    await flush()

    expect(room.serial.value).toBe('')
    expect(room.status.value).toBe('closed')
    expect(room.inviteURL.value).toBe('')
    // And the old key is gone, so a reload does not resurrect it.
    expect(useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService()).serial.value).toBe('')
  })

  // Only when a room is actually open: a host who never opened one must not have a rebind
  // fired at them every time they restart.
  it('does not rebind when no room is open', async () => {
    const service = fakeService()
    const gameSerial = ref('game-1')
    useHostedRoom(gameSerial, ref('zh-tw'), service)

    gameSerial.value = 'game-2'
    await flush()

    expect(service.rebind).not.toHaveBeenCalled()
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

  /*
  The reload, from the room's point of view. The server's record of the pairing came from the
  host's last vote sync, and a resume can legitimately land on a different pair — so picking
  the room back up has to say which pair is on screen now, or everybody seated keeps looking
  at the pre-reload match and their poll keeps confirming it.
  */
  it('reports the pairing when it picks a stored room back up', async () => {
    await useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService()).open()

    const serial = ref('')
    const service = fakeService()
    const room = useHostedRoom(serial, ref('zh-tw'), service, () => [33, 44])

    serial.value = 'game-1'
    await flush()

    expect(service.open).toHaveBeenCalledWith('game-1', [33, 44])
    expect(room.serial.value).toBe('abcdefgh')
    expect(room.status.value).toBe('open')
    room.stopWatching()
  })

  // Nothing to report between rounds, and a request that named no pair would broadcast a
  // pairing change that did not happen.
  it('reports no pairing when none is on screen', async () => {
    await useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService()).open()

    const serial = ref('')
    const service = fakeService()
    const room = useHostedRoom(serial, ref('zh-tw'), service, () => undefined)

    serial.value = 'game-1'
    await flush()

    expect(service.open).not.toHaveBeenCalled()
    expect(room.hosting.value).toBe(true)
    room.stopWatching()
  })

  // The report is a courtesy, not a precondition: a host whose room the server has forgotten
  // keeps playing, and the room's own state read is what will notice.
  it('keeps hosting when the pairing report fails', async () => {
    await useHostedRoom(ref('game-1'), ref('zh-tw'), fakeService()).open()

    const serial = ref('')
    const service = fakeService({ open: vi.fn().mockRejectedValue(apiError(500)) })
    const room = useHostedRoom(serial, ref('zh-tw'), service, () => [33, 44])

    serial.value = 'game-1'
    await flush()

    expect(room.serial.value).toBe('abcdefgh')
    expect(room.status.value).toBe('open')
    room.stopWatching()
  })

  /**
   * The re-pick a reload performs happens after the room has been picked back up, so the
   * game view reports it itself. Calling that on a game with no room must stay a no-op:
   * opening would create one, and a solo host would find themselves hosting.
   */
  it('reports nothing on a game with no room open', async () => {
    const service = fakeService()
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service, () => [33, 44])

    await room.reportPair()

    expect(service.open).not.toHaveBeenCalled()
    expect(room.hosting.value).toBe(false)
  })

  // The point of exposing it: the pair on screen changed without a vote, so the room has to
  // be told again after it was already adopted.
  it('reports the pairing again on demand while hosting', async () => {
    const service = fakeService()
    const pair = ref([11, 22])
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service, () => pair.value)
    await room.open(pair.value)

    pair.value = [33, 44]
    await room.reportPair()

    expect(service.open).toHaveBeenLastCalledWith('game-1', [33, 44])
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
