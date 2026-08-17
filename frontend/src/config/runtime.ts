/**
 * The ad slots the pages know about. Each name is one AdSense unit, so a deployment
 * can turn a single position off by leaving its id out.
 */
export type AdSlotName =
  | 'homeTop'
  | 'homeRail'
  | 'homeRailBottom'
  | 'homeFeed'
  | 'homeFooter'
  | 'rankList'
  | 'gameResult'

export const adSlotNames: readonly AdSlotName[] = [
  'homeTop', 'homeRail', 'homeRailBottom', 'homeFeed', 'homeFooter', 'rankList', 'gameResult',
]

export interface AdsConfig {
  /**
   * The AdSense publisher id. Empty disables every slot, which is what keeps local
   * builds, tests and previews ad-free without a second code path.
   */
  publisherId: string
  slots: Record<AdSlotName, string>
}

export interface RuntimeConfig {
  apiBaseUrl: string
  contactEmail: string
  ads: AdsConfig
  /**
   * The realtime endpoint, for the live game room leaderboard.
   *
   * Empty when unset, which disables the websocket and leaves the room on its poll. That
   * is a deliberate degradation rather than an error: a deployment without Soketi should
   * still be playable.
   */
  realtime: RealtimeConfig
}

export interface RealtimeConfig {
  key: string
  host: string
  port: number
  secure: boolean
  cluster: string
}

const emptyAds: AdsConfig = {
  publisherId: '',
  slots: {
    homeTop: '', homeRail: '', homeRailBottom: '', homeFeed: '', homeFooter: '',
    rankList: '', gameResult: '',
  },
}

const defaultConfig: RuntimeConfig = {
  apiBaseUrl: '/api/v1',
  contactEmail: '',
  ads: emptyAds,
  realtime: { key: '', host: '', port: 6001, secure: false, cluster: '' },
}

export function getRuntimeConfig(): RuntimeConfig {
  if (typeof window === 'undefined') {
    return defaultConfig
  }

  const configuredBaseUrl = window.__APP_CONFIG__?.apiBaseUrl?.trim()
  const realtime = window.__APP_CONFIG__?.realtime
  return {
    apiBaseUrl: normalizeBaseUrl(configuredBaseUrl || defaultConfig.apiBaseUrl),
    contactEmail: window.__APP_CONFIG__?.contactEmail?.trim() || defaultConfig.contactEmail,
    ads: readAds(window.__APP_CONFIG__?.ads),
    realtime: {
      key: realtime?.key?.trim() || '',
      // Defaults to the page's own host, which is right whenever the websocket is
      // proxied alongside the API — the arrangement the migration plan calls for.
      host: realtime?.host?.trim() || window.location.hostname,
      port: numberOr(realtime?.port, defaultConfig.realtime.port),
      // Follows the page when unset: a wss page cannot open a ws socket, and hard-coding
      // either one breaks the other environment.
      secure: realtime?.secure ?? window.location.protocol === 'https:',
      cluster: realtime?.cluster?.trim() || '',
    },
  }
}

/**
 * Reads the ad configuration, dropping every slot when there is no publisher id.
 *
 * A slot id without a publisher is not a half-working ad, it is an `ins` element the
 * tag can never fill, so the whole block is treated as absent.
 */
function readAds(ads: NonNullable<Window['__APP_CONFIG__']>['ads']): AdsConfig {
  const publisherId = ads?.publisherId?.trim() || ''
  if (!publisherId) return emptyAds

  const slots = { ...emptyAds.slots }
  for (const name of adSlotNames) {
    slots[name] = ads?.slots?.[name]?.trim() || ''
  }
  return { publisherId, slots }
}

/** Reads a port that an environment file may have written as a string. */
function numberOr(value: number | string | undefined, fallback: number): number {
  const parsed = typeof value === 'string' ? Number.parseInt(value, 10) : value
  return typeof parsed === 'number' && Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function normalizeBaseUrl(value: string): string {
  return value === '/' ? '' : value.replace(/\/+$/, '')
}
