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

describe('vote candidate media', () => {
  it('fits the whole option inside the frame without growing the card', () => {
    const frame = rule('\n.game-candidate-media')
    const media = rule('.game-candidate-media img,\n.game-candidate-media video,\n.game-candidate-media iframe')

    // The option being voted on is shown whole; the blurred copy behind it fills
    // whatever the aspect ratio leaves over, as on the setup previews.
    expect(media).toContain('object-fit: contain')
    expect(rule('.game-candidate-backdrop')).toMatch(/filter:\s*blur\(/)
    // While playing the frame is a 1fr row of a card sized to the viewport. In
    // flow, height: 100% resolved against that indefinite row, fell back to auto,
    // and a portrait picture grew the card past the fold, blowing the option up.
    expect(frame).toContain('position: relative')
    expect(media).toContain('position: absolute')
    expect(media).toContain('inset: 0')
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
  it('gives both options one capped portrait frame that fits the picture whole', () => {
    const preview = rule('.game-preview')
    const media = rule('.game-preview-media')
    const picture = rule('.game-preview-media img,\n.game-preview-media video')

    // One frame size for the pair, so a portrait and a landscape option are the
    // same box rather than one filling it and the other rendering 81px wide.
    expect(preview).toMatch(/--game-preview-size:\s*clamp\([^)]*\)/)
    expect(preview).toContain('grid-template-columns: repeat(2, minmax(0, var(--game-preview-size)))')
    // Portrait, because most option artwork is a standing product shot.
    expect(media).toMatch(/aspect-ratio:\s*3\s*\/\s*4/)
    // A taller frame (aspect-ratio: 2 / 3) once pushed the count options and the
    // start button below the fold on a laptop. The clamp ceiling bounds the
    // frame's height too, so 3:4 off a capped width stays above it.
    expect(preview).toMatch(/--game-preview-size:\s*clamp\([^)]*,\s*[\d.]+rem\)/)
    // Never cropped: the picture is fitted inside the frame and the blurred copy
    // behind it fills what is left over. It is positioned against the frame's own
    // aspect-ratio box, because height: 100% on a grid child would resolve to auto
    // and let a tall picture overflow.
    expect(picture).toContain('object-fit: contain')
    expect(picture).toContain('position: absolute')
    expect(picture).toContain('inset: 0')
    expect(rule('.game-preview-backdrop')).toMatch(/filter:\s*blur\(/)
  })
})

describe('my ranking layout', () => {
  it('makes the picture the main object instead of a cropped square', () => {
    const hero = rule('.game-personal-hero .game-personal-media')
    const picture = rule('.game-personal-media img')

    // The player ranked pictures, so the top three get a picture-first card.
    expect(rule('.game-personal-podium')).toContain('repeat(3, minmax(0, 1fr))')
    expect(hero).toMatch(/aspect-ratio:\s*4\s*\/\s*5/)
    // Never cropped, as on the setup previews and in the vote arena: contained
    // against the frame's own box, with a blurred copy filling the rest.
    expect(picture).toContain('object-fit: contain')
    expect(picture).toContain('position: absolute')
    expect(picture).toContain('inset: 0')
    expect(rule('.game-personal-media')).toContain('position: relative')
    expect(rule('.game-personal-backdrop')).toMatch(/filter:\s*blur\(/)
    // Three cards across a 390px viewport would be smaller than the 4rem
    // thumbnail this layout replaced.
    expect(css).toMatch(/@media \(max-width: 640px\)[\s\S]*?\.game-personal-podium \{[\s\S]*?repeat\(2, minmax\(0, 1fr\)\)/)
  })
})

describe('ranking picture zoom', () => {
  it('opens the picture larger than the list frame it was clicked in', () => {
    const zoom = rule('.game-rank-zoom img')

    // Sizing to the picture rendered a zoom smaller than the card whenever the
    // stored source file is smaller than the thumbnail beside it.
    expect(zoom).toContain('width: min(92vw, 60rem)')
    expect(zoom).toContain('height: min(78vh, 42rem)')
    expect(zoom).toContain('object-fit: contain')
  })
})

describe('ranking row thumbnails', () => {
  it('separates the picture from the rank number and gives a video room', () => {
    const row = rule('\n.game-community-row')
    const thumb = rule('\n.game-community-thumb')

    // A 16:9 still inside a 4.5rem square rendered a 26px strip, and the
    // 0.7rem gap read as the number and the picture being one box.
    expect(row).toContain('grid-template-columns: 2rem 7rem minmax(0, 1fr)')
    expect(row).toContain('gap: 1rem')
    expect(thumb).toContain('width: 7rem')
    expect(thumb).toMatch(/aspect-ratio:\s*4\s*\/\s*3/)
    expect(thumb).not.toMatch(/height:/)
  })
})

describe('ranking video player', () => {
  it('keeps one player element that docks bottom left instead of stopping', () => {
    const player = rule('\n.game-rank-player')
    const docked = rule('.game-rank-player.is-docked')

    // Two elements would mean two iframes, and re-mounting the iframe restarts
    // the video — the dock exists so playback survives leaving the big view.
    expect(player).toContain('position: fixed')
    expect(player).toContain('inset: 0')
    expect(docked).toContain('inset: auto auto 1rem 1rem')
    expect(docked).toContain('background: none')
    expect(rule('.game-rank-player.is-docked .game-rank-player-frame')).toContain('width: min(80vw, 22rem)')
    expect(rule('.game-rank-player-frame')).toMatch(/aspect-ratio:\s*16\s*\/\s*9/)
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
