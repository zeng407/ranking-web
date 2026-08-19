// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { closeImageViewer, isImageViewerOpen, openImageViewer } from './imageViewer'

/*
viewer.js measures the picture against the viewport to lay its overlay out, which
happy-dom cannot do. The mock keeps the parts this module is responsible for: the
element it is handed, the options, and the two calls it makes on the instance.
*/
const instances = vi.hoisted(() => [] as {
  element: HTMLElement
  options: { title: () => string; hidden: () => void }
  shown: number
  destroyed: number
}[])

vi.mock('viewerjs', () => ({
  default: class {
    constructor(element: HTMLElement, options: { title: () => string; hidden: () => void }) {
      instances.push({ element, options, shown: 0, destroyed: 0 })
    }

    show(): void {
      instances[instances.length - 1]!.shown += 1
    }

    destroy(): void {
      instances[instances.length - 1]!.destroyed += 1
    }
  },
}))

beforeEach(() => {
  closeImageViewer()
  instances.length = 0
})

describe('image viewer', () => {
  it('hands viewer.js the full-size picture and its caption', () => {
    openImageViewer({ image: 'https://cdn.test/full.jpg', title: '#3 Assam' })

    const viewer = instances[0]!
    const image = viewer.element.querySelector('img')!
    expect(image.getAttribute('src')).toBe('https://cdn.test/full.jpg')
    expect(image.alt).toBe('#3 Assam')
    expect(viewer.options.title()).toBe('#3 Assam')
    expect(viewer.shown).toBe(1)
    expect(isImageViewerOpen()).toBe(true)
  })

  it('forgets the picture once the viewer reports itself hidden', () => {
    openImageViewer({ image: 'https://cdn.test/full.jpg', title: '#3 Assam' })
    instances[0]!.options.hidden()

    expect(isImageViewerOpen()).toBe(false)
    expect(instances[0]!.destroyed).toBe(1)
  })

  it('destroys the viewer once when closed from the page', () => {
    openImageViewer({ image: 'https://cdn.test/full.jpg', title: '#3 Assam' })
    // destroy() fires the hidden handler in turn, which must not destroy again.
    closeImageViewer()
    instances[0]!.options.hidden()

    expect(instances[0]!.destroyed).toBe(1)
    expect(isImageViewerOpen()).toBe(false)
  })

  it('replaces the open picture rather than stacking a second viewer', () => {
    openImageViewer({ image: 'https://cdn.test/one.jpg', title: '#1' })
    openImageViewer({ image: 'https://cdn.test/two.jpg', title: '#2' })

    expect(instances).toHaveLength(2)
    expect(instances[0]!.destroyed).toBe(1)
    expect(instances[1]!.element.querySelector('img')!.getAttribute('src'))
      .toBe('https://cdn.test/two.jpg')
  })
})
