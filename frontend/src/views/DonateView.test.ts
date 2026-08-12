// @vitest-environment happy-dom

import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import DonateView from './DonateView.vue'

vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRoute: () => ({ params: { locale: 'zh-tw' } }),
}))

function mountDonate() {
  return mount(DonateView)
}

describe('DonateView layout regressions', () => {
  it('draws the support tiers with line icons instead of emoji', () => {
    const wrapper = mountDonate()
    const icons = wrapper.findAll('.tier-icon')

    expect(icons).toHaveLength(4)
    for (const icon of icons) {
      // Emoji render in a different typeface on every platform, carry their own
      // colour, and cannot inherit the accent the rest of the icons use.
      expect(icon.find('svg path').exists()).toBe(true)
      expect(icon.text()).toBe('')
      expect(/\p{Extended_Pictographic}/u.test(icon.html())).toBe(false)
    }
    wrapper.unmount()
  })

  it('labels each section once instead of stacking an eyebrow above every heading', () => {
    const wrapper = mountDonate()

    // The hero keeps its kicker; the two section eyebrows only repeated the
    // heading sitting directly beneath them.
    expect(wrapper.findAll('.eyebrow')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('PAYMENT')
    expect(wrapper.text()).not.toContain('SMALL SUPPORT')
    wrapper.unmount()
  })

  it('keeps every section heading on the same left edge', () => {
    const wrapper = mountDonate()

    // section-heading-inline is a space-between flex row. With only a heading
    // inside it, the heading was pushed to the right while its sibling section
    // stayed left, so the page alternated alignment for no reason.
    expect(wrapper.findAll('.section-heading-inline')).toHaveLength(0)
    expect(wrapper.findAll('.section-heading')).toHaveLength(2)
    wrapper.unmount()
  })
})
