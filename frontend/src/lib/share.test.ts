// @vitest-environment happy-dom

import { afterEach, describe, expect, it, vi } from 'vitest'

import { prefersShareSheet, shareOrCopyLink } from './share'

interface FakeMediaQueries {
  coarsePointer: boolean
  finePointer: boolean
}

function stubDevice(options: {
  share?: (data: ShareData) => Promise<void>
  clipboard?: (text: string) => Promise<void>
  mobileHint?: boolean
  queries?: FakeMediaQueries
  touchPoints?: number
}): void {
  define('share', options.share)
  define('clipboard', options.clipboard ? { writeText: options.clipboard } : undefined)
  define('userAgentData', options.mobileHint === undefined ? undefined : { mobile: options.mobileHint })
  define('maxTouchPoints', options.touchPoints ?? 0)

  const queries = options.queries ?? { coarsePointer: false, finePointer: true }
  vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
    matches: query.includes('pointer: coarse') ? queries.coarsePointer : queries.finePointer,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }) as MediaQueryList)
}

function define(property: string, value: unknown): void {
  Object.defineProperty(navigator, property, { configurable: true, value })
}

afterEach(() => {
  vi.restoreAllMocks()
  for (const property of ['share', 'clipboard', 'userAgentData', 'maxTouchPoints']) {
    Reflect.deleteProperty(navigator, property)
  }
})

describe('prefersShareSheet', () => {
  it('says no on a desktop that happens to implement navigator.share', () => {
    // Desktop Chrome and Edge do. Calling it there is the modal interruption in front of a
    // copy that already worked, which is exactly what this check exists to prevent.
    stubDevice({ share: vi.fn(), mobileHint: false })
    expect(prefersShareSheet()).toBe(false)
  })

  it('says yes on a phone', () => {
    stubDevice({ share: vi.fn(), mobileHint: true })
    expect(prefersShareSheet()).toBe(true)
  })

  it('falls back to pointer hardware when the browser states no form factor', () => {
    stubDevice({
      share: vi.fn(),
      queries: { coarsePointer: true, finePointer: false },
      touchPoints: 5,
    })
    expect(prefersShareSheet()).toBe(true)
  })

  it('treats a touchscreen laptop as a desktop', () => {
    // A trackpad is a fine pointer, so the share sheet is not this device's natural route.
    stubDevice({
      share: vi.fn(),
      queries: { coarsePointer: true, finePointer: true },
      touchPoints: 10,
    })
    expect(prefersShareSheet()).toBe(false)
  })

  it('says no where there is no share sheet at all', () => {
    stubDevice({ mobileHint: true })
    expect(prefersShareSheet()).toBe(false)
  })
})

describe('shareOrCopyLink', () => {
  it('copies on a desktop without opening a share sheet', async () => {
    const share = vi.fn()
    const clipboard = vi.fn().mockResolvedValue(undefined)
    stubDevice({ share, clipboard, mobileHint: false })

    expect(await shareOrCopyLink('https://2pick.app/r/x', '標題')).toBe('copied')
    expect(share).not.toHaveBeenCalled()
    expect(clipboard).toHaveBeenCalledWith('https://2pick.app/r/x')
  })

  it('opens the share sheet on a phone and leaves the clipboard alone', async () => {
    const share = vi.fn().mockResolvedValue(undefined)
    const clipboard = vi.fn().mockResolvedValue(undefined)
    stubDevice({ share, clipboard, mobileHint: true })

    expect(await shareOrCopyLink('https://2pick.app/r/x', '標題')).toBe('shared')
    expect(share).toHaveBeenCalledWith({ title: '標題', url: 'https://2pick.app/r/x' })
    expect(clipboard).not.toHaveBeenCalled()
  })

  it('respects a dismissed share sheet', async () => {
    // Cancelling is a decision. The link must not land on the clipboard behind the user's
    // back, and no "copied" confirmation may be shown for something that was not copied.
    const clipboard = vi.fn().mockResolvedValue(undefined)
    stubDevice({
      share: vi.fn().mockRejectedValue(new DOMException('cancelled', 'AbortError')),
      clipboard,
      mobileHint: true,
    })

    expect(await shareOrCopyLink('https://2pick.app/r/x', '標題')).toBe('dismissed')
    expect(clipboard).not.toHaveBeenCalled()
  })

  it('copies when the share sheet fails to open', async () => {
    const clipboard = vi.fn().mockResolvedValue(undefined)
    stubDevice({
      share: vi.fn().mockRejectedValue(new DOMException('blocked', 'NotAllowedError')),
      clipboard,
      mobileHint: true,
    })

    expect(await shareOrCopyLink('https://2pick.app/r/x', '標題')).toBe('copied')
    expect(clipboard).toHaveBeenCalledWith('https://2pick.app/r/x')
  })

  it('reports failure when the clipboard is denied', async () => {
    stubDevice({ clipboard: vi.fn().mockRejectedValue(new Error('denied')), mobileHint: false })
    expect(await shareOrCopyLink('https://2pick.app/r/x', '標題')).toBe('failed')
  })
})
