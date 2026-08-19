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
      rule('.auth-field-hint,\n.form-hint'),
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

  it('keeps the carousel dots painted inside their padded hit box', () => {
    // The phone rules give each dot a 38x43px hit box and clip its paint to the
    // content box. The background shorthand resets background-clip, so the active dot
    // filled the whole hit box as a 53x43px block instead of staying a dot.
    expect(rule('.highlight-dots button')).toContain('background-color: var(--border-strong)')
    expect(rule('.highlight-dots button.active')).toContain('background-color: var(--accent)')
    expect(css).not.toMatch(/\.highlight-dots button[^{]*\{[^}]*background:\s/)
  })
})

describe('game heading on a phone', () => {
  it('drops the round controls to their own row instead of overflowing the screen', () => {
    // Two @media (max-width: 640px) blocks carry phone rules; the game heading is in
    // the second one.
    const mobile = [...css.matchAll(/@media \(max-width: 640px\) \{([\s\S]*?)\n\}/g)]
      .map((match) => match[1])
      .join('\n')

    // Two pills at 111px and 56px plus five 38px buttons come to 398px against a
    // 336px shell on a 360px phone: the page scrolled sideways and the title, the
    // only item able to shrink, was squeezed to 0px wide.
    expect(rule('.game-controls')).toContain('gap: 0.45rem')
    expect(mobile).toMatch(/\.game-heading \{[\s\S]*?flex-wrap:\s*wrap/)
    expect(mobile).toMatch(/\.game-stats \{[\s\S]*?display:\s*contents/)
    expect(mobile).toMatch(/\.game-controls \{[\s\S]*?flex:\s*1 1 100%/)
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
    expect(preview).toContain('grid-template-columns: repeat(2, minmax(0, 1fr))')
    // The size is a cap rather than a fixed column width, so a phone can lift it.
    expect(preview).toContain('max-width: calc(var(--game-preview-size) * 2 + var(--game-preview-gap))')
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

  it('gives the pair the whole row on a phone', () => {
    // The frame size bottoms out at 9rem: two 144px frames in a 336px shell left the
    // pair 38px short of the right edge on a 360px phone, reading as pushed aside.
    const mobile = [...css.matchAll(/@media \(max-width: 640px\) \{([\s\S]*?)\n\}/g)]
      .map((match) => match[1])
      .join('\n')

    expect(mobile).toMatch(/\.game-preview \{[\s\S]*?max-width:\s*none/)
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
    // The picture was also 9px off the row's own top and bottom borders.
    expect(row).toContain('padding: 0.9rem 1rem')
    expect(thumb).toContain('width: 7rem')
    expect(thumb).toMatch(/aspect-ratio:\s*4\s*\/\s*3/)
    expect(thumb).not.toMatch(/height:/)
  })

  it('gives the selected rank video the same room as a picture', () => {
    const video = rule('\n.game-rank-video')

    // A 7.5rem strip read as a cropped band glued to the card above it.
    expect(video).toMatch(/aspect-ratio:\s*16\s*\/\s*9/)
    expect(video).toContain('margin: 0.9rem 0 1rem')
    expect(video).not.toMatch(/height:\s*7\.5rem/)
    // One box before and after the click, so playing does not resize the card.
    expect(css).not.toContain('.game-rank-video.is-playing')
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

describe('ad slots', () => {
  it('reserves each slot height before the unit arrives', () => {
    // An ad that grows after it loads pushes away whatever the reader was
    // looking at, so every shape carries a floor.
    expect(rule('.ad-slot-horizontal')).toContain('min-height: 7.5rem')
    expect(rule('.ad-slot-rectangle')).toContain('min-height: 17rem')
    expect(rule('.ad-slot-card')).toContain('min-height: 20rem')
    expect(rule('.ad-slot ')).toContain('border: 1px solid var(--border)')
  })

  it('keeps the tall unit to the wide layout', () => {
    // Stacked into one column a 600px unit is a wall between reader and votes.
    expect(rule('.ad-slot-vertical')).toContain('display: none')
    const wide = css.slice(css.indexOf('@media (min-width: 921px)'))
    expect(wide).toContain('.ad-slot-vertical')
    expect(wide.slice(0, wide.indexOf('}', wide.indexOf('.ad-slot-vertical')))).toContain('min-height: 38rem')
  })
})

/*
The blur on adult thumbnails is a rule the page cannot express any other way: the markup
just adds `is-censored`, so if the declaration stops reaching the media the preview goes
out uncensored with nothing failing.

The game page's preview renders a video element for a video option, which is why both tags
are covered — and why the rule that positions them must not reintroduce `filter`, since it
is written later in the file and would win the tie.
*/
describe('adult thumbnails', () => {
  /*
   * Both rules below are written for two tags at once, and `rule` only finds a selector
   * that opens its own block, so it reports nothing for the first name in a list.
   */
  function listedRule(selector: string): string {
    const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const block = css.match(new RegExp(`[,\\n]\\s*${escaped}\\s*(?:,[^{}]*)?\\{([\\s\\S]*?)\\}`))
    return block?.[1] ?? ''
  }

  it('blurs both images and videos', () => {
    for (const selector of ['.is-censored img', '.is-censored video']) {
      expect(listedRule(selector), `${selector} does not blur`).toContain('blur(')
    }
  })

  it('does not let the game preview undo the blur', () => {
    const positioning = listedRule('.game-preview-media img')
    expect(positioning, 'the game preview rule was renamed').toContain('object-fit')
    expect(positioning).not.toContain('filter')
  })
})

describe('status colours', () => {
  it('defines the danger and success tokens the account pages ask for', () => {
    // Both were used with a hardcoded fallback and never defined, so the account,
    // my-posts and editor pages painted #c0392b whatever the theme.
    expect(rule(':root')).toMatch(/--danger:\s*#[0-9a-f]{6}/i)
    expect(rule(':root')).toMatch(/--success:\s*#[0-9a-f]{6}/i)
  })

  it('keeps them readable on the dark surface', () => {
    const surface = darkVariable('--surface')

    expect(contrast(darkVariable('--danger'), surface)).toBeGreaterThan(4.5)
    expect(contrast(darkVariable('--success'), surface)).toBeGreaterThan(4.5)
  })
})

describe('form controls', () => {
  it('draws the account pages with the same field and buttons as the login page', () => {
    // The account and my-posts pages shipped browser-default inputs and buttons; the
    // shapes below are the login page's, shared rather than copied.
    expect(css).toMatch(/\.auth-field input,\n\.form-field input,\n\.form-field textarea \{[\s\S]*?border-radius: 0\.7rem/)
    expect(css).toMatch(/\.auth-submit,\n\.form-submit \{[\s\S]*?background: var\(--accent\)/)
    expect(css).toMatch(/\.auth-google,\n\.form-button \{[\s\S]*?border-radius: 999px/)
  })

  it('keeps the quiet button on a 44px touch target', () => {
    // 0.75rem of padding around a 0.85rem line: the mobile sweep found these buttons
    // at 24px tall.
    expect(css).toMatch(/\.auth-google,\n\.form-button \{[\s\S]*?padding: 0\.75rem 1rem/)
  })
})
