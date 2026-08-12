import privacyEnSource from './privacy.en.blade.html?raw'
import privacyZhSource from './privacy.zh_TW.blade.html?raw'
import tosEnSource from './tos.en.blade.html?raw'
import tosZhSource from './tos.zh_TW.blade.html?raw'

import type { Locale } from '../i18n'

export type LegalDocument = 'privacy' | 'tos'

const sources: Record<LegalDocument, Record<'en' | 'zh_TW', string>> = {
  privacy: { en: privacyEnSource, zh_TW: privacyZhSource },
  tos: { en: tosEnSource, zh_TW: tosZhSource },
}

export function legalDocumentHTML(
  document: LegalDocument,
  locale: Locale,
  contactEmail = '',
): string {
  const contentLocale = locale === 'zh_TW' ? 'zh_TW' : 'en'
  const source = sources[document][contentLocale]
  const section = source.match(/@section\('content'\)([\s\S]*?)@endsection/)

  const extracted = section?.[1]
  if (!extracted) throw new Error(`Unable to extract ${document}.${contentLocale}`)

  const feedbackLabel = locale === 'zh_TW' ? '意見回饋表' : 'feedback form'
  const contact = contactEmail
    ? `<a href="mailto:${escapeHTML(contactEmail)}">${escapeHTML(contactEmail)}</a>`
    : `<a href="https://forms.gle/DfCfZGUjFncHJdN66" target="_blank" rel="noopener noreferrer">${feedbackLabel}</a>`

  return extracted
    .replace(/^\s*@include\([^\n]+\)\s*$/gm, '')
    .replace(/\{\{\s*config\('app\.name'\)\s*\}\}/g, '2Pick')
    .replace(/\{\{\s*config\('app\.contact_email'\)\s*\}\}/g, contact)
    .replace(/target="_blank"(?!\s+rel=)/g, 'target="_blank" rel="noopener noreferrer"')
    .trim()
}

function escapeHTML(value: string): string {
  return value.replace(/[&<>'"]/g, (character) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    "'": '&#39;',
    '"': '&quot;',
  })[character] ?? character)
}
