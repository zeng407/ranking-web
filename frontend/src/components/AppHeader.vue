<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import {
  htmlLanguage,
  localeDefinitions,
  localeLabel,
  localizedPath,
  normalizeLocale,
  pathWithoutLocale,
  translate,
  type Locale,
} from '../i18n'
import { useAuth } from '../composables/useAuth'
import { useTheme } from '../composables/useTheme'

const route = useRoute()
const menuOpen = ref(false)
const localeMenuOpen = ref(false)
const accountMenuOpen = ref(false)
const headerElement = ref<HTMLElement | null>(null)
const locale = computed(() => normalizeLocale(route.params.locale))
const { theme, toggleTheme } = useTheme()
const { authenticated, user, loading, isAdmin, refreshAuthState, enterAdminConsole, signOut } = useAuth()
const adminEntryFailed = ref(false)

const canonicalPath = computed(() => pathWithoutLocale(route.path))

// Paths that exist under every locale, so switching language can stay put.
// Anything else falls back to the localized home page.
const localePreservedPaths = new Set(['/hot', '/new', '/donate', '/tos', '/privacy', '/login'])

function localeTarget(nextLocale: Locale): string {
  const path = /^\/(?:g|r)\/[^/]+$/.test(canonicalPath.value)
    || localePreservedPaths.has(canonicalPath.value)
    ? canonicalPath.value
    : '/'
  return localizedPath(path, nextLocale)
}

const loginTarget = computed(() => localizedPath('/login', locale.value))
const avatarUrl = computed(() => user.value?.avatar_url ?? '')

function closeDropdowns(): void {
  localeMenuOpen.value = false
  accountMenuOpen.value = false
}

function toggleLocaleMenu(): void {
  accountMenuOpen.value = false
  localeMenuOpen.value = !localeMenuOpen.value
}

function toggleAccountMenu(): void {
  localeMenuOpen.value = false
  accountMenuOpen.value = !accountMenuOpen.value
}

/**
 * Leaves for the back office, which is a separate bundle rather than a route: the pass
 * cookie is minted first, and the navigation is a full page load.
 */
async function handleAdminConsole(): Promise<void> {
  adminEntryFailed.value = !(await enterAdminConsole())
  if (!adminEntryFailed.value) closeDropdowns()
}

async function handleLogout(): Promise<void> {
  closeDropdowns()
  menuOpen.value = false
  await signOut()
}

function onPointerDown(event: Event): void {
  const target = event.target as Node | null
  if (target && headerElement.value?.contains(target)) return
  closeDropdowns()
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') closeDropdowns()
}

onMounted(() => {
  refreshAuthState(locale.value)
  document.addEventListener('pointerdown', onPointerDown)
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onPointerDown)
  document.removeEventListener('keydown', onKeydown)
})

watch(() => route.fullPath, () => {
  menuOpen.value = false
  closeDropdowns()
  document.documentElement.lang = htmlLanguage(locale.value)
}, { immediate: true })
</script>

