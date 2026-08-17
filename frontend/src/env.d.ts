/// <reference types="vite/client" />

interface Window {
  adsbygoogle?: unknown[]
  __APP_CONFIG__?: {
    apiBaseUrl?: string
    contactEmail?: string
    ads?: {
      publisherId?: string
      slots?: Record<string, string | undefined>
    }
    realtime?: {
      key?: string
      host?: string
      port?: number | string
      secure?: boolean
      cluster?: string
    }
  }
}
