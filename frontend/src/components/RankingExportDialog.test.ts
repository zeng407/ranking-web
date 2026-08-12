// @vitest-environment happy-dom

import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import RankingExportDialog from './RankingExportDialog.vue'

const exportMocks = vi.hoisted(() => ({
  create: vi.fn(),
  dispose: vi.fn(),
  download: vi.fn(),
}))

vi.mock('../rank/exportRanking', async (importOriginal) => ({
  ...await importOriginal<typeof import('../rank/exportRanking')>(),
  createPersonalRankingExport: exportMocks.create,
  disposePersonalRankingExport: exportMocks.dispose,
  downloadPersonalRankingExport: exportMocks.download,
}))

const items = [
  { rank: 1, title: '冠軍', imageUrl: 'https://example.test/winner.webp' },
  { rank: 2, title: '亞軍', imageUrl: 'https://example.test/runner-up.webp' },
]

function stubMobileViewport(matches: boolean): void {
  vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
    matches,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }))
}

describe('ranking export dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    exportMocks.create.mockResolvedValue({
      imageUrl: 'blob:https://2pick.test/ranking-preview',
      blob: new Blob(['ranking'], { type: 'image/png' }),
      filename: '2pick-test-top10.png',
      text: '#1 冠軍\n#2 亞軍',
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  it('opens an image preview, downloads it on desktop, and copies ranking text', async () => {
    stubMobileViewport(false)
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    const wrapper = mount(RankingExportDialog, {
      props: { open: true, title: '測試投票', items, locale: 'zh_TW' },
    })
    await flushPromises()

    expect(wrapper.get('.ranking-export-preview').attributes('src')).toBe('blob:https://2pick.test/ranking-preview')
    expect((wrapper.get('.ranking-export-text').element as HTMLTextAreaElement).value).toBe('#1 冠軍\n#2 亞軍')
    expect(wrapper.find('.ranking-export-mobile-hint').exists()).toBe(false)

    await wrapper.get('.ranking-export-copy').trigger('click')
    expect(writeText).toHaveBeenCalledWith('#1 冠軍\n#2 亞軍')
    expect(wrapper.get('.ranking-export-copy').text()).toContain('已複製')

    await wrapper.get('.ranking-export-download').trigger('click')
    expect(exportMocks.download).toHaveBeenCalledWith(expect.objectContaining({ filename: '2pick-test-top10.png' }))

    await wrapper.get('.ranking-export-close').trigger('click')
    expect(exportMocks.dispose).toHaveBeenCalledWith(expect.objectContaining({ imageUrl: 'blob:https://2pick.test/ranking-preview' }))
    wrapper.unmount()
  })

  it('replaces the unsupported mobile download action with a save-image instruction', async () => {
    stubMobileViewport(true)
    const wrapper = mount(RankingExportDialog, {
      props: { open: true, title: '測試投票', items, locale: 'zh_TW' },
    })
    await flushPromises()

    expect(wrapper.find('.ranking-export-download').exists()).toBe(false)
    expect(wrapper.get('.ranking-export-mobile-hint').text()).toContain('長按預覽圖片')
    expect(wrapper.get('.ranking-export-preview').attributes('src')).toContain('ranking-preview')
    wrapper.unmount()
  })
})
