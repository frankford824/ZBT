import { create } from 'zustand'
import { clearStoredSession, getStoredSession, storeSession } from '../../shared/auth/session'

export type ModulePermission = 'none' | 'read' | 'full'

export type SessionUser = {
  id: string
  email: string
  name: string
  role: string
}

export type Tenant = {
  id: string
  name: string
}

type SessionState = {
  collapsed: boolean
  isAuthenticated: boolean
  token: string | null
  user: SessionUser
  tenant: Tenant
  permissions: Record<string, ModulePermission>
  toggleCollapsed: () => void
  setSession: (payload: LoginSessionPayload) => void
  setTenant: (tenant: Tenant) => void
  logout: () => void
}

export type LoginSessionPayload = {
  access_token: string
  token_type?: string
  expires_in?: number
  expires_at?: string
  session: {
    user: {
      id: string
      email: string
      name: string
    }
    tenant: Tenant
    role: {
      id: string
      code: string
      name: string
    }
    roles?: {
      id: string
      code: string
      name: string
      permissions?: Record<string, ModulePermission>
    }[]
    permissions: Record<string, ModulePermission>
  }
}

function readStoredSession(): Partial<SessionState> {
  const parsed = getStoredSession()
  return parsed ? toSessionState(parsed) : {}
}

function toSessionState(payload: LoginSessionPayload): Partial<SessionState> {
  return {
    isAuthenticated: true,
    token: payload.access_token,
    user: {
      id: payload.session.user.id,
      email: payload.session.user.email,
      name: payload.session.user.name,
      role: payload.session.role.name,
    },
    tenant: payload.session.tenant,
    permissions: payload.session.permissions ?? {},
  }
}

const storedSession = readStoredSession()

export const useSessionStore = create<SessionState>((set) => ({
  collapsed: localStorage.getItem('zbt.sidebar.collapsed') === 'true',
  isAuthenticated: false,
  token: null,
  user: {
    id: '',
    email: '',
    name: '',
    role: '',
  },
  tenant: {
    id: '',
    name: '',
  },
  permissions: {},
  ...storedSession,
  toggleCollapsed: () =>
    set((state) => {
      const next = !state.collapsed
      localStorage.setItem('zbt.sidebar.collapsed', String(next))
      return { collapsed: next }
    }),
  setSession: (payload) => {
    const stored = storeSession(payload)
    set(toSessionState(stored))
  },
  setTenant: (tenant) => {
    const stored = getStoredSession()
    if (stored) {
      storeSession({ ...stored, session: { ...stored.session, tenant } })
    }
    set({ tenant })
  },
  logout: () => {
    clearStoredSession()
    set({
      isAuthenticated: false,
      token: null,
      user: { id: '', email: '', name: '', role: '' },
      tenant: { id: '', name: '' },
      permissions: {},
    })
  },
}))
