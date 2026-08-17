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
    // Full-width brackets are legal in a filename; the separators and the
    // Windows-reserved characters around them are not.
    expect(rankingExportFilename('【7-17更新】1024首完整版OP\\ED|主題曲?'))
      .toBe('2pick-【7-17更新】1024首完整版OP-ED-主題曲-top10.png')
  })

  it('keeps a long title inside the filesystem byte limit', () => {
    // Name limits are 255 bytes, and a Chinese character is three of them.
    const name = rankingExportFilename('動'.repeat(200))

    expect(new TextEncoder().encode(name).length).toBeLessThanOrEqual(255)
    expect(name.startsWith('2pick-動動')).toBe(true)
    expect(name.endsWith('-top10.png')).toBe(true)
  })

  it('never cuts a surrogate pair in half', () => {
    // 150 bytes of budget lands mid-emoji at four bytes each without a guard,
    // and a lone surrogate in a download name is not a valid filename.
    const name = rankingExportFilename('🎵'.repeat(60))

    // Iterating by code point pairs the surrogates up, so anything still in the
    // surrogate range on its own is a half character.
    const halves = [...name].filter((character) => {
      const code = character.codePointAt(0) ?? 0
      return code >= 0xd800 && code <= 0xdfff
    })

    expect(halves).toHaveLength(0)
  })

  it('falls back to a name when the title is all reserved characters', () => {
    expect(rankingExportFilename('///???')).toBe('2pick-ranking-top10.png')
  })

  it('creates the same copyable numbered text shown beside the preview', () => {
    expect(rankingExportText([
      { rank: 1, title: '冠軍', imageUrl: null },
      { rank: 2, title: '亞軍', imageUrl: null },
    ])).toBe('#1 冠軍\n#2 亞軍')
  })
})
