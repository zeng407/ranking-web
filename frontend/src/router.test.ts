// @vitest-environment happy-dom

import { beforeEach, describe, expect, it } from 'vitest'

import router from './router'

/**
 * Links minted by the Blade pages are still in circulation, so the routes they name have
 * to keep resolving here. The export page in particular no longer exists as a page: its
 * image generator now runs from the result screen, so its URL is a way in to that screen.
 */
describe('legacy export links', () => {
  beforeEach(async () => {
    await router.replace('/')
    await router.isReady()
  })

  it('sends the plain export URL to the ranking page with its query intact', async () => {
    await router.push('/r/post-1/export?g=game-1')
    expect(router.currentRoute.value.path).toBe('/r/post-1')
    expect(router.currentRoute.value.query).toEqual({ g: 'game-1' })
  })

  it('keeps the locale of a localized export URL and its legacy s parameter', async () => {
    await router.push('/en/r/post-1/export?s=game-1')
    expect(router.currentRoute.value.path).toBe('/en/r/post-1')
    expect(router.currentRoute.value.query).toEqual({ s: 'game-1' })
  })
})

/**
 * The reset links Laravel mailed carry no locale, and the ones already in inboxes have an
 * hour of life left after the cutover. A dead link there means a user who cannot get back
 * into their account, so both shapes have to resolve.
 */
describe('password reset routes', () => {
  beforeEach(async () => {
    await router.replace('/')
    await router.isReady()
  })

  it('resolves a localized reset link with its token', async () => {
    await router.push('/ja/password/reset/the-mailed-token')
    expect(router.currentRoute.value.name).toBe('password-reset-localized')
    expect(router.currentRoute.value.params.token).toBe('the-mailed-token')
    expect(router.currentRoute.value.params.locale).toBe('ja')
  })

  it('sends an unprefixed Laravel-shaped reset link to the default locale', async () => {
    await router.push('/password/reset/the-mailed-token')
    expect(router.currentRoute.value.path).toBe('/zh-tw/password/reset/the-mailed-token')
    expect(router.currentRoute.value.params.token).toBe('the-mailed-token')
  })

  it('resolves the forgot form in both shapes', async () => {
    await router.push('/en/password/forgot')
    expect(router.currentRoute.value.name).toBe('password-forgot-localized')

    await router.push('/password/forgot')
    expect(router.currentRoute.value.path).toBe('/zh-tw/password/forgot')
  })
})
