/**
 * Handing a link to somebody else.
 *
 * Two mechanisms exist and they are not interchangeable. A phone has a share sheet, which
 * is how its owner passes a link to a chat app; a desktop has a clipboard and a window
 * manager, and the share sheet it sometimes offers (Windows, ChromeOS) is a modal
 * interruption in front of something the copy button has already done. So the sheet is
 * offered only on a device that is actually held in a hand, and everywhere else a copy
 * button copies.
 */

/** The parts of the UA-CH surface used here; not in every browser's lib.dom yet. */
interface NavigatorUserAgentData {
  mobile?: boolean
}

/**
 * Whether this device's share sheet is the natural way to pass a link on.
 *
 * Not simply `'share' in navigator`: desktop Chrome and Edge implement it too, and calling
 * it there is the interruption this exists to avoid.
 */
export function prefersShareSheet(): boolean {
  if (typeof navigator === 'undefined' || typeof navigator.share !== 'function') return false

  // The browser's own statement about the form factor, where it makes one. Preferred over
  // any inference from input hardware.
  const hints = (navigator as Navigator & { userAgentData?: NavigatorUserAgentData }).userAgentData
  if (typeof hints?.mobile === 'boolean') return hints.mobile

  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
  // A touch screen AND no mouse-grade pointer. Both halves are needed: a touchscreen
  // laptop still has a trackpad, and that is a desktop.
  return window.matchMedia('(pointer: coarse)').matches
    && !window.matchMedia('(any-pointer: fine)').matches
    && (navigator.maxTouchPoints ?? 0) > 0
}

/**
 * What happened to the link.
 *
 * `dismissed` is separate from `copied` because a cancelled share sheet is a decision: the
 * link must not then land on the clipboard, and no "copied" confirmation may be shown for
 * something that was not copied.
 */
export type LinkHandoff = 'shared' | 'copied' | 'dismissed' | 'failed'

/** Offers the share sheet on a handheld, and copies everywhere else. */
export async function shareOrCopyLink(url: string, title: string): Promise<LinkHandoff> {
  if (prefersShareSheet()) {
    try {
      await navigator.share({ title, url })
      return 'shared'
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') return 'dismissed'
      // Anything else — a sheet that failed to open, a policy block — falls through to the
      // clipboard, which is a working way to pass the link on.
    }
  }

  try {
    await navigator.clipboard.writeText(url)
    return 'copied'
  } catch {
    // Clipboard access can be denied outright. Callers show the link as text as well, so
    // there is still a way to pass it on by hand.
    return 'failed'
  }
}
