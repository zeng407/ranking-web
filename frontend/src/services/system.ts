import type { APIClient } from '../lib/api'
import { getAPIClient } from '../lib/api'

export interface SystemInfo {
  service: string
  version: string
  commit: string
  environment: string
  time: string
}

export function fetchSystemInfo(
  signal?: AbortSignal,
  client: APIClient = getAPIClient(),
): Promise<SystemInfo> {
  return client.get<SystemInfo>('/system/info', signal)
}