<template>
  <header ref="headerElement" class="site-header">
    <div class="header-inner">
      <RouterLink class="brand" :to="localizedPath('/', locale)" aria-label="2Pick 首頁">
        <span class="brand-mark" aria-hidden="true"><img src="/brand-mark.svg" alt="" width="42" height="42"></span>
        <span>2Pick</span>
      </RouterLink>

      <nav class="desktop-nav" :aria-label="translate(locale, 'menu')">
        <RouterLink :to="localizedPath('/', locale)">{{ translate(locale, 'home') }}</RouterLink>
        <RouterLink :to="localizedPath('/hot', locale)">{{ translate(locale, 'popular') }}</RouterLink>
        <RouterLink :to="localizedPath('/new', locale)">{{ translate(locale, 'latest') }}</RouterLink>
        <RouterLink :to="localizedPath('/donate', locale)">{{ translate(locale, 'donate') }}</RouterLink>
      </nav>

      <div class="header-actions">
        <!-- Options come from the locale registry in i18n.ts, so a new
             language needs no change here. -->
        <div class="header-dropdown locale-dropdown">
          <button
            class="dropdown-toggle"
            type="button"
            aria-haspopup="menu"
            :aria-expanded="localeMenuOpen"
            :aria-label="translate(locale, 'language')"
            @click="toggleLocaleMenu"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <circle cx="12" cy="12" r="9" />
              <path d="M3 12h18M12 3c2.5 2.6 2.5 15.4 0 18M12 3c-2.5 2.6-2.5 15.4 0 18" />
            </svg>
            <span class="dropdown-toggle-label">{{ localeLabel(locale) }}</span>
            <svg class="dropdown-caret" viewBox="0 0 24 24" aria-hidden="true"><path d="m6 9 6 6 6-6" /></svg>
          </button>
          <div v-if="localeMenuOpen" class="dropdown-panel" role="menu">
            <RouterLink
              v-for="definition in localeDefinitions"
              :key="definition.code"
              class="dropdown-item"
              role="menuitem"
              :to="localeTarget(definition.code)"
              :class="{ active: locale === definition.code }"
              :lang="definition.htmlLang"
              :aria-current="locale === definition.code ? 'true' : undefined"
            >{{ definition.label }}</RouterLink>
          </div>
        </div>

        <button
          class="icon-button theme-toggle"
          type="button"
          :aria-label="translate(locale, theme === 'dark' ? 'lightMode' : 'darkMode')"
          @click="toggleTheme"
        >
          <svg v-if="theme === 'dark'" viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.42-1.41M17.66 6.34l1.41-1.41" />
          </svg>
          <svg v-else viewBox="0 0 24 24" aria-hidden="true">
            <path d="M20.6 15.7A9 9 0 0 1 8.3 3.4 9 9 0 1 0 20.6 15.7Z" />
          </svg>
        </button>

        <!-- Auth state is resolved client-side because the shell HTML is
             publicly cached; the placeholder avoids flashing a Login link at a
             signed-in user. -->
        <span v-if="loading" class="auth-placeholder" aria-hidden="true"></span>

        <div v-else-if="authenticated" class="header-dropdown account-dropdown">
          <button
            class="icon-button account-toggle"
            type="button"
            aria-haspopup="menu"
            :aria-expanded="accountMenuOpen"
            :aria-label="translate(locale, 'accountMenu')"
            @click="toggleAccountMenu"
          >
            <img
              v-if="avatarUrl"
              class="account-avatar"
              :src="avatarUrl"
              :alt="translate(locale, 'avatarAlt')"
              width="28"
              height="28"
            >
            <svg v-else viewBox="0 0 24 24" aria-hidden="true">
              <circle cx="12" cy="8.5" r="3.5" />
              <path d="M5 20a7 7 0 0 1 14 0" />
            </svg>
          </button>
          <div v-if="accountMenuOpen" class="dropdown-panel dropdown-panel-end" role="menu">
            <RouterLink
              class="dropdown-item"
              role="menuitem"
              :to="localizedPath('/account/posts', locale)"
              @click="accountMenuOpen = false"
            >
              {{ translate(locale, 'myPosts') }}
            </RouterLink>
            <RouterLink
              class="dropdown-item"
              role="menuitem"
              :to="localizedPath('/account', locale)"
              @click="accountMenuOpen = false"
            >
              {{ translate(locale, 'accountTitle') }}
            </RouterLink>
            <button
              v-if="isAdmin"
              class="dropdown-item"
              type="button"
              role="menuitem"
              @click="handleAdminConsole"
            >
              {{ translate(locale, 'adminConsole') }}
            </button>
            <button class="dropdown-item" type="button" role="menuitem" @click="handleLogout">
              {{ translate(locale, 'logout') }}
            </button>
            <p v-if="adminEntryFailed" class="dropdown-item" role="none">
              {{ translate(locale, 'adminConsoleUnavailable') }}
            </p>
          </div>
        </div>

        <RouterLink v-else class="login-link" :to="loginTarget">{{ translate(locale, 'login') }}</RouterLink>

        <button
          class="icon-button menu-toggle"
          type="button"
          :aria-expanded="menuOpen"
          :aria-label="translate(locale, menuOpen ? 'closeMenu' : 'menu')"
          aria-controls="mobile-navigation"
          @click="menuOpen = !menuOpen"
        >
          <svg v-if="!menuOpen" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
          <svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" /></svg>
        </button>
      </div>
    </div>

    <Transition name="menu">
      <nav v-if="menuOpen" id="mobile-navigation" class="mobile-nav" :aria-label="translate(locale, 'menu')">
        <RouterLink :to="localizedPath('/', locale)">{{ translate(locale, 'home') }}</RouterLink>
        <RouterLink :to="localizedPath('/hot', locale)">{{ translate(locale, 'popular') }}</RouterLink>
        <RouterLink :to="localizedPath('/new', locale)">{{ translate(locale, 'latest') }}</RouterLink>
        <RouterLink :to="localizedPath('/donate', locale)">{{ translate(locale, 'donate') }}</RouterLink>

        <RouterLink v-if="!loading && !authenticated" class="mobile-auth" :to="loginTarget">
          {{ translate(locale, 'login') }}
        </RouterLink>
        <button v-else-if="!loading" class="mobile-auth" type="button" @click="handleLogout">
          {{ translate(locale, 'logout') }}
        </button>

        <div class="mobile-locales" :aria-label="translate(locale, 'language')">
          <RouterLink
            v-for="definition in localeDefinitions"
            :key="definition.code"
            :to="localeTarget(definition.code)"
            :class="{ active: locale === definition.code }"
            :lang="definition.htmlLang"
            :aria-current="locale === definition.code ? 'true' : undefined"
          >{{ definition.label }}</RouterLink>
        </div>
      </nav>
    </Transition>
  </header>
</template>
