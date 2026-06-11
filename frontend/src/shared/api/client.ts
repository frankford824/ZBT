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

export type FileAssetDTO = {
  id: string
  biz_type: string
  biz_id: string | null
  filename: string
  content_type: string
  size_bytes: number
  status: 'pending' | 'ready' | 'failed' | 'deleted'
  created_at: string
  updated_at: string
}

export type PresignUploadDTO = {
  file: FileAssetDTO
  upload_url: string
  method: 'PUT'
  headers: Record<string, string>
  expires_at: string
}

export type PresignedFileUrlDTO = {
  file: FileAssetDTO
  url: string
  expires_at: string
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

export async function fetchKnowledgeDocuments(): Promise<FileAssetDTO[]> {
  const { data } = await apiClient.get<{ items: FileAssetDTO[] }>('/knowledge/documents')
  return data.items
}

export async function createPresignedUpload(payload: {
  filename: string
  content_type: string
  size_bytes: number
  biz_type?: string
  biz_id?: string
}): Promise<PresignUploadDTO> {
  const { data } = await apiClient.post<PresignUploadDTO>('/files/presign-upload', {
    biz_type: 'knowledge',
    ...payload,
  })
  return data
}

export async function uploadToPresignedUrl(
  upload: PresignUploadDTO,
  file: File,
): Promise<void> {
  await axios.put(upload.upload_url, file, {
    headers: upload.headers,
  })
}

export async function confirmFileUpload(fileId: string): Promise<FileAssetDTO> {
  const { data } = await apiClient.post<FileAssetDTO>(`/files/${fileId}/confirm`)
  return data
}

export async function fetchFileURL(
  fileId: string,
  mode: 'download' | 'preview',
): Promise<PresignedFileUrlDTO> {
  const endpoint = mode === 'preview' ? 'preview-url' : 'download-url'
  const { data } = await apiClient.get<PresignedFileUrlDTO>(`/files/${fileId}/${endpoint}`)
  return data
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
