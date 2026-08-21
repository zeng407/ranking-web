/**
 * The invite QR code.
 *
 * A thin wrapper over the `qrcode` package — the same one the old UI used — for two
 * reasons. The import is dynamic, so the encoder stays out of the main bundle and is
 * fetched only when a host actually opens the invite; and the two things the app does with
 * a code (draw it, save it) are named here rather than repeated at the call site.
 */

/** Pixels. The canvas is scaled by CSS; this is what gets saved. */
const CODE_SIZE = 240

/**
 * Draws `text` as a QR code into `canvas`.
 *
 * Fixed black on white, never the page's palette: a code is scanned by a camera pointed at
 * a screen, and inverted or low-contrast modules are the one way to make it unreadable.
 */
export async function drawQRCode(canvas: HTMLCanvasElement, text: string): Promise<void> {
  const { toCanvas } = await import('qrcode')
  await toCanvas(canvas, text, {
    width: CODE_SIZE,
    // One module of quiet zone. Scanners need some, and the default four makes the code
    // small inside its own box.
    margin: 1,
    color: { dark: '#0f172a', light: '#ffffff' },
  })
}

/**
 * Saves what is on `canvas` as a PNG.
 *
 * The anchor is synthetic and never enters the document: a click on a detached element
 * still starts the download, and appending it would flash a stray node into the dialog.
 */
export function downloadQRCode(canvas: HTMLCanvasElement, filename: string): void {
  const link = document.createElement('a')
  link.download = filename
  link.href = canvas.toDataURL('image/png')
  link.click()
}
