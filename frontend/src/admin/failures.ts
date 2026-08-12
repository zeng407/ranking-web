import type { AdminFieldErrors, AdminOutcome } from '../services/admin'

/**
 * Turns a failed outcome into something a moderator can read.
 *
 * The back office is written in zh-TW only, without the locale registry the public pages
 * use: it is an internal tool with one audience, and translating it would mean maintaining
 * every string in three languages for nobody's benefit.
 */
export function describeFailure(outcome: Extract<AdminOutcome<unknown>, { ok: false }>): string {
  switch (outcome.kind) {
    case 'signed-out':
      // The bundle loaded, so the pass cookie is still valid; it is the access token that
      // is gone. Signing in again happens on the public site, so say where to go.
      return '登入狀態已失效，請回前台重新登入。'
    case 'forbidden':
      return '這個帳號沒有管理權限。'
    case 'not-found':
      return '找不到這筆資料，可能已經被刪除。'
    case 'validation':
      return `資料不符合規則：${describeFieldErrors(outcome.errors)}`
    case 'conflict':
      return refusalText[outcome.code] ?? '這個動作在目前狀態下不被允許。'
    case 'unavailable':
      return (outcome.code && refusalText[outcome.code]) || '伺服器暫時無法處理這個請求，請稍後再試。'
  }
}

/** Refusals the API states by code rather than by field. */
const refusalText: Record<string, string> = {
  cannot_ban_administrator: '管理員帳號不能被封鎖。',
  announcements_not_configured: '這台伺服器沒有設定公告儲存位置，公告無法讀寫。',
  admin_not_configured: '這台伺服器沒有啟用後台服務。',
}

/** The API answers with machine codes; these are the ones the admin endpoints send. */
const codeText: Record<string, string> = {
  required: '必填',
  too_long: '太長',
  too_short: '太短',
  invalid: '格式不正確',
  unsupported_image: '不支援的圖片格式',
  too_large: '檔案太大',
  unsupported: '不支援',
  out_of_range: '超出範圍',
}

export function describeFieldErrors(errors: AdminFieldErrors): string {
  const parts: string[] = []
  for (const [field, codes] of Object.entries(errors)) {
    const readable = codes.map((code) => codeText[code] ?? code).join('、')
    parts.push(`${field} ${readable}`)
  }
  return parts.join('；')
}
