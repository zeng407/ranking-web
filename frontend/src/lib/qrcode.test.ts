// @vitest-environment happy-dom

import { describe, expect, it, vi } from 'vitest'

import { downloadQRCode } from './qrcode'

/*
A canvas stub rather than a real one: happy-dom has no 2d context, and what this function
does with a canvas is ask it for a data URL.
*/
function fakeCanvas(dataURL = 'data:image/png;base64,AAA'): HTMLCanvasElement {
  return { toDataURL: vi.fn().mockReturnValue(dataURL) } as unknown as HTMLCanvasElement
}

describe('downloadQRCode', () => {
  it('saves the drawn code under the given name', () => {
    const clicked: { download: string; href: string }[] = []
    const anchor = document.createElement('a')
    anchor.click = () => { clicked.push({ download: anchor.download, href: anchor.href }) }
    vi.spyOn(document, 'createElement').mockReturnValue(anchor)

    downloadQRCode(fakeCanvas(), '2pick-room-abcdefgh.png')

    expect(clicked).toEqual([{
      download: '2pick-room-abcdefgh.png',
      href: 'data:image/png;base64,AAA',
    }])
    vi.restoreAllMocks()
  })
})
