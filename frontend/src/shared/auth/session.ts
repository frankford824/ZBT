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
  return (
    typeof value.access_token === 'string' &&
    value.access_token.trim() !== '' &&
    typeof value.session?.tenant?.id === 'string' &&
    value.session.tenant.id.trim() !== ''
  )
}
