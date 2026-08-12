// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

import { APIError } from '../lib/api'
import { onScreenPairForBatch, useHostedRoom } from './useHostedRoom'
import type { GameRoomService } from '../services/gameRoom'

function fakeService(overrides: Partial<GameRoomService> = {}): GameRoomService {
  return {
    open: vi.fn().mockResolvedValue({ serial: 'abcdefgh', game_serial: 'game-1' }),
    state: vi.fn(),
    leaderboard: vi.fn(),
    bet: vi.fn(),
    rename: vi.fn(),
    ...overrides,
  } as unknown as GameRoomService
}

function apiError(status: number): APIError {
  return new APIError(status, { error: { code: 'x', message: 'no' } } as never)
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
