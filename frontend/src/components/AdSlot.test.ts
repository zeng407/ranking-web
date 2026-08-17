// @vitest-environment happy-dom

import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AdSlot from './AdSlot.vue'
import { resetAdScriptForTests } from '../ads/adsense'

interface ObservedTarget {
  trigger: () => void
}

const observed: ObservedTarget[] = []

class FakeIntersectionObserver {
  constructor(private readonly callback: IntersectionObserverCallback) {
    observed.push({ trigger: () => this.callback([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver) })
  }

  observe(): void {}
  disconnect(): void {}
  unobserve(): void {}
  takeRecords(): IntersectionObserverEntry[] { return [] }
  readonly root = null
  readonly rootMargin = ''
  readonly thresholds = []
}

/** Puts a matching script tag in the page so the loader treats the tag as present. */
function stubLoadedTag(): void {
  const script = document.createElement('script')
  script.dataset.stub = 'adsbygoogle'
  script.type = 'text/plain'
  script.setAttribute('src', 'https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-1')
  document.head.appendChild(script)
}

function configure(ads: unknown): void {
  window.__APP_CONFIG__ = { apiBaseUrl: '/api/v1', ads } as Window['__APP_CONFIG__']
}

beforeEach(() => {
  observed.length = 0
  resetAdScriptForTests()
  window.adsbygoogle = undefined
  vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
})

afterEach(() => {
  vi.unstubAllGlobals()
  window.__APP_CONFIG__ = undefined
  document.head.querySelectorAll('script[src*="adsbygoogle"]').forEach((script) => script.remove())
})

describe('AdSlot', () => {
  it('renders nothing without a publisher id', () => {
    configure({ publisherId: '', slots: { homeTop: '1234567890' } })

    const wrapper = mount(AdSlot, { props: { name: 'homeTop', locale: 'zh_TW' } })

    expect(wrapper.find('.ad-slot').exists()).toBe(false)
  })

  it('renders nothing when this position has no slot id', () => {
    configure({ publisherId: 'ca-pub-1', slots: { homeFeed: '1234567890' } })

    const wrapper = mount(AdSlot, { props: { name: 'homeTop', locale: 'zh_TW' } })

    expect(wrapper.find('.ad-slot').exists()).toBe(false)
  })

  it('reserves the box first and only requests the unit once it is in view', async () => {
    configure({ publisherId: 'ca-pub-1', slots: { homeTop: '1234567890' } })
    // Stands in for the loaded tag: happy-dom refuses to fetch scripts, and what this
    // test is about is when the unit appears, not how the tag arrives.
    stubLoadedTag()

    const wrapper = mount(AdSlot, { props: { name: 'homeTop', locale: 'zh_TW' }, attachTo: document.body })

    // Reserved, labelled, and no ad markup or fill request yet.
    expect(wrapper.find('.ad-slot.ad-slot-horizontal').exists()).toBe(true)
    expect(wrapper.text()).toContain('廣告')
    expect(wrapper.find('ins').exists()).toBe(false)
    expect(window.adsbygoogle).toBeUndefined()

    observed[0]!.trigger()
    await flushPromises()

    const unit = wrapper.get('ins')
    expect(unit.classes()).toContain('adsbygoogle')
    expect(unit.attributes('data-ad-client')).toBe('ca-pub-1')
    expect(unit.attributes('data-ad-slot')).toBe('1234567890')
    expect(window.adsbygoogle).toHaveLength(1)

    wrapper.unmount()
  })

  it('loads the tag straight away where there is no observer', async () => {
    configure({ publisherId: 'ca-pub-1', slots: { gameResult: '99' } })
    stubLoadedTag()
    vi.stubGlobal('IntersectionObserver', undefined)

    const wrapper = mount(AdSlot, { props: { name: 'gameResult', locale: 'en' }, attachTo: document.body })
    await flushPromises()

    expect(wrapper.get('ins').attributes('data-ad-slot')).toBe('99')
    wrapper.unmount()
  })
})
