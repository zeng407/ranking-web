import { describe, expect, it } from 'vitest'

import {
  isRankTrendCharted,
  rankTrendCoordinates,
  rankTrendDomain,
  rankTrendGridlines,
  rankTrendPointRadius,
  rankTrendPolyline,
  rankTrendViewBox,
} from './rankTrend'

// Derived, not hardcoded: the chart geometry is deliberately tuned and these
// assertions are about the mapping, not about a particular box size.
const plotTop = rankTrendViewBox.paddingY
const plotBottom = rankTrendViewBox.height - rankTrendViewBox.paddingY

describe('rank trend chart', () => {
  it('orders points chronologically and draws a better rank higher', () => {
    const points = rankTrendCoordinates([
      { rank: 9, win_rate: '48.0', date: '2026-07-30' },
      { rank: 2, win_rate: '72.0', date: '2026-07-01' },
      { rank: 5, win_rate: '61.0', date: '2026-07-15' },
    ])

    expect(points.map((point) => point.date)).toEqual(['2026-07-01', '2026-07-15', '2026-07-30'])
    expect(points[0]!.y).toBeLessThan(points[1]!.y)
    expect(points[1]!.y).toBeLessThan(points[2]!.y)
    expect(rankTrendPolyline(points)).toContain(',')
  })

  it('centres a single history point', () => {
    const [point] = rankTrendCoordinates([{ rank: 1, win_rate: '100.0', date: '2026-08-01' }])

    expect(point!.x).toBe(rankTrendViewBox.width / 2)
    expect(point!.y).toBe(plotTop)
  })
})

describe('shared vertical scale', () => {
  it('puts a given rank at the same height regardless of the rest of the series', () => {
    // Scaling to each series' own best/worst made every card's axis mean
    // something different, so two charts could not be compared at a glance.
    const tightSeries = rankTrendCoordinates([
      { rank: 3, win_rate: '80.0', date: '2026-07-01' },
      { rank: 4, win_rate: '79.0', date: '2026-07-02' },
    ])
    const wideSeries = rankTrendCoordinates([
      { rank: 3, win_rate: '80.0', date: '2026-07-01' },
      { rank: 10, win_rate: '40.0', date: '2026-07-02' },
    ])

    expect(tightSeries[0]!.y).toBe(wideSeries[0]!.y)
  })

  it('pins the domain ends to the top and bottom of the plot area', () => {
    const [best] = rankTrendCoordinates([{ rank: rankTrendDomain.best, win_rate: '99.0', date: '2026-07-01' }])
    const [worst] = rankTrendCoordinates([{ rank: rankTrendDomain.worst, win_rate: '10.0', date: '2026-07-01' }])

    expect(best!.y).toBe(plotTop)
    expect(worst!.y).toBe(plotBottom)
  })

  it('holds an out-of-scale rank on the boundary and flags it', () => {
    const points = rankTrendCoordinates([
      { rank: 4, win_rate: '70.0', date: '2026-07-01' },
      { rank: 180, win_rate: '9.0', date: '2026-07-02' },
    ])

    // The real rank stays intact for the tooltip; only the plotted position is
    // bounded, and the flag lets the marker be drawn differently.
    expect(points[1]!.rank).toBe(180)
    expect(points[1]!.y).toBe(plotBottom)
    expect(points[1]!.clamped).toBe(true)
    expect(points[0]!.clamped).toBe(false)
  })
})

describe('chart eligibility', () => {
  it('charts only the ranks the shared scale covers', () => {
    expect(isRankTrendCharted(1)).toBe(true)
    expect(isRankTrendCharted(rankTrendDomain.worst)).toBe(true)
    expect(isRankTrendCharted(rankTrendDomain.worst + 1)).toBe(false)
    expect(isRankTrendCharted(0)).toBe(false)
    expect(isRankTrendCharted(null)).toBe(false)
    expect(isRankTrendCharted(undefined)).toBe(false)
  })
})

describe('marker size', () => {
  it('shrinks the marker as the history gets denser', () => {
    // 78 points a few pixels apart at the old fixed 4.5px radius merged into one
    // thick rope and hid the line they annotate.
    const short = rankTrendPointRadius(6)
    const medium = rankTrendPointRadius(25)
    const long = rankTrendPointRadius(80)

    expect(short).toBeGreaterThan(medium)
    expect(medium).toBeGreaterThan(long)
    expect(long).toBeLessThan(2)
  })
})

describe('chart geometry', () => {
  it('keeps the gridlines on the plot area the points are mapped into', () => {
    // The template draws these rules and the axis labels from the same module, so
    // a geometry change cannot leave them describing a different box.
    expect(rankTrendGridlines[0]).toBe(plotTop)
    expect(rankTrendGridlines.at(-1)).toBe(plotBottom)
    expect(rankTrendGridlines).toHaveLength(3)
  })

  it('stays a wide, shallow box beside the ranking list', () => {
    expect(rankTrendViewBox.width / rankTrendViewBox.height).toBeGreaterThan(3.5)
  })
})
