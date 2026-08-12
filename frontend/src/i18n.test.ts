import { describe, expect, it } from 'vitest'

import {
  htmlLanguage,
  localeCodePattern,
  localeDefinitions,
  localeLabel,
  localePrefix,
  localePrefixPattern,
  localizedPath,
  normalizeLocale,
  pathWithoutLocale,
  supportedLocales,
  translate,
} from './i18n'

describe('locale helpers', () => {
  it('normalizes unsupported locales to Traditional Chinese', () => {
    expect(normalizeLocale('en')).toBe('en')
    expect(normalizeLocale('zh-tw')).toBe('zh_TW')
    expect(normalizeLocale('fr')).toBe('zh_TW')
    expect(normalizeLocale(undefined)).toBe('zh_TW')
  })

  it('builds the existing localized URL contract', () => {
    expect(localizedPath('/', 'ja')).toBe('/ja/')
    expect(localizedPath('/privacy', 'en')).toBe('/en/privacy')
    expect(localizedPath('/donate', 'zh_TW')).toBe('/zh-tw/donate')
  })

  it('provides language metadata and translated labels', () => {
    expect(htmlLanguage('zh_TW')).toBe('zh-Hant')
    expect(translate('ja', 'donate')).toBe('サポート')
  })
})

describe('locale registry', () => {
  it('derives the switcher options, prefixes and route patterns from one registry', () => {
    expect(supportedLocales).toEqual(localeDefinitions.map((definition) => definition.code))
    expect(localePrefixPattern).toBe(localeDefinitions.map((definition) => definition.prefix).join('|'))
    expect(localeCodePattern).toBe(localeDefinitions.map((definition) => definition.code).join('|'))

    for (const definition of localeDefinitions) {
      expect(localePrefix(definition.code)).toBe(definition.prefix)
      expect(localeLabel(definition.code)).toBe(definition.label)
      expect(htmlLanguage(definition.code)).toBe(definition.htmlLang)
    }
  })

  it('keeps the established locale codes and public URL prefixes', () => {
    // Translation files are keyed by code and existing URLs use the prefixes,
    // so neither list may drift.
    expect(localeDefinitions.map((definition) => definition.code)).toEqual(['zh_TW', 'en', 'ja'])
    expect(localeDefinitions.map((definition) => definition.prefix)).toEqual(['zh-tw', 'en', 'ja'])
  })

  it('gives every locale a switcher label', () => {
    for (const definition of localeDefinitions) {
      expect(definition.label.trim()).not.toBe('')
    }
  })

  it('strips only the locale segment from a path', () => {
    expect(pathWithoutLocale('/zh-tw/donate')).toBe('/donate')
    expect(pathWithoutLocale('/en/g/abc')).toBe('/g/abc')
    expect(pathWithoutLocale('/ja')).toBe('/')
    expect(pathWithoutLocale('/g/abc')).toBe('/g/abc')
  })
})
