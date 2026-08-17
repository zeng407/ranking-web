// @vitest-environment happy-dom

import { beforeEach, describe, expect, it } from 'vitest'

import { loadAdScript, pushAdUnit, resetAdScriptForTests } from './adsense'

interface FakeScript {
  src: string
  async: boolean
  crossOrigin: string | null
  listeners: Record<string, Array<() => void>>
  addEventListener(event: string, listener: () => void): void
}

function fakeDocument(): { target: Document; scripts: FakeScript[] } {
  const scripts: FakeScript[] = []
  const target = {
    createElement: (): FakeScript => {
      const script: FakeScript = {
        src: '', async: false, crossOrigin: null, listeners: {},
        addEventListener(event, listener) {
          (this.listeners[event] ||= []).push(listener)
        },
      }
      return script
    },
    querySelector: (): null => null,
    head: { appendChild: (script: FakeScript) => scripts.push(script) },
  } as unknown as Document
  return { target, scripts }
}

beforeEach(() => {
  resetAdScriptForTests()
  window.adsbygoogle = undefined
})

describe('loadAdScript', () => {
  it('loads the tag once, for the configured publisher', async () => {
    const { target, scripts } = fakeDocument()

    const first = loadAdScript('ca-pub-42', target)
    const second = loadAdScript('ca-pub-42', target)

    expect(scripts).toHaveLength(1)
    expect(scripts[0]!.src).toBe('https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-42')
    expect(scripts[0]!.async).toBe(true)
    expect(scripts[0]!.crossOrigin).toBe('anonymous')

    scripts[0]!.listeners.load!.forEach((listener) => listener())
    await expect(first).resolves.toBeUndefined()
    await expect(second).resolves.toBeUndefined()
  })

  it('rejects on a blocked tag and allows a later attempt', async () => {
    const { target, scripts } = fakeDocument()

    const attempt = loadAdScript('ca-pub-42', target)
    scripts[0]!.listeners.error!.forEach((listener) => listener())
    await expect(attempt).rejects.toThrow('adsense script failed to load')

    // An ad blocker is not a permanent verdict on the next page view.
    loadAdScript('ca-pub-42', target)
    expect(scripts).toHaveLength(2)
  })
})

describe('pushAdUnit', () => {
  it('queues one fill request per call', () => {
    pushAdUnit()
    pushAdUnit()

    expect(window.adsbygoogle).toHaveLength(2)
  })
})
