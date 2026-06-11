import { create } from 'zustand'

export type ModulePermission = 'none' | 'read' | 'full'

export type SessionUser = {
  id: string
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
  user: SessionUser
  tenant: Tenant
  permissions: Record<string, ModulePermission>
  toggleCollapsed: () => void
  loginAsDemo: () => void
  logout: () => void
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

export const useSessionStore = create<SessionState>((set) => ({
  collapsed: localStorage.getItem('zbt.sidebar.collapsed') === 'true',
  isAuthenticated: true,
  user: {
    id: 'user-demo',
    name: '陈思远',
    role: '企业管理员',
  },
  tenant: {
    id: 'tenant-demo',
    name: '杭州智建科技有限公司',
  },
  permissions: fullPermissions,
  toggleCollapsed: () =>
    set((state) => {
      const next = !state.collapsed
      localStorage.setItem('zbt.sidebar.collapsed', String(next))
      return { collapsed: next }
    }),
  loginAsDemo: () => set({ isAuthenticated: true }),
  logout: () => set({ isAuthenticated: false }),
}))
