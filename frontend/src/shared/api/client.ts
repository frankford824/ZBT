import axios from 'axios'
import type { LoginSessionPayload, ModulePermission } from '../../app/store/session'

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 15_000,
})

apiClient.interceptors.request.use((config) => {
  const raw = localStorage.getItem('zbt.session')
  if (raw) {
    try {
      const session = JSON.parse(raw) as LoginSessionPayload
      config.headers.set('Authorization', `Bearer ${session.access_token}`)
      config.headers.set('X-Tenant-ID', session.session.tenant.id)
    } catch {
      localStorage.removeItem('zbt.session')
    }
  }
  return config
})

export type RoleDTO = {
  id: string
  code: string
  name: string
  permissions: Record<string, ModulePermission>
}

export type MemberDTO = {
  id: string
  user: {
    id: string
    email: string
    name: string
  }
  status: string
  roles: RoleDTO[]
}

export type NotificationDTO = {
  id: string
  title: string
  body: string
  read_at: string | null
}

export async function login(payload: {
  email: string
  password: string
  tenant_id?: string
}): Promise<LoginSessionPayload> {
  const { data } = await apiClient.post<LoginSessionPayload>('/auth/login', payload)
  return data
}

export async function fetchMembers(): Promise<MemberDTO[]> {
  const { data } = await apiClient.get<{ items: MemberDTO[] }>('/tenant/members')
  return data.items
}

export async function inviteMember(payload: {
  email: string
  name: string
  role_code?: string
}): Promise<MemberDTO> {
  const { data } = await apiClient.post<MemberDTO>('/tenant/members/invite', payload)
  return data
}

export async function fetchRoles(): Promise<RoleDTO[]> {
  const { data } = await apiClient.get<{ items: RoleDTO[] }>('/roles')
  return data.items
}

export async function fetchNotifications(): Promise<NotificationDTO[]> {
  const { data } = await apiClient.get<{ items: NotificationDTO[] }>('/notifications')
  return data.items
}

export type PlatformStats = {
  activeProjects: number
  monthlyBids: number
  compliancePassRate: number
  winRate: number
  pendingTasks: number
  knowledgeDocs: number
}

export async function fetchPlatformStats(): Promise<PlatformStats> {
  return {
    activeProjects: 18,
    monthlyBids: 34,
    compliancePassRate: 92,
    winRate: 38,
    pendingTasks: 11,
    knowledgeDocs: 426,
  }
}
