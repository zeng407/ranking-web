// @vitest-environment happy-dom

import { describe, expect, it } from 'vitest'

import { rankingExportFilename, rankingExportLayout, rankingExportText } from './exportRanking'

describe('personal ranking image export', () => {
  it('limits the image to ten ranked entries and creates stable grid slots', () => {
    const slots = rankingExportLayout(12)

    expect(slots).toHaveLength(10)
    expect(slots[0]).toMatchObject({ rank: 1, column: 0, row: 0, columnSpan: 3 })
    expect(slots[9]).toMatchObject({ rank: 10, row: 3 })
  })

  it('uses a filesystem-safe PNG filename', () => {
    expect(rankingExportFilename('飲料 / 1v1：排行')).toBe('2pick-飲料-1v1-排行-top10.png')
  })

  it('creates the same copyable numbered text shown beside the preview', () => {
    expect(rankingExportText([
      { rank: 1, title: '冠軍', imageUrl: null },
      { rank: 2, title: '亞軍', imageUrl: null },
    ])).toBe('#1 冠軍\n#2 亞軍')
  })
})
