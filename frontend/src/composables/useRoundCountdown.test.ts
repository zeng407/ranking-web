import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { COUNTDOWN_TICK_MS, useRoundCountdown } from './useRoundCountdown'

describe('useRoundCountdown', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('counts the server\'s remainder down and expires once', () => {
    const expire = vi.fn()
    const clock = useRoundCountdown(expire)

    clock.seed(2)
    expect(clock.display.value).toBe(2)
    expect(clock.expired.value).toBe(false)

    // Rounded up, so the pill reads "1" through the whole last second rather than
    // sitting on "0" while the round is still open.
    vi.advanceTimersByTime(COUNTDOWN_TICK_MS * 5)
    expect(clock.display.value).toBe(1)
    expect(expire).not.toHaveBeenCalled()

    vi.advanceTimersByTime(COUNTDOWN_TICK_MS * 3)
    expect(clock.secondsLeft.value).toBe(0)
    expect(clock.expired.value).toBe(true)
    expect(expire).toHaveBeenCalledTimes(1)

    // A poll landing after expiry re-seeds 0. The round is already being settled, and
    // settling it a second time would eliminate a candidate nobody was shown.
    clock.seed(0)
    expect(expire).toHaveBeenCalledTimes(1)
  })

  it('fires at once for a round that was already over when the page opened', () => {
    const expire = vi.fn()
    const clock = useRoundCountdown(expire)

    clock.seed(0)
    expect(expire).toHaveBeenCalledTimes(1)
    expect(clock.display.value).toBe(0)
  })

  it('re-seeding corrects local drift instead of accumulating it', () => {
    const clock = useRoundCountdown()

    clock.seed(10)
    vi.advanceTimersByTime(COUNTDOWN_TICK_MS * 8)
    expect(clock.display.value).toBe(8)

    // The server is the authority: whatever it says now replaces whatever was counted here.
    clock.seed(5)
    expect(clock.display.value).toBe(5)
    vi.advanceTimersByTime(COUNTDOWN_TICK_MS * 4)
    expect(clock.display.value).toBe(4)
  })

  it('stops when the room stops counting', () => {
    const expire = vi.fn()
    const clock = useRoundCountdown(expire)

    clock.seed(3)
    // A manual-end room, and a host-decided room, both report no deadline at all.
    clock.seed(null)
    expect(clock.display.value).toBeNull()
    expect(clock.expired.value).toBe(false)

    vi.advanceTimersByTime(COUNTDOWN_TICK_MS * 40)
    expect(expire).not.toHaveBeenCalled()
  })
})
