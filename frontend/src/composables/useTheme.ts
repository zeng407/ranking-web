import { readonly, ref } from 'vue'

export type Theme = 'light' | 'dark'

const STORAGE_KEY = '2pick-theme'
const theme = ref<Theme>('light')
let initialized = false

export function resolveThemePreference(stored: string | null, prefersDark: boolean): Theme {
  if (stored === 'light' || stored === 'dark') return stored
  return prefersDark ? 'dark' : 'light'
}

export function initializeTheme(): void {
  if (initialized || typeof window === 'undefined') return
  initialized = true

  let stored: string | null = null
  try {
    stored = window.localStorage.getItem(STORAGE_KEY)
  } catch {
    // Storage may be disabled. The system preference remains a safe fallback.
  }

  setTheme(resolveThemePreference(stored, window.matchMedia('(prefers-color-scheme: dark)').matches), false)
}

export function setTheme(nextTheme: Theme, persist = true): void {
  theme.value = nextTheme

  if (typeof document !== 'undefined') {
    document.documentElement.dataset.theme = nextTheme
    document.documentElement.style.colorScheme = nextTheme
    document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')
      ?.setAttribute('content', nextTheme === 'dark' ? '#11100f' : '#f7f5f0')
  }

  if (persist && typeof window !== 'undefined') {
    try {
      window.localStorage.setItem(STORAGE_KEY, nextTheme)
    } catch {
      // Theme switching should still work when storage is unavailable.
    }
  }
}

export function useTheme() {
  initializeTheme()

  return {
    theme: readonly(theme),
    toggleTheme: () => setTheme(theme.value === 'dark' ? 'light' : 'dark'),
  }
}
