import { describe, expect, it } from 'vitest'

import { resolveThemePreference } from './useTheme'

describe('theme preference', () => {
  it('uses an explicit saved theme before the system preference', () => {
    expect(resolveThemePreference('light', true)).toBe('light')
    expect(resolveThemePreference('dark', false)).toBe('dark')
  })

  it('falls back to the system preference', () => {
    expect(resolveThemePreference(null, true)).toBe('dark')
    expect(resolveThemePreference('unsupported', false)).toBe('light')
  })
})
