import type { LoginSessionPayload } from '../../app/store/session'

export const sessionStorageKey = 'zbt.session'

let loginRedirectInFlight = false

export function getStoredSession() {
  const raw = localStorage.getItem(sessionStorageKey)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<LoginSessionPayload>
    if (!isUsableSessionPayload(parsed)) {
      localStorage.removeItem(sessionStorageKey)
      return null
    }
    return parsed
  } catch {
    localStorage.removeItem(sessionStorageKey)
    return null
  }
}

export function clearStoredSession() {
  localStorage.removeItem(sessionStorageKey)
}

export function expireSessionAndRedirect() {
  clearStoredSession()
  if (typeof window === 'undefined') return
  if (window.location.pathname.startsWith('/login')) return
  if (loginRedirectInFlight) return
  loginRedirectInFlight = true
  const from = encodeURIComponent(window.location.pathname + window.location.search)
  window.location.assign(`/login?session=expired&from=${from}`)
}

export function safeReturnPath(raw: string | null | undefined) {
  if (!raw || !raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/login')) {
    return '/dashboard'
  }
  return raw
}

function isUsableSessionPayload(value: Partial<LoginSessionPayload>): value is LoginSessionPayload {
  const session = value.session
  return (
    isNonEmptyString(value.access_token) &&
    Boolean(session) &&
    isObject(session) &&
    isObject(session.user) &&
    isNonEmptyString(session.user.id) &&
    typeof session.user.email === 'string' &&
    typeof session.user.name === 'string' &&
    isObject(session.tenant) &&
    isNonEmptyString(session.tenant.id) &&
    typeof session.tenant.name === 'string' &&
    isObject(session.role) &&
    isNonEmptyString(session.role.id) &&
    isNonEmptyString(session.role.code) &&
    typeof session.role.name === 'string' &&
    isPermissionRecord(session.permissions)
  )
}

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.trim() !== ''
}

function isPermissionRecord(value: unknown): value is LoginSessionPayload['session']['permissions'] {
  if (!isObject(value)) return false
  return Object.values(value).every((level) => level === 'none' || level === 'read' || level === 'full')
}
