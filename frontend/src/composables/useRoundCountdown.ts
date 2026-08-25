import { getCurrentScope, onScopeDispose, ref, type Ref } from 'vue'

/**
 * The clock a majority round runs on.
 *
 * WHY THE SERVER SENDS A REMAINDER AND NOT A DEADLINE. The host and everybody watching have
 * to be counting down to the same instant, and their device clocks are not comparable — a
 * phone whose clock is two minutes fast would settle the round two minutes early. So the
 * server sends how much time is left, measured by the clock that armed it, and this counts
 * that number down locally between reads.
 *
 * Local ticking is only ever an interpolation. Every read and every pushed frame re-seeds
 * it, so drift lasts at most one poll interval and is corrected rather than accumulated.
 */

/**
 * How often the local clock ticks. Four times a second: fast enough that the last second
 * does not visibly stall, cheap enough to leave running for the length of a game.
 */
export const COUNTDOWN_TICK_MS = 250

export interface UseRoundCountdown {
  /** Seconds remaining, floored at 0, or null when nothing is counting down. */
  secondsLeft: Ref<number | null>
  /** What a countdown pill shows: whole seconds, rounded up so it ends on "1" and not "0". */
  display: Ref<number | null>
  /** True the moment a running countdown reaches zero. False when none is armed. */
  expired: Ref<boolean>
  /** Takes the server's remainder. Null stops the clock. */
  seed(seconds: number | null | undefined): void
  /** Stops and forgets. */
  reset(): void
}

export function useRoundCountdown(
  onExpire?: () => void,
  tickMs: number = COUNTDOWN_TICK_MS,
): UseRoundCountdown {
  const secondsLeft = ref<number | null>(null)
  const display = ref<number | null>(null)
  const expired = ref(false)
  let timer: ReturnType<typeof setInterval> | undefined

  function apply(value: number | null): void {
    secondsLeft.value = value
    display.value = value === null ? null : Math.max(0, Math.ceil(value))
  }

  function seed(seconds: number | null | undefined): void {
    if (seconds === null || seconds === undefined || Number.isNaN(seconds)) {
      reset()
      return
    }
    const value = Math.max(0, seconds)
    apply(value)
    // A round the server already considers over fires at once rather than waiting a tick:
    // this is the reload case, where the page opens onto an expired round.
    if (value === 0) {
      fire()
      return
    }
    expired.value = false
    start()
  }

  function start(): void {
    if (timer) return
    timer = setInterval(() => {
      if (secondsLeft.value === null) return
      const next = Math.max(0, secondsLeft.value - tickMs / 1000)
      apply(next)
      if (next === 0) fire()
    }, tickMs)
  }

  function fire(): void {
    stop()
    // Guarded: a poll that lands after expiry re-seeds 0 and would otherwise settle the
    // same round twice.
    if (expired.value) return
    expired.value = true
    onExpire?.()
  }

  function stop(): void {
    if (timer) clearInterval(timer)
    timer = undefined
  }

  function reset(): void {
    stop()
    apply(null)
    expired.value = false
  }

  // Guarded: this is also constructed directly in tests, where there is no scope to
  // register against and Vue would warn about it.
  if (getCurrentScope()) onScopeDispose(stop)

  return { secondsLeft, display, expired, seed, reset }
}
