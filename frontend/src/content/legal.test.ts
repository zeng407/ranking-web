import { describe, expect, it } from 'vitest'

import { legalDocumentHTML } from './legal'

describe('legal document snapshots', () => {
  it('extracts the Traditional Chinese terms without Blade syntax', () => {
    const html = legalDocumentHTML('tos', 'zh_TW')

    expect(html).toContain('服務條款')
    expect(html).toContain('隱私權保護政策')
    expect(html).not.toContain('@section')
    expect(html).not.toContain('{{')
  })

  it('uses English as the existing Japanese fallback', () => {
    expect(legalDocumentHTML('privacy', 'ja')).toContain('Privacy Policy')
  })

  it('escapes runtime contact email before injecting it', () => {
    const html = legalDocumentHTML('privacy', 'en', '\"><script>alert(1)</script>')

    expect(html).not.toContain('<script>alert(1)</script>')
    expect(html).toContain('&lt;script&gt;')
  })
})
