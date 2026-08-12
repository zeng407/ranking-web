/// <reference types="vite/client" />

interface Window {
  __APP_CONFIG__?: {
    apiBaseUrl?: string
    contactEmail?: string
    realtime?: {
      key?: string
      host?: string
      port?: number | string
      secure?: boolean
      cluster?: string
    }
  }
}
