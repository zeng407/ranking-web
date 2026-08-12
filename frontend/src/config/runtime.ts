export interface RuntimeConfig {
  apiBaseUrl: string
  contactEmail: string
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

const defaultConfig: RuntimeConfig = {
  apiBaseUrl: '/api/v1',
  contactEmail: '',
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

/** Reads a port that an environment file may have written as a string. */
function numberOr(value: number | string | undefined, fallback: number): number {
  const parsed = typeof value === 'string' ? Number.parseInt(value, 10) : value
  return typeof parsed === 'number' && Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function normalizeBaseUrl(value: string): string {
  return value === '/' ? '' : value.replace(/\/+$/, '')
}
