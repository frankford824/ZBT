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
  created_at: string
}

export type ApprovalStepDTO = {
  order: number
  name: string
  role_code: string
  user_id?: string
  required: boolean
  condition: string
}

export type ApprovalChainDTO = {
  id: string
  name: string
  description: string
  resource_type: 'bid'
  steps: ApprovalStepDTO[]
  enabled: boolean
  created_at: string
  updated_at: string
}

export type ApprovalInstanceDTO = {
  id: string
  chain_id: string | null
  bid_document_id: string | null
  bid_title: string
  title: string
  status: 'pending' | 'approved' | 'rejected' | 'cancelled'
  current_step: number
  submitted_by: string | null
  submitted_by_name: string
  snapshot: ApprovalStepDTO[]
  action_count: number
  started_at: string
  completed_at: string | null
  created_at: string
  updated_at: string
}

export type ApprovalActionDTO = {
  id: string
  instance_id: string
  actor_user_id: string | null
  actor_name: string
  action: 'submit' | 'approve' | 'reject' | 'cancel'
  step_order: number
  comment: string
  created_at: string
  updated_at: string
}

export type ApprovalDetailDTO = {
  instance: ApprovalInstanceDTO
  actions: ApprovalActionDTO[]
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

export type TenderDTO = {
  id: string
  source_id: string | null
  source_name: string
  title: string
  purchaser: string
  region: string
  budget_amount: number | null
  budget_text: string
  publish_date: string | null
  deadline: string | null
  status: 'open' | 'closed' | 'awarded' | 'cancelled'
  match_score: number
  summary: string
  requirements: string[]
  risk_flags: string[]
  source_url: string
  metadata: Record<string, unknown>
  favorite: boolean
  created_at: string
  updated_at: string
}

export type TenderSourceDTO = {
  id: string
  name: string
  source_type: string
  url: string
  status: 'active' | 'inactive' | 'failed'
  last_verified_at: string | null
  last_verify_status: string | null
  last_verify_message: string
  config: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type TenderProjectDTO = {
  id: string
  name: string
  status: string
  created_at: string
  updated_at: string
}

export type ProjectDTO = {
  id: string
  name: string
  status: 'opportunity' | 'bidding' | 'compliance_review' | 'submitted' | 'closed'
  result: 'won' | 'lost' | 'pending' | null
  owner_id: string | null
  owner_name: string
  bid_count: number
  milestone_count: number
  created_at: string
  updated_at: string
}

export type ProjectMilestoneDTO = {
  id: string
  project_id: string
  title: string
  status: 'pending' | 'done'
  due_date: string | null
  completed_at: string | null
  sort_order: number
  note: string
  created_at: string
  updated_at: string
}

export type ProjectActivityDTO = {
  id: string
  project_id: string
  actor_user_id: string | null
  actor_name: string
  action: string
  metadata: Record<string, unknown>
  created_at: string
}

export type CostProjectDTO = {
  id: string
  project_id: string
  project_name?: string
  name: string
  status: 'draft' | 'active' | 'closed'
  budget_amount: number | null
  total_budget?: number
  total_actual?: number
  margin_amount?: number
  margin_rate?: number
  item_count?: number
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type CostItemDTO = {
  id: string
  cost_project_id: string
  category: string
  name: string
  cost_type: 'labor' | 'material' | 'equipment' | 'service' | 'other'
  budget_amount: number
  actual_amount: number
  status: 'planned' | 'committed' | 'actual'
  vendor: string
  note: string
  created_at: string
  updated_at: string
}

export type CostAnalysisDTO = {
  project: CostProjectDTO
  category_totals: Array<{
    category: string
    total_budget: number
    total_actual: number
    margin_amount: number
  }>
  overrun_items: CostItemDTO[]
  recommendations: string[]
}

export type CostReportDTO = {
  id: string
  cost_project_id: string
  report_type: string
  status: 'queued' | 'generated' | 'failed'
  summary: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type ComplianceSeverity = 'pass' | 'warn' | 'fail_candidate' | 'fail'
export type ComplianceIssueStatus = 'open' | 'fixed' | 'ignored' | 'confirmed_fail'

export type ComplianceCheckDTO = {
  id: string
  bid_document_id: string | null
  bid_title: string
  name: string
  status: 'queued' | 'running' | 'done' | 'failed'
  result_status: ComplianceSeverity
  score: number
  config: Record<string, unknown>
  task_id: string | null
  issue_count: number
  started_at: string | null
  completed_at: string | null
  created_at: string
  updated_at: string
}

export type ComplianceIssueDTO = {
  id: string
  check_id: string
  rule_id: string | null
  category: string
  severity: ComplianceSeverity
  status: ComplianceIssueStatus
  title: string
  evidence: string
  suggestion: string
  location: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type ComplianceRuleDTO = {
  id: string
  code: string
  name: string
  category: string
  level: 'L1' | 'L2' | 'L3' | 'L4'
  severity: ComplianceSeverity
  description: string
  enabled: boolean
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type ComplianceReportDTO = {
  id: string
  check_id: string
  status: 'queued' | 'generated' | 'failed'
  summary: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type ComplianceSnapshotDTO = {
  check: ComplianceCheckDTO
  issues: ComplianceIssueDTO[]
  updated_at: string
}

export type CreateBidFromTenderDTO = {
  tender: TenderDTO
  bid: BidDocumentDTO
}

export type BidGenerationSnapshotDTO = {
  bid_id: string
  summary: {
    total_chapters: number
    generating_chapters: number
    generated_chapters: number
    accepted_chapters: number
    needs_fix_chapters: number
    queued_tasks: number
    running_tasks: number
    done_tasks: number
    failed_tasks: number
  }
  chapters: Array<{
    id: string
    bid_part_id: string
    title: string
    status: string
    sort_order: number
    source_ref_count: number
    needs_human_input_count: number
    updated_at: string
  }>
  tasks: Array<{
    id: string
    external_task_id: string | null
    chapter_id: string
    chapter_title: string
    status: AITaskDTO['status']
    error_message: string | null
    created_at: string
    updated_at: string
  }>
  generated_at: string
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

export type KnowledgeDocumentReferenceDTO = {
  id: string
  source_document_id: string
  bid_document_id: string | null
  bid_title: string
  chapter_id: string | null
  chapter_title: string
  chunk_id: string | null
  title: string
  metadata: Record<string, unknown>
  created_at: string
}

export type KnowledgeDocumentTemplateDTO = {
  id: string
  name: string
  category: string
  description: string
  version: string
  content: Record<string, unknown>
  usage_count: number
  status: 'active' | 'archived'
  created_at: string
  updated_at: string
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

export type BidTemplateDTO = {
  id: string
  name: string
  bid_type: 'combined' | 'separated' | 'custom'
  category: string
  description: string
  version: string
  content: Record<string, unknown>
  usage_count: number
  status: 'active' | 'archived'
  created_at: string
  updated_at: string
}

export type UseBidTemplateDTO = {
  template: BidTemplateDTO
  bid: BidDocumentDTO
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

export type BidChapterDTO = {
  id: string
  bid_document_id: string
  bid_part_id: string
  title: string
  content: Record<string, unknown>
  plain_text: string
  status: 'pending' | 'generating' | 'generated' | 'accepted' | 'edited' | 'needs_fix'
  sort_order: number
  source_refs: unknown[]
  needs_human_input: string[]
  created_at: string
  updated_at: string
}

export type BidChapterVersionDTO = {
  id: string
  chapter_id: string
  bid_document_id: string
  bid_part_id: string
  version_no: number
  title: string
  content: Record<string, unknown>
  plain_text: string
  status: string
  source_refs: unknown[]
  needs_human_input: string[]
  change_reason: string
  model_metadata: Record<string, unknown>
  token_usage: Record<string, number>
  created_by: string | null
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

export type ChapterRegenerateDTO = {
  chapter: BidChapterDTO
  task: AITaskDTO
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

export async function markNotificationsRead(ids?: string[]): Promise<{ updated: number }> {
  const { data } = await apiClient.post<{ updated: number }>('/notifications/read', { ids: ids ?? [] })
  return data
}

export async function fetchApprovalChains(): Promise<ApprovalChainDTO[]> {
  const { data } = await apiClient.get<{ items: ApprovalChainDTO[] }>('/approval-chains')
  return data.items
}

export async function createApprovalChain(payload: {
  name: string
  description?: string
  resource_type?: 'bid'
  steps: ApprovalStepDTO[]
  enabled: boolean
}): Promise<ApprovalChainDTO> {
  const { data } = await apiClient.post<ApprovalChainDTO>('/approval-chains', {
    resource_type: 'bid',
    ...payload,
  })
  return data
}

export async function updateApprovalChain(
  chainId: string,
  payload: {
    name: string
    description?: string
    resource_type?: 'bid'
    steps: ApprovalStepDTO[]
    enabled: boolean
  },
): Promise<ApprovalChainDTO> {
  const { data } = await apiClient.patch<ApprovalChainDTO>(`/approval-chains/${chainId}`, {
    resource_type: 'bid',
    ...payload,
  })
  return data
}

export async function deleteApprovalChain(chainId: string): Promise<void> {
  await apiClient.delete(`/approval-chains/${chainId}`)
}

export async function submitBidForApproval(bidId: string): Promise<ApprovalDetailDTO> {
  const { data } = await apiClient.post<ApprovalDetailDTO>(`/bids/${bidId}/submit-for-approval`)
  return data
}

export async function fetchApprovals(params?: { status?: string }): Promise<ApprovalInstanceDTO[]> {
  const { data } = await apiClient.get<{ items: ApprovalInstanceDTO[] }>('/approvals', { params })
  return data.items
}

export async function fetchApproval(approvalId: string): Promise<ApprovalDetailDTO> {
  const { data } = await apiClient.get<ApprovalDetailDTO>(`/approvals/${approvalId}`)
  return data
}

export async function approveApproval(approvalId: string, comment?: string): Promise<ApprovalDetailDTO> {
  const { data } = await apiClient.post<ApprovalDetailDTO>(`/approvals/${approvalId}/approve`, { comment })
  return data
}

export async function rejectApproval(approvalId: string, comment?: string): Promise<ApprovalDetailDTO> {
  const { data } = await apiClient.post<ApprovalDetailDTO>(`/approvals/${approvalId}/reject`, { comment })
  return data
}

export async function fetchTenders(params?: {
  q?: string
  region?: string
  status?: string
  source_id?: string
  favorite?: boolean
  recommended?: boolean
}): Promise<TenderDTO[]> {
  const { data } = await apiClient.get<{ items: TenderDTO[] }>('/tenders', { params })
  return data.items
}

export async function fetchTender(tenderId: string): Promise<TenderDTO> {
  const { data } = await apiClient.get<TenderDTO>(`/tenders/${tenderId}`)
  return data
}

export async function createTender(payload: Partial<TenderDTO>): Promise<TenderDTO> {
  const { data } = await apiClient.post<TenderDTO>('/tenders', payload)
  return data
}

export async function favoriteTender(tenderId: string): Promise<TenderDTO> {
  const { data } = await apiClient.post<TenderDTO>(`/tenders/${tenderId}/favorite`)
  return data
}

export async function unfavoriteTender(tenderId: string): Promise<TenderDTO> {
  const { data } = await apiClient.delete<TenderDTO>(`/tenders/${tenderId}/favorite`)
  return data
}

export async function createProjectFromTender(tenderId: string): Promise<TenderProjectDTO> {
  const { data } = await apiClient.post<TenderProjectDTO>(`/tenders/${tenderId}/create-project`)
  return data
}

export async function createBidFromTender(tenderId: string): Promise<CreateBidFromTenderDTO> {
  const { data } = await apiClient.post<CreateBidFromTenderDTO>(`/tenders/${tenderId}/create-bid`)
  return data
}

export async function fetchTenderSources(): Promise<TenderSourceDTO[]> {
  const { data } = await apiClient.get<{ items: TenderSourceDTO[] }>('/tender-sources')
  return data.items
}

export async function createTenderSource(payload: {
  name: string
  source_type: string
  url: string
}): Promise<TenderSourceDTO> {
  const { data } = await apiClient.post<TenderSourceDTO>('/tender-sources', payload)
  return data
}

export async function verifyTenderSource(sourceId: string): Promise<TenderSourceDTO> {
  const { data } = await apiClient.post<TenderSourceDTO>(`/tender-sources/${sourceId}/verify`)
  return data
}

export async function fetchProjects(params?: { status?: string }): Promise<ProjectDTO[]> {
  const { data } = await apiClient.get<{ items: ProjectDTO[] }>('/projects', { params })
  return data.items
}

export async function createProject(payload: {
  name: string
  status?: ProjectDTO['status']
  result?: ProjectDTO['result']
}): Promise<ProjectDTO> {
  const { data } = await apiClient.post<ProjectDTO>('/projects', payload)
  return data
}

export async function fetchProject(projectId: string): Promise<ProjectDTO> {
  const { data } = await apiClient.get<ProjectDTO>(`/projects/${projectId}`)
  return data
}

export async function updateProject(
  projectId: string,
  payload: Partial<Pick<ProjectDTO, 'name' | 'status' | 'result'>>,
): Promise<ProjectDTO> {
  const { data } = await apiClient.patch<ProjectDTO>(`/projects/${projectId}`, payload)
  return data
}

export async function transitionProject(
  projectId: string,
  payload: { status: ProjectDTO['status']; result?: ProjectDTO['result'] },
): Promise<ProjectDTO> {
  const { data } = await apiClient.post<ProjectDTO>(`/projects/${projectId}/transition`, payload)
  return data
}

export async function fetchProjectMilestones(projectId: string): Promise<ProjectMilestoneDTO[]> {
  const { data } = await apiClient.get<{ items: ProjectMilestoneDTO[] }>(`/projects/${projectId}/milestones`)
  return data.items
}

export async function createProjectMilestone(
  projectId: string,
  payload: { title: string; status?: ProjectMilestoneDTO['status']; due_date?: string; sort_order?: number; note?: string },
): Promise<ProjectMilestoneDTO> {
  const { data } = await apiClient.post<ProjectMilestoneDTO>(`/projects/${projectId}/milestones`, payload)
  return data
}

export async function updateProjectMilestone(
  projectId: string,
  milestoneId: string,
  payload: { title: string; status?: ProjectMilestoneDTO['status']; due_date?: string; sort_order?: number; note?: string },
): Promise<ProjectMilestoneDTO> {
  const { data } = await apiClient.patch<ProjectMilestoneDTO>(`/projects/${projectId}/milestones/${milestoneId}`, payload)
  return data
}

export async function deleteProjectMilestone(projectId: string, milestoneId: string): Promise<void> {
  await apiClient.delete(`/projects/${projectId}/milestones/${milestoneId}`)
}

export async function createCostProject(projectId: string): Promise<CostProjectDTO> {
  const { data } = await apiClient.post<CostProjectDTO>(`/projects/${projectId}/create-cost-project`)
  return data
}

export async function fetchProjectActivities(projectId: string): Promise<ProjectActivityDTO[]> {
  const { data } = await apiClient.get<{ items: ProjectActivityDTO[] }>(`/projects/${projectId}/activities`)
  return data.items
}

export async function fetchCostProjects(): Promise<CostProjectDTO[]> {
  const { data } = await apiClient.get<{ items: CostProjectDTO[] }>('/cost-projects')
  return data.items
}

export async function createCostProjectRecord(payload: {
  project_id: string
  name?: string
  status?: CostProjectDTO['status']
  budget_amount?: number
}): Promise<CostProjectDTO> {
  const { data } = await apiClient.post<CostProjectDTO>('/cost-projects', payload)
  return data
}

export async function fetchCostProject(costProjectId: string): Promise<CostProjectDTO> {
  const { data } = await apiClient.get<CostProjectDTO>(`/cost-projects/${costProjectId}`)
  return data
}

export async function updateCostProject(
  costProjectId: string,
  payload: Partial<Pick<CostProjectDTO, 'name' | 'status' | 'budget_amount'>>,
): Promise<CostProjectDTO> {
  const { data } = await apiClient.patch<CostProjectDTO>(`/cost-projects/${costProjectId}`, payload)
  return data
}

export async function fetchCostItems(costProjectId: string): Promise<CostItemDTO[]> {
  const { data } = await apiClient.get<{ items: CostItemDTO[] }>(`/cost-projects/${costProjectId}/items`)
  return data.items
}

export async function createCostItem(
  costProjectId: string,
  payload: Partial<CostItemDTO> & { name: string },
): Promise<CostItemDTO> {
  const { data } = await apiClient.post<CostItemDTO>(`/cost-projects/${costProjectId}/items`, payload)
  return data
}

export async function updateCostItem(
  costItemId: string,
  payload: Partial<CostItemDTO> & { name: string },
): Promise<CostItemDTO> {
  const { data } = await apiClient.patch<CostItemDTO>(`/cost-items/${costItemId}`, payload)
  return data
}

export async function deleteCostItem(costItemId: string): Promise<void> {
  await apiClient.delete(`/cost-items/${costItemId}`)
}

export async function fetchCostAnalysis(costProjectId: string): Promise<CostAnalysisDTO> {
  const { data } = await apiClient.get<CostAnalysisDTO>(`/cost-projects/${costProjectId}/analysis`)
  return data
}

export async function createCostAdvice(costProjectId: string): Promise<CostAnalysisDTO> {
  const { data } = await apiClient.post<CostAnalysisDTO>(`/cost-projects/${costProjectId}/ai-advice`)
  return data
}

export async function createCostReport(costProjectId: string): Promise<CostReportDTO> {
  const { data } = await apiClient.post<CostReportDTO>(`/cost-projects/${costProjectId}/report`)
  return data
}

export async function fetchComplianceChecks(): Promise<ComplianceCheckDTO[]> {
  const { data } = await apiClient.get<{ items: ComplianceCheckDTO[] }>('/compliance/checks')
  return data.items
}

export async function createComplianceCheck(payload: {
  name?: string
  bid_document_id?: string
  levels?: string[]
}): Promise<ComplianceSnapshotDTO> {
  const { data } = await apiClient.post<ComplianceSnapshotDTO>('/compliance/checks', payload)
  return data
}

export async function fetchComplianceCheck(checkId: string): Promise<ComplianceCheckDTO> {
  const { data } = await apiClient.get<ComplianceCheckDTO>(`/compliance/checks/${checkId}`)
  return data
}

export async function fetchComplianceIssues(checkId: string): Promise<ComplianceIssueDTO[]> {
  const { data } = await apiClient.get<{ items: ComplianceIssueDTO[] }>(`/compliance/checks/${checkId}/issues`)
  return data.items
}

export async function autofixComplianceIssue(issueId: string): Promise<ComplianceIssueDTO> {
  const { data } = await apiClient.post<ComplianceIssueDTO>(`/compliance/issues/${issueId}/autofix`)
  return data
}

export async function ignoreComplianceIssue(issueId: string): Promise<ComplianceIssueDTO> {
  const { data } = await apiClient.post<ComplianceIssueDTO>(`/compliance/issues/${issueId}/ignore`)
  return data
}

export async function confirmFailComplianceIssue(issueId: string): Promise<ComplianceIssueDTO> {
  const { data } = await apiClient.post<ComplianceIssueDTO>(`/compliance/issues/${issueId}/confirm-fail`)
  return data
}

export async function createComplianceReport(checkId: string): Promise<ComplianceReportDTO> {
  const { data } = await apiClient.post<ComplianceReportDTO>(`/compliance/checks/${checkId}/report`)
  return data
}

export async function fetchComplianceRules(): Promise<ComplianceRuleDTO[]> {
  const { data } = await apiClient.get<{ items: ComplianceRuleDTO[] }>('/compliance/rules')
  return data.items
}

export async function createComplianceRule(payload: {
  code: string
  name: string
  category: string
  level: ComplianceRuleDTO['level']
  severity: ComplianceSeverity
  description?: string
  enabled?: boolean
  metadata?: Record<string, unknown>
}): Promise<ComplianceRuleDTO> {
  const { data } = await apiClient.post<ComplianceRuleDTO>('/compliance/rules', payload)
  return data
}

export async function updateComplianceRule(
  ruleId: string,
  payload: {
    code: string
    name: string
    category: string
    level: ComplianceRuleDTO['level']
    severity: ComplianceSeverity
    description?: string
    enabled?: boolean
    metadata?: Record<string, unknown>
  },
): Promise<ComplianceRuleDTO> {
  const { data } = await apiClient.patch<ComplianceRuleDTO>(`/compliance/rules/${ruleId}`, payload)
  return data
}

export async function deleteComplianceRule(ruleId: string): Promise<void> {
  await apiClient.delete(`/compliance/rules/${ruleId}`)
}

export async function fetchKnowledgeCategories(): Promise<KnowledgeCategoryDTO[]> {
  const { data } = await apiClient.get<{ items: KnowledgeCategoryDTO[] }>('/knowledge/categories')
  return data.items
}

export async function createKnowledgeCategory(payload: {
  name: string
  description?: string
}): Promise<KnowledgeCategoryDTO> {
  const { data } = await apiClient.post<KnowledgeCategoryDTO>('/knowledge/categories', payload)
  return data
}

export async function updateKnowledgeCategory(
  categoryId: string,
  payload: { name?: string; description?: string },
): Promise<KnowledgeCategoryDTO> {
  const { data } = await apiClient.patch<KnowledgeCategoryDTO>(`/knowledge/categories/${categoryId}`, payload)
  return data
}

export async function deleteKnowledgeCategory(categoryId: string): Promise<void> {
  await apiClient.delete(`/knowledge/categories/${categoryId}`)
}

export async function fetchKnowledgeTags(): Promise<KnowledgeTagDTO[]> {
  const { data } = await apiClient.get<{ items: KnowledgeTagDTO[] }>('/knowledge/tags')
  return data.items
}

export async function createKnowledgeTag(payload: {
  name: string
  color?: string
}): Promise<KnowledgeTagDTO> {
  const { data } = await apiClient.post<KnowledgeTagDTO>('/knowledge/tags', payload)
  return data
}

export async function updateKnowledgeTag(
  tagId: string,
  payload: { name?: string; color?: string },
): Promise<KnowledgeTagDTO> {
  const { data } = await apiClient.patch<KnowledgeTagDTO>(`/knowledge/tags/${tagId}`, payload)
  return data
}

export async function deleteKnowledgeTag(tagId: string): Promise<void> {
  await apiClient.delete(`/knowledge/tags/${tagId}`)
}

export async function fetchKnowledgeDocuments(): Promise<KnowledgeDocumentDTO[]> {
  const { data } = await apiClient.get<{ items: KnowledgeDocumentDTO[] }>('/knowledge/documents')
  return data.items
}

export async function updateKnowledgeDocument(
  documentId: string,
  payload: {
    title?: string
    doc_type?: string
    category_id?: string | null
    tag_ids?: string[]
    summary?: string
  },
): Promise<KnowledgeDocumentDTO> {
  const { data } = await apiClient.patch<KnowledgeDocumentDTO>(`/knowledge/documents/${documentId}`, payload)
  return data
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

export async function fetchAITask(taskId: string): Promise<AITaskDTO> {
  const { data } = await apiClient.get<AITaskDTO>(`/ai-tasks/${taskId}`)
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

export async function fetchKnowledgeDocumentReferences(
  documentId: string,
): Promise<KnowledgeDocumentReferenceDTO[]> {
  const { data } = await apiClient.get<{ items: KnowledgeDocumentReferenceDTO[] }>(
    `/knowledge/documents/${documentId}/references`,
  )
  return data.items
}

export async function fetchKnowledgeTemplates(): Promise<KnowledgeDocumentTemplateDTO[]> {
  const { data } = await apiClient.get<{ items: KnowledgeDocumentTemplateDTO[] }>('/knowledge/templates')
  return data.items
}

export async function createKnowledgeTemplate(payload: {
  name: string
  category?: string
  description?: string
  version?: string
  content?: Record<string, unknown>
}): Promise<KnowledgeDocumentTemplateDTO> {
  const { data } = await apiClient.post<KnowledgeDocumentTemplateDTO>('/knowledge/templates', payload)
  return data
}

export async function fetchBids(): Promise<BidDocumentDTO[]> {
  const { data } = await apiClient.get<{ items: BidDocumentDTO[] }>('/bids')
  return data.items
}

export async function fetchBidTemplates(): Promise<BidTemplateDTO[]> {
  const { data } = await apiClient.get<{ items: BidTemplateDTO[] }>('/bid-templates')
  return data.items
}

export async function useBidTemplate(payload: {
  templateId: string
  title?: string
  project_name?: string
}): Promise<UseBidTemplateDTO> {
  const { data } = await apiClient.post<UseBidTemplateDTO>(`/bid-templates/${payload.templateId}/use`, {
    title: payload.title,
    project_name: payload.project_name,
  })
  return data
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

export async function fetchBidChapters(bidId: string): Promise<BidChapterDTO[]> {
  const { data } = await apiClient.get<{ items: BidChapterDTO[] }>(`/bids/${bidId}/chapters`)
  return data.items
}

export async function updateChapterContent(
  chapterId: string,
  payload: { title?: string; content?: Record<string, unknown>; plain_text?: string },
): Promise<BidChapterVersionDTO> {
  const { data } = await apiClient.put<BidChapterVersionDTO>(`/chapters/${chapterId}/content`, payload)
  return data
}

export async function acceptChapter(chapterId: string): Promise<BidChapterVersionDTO> {
  const { data } = await apiClient.post<BidChapterVersionDTO>(`/chapters/${chapterId}/accept`)
  return data
}

export async function regenerateChapter(chapterId: string): Promise<ChapterRegenerateDTO> {
  const { data } = await apiClient.post<ChapterRegenerateDTO>(`/chapters/${chapterId}/regenerate`)
  return data
}

export async function fetchChapterVersions(chapterId: string): Promise<BidChapterVersionDTO[]> {
  const { data } = await apiClient.get<{ items: BidChapterVersionDTO[] }>(`/chapters/${chapterId}/versions`)
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
