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
    votes: vi.fn().mockResolvedValue({ votes: null, voting: null }),
    setVoting: vi.fn(),
    bet: vi.fn(),
    history: vi.fn().mockResolvedValue([]),
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
    const service = fakeService({
      votes: vi.fn().mockResolvedValue({ votes: tally, voting: null }),
    })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    // Opening reads once, because the same call is what reports whether this room decides
    // its own rounds — but the counts stay off screen until they are asked for.
    expect(room.votes.value).toBeNull()

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

  it('hands the round to the room and starts the clock the server armed', async () => {
    const service = fakeService({
      setVoting: vi.fn().mockResolvedValue({ mode: 'majority', round_seconds: 20, seconds_left: 20 }),
      votes: vi.fn().mockResolvedValue({
        votes: null,
        voting: { mode: 'majority', round_seconds: 20, seconds_left: 18.5 },
      }),
    })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    await room.setVoting('majority', 20)
    expect(service.setVoting).toHaveBeenCalledWith('abcdefgh', 'game-1', 'majority', 20)
    expect(room.majority.value).toBe(true)

    // 18.5 and not the 20 the write returned: SetVoting arms a fresh deadline, and the
    // remainder it reports was measured before the response travelled home. Rounded up, so
    // the pill and everybody else's pill read the same number.
    expect(room.secondsLeft.value).toBe(19)
    room.stopWatching()
  })

  it('leaves the clock alone in a manually ended room', async () => {
    const service = fakeService({
      setVoting: vi.fn().mockResolvedValue({ mode: 'majority', round_seconds: 0, seconds_left: null }),
      votes: vi.fn().mockResolvedValue({
        votes: null,
        voting: { mode: 'majority', round_seconds: 0, seconds_left: null },
      }),
    })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    await room.setVoting('majority', 0)
    expect(room.majority.value).toBe(true)
    expect(room.secondsLeft.value).toBeNull()
    expect(room.roundExpired.value).toBe(false)
    room.stopWatching()
  })

  it('gives the round to the side the room voted for', async () => {
    const service = fakeService({
      votes: vi.fn().mockResolvedValue({
        votes: {
          first_candidate: 11,
          second_candidate: 22,
          first_candidate_votes: 2,
          second_candidate_votes: 7,
          remain_elements: 2,
          total_votes: 9,
          current_round: 1,
          of_round: 1,
        },
        voting: { mode: 'majority', round_seconds: 10, seconds_left: 4 },
      }),
    })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    // Either order of the pair: which one the server calls "first" is its own business.
    expect(await room.majorityWinner([11, 22])).toBe(22)
    expect(await room.majorityWinner([22, 11])).toBe(22)

    // The counts are on screen without anyone opening a black box: in this mode they are
    // the rule the round was decided by, not a hint the host is hiding from guessers.
    expect(room.blackBox.value).toBe(false)
    expect(room.showVotes.value).toBe(true)
    expect(room.votes.value?.second_candidate_votes).toBe(7)
    room.stopWatching()
  })

  /*
  The host's own vote. In host mode a click IS the verdict, so the buttons were locked; in a
  majority room it is a wager like everybody else's and has to be counted like one — which
  means it must not end the round it was cast in.
  */
  it('places the host\'s own vote as a wager, and settles nothing', async () => {
    const service = fakeService({
      votes: vi.fn().mockResolvedValue({
        votes: {
          first_candidate: 11,
          second_candidate: 22,
          first_candidate_votes: 1,
          second_candidate_votes: 0,
          remain_elements: 2,
          total_votes: 1,
          current_round: 1,
          of_round: 1,
        },
        voting: { mode: 'majority', round_seconds: 0, seconds_left: null },
      }),
    })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    await room.placeBet(11, 22)
    expect(service.bet).toHaveBeenCalledWith('abcdefgh', { winner_id: 11, loser_id: 22 })
    expect(room.ownPick.value).toBe(11)
    // The vote went into the tally the round will be decided by, and nothing else moved.
    expect(room.votes.value?.first_candidate_votes).toBe(1)
    room.stopWatching()
  })

  // A candidate that wins carries into the next round, so "my pick" cannot be remembered by
  // its id alone: the mark would follow the survivor onto a match never wagered on.
  it('takes the host\'s mark off a pick once the pairing moves on', async () => {
    const tally = (first: number, second: number): unknown => ({
      votes: {
        first_candidate: first,
        second_candidate: second,
        first_candidate_votes: 1,
        second_candidate_votes: 0,
        remain_elements: 2,
        total_votes: 1,
        current_round: 1,
        of_round: 1,
      },
      voting: { mode: 'majority', round_seconds: 0, seconds_left: null },
    })
    const service = fakeService({ votes: vi.fn().mockResolvedValue(tally(11, 22)) })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    await room.placeBet(11, 22)
    expect(room.ownPick.value).toBe(11)

    // 11 survived into a match against 33, which nobody has wagered on yet.
    service.votes = vi.fn().mockResolvedValue(tally(11, 33))
    await room.majorityWinner([11, 33])
    expect(room.ownPick.value).toBeNull()
    room.stopWatching()
  })

  it('refuses to place a bet the server rejects', async () => {
    const service = fakeService({
      bet: vi.fn().mockRejectedValue(apiError(409)),
      votes: vi.fn().mockResolvedValue({
        votes: null,
        voting: { mode: 'majority', round_seconds: 0, seconds_left: null },
      }),
    })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    await room.placeBet(11, 22)
    // No mark: a wager the room never recorded must not be shown as one that was.
    expect(room.ownPick.value).toBeNull()
    room.stopWatching()
  })

  it('tosses a coin on a tie, and on a room nobody voted in', async () => {
    const tally = (first: number, second: number): unknown => ({
      votes: {
        first_candidate: 11,
        second_candidate: 22,
        first_candidate_votes: first,
        second_candidate_votes: second,
        remain_elements: 2,
        total_votes: first + second,
        current_round: 1,
        of_round: 1,
      },
      voting: { mode: 'majority', round_seconds: 10, seconds_left: 4 },
    })
    const service = fakeService({ votes: vi.fn().mockResolvedValue(tally(4, 4)) })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    expect(await room.majorityWinner([11, 22], () => 0.2)).toBe(11)
    expect(await room.majorityWinner([11, 22], () => 0.8)).toBe(22)

    // 0-0 is a tie like any other, and it is what an unwatched room produces every round.
    // Deciding it any other way would bias every bracket a quiet room ever played.
    service.votes = vi.fn().mockResolvedValue(tally(0, 0))
    expect(await room.majorityWinner([11, 22], () => 0.2)).toBe(11)
    expect(await room.majorityWinner([11, 22], () => 0.8)).toBe(22)
    room.stopWatching()
  })

  it('reads no votes off a tally about some other pairing', async () => {
    const service = fakeService({
      votes: vi.fn().mockResolvedValue({
        votes: {
          first_candidate: 41,
          second_candidate: 42,
          first_candidate_votes: 9,
          second_candidate_votes: 0,
          remain_elements: 2,
          total_votes: 9,
          current_round: 2,
          of_round: 2,
        },
        voting: { mode: 'majority', round_seconds: 10, seconds_left: 4 },
      }),
    })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    // A pair the server has not caught up with has no votes yet — reading the ones it does
    // have would credit them to whichever side happened to be listed in the same slot.
    expect(await room.majorityWinner([11, 22], () => 0.2)).toBe(11)
    expect(await room.majorityWinner([11, 22], () => 0.8)).toBe(22)
    room.stopWatching()
  })

  it('still ends the round when the tally cannot be read at all', async () => {
    const service = fakeService({ votes: vi.fn().mockRejectedValue(apiError(500)) })
    const room = useHostedRoom(ref('game-1'), ref('zh-tw'), service)
    await room.open()
    await flush()

    // Stalling the bracket on a failed read would be worse than a coin toss: the game stops.
    expect(await room.majorityWinner([11, 22], () => 0.2)).toBe(11)
    expect(await room.majorityWinner([11, 22], () => 0.8)).toBe(22)
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
