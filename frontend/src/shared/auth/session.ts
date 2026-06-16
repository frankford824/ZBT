import type { LoginSessionPayload } from '../../app/store/session'

export const sessionStorageKey = 'zbt.session'
export const sessionRefreshLeewayMs = 2 * 60 * 1000

let loginRedirectInFlight = false

export function getStoredSession() {
  const raw = localStorage.getItem(sessionStorageKey)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<LoginSessionPayload>
    if (!isUsableSessionPayload(parsed) || isSessionExpired(parsed)) {
      localStorage.removeItem(sessionStorageKey)
      return null
    }
    const normalized = normalizeSession(parsed)
    if (normalized.expires_at !== parsed.expires_at) {
      localStorage.setItem(sessionStorageKey, JSON.stringify(normalized))
    }
    return normalized
  } catch {
    localStorage.removeItem(sessionStorageKey)
    return null
  }
}

export function storeSession(payload: LoginSessionPayload) {
  const normalized = normalizeSession(payload)
  localStorage.setItem(sessionStorageKey, JSON.stringify(normalized))
  return normalized
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

export function shouldRefreshSession(payload: LoginSessionPayload, now = Date.now()) {
  const expiresAt = sessionExpiresAtMs(payload, now)
  return Boolean(expiresAt && expiresAt > now && expiresAt - now <= sessionRefreshLeewayMs)
}

export function isSessionExpired(payload: LoginSessionPayload, now = Date.now()) {
  const expiresAt = sessionExpiresAtMs(payload, now)
  return Boolean(expiresAt && expiresAt <= now)
}

function normalizeSession(payload: LoginSessionPayload, now = Date.now()): LoginSessionPayload {
  const expiresAt = sessionExpiresAtMs(payload, now)
  if (!expiresAt) return payload
  return { ...payload, expires_at: new Date(expiresAt).toISOString() }
}

function sessionExpiresAtMs(payload: Partial<LoginSessionPayload>, now = Date.now()) {
  const explicit = parseDateMs(payload.expires_at)
  if (explicit) return explicit
  if (typeof payload.expires_in === 'number' && Number.isFinite(payload.expires_in) && payload.expires_in > 0) {
    return now + payload.expires_in * 1000
  }
  return jwtExpiresAtMs(payload.access_token)
}

function parseDateMs(value: unknown) {
  if (typeof value !== 'string' || !value.trim()) return 0
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function jwtExpiresAtMs(token: unknown) {
  if (typeof token !== 'string') return 0
  const payload = token.split('.')[1]
  if (!payload) return 0
  try {
    const decoded = JSON.parse(globalThis.atob(base64UrlToBase64(payload))) as { exp?: unknown }
    return typeof decoded.exp === 'number' && Number.isFinite(decoded.exp) ? decoded.exp * 1000 : 0
  } catch {
    return 0
  }
}

function base64UrlToBase64(value: string) {
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/')
  const padding = base64.length % 4
  return padding ? base64 + '='.repeat(4 - padding) : base64
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
