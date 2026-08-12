export function getAnonymousID(): string {
  const key = '2pick:anonymous-id'
  const current = localStorage.getItem(key)
  if (current) return current
  const value = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
  localStorage.setItem(key, value)
  return value
}
