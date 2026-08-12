export interface RankTrendPoint {
  rank: number
  win_rate: string
  date: string
}

export interface RankTrendCoordinate extends RankTrendPoint {
  x: number
  y: number
  // True when the real rank sits outside the fixed scale and the point had to be
  // drawn on the boundary. The tooltip still reports the real rank.
  clamped: boolean
}

/**
 * Deliberately wide and shallow. The trend is a supporting detail next to the
 * ranking list, and a 240-tall box spent most of its height on empty scale: with
 * the fixed domain below, a top-five element only ever draws in the upper third.
 */
export const rankTrendViewBox = {
  width: 640,
  height: 150,
  paddingX: 30,
  paddingY: 20,
} as const

/** Horizontal rules at the top, middle and bottom of the plot area. */
export const rankTrendGridlines: readonly number[] = [
  rankTrendViewBox.paddingY,
  rankTrendViewBox.height / 2,
  rankTrendViewBox.height - rankTrendViewBox.paddingY,
]

/**
 * Every chart shares this vertical scale.
 *
 * Scaling each element to its own best/worst rank made the axis mean something
 * different in every card: a line that climbed from #4 to #1 looked identical to
 * one that climbed from #180 to #170. A fixed window is what makes two charts
 * comparable, and it doubles as the cut-off for which ranks get a chart at all.
 */
export const rankTrendDomain = { best: 1, worst: 10 } as const

export function isRankTrendCharted(rank: number | null | undefined): boolean {
  return typeof rank === 'number' && rank >= rankTrendDomain.best && rank <= rankTrendDomain.worst
}

export function rankTrendCoordinates(points: RankTrendPoint[]): RankTrendCoordinate[] {
  if (!points.length) return []

  const chronological = [...points].sort((left, right) => left.date.localeCompare(right.date))
  const { best, worst } = rankTrendDomain
  const rankSpan = worst - best
  const chartWidth = rankTrendViewBox.width - rankTrendViewBox.paddingX * 2
  const chartHeight = rankTrendViewBox.height - rankTrendViewBox.paddingY * 2

  return chronological.map((point, index) => {
    const bounded = Math.min(Math.max(point.rank, best), worst)
    return {
      ...point,
      x: rankTrendViewBox.paddingX
        + (chronological.length === 1 ? chartWidth / 2 : (index / (chronological.length - 1)) * chartWidth),
      // A better (smaller) rank is drawn higher on the chart.
      y: rankTrendViewBox.paddingY + ((bounded - best) / rankSpan) * chartHeight,
      clamped: bounded !== point.rank,
    }
  })
}

export function rankTrendPolyline(points: RankTrendCoordinate[]): string {
  return points.map((point) => `${point.x.toFixed(2)},${point.y.toFixed(2)}`).join(' ')
}

/**
 * A long history packs points a few pixels apart, where a fixed 4.5px marker
 * merged into one thick rope and hid the line it was supposed to annotate.
 */
export function rankTrendPointRadius(count: number): number {
  if (count > 40) return 1.6
  if (count > 18) return 2.4
  return 3.4
}
