/**
 * The AdSense loader, kept out of the component so the script is fetched once per
 * page rather than once per slot.
 *
 * Nothing here runs until a slot is actually about to be seen: the home page mounts
 * up to five units, and loading the tag for units nobody scrolls to costs the reader
 * bandwidth and Google an unfilled impression.
 */

const SCRIPT_URL = 'https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js'

let scriptPromise: Promise<void> | null = null

/** Loads the AdSense tag, at most once, and resolves when it is ready to fill units. */
export function loadAdScript(publisherId: string, target: Document = document): Promise<void> {
  if (scriptPromise) return scriptPromise

  scriptPromise = new Promise<void>((resolve, reject) => {
    const existing = target.querySelector<HTMLScriptElement>(`script[src^="${SCRIPT_URL}"]`)
    if (existing) {
      resolve()
      return
    }
    const script = target.createElement('script')
    script.src = `${SCRIPT_URL}?client=${encodeURIComponent(publisherId)}`
    script.async = true
    script.crossOrigin = 'anonymous'
    script.addEventListener('load', () => resolve())
    script.addEventListener('error', () => {
      // A blocked or failed tag is not an error the reader should ever see: the slot
      // stays an empty reserved box and the page carries on.
      scriptPromise = null
      reject(new Error('adsense script failed to load'))
    })
    target.head.appendChild(script)
  })
  return scriptPromise
}

/**
 * Asks AdSense to fill the units it has not filled yet.
 *
 * The queue takes no element reference — it always fills every unfilled `ins` in the
 * document — so a slot pushes once and only once, after its own element exists.
 */
export function pushAdUnit(): void {
  const queue = (window.adsbygoogle = window.adsbygoogle || [])
  queue.push({})
}

/** Test seam: forgets the loaded script so each test starts from a clean page. */
export function resetAdScriptForTests(): void {
  scriptPromise = null
}
