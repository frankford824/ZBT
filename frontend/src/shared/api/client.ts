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

export type KnowledgeCategoryDTO = {
  id: string
  name: string
  description: string
  parent_id: string | null
  created_at: string
  updated_at: string
}

export type KnowledgeTagDTO = {
  id: string
  name: string
  color: string
  created_at: string
  updated_at: string
}

export type KnowledgeDocumentDTO = {
  id: string
  title: string
  doc_type: string
  parse_status: 'ready' | 'queued' | 'processing' | 'processed' | 'failed'
  summary: string
  metadata: Record<string, unknown>
  file: {
    id: string
    filename: string
    content_type: string
    size_bytes: number
    status: string
  }
  category: KnowledgeCategoryDTO | null
  tags: KnowledgeTagDTO[]
  processed_at: string | null
  created_at: string
  updated_at: string
}

export type ConfirmUploadDTO = {
  file: FileAssetDTO
  document?: KnowledgeDocumentDTO
}

export type KnowledgeStatsDTO = {
  document_count: number
  ready_count: number
  queued_count: number
  processed_count: number
  failed_count: number
  category_counts: Record<string, number>
  tag_counts: Record<string, number>
}

export type AITaskDTO = {
  id: string
  task_type: string
  status: 'queued' | 'running' | 'done' | 'failed' | 'cancelled'
  external_task_id: string | null
  resource_type: string
  resource_id: string
  payload: Record<string, unknown>
  route: Record<string, unknown>
  result: Record<string, unknown>
  error_message: string | null
  created_at: string
  updated_at: string
}

export type KnowledgeSourceRefDTO = {
  chunk_id: string
  document_id: string
  title: string
  page_start: number | null
  page_end: number | null
}

export type KnowledgeSearchResultDTO = {
  chunk_id: string
  document_id: string
  document: KnowledgeDocumentDTO
  title: string
  content: string
  section_path: string
  page_start: number | null
  page_end: number | null
  metadata: Record<string, unknown>
  score: number
  source_ref: KnowledgeSourceRefDTO
  created_at: string
}

export type KnowledgeSearchResponseDTO = {
  items: KnowledgeSearchResultDTO[]
  source_refs: KnowledgeSourceRefDTO[]
}

export type BidDocumentDTO = {
  id: string
  project_id: string | null
  project_name: string
  title: string
  bid_type: 'combined' | 'separated' | 'custom'
  status: string
  created_at: string
  updated_at: string
}

export type BidPartDTO = {
  id: string
  bid_document_id: string
  code: 'combined_body' | 'tech' | 'business' | 'boq' | 'attachment'
  title: string
  sort_order: number
  status: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type BidExportDTO = {
  id: string
  bid_document_id: string
  bid_part_id: string | null
  export_type: 'docx' | 'pdf' | 'zip'
  part_code: string
  status: 'queued' | 'running' | 'done' | 'failed' | 'cancelled'
  file_asset_id: string | null
  filename: string
  metadata: Record<string, unknown>
  error_message: string | null
  completed_at: string | null
  created_at: string
  updated_at: string
}

export type CreateBidExportDTO = {
  export: BidExportDTO
  task: AITaskDTO
}

export type BidExportDetailDTO = {
  export: BidExportDTO
  download?: PresignedFileUrlDTO
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

export async function fetchKnowledgeCategories(): Promise<KnowledgeCategoryDTO[]> {
  const { data } = await apiClient.get<{ items: KnowledgeCategoryDTO[] }>('/knowledge/categories')
  return data.items
}

export async function fetchKnowledgeTags(): Promise<KnowledgeTagDTO[]> {
  const { data } = await apiClient.get<{ items: KnowledgeTagDTO[] }>('/knowledge/tags')
  return data.items
}

export async function fetchKnowledgeDocuments(): Promise<KnowledgeDocumentDTO[]> {
  const { data } = await apiClient.get<{ items: KnowledgeDocumentDTO[] }>('/knowledge/documents')
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

export async function confirmFileUpload(fileId: string): Promise<ConfirmUploadDTO> {
  const { data } = await apiClient.post<ConfirmUploadDTO>(`/files/${fileId}/confirm`)
  return data
}

export async function processKnowledgeDocument(documentId: string): Promise<AITaskDTO> {
  const { data } = await apiClient.post<AITaskDTO>(`/knowledge/documents/${documentId}/process`)
  return data
}

export async function fetchKnowledgeStats(): Promise<KnowledgeStatsDTO> {
  const { data } = await apiClient.get<KnowledgeStatsDTO>('/knowledge/stats')
  return data
}

export async function searchKnowledge(payload: {
  query: string
  limit?: number
  doc_type?: string
}): Promise<KnowledgeSearchResponseDTO> {
  const { data } = await apiClient.post<KnowledgeSearchResponseDTO>('/knowledge/search', payload)
  return data
}

export async function fetchBids(): Promise<BidDocumentDTO[]> {
  const { data } = await apiClient.get<{ items: BidDocumentDTO[] }>('/bids')
  return data.items
}

export async function createBid(payload: {
  title: string
  project_name?: string
  bid_type: 'combined' | 'separated' | 'custom'
}): Promise<BidDocumentDTO> {
  const { data } = await apiClient.post<BidDocumentDTO>('/bids', payload)
  return data
}

export async function fetchBid(bidId: string): Promise<BidDocumentDTO> {
  const { data } = await apiClient.get<BidDocumentDTO>(`/bids/${bidId}`)
  return data
}

export async function fetchBidParts(bidId: string): Promise<BidPartDTO[]> {
  const { data } = await apiClient.get<{ items: BidPartDTO[] }>(`/bids/${bidId}/parts`)
  return data.items
}

export async function fetchBidExports(bidId: string): Promise<BidExportDTO[]> {
  const { data } = await apiClient.get<{ items: BidExportDTO[] }>(`/bids/${bidId}/exports`)
  return data.items
}

export async function createBidExport(
  bidId: string,
  payload: { export_type: 'docx' | 'zip'; part_code: string },
): Promise<CreateBidExportDTO> {
  const { data } = await apiClient.post<CreateBidExportDTO>(`/bids/${bidId}/exports`, payload)
  return data
}

export async function fetchBidExport(exportId: string): Promise<BidExportDetailDTO> {
  const { data } = await apiClient.get<BidExportDetailDTO>(`/bid-exports/${exportId}`)
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
