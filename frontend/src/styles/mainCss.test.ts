import { readFileSync } from 'node:fs'

import { describe, expect, it } from 'vitest'

const css = readFileSync(new URL('./main.css', import.meta.url), 'utf8')

function darkVariable(name: string): string {
  const darkBlock = css.match(/:root\[data-theme="dark"\]\s*\{([\s\S]*?)\}/)?.[1] ?? ''
  return darkBlock.match(new RegExp(`${name}:\\s*(#[0-9a-fA-F]{6})`))?.[1] ?? ''
}

function luminance(hex: string): number {
  const channels = hex.slice(1).match(/.{2}/g)?.map((value) => Number.parseInt(value, 16) / 255) ?? []
  const linear = channels.map((value) => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4)
  return 0.2126 * (linear[0] ?? 0) + 0.7152 * (linear[1] ?? 0) + 0.0722 * (linear[2] ?? 0)
}

function contrast(left: string, right: string): number {
  const values = [luminance(left), luminance(right)].sort((a, b) => b - a)
  return ((values[0] ?? 0) + 0.05) / ((values[1] ?? 0) + 0.05)
}

function rule(selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return css.match(new RegExp(`${escaped}\\s*\\{([\\s\\S]*?)\\}`))?.[1] ?? ''
}

describe('dark-mode ranking tabs', () => {
  it('keeps selected and unselected tab labels at WCAG AA contrast', () => {
    const activeText = darkVariable('--rank-tab-active-text')
    const activeBackground = darkVariable('--rank-tab-active-bg')
    const inactiveText = darkVariable('--rank-tab-inactive-text')
    const inactiveBackground = darkVariable('--rank-tab-bg')

    expect(activeText).toMatch(/^#[0-9a-f]{6}$/i)
    expect(activeBackground).toMatch(/^#[0-9a-f]{6}$/i)
    expect(inactiveText).toMatch(/^#[0-9a-f]{6}$/i)
    expect(inactiveBackground).toMatch(/^#[0-9a-f]{6}$/i)
    expect(contrast(activeText, activeBackground)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(inactiveText, inactiveBackground)).toBeGreaterThanOrEqual(4.5)
  })
})

describe('public ranking heading', () => {
  it('keeps the start-game action readable instead of shrinking or wrapping', () => {
		const actions = rule('.game-public-ranking-actions')
		const action = rule('.game-public-ranking-actions .button')
    const title = rule('.game-public-ranking-heading > div')

		expect(actions).toContain('flex: 0 0 auto')
    expect(action).toContain('white-space: nowrap')
    expect(title).toContain('min-width: 0')
  })
})

describe('small-type floor', () => {
  it('keeps eyebrows and card meta above the CJK legibility floor', () => {
    // 0.62rem and 0.58rem eyebrows rendered at 9.9px and 9.3px, and card meta
    // at 11.2px. CJK strokes collapse below 12px.
    const sizes = [
      rule('.eyebrow'),
      rule('.vote-card-meta,\n.vote-card-tags'),
    ].map((block) => Number.parseFloat(block.match(/font-size:\s*([\d.]+)rem/)?.[1] ?? '0'))

    for (const size of sizes) {
      expect(size).toBeGreaterThanOrEqual(0.75)
    }
    // Section-level rules must not undercut the base eyebrow again.
    expect(rule('.home-highlight-section .section-heading .eyebrow')).not.toMatch(/font-size/)
    expect(rule('.champion-marquee-heading .eyebrow')).not.toMatch(/font-size/)
  })
})

describe('heading scale', () => {
  it('gives the two rail section headings one shared size', () => {
    // Both are h2. They rendered at 29.6px and 15.2px, which made the heading
    // level carry no meaning and let the rail heading compete with the page h1.
    const rail = rule('.home-highlight-section .section-heading h2')
    const marquee = rule('.champion-marquee-heading h2')

    expect(rail).toContain('font-size: 1.35rem')
    expect(marquee).toContain('font-size: 1.35rem')
  })

  it('lets the browse section carry an h1', () => {
    expect(css).toMatch(/\.public-content-section \.section-heading :is\(h1, h2\)/)
  })
})

describe('vote candidate title', () => {
  it('outweighs the embedded player chrome beside it', () => {
    const title = rule('.game-candidate h2')

    // A YouTube iframe renders its channel name bold white on black at the top
    // of the frame; at the old 1.28rem cap the option being voted on lost.
    const max = Number.parseFloat(title.match(/font-size:\s*clamp\([^,]+,[^,]+,\s*([\d.]+)rem/)?.[1] ?? '0')
    expect(max).toBeGreaterThanOrEqual(1.6)
    expect(title).toContain('font-weight: 800')
  })
})

describe('vote card actions', () => {
  it('promotes opening a vote over viewing the ranking', () => {
    const actions = rule('.vote-card-actions')
    const primary = rule('.vote-card-actions .vote-card-action-primary')

    // Two equal 1fr columns in identical styling made the card ask the reader
    // to choose twice.
    expect(actions).not.toContain('repeat(2, minmax(0, 1fr))')
    expect(primary).toContain('background: var(--accent)')
    expect(primary).not.toContain('!important')
  })
})

describe('mobile touch targets', () => {
  it('grows header controls and carousel dots to a usable size', () => {
    // 31 controls sat under 44px on a 390px viewport; the dots were 8.8px.
    const mobile = css.match(/@media \(max-width: 640px\) \{([\s\S]*?)\n\}/)?.[1] ?? ''

    expect(mobile).toMatch(/\.icon-button::after[\s\S]*?width:\s*max\(100%,\s*44px\)/)
    expect(mobile).toMatch(/\.login-link,|\.login-link::after/)
    expect(mobile).toMatch(/\.highlight-dots button \{[\s\S]*?padding:\s*1rem 0\.85rem/)
  })
})

describe('donate hero', () => {
  it('gives the display heading the width it needs to stop ragging', () => {
    const hero = rule('.donate-hero')
    const heading = rule('.donate-hero h1')

    // At 1.1fr/0.9fr the heading was squeezed into ~578px and broke into four
    // lines with two-character orphans while the column beside it sat half empty.
    expect(hero).toContain('minmax(0, 1.5fr) minmax(0, 0.5fr)')
    expect(heading).toContain('text-wrap: balance')
  })
})

describe('game setup layout', () => {
  it('keeps the setup screen off the full-width game shell', () => {
    const setup = rule('.game-setup')

    // .page-shell.game-page-shell is full width for the playing screen's two
    // video cards. Inheriting that put the columns 128px apart at 1700px with a
    // ~470px hole between the previews and the panel.
    expect(setup).toContain('width: min(100%, 1180px)')
    expect(setup).toContain('margin-inline: auto')
  })

  it('centres the setup content against the shell rather than a short box', () => {
    // A fixed min-height shorter than the viewport centred the content inside
    // that box, pushing everything into the upper half of the screen.
    expect(rule('.game-setup')).toContain('min-height: 100%')
  })

  it('holds the setup title to the site-wide h1 size', () => {
    const heading = rule('.game-setup h1')
    const max = Number.parseFloat(heading.match(/font-size:\s*clamp\([^,]+,[^,]+,\s*([\d.]+)rem/)?.[1] ?? '99')

    // The old 5.8rem cap rendered at 92.8px on a 1700px viewport — bigger than
    // anything else on a screen whose job is picking a size and starting.
    expect(max).toBeLessThanOrEqual(3.5)
  })
})

describe('game setup option previews', () => {
  it('sizes the preview frames from the viewport height so the setup screen needs no scrolling', () => {
    const preview = rule('.game-preview')
    const media = rule('.game-preview-media')

    // A fixed portrait frame (aspect-ratio: 2 / 3) made the two previews taller
    // than the viewport on a laptop, pushing the count options and the start
    // button below the fold. Frame height must stay viewport-bound instead.
    expect(media).not.toMatch(/aspect-ratio\s*:/)
    expect(media).toContain('height: var(--game-preview-height)')
    expect(preview).toMatch(/--game-preview-height:\s*clamp\([^)]*vh[^)]*\)/)
    // Frame width follows frame height, so a wide window grows the artwork
    // rather than the blurred backdrop beside it.
    expect(preview).toMatch(/max-width:\s*min\(100%,\s*calc\(.*var\(--game-preview-height\)/)
  })
})

describe('home vote-card alignment', () => {
  it('reserves equal content space and pins every action row to the bottom', () => {
    const card = rule('.vote-card')
    const main = rule('.vote-card-main')
    const copy = rule('.vote-card-copy')

    expect(card).toContain('display: flex')
    expect(card).toContain('flex-direction: column')
    expect(main).toContain('flex: 1')
    expect(copy).toContain('height: 8.5rem')
  })
})
