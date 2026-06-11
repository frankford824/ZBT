import { create } from 'zustand'

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
  logout: () => void
}

export type LoginSessionPayload = {
  access_token: string
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
    permissions: Record<string, ModulePermission>
  }
}

const fullPermissions = {
  dashboard: 'full',
  tender: 'full',
  bid: 'full',
  compliance: 'full',
  project: 'full',
  cost: 'full',
  knowledge: 'full',
  team: 'full',
} satisfies Record<string, ModulePermission>

const storageKey = 'zbt.session'

function readStoredSession(): Partial<SessionState> {
  const raw = localStorage.getItem(storageKey)
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw) as LoginSessionPayload
    return toSessionState(parsed)
  } catch {
    localStorage.removeItem(storageKey)
    return {}
  }
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
    permissions: payload.session.permissions,
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
  permissions: fullPermissions,
  ...storedSession,
  toggleCollapsed: () =>
    set((state) => {
      const next = !state.collapsed
      localStorage.setItem('zbt.sidebar.collapsed', String(next))
      return { collapsed: next }
    }),
  setSession: (payload) => {
    localStorage.setItem(storageKey, JSON.stringify(payload))
    set(toSessionState(payload))
  },
  logout: () => {
    localStorage.removeItem(storageKey)
    set({
      isAuthenticated: false,
      token: null,
      user: { id: '', email: '', name: '', role: '' },
      tenant: { id: '', name: '' },
      permissions: {},
    })
  },
}))
