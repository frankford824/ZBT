import axios, { AxiosHeaders, isAxiosError } from 'axios'
import type { LoginSessionPayload, ModulePermission, Tenant } from '../../app/store/session'
import { expireSessionAndRedirect, getStoredSession, shouldRefreshSession, storeSession } from '../auth/session'

const apiBaseURL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

export const apiClient = axios.create({
  baseURL: apiBaseURL,
  timeout: 15_000,
})

const authRefreshClient = axios.create({
  baseURL: apiBaseURL,
  timeout: 15_000,
})

let refreshInFlight: Promise<LoginSessionPayload> | null = null

class StaleSessionRefreshError extends Error {
  constructor() {
    super('session refresh result is stale')
    this.name = 'StaleSessionRefreshError'
  }
}

export function isStaleSessionRefreshError(error: unknown) {
  return error instanceof StaleSessionRefreshError
}

apiClient.interceptors.request.use(async (config) => {
  let session = getStoredSession()
  if (session && shouldRefreshSession(session) && !isAuthSessionRequest(config.url)) {
    try {
      session = await refreshStoredSession(session)
    } catch (error) {
      if (!isStaleSessionRefreshError(error)) {
        expireSessionAndRedirect()
        throw new Error('登录状态已过期，请重新登录', { cause: error })
      }
      throw error
    }
  }
  if (session) {
    const headers = AxiosHeaders.from(config.headers)
    headers.set('Authorization', `Bearer ${session.access_token}`)
    headers.set('X-Tenant-ID', session.session.tenant.id)
    config.headers = headers
  }
  return config
})

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error?.response?.status
    const url: string = error?.config?.url ?? ''
    const isAuthRequest = url.includes('/auth/login') || url.includes('/auth/register')
    if (status === 401 && !isAuthRequest) {
      expireSessionAndRedirect()
    }
    return Promise.reject(error)
  },
)

function refreshStoredSession(session: LoginSessionPayload | null = getStoredSession()) {
  if (!session) {
    return Promise.reject(new StaleSessionRefreshError())
  }
  if (!refreshInFlight) {
    const refreshingToken = session.access_token
    refreshInFlight = authRefreshClient
      .post<LoginSessionPayload>(
        '/auth/refresh',
        undefined,
        {
          headers: {
            Authorization: `Bearer ${session.access_token}`,
            'X-Tenant-ID': session.session.tenant.id,
          },
        },
      )
      .then((response) => {
        const currentSession = getStoredSession()
        if (!currentSession || currentSession.access_token !== refreshingToken) {
          throw new StaleSessionRefreshError()
        }
        return storeSession(response.data)
      })
      .finally(() => {
        refreshInFlight = null
      })
  }
  return refreshInFlight
}

function isAuthSessionRequest(url: string | undefined) {
  return Boolean(url && /^\/?auth\/(login|register|refresh|logout)\b/.test(url))
}

type ApiErrorBody = {
  error?: unknown
  message?: unknown
}

export function getApiErrorMessage(error: unknown, fallback = '操作失败'): string {
  if (!isAxiosError(error)) {
    return fallback
  }

  if (!error.response) {
    if (error.code === 'ECONNABORTED') {
      return '请求超时，请稍后重试'
    }
    return '网络连接异常，请检查连接后重试'
  }

  const data = error.response.data as ApiErrorBody | string | undefined
  if (typeof data === 'string' && data.trim()) {
    return userFacingMessageOrFallback(data, fallback)
  }
  if (data && typeof data === 'object') {
    if (typeof data.error === 'string' && data.error.trim()) {
      return userFacingMessageOrFallback(data.error, fallback)
    }
    if (typeof data.message === 'string' && data.message.trim()) {
      return userFacingMessageOrFallback(data.message, fallback)
    }
  }
  return fallback
}

export function getUserFacingErrorMessage(message: string | null | undefined, fallback = '操作失败'): string {
  return userFacingMessageOrFallback(message ?? '', fallback)
}

function userFacingMessageOrFallback(message: string, fallback: string): string {
  const text = message.trim()
  if (!text) {
    return fallback
  }
  return /[\u4e00-\u9fff]/.test(text) ? text : fallback
}

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
  status: 'active' | 'invited' | 'disabled'
  roles: RoleDTO[]
}

export type NotificationDTO = {
  id: string
  title: string
  body: string
  read_at: string | null
  created_at: string
}

export type ExternalToolProviderPresetDTO = {
  provider_key: string
  name: string
  category: string
  description: string
  transport: 'streamable_http'
  endpoint_hint: string
  token_env: string
  requires_token: boolean
  read_only: boolean
  strict_allowed_tools: boolean
  default_allowed_tools: string[]
  recommended_use: string[]
  data_boundary: string[]
  source_url: string
}

export type ExternalToolConfigDTO = {
  id: string
  provider_key: string
  name: string
  transport: 'streamable_http'
  endpoint: string
  command: string
  enabled: boolean
  allowed_tools: string[]
  timeout_ms: number
  monthly_budget: number
  redaction_policy: 'summary_only' | 'no_sensitive' | 'disabled'
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type ExternalToolAuditLogDTO = {
  id: string
  user_id: string | null
  config_id: string | null
  tool_provider: string
  tool_name: string
  request_hash: string
  request_summary: string
  response_summary: string
  latency_ms: number
  status: 'success' | 'failed' | 'blocked'
  error_message: string
  estimated_cost: number
  resource_type: string
  resource_id: string | null
  metadata: Record<string, unknown>
  created_at: string
}

export type ExternalToolInvokeResultDTO = {
  provider_key: string
  tool_name: string
  status: 'success' | 'failed' | 'blocked'
  result: unknown
  audit: ExternalToolAuditLogDTO
  metadata: Record<string, unknown>
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
  error_message: string | null
  file: {
    id: string
    filename: string
    content_type: string
    size_bytes: number
    status: FileAssetDTO['status']
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

export type AICallLogDTO = {
  id: string
  user_id: string | null
  user_name: string
  trace_id: string
  task_type: string
  provider: string
  model: string
  input_tokens: number
  output_tokens: number
  estimated_cost: number
  latency_ms: number
  status: AITaskDTO['status']
  error_message: string | null
  fallback_from: string | null
  biz_ref: Record<string, unknown>
  created_at: string
}

export type AIModelPricingRateDTO = {
  input_per_1k?: number
  output_per_1k?: number
  input_per_1m?: number
  output_per_1m?: number
  currency?: string
  display_name?: string
  last_reviewed?: string
}

export type AIConfigDTO = {
  id: string
  enabled: boolean
  llm_provider: string
  llm_model: string
  embedding_provider: string
  embedding_model: string
  rerank_provider: string
  rerank_model: string
  ocr_provider: string
  ocr_endpoint: string
  monthly_budget: number
  pricing: Record<string, AIModelPricingRateDTO>
  mock_fallback_allowed: boolean
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type AIConfigPayload = {
  enabled: boolean
  llm_provider: string
  llm_model: string
  embedding_provider: string
  embedding_model: string
  rerank_provider: string
  rerank_model: string
  ocr_provider: string
  ocr_endpoint?: string
  monthly_budget?: number
  pricing?: Record<string, AIModelPricingRateDTO>
  mock_fallback_allowed?: boolean
  metadata?: Record<string, unknown>
}

export type AIProviderOptionDTO = {
  provider_key: string
  name: string
  category: string
  capabilities: string[]
  secret_keys: string[]
}

export type AIOCRProviderOptionDTO = {
  provider_key: string
  name: string
  endpoint_key: string
}

export type AIRouteOptionDTO = {
  task_type: string
  name: string
  capability: string
  track: string
}

export type AIRuntimeModelSetDTO = {
  llm_provider: string
  llm_model: string
  embedding_provider: string
  embedding_model: string
  rerank_provider: string
  rerank_model: string
  ocr_provider: string
  ocr_endpoint: string
}

export type AIRuntimeRouteDTO = {
  task_type: string
  name: string
  capability: string
  track: string
  active_provider: string
  active_model: string
  saved_provider: string
  saved_model: string
}

export type AISecretStatusDTO = {
  key: string
  name: string
  provider: string
  configured: boolean
}

export type AIRuntimeStatusDTO = {
  service_reachable: boolean
  service_status: string
  provider_health: Record<string, boolean>
  active: AIRuntimeModelSetDTO
  routes: AIRuntimeRouteDTO[]
  secrets: AISecretStatusDTO[]
  runtime_pricing_keys: string[]
  mock_fallback_allowed: boolean
  mock_providers_enabled: boolean
  saved_config_enabled: boolean
  saved_config_active: boolean
  pending_deploy_fields: string[]
  error_message: string
  checked_at: string
}

export type AIProviderUsageDTO = {
  provider: string
  model: string
  calls: number
  estimated_cost: number
}

export type AICostSummaryDTO = {
  total_calls: number
  successful_calls: number
  failed_calls: number
  input_tokens: number
  output_tokens: number
  estimated_cost: number
  monthly_budget: number
  currency: string
  provider_usage: AIProviderUsageDTO[]
}

export type AIConfigOverviewDTO = {
  config: AIConfigDTO
  runtime: AIRuntimeStatusDTO
  summary: AICostSummaryDTO
  provider_catalog: AIProviderOptionDTO[]
  ocr_catalog: AIOCRProviderOptionDTO[]
  route_catalog: AIRouteOptionDTO[]
}

export type AIConfigCheckDTO = {
  key: string
  name: string
  status: 'passed' | 'warning' | 'failed'
  message: string
}

export type AIConfigCheckResultDTO = {
  runtime: AIRuntimeStatusDTO
  checks: AIConfigCheckDTO[]
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

export type TenderPayload = {
  source_id?: string | null
  title: string
  purchaser?: string
  region?: string
  budget_amount?: number | null
  budget_text?: string
  publish_date?: string | null
  deadline?: string | null
  status?: TenderDTO['status']
  match_score?: number
  summary?: string
  requirements?: string[]
  risk_flags?: string[]
  source_url?: string
  metadata?: Record<string, unknown>
}

export type TenderUpdatePayload = Partial<TenderPayload>

export type TenderSourceDTO = {
  id: string
  name: string
  source_type: string
  url: string
  status: 'active' | 'inactive' | 'failed'
  last_verified_at: string | null
  last_verify_status: 'ok' | 'failed' | null
  last_verify_message: string
  config: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type TenderSourcePayload = {
  name: string
  source_type?: string
  url: string
  status?: TenderSourceDTO['status']
  config?: Record<string, unknown>
}

export type TenderSourceUpdatePayload = Partial<TenderSourcePayload>

export type TenderProjectDTO = {
  id: string
  name: string
  status: ProjectDTO['status']
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

export type ProjectMilestonePayload = {
  title: string
  status?: ProjectMilestoneDTO['status']
  due_date?: string
  sort_order?: number
  note?: string
}

export type ProjectMilestoneUpdatePayload = Partial<ProjectMilestonePayload>

export type ProjectActivityDTO = {
  id: string
  project_id: string
  actor_user_id: string | null
  actor_name: string
  action: string
  metadata: Record<string, unknown>
  created_at: string
}

export type ProjectKnowledgeCaseDTO = {
  project_id: string
  document_id: string
  file_id: string
  chunk_id: string
  title: string
  summary: string
  created_at: string
}

export type ArchiveProjectCaseDTO = {
  case: ProjectKnowledgeCaseDTO
  file: FileAssetDTO
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

export type CostItemPayload = {
  category?: string
  name: string
  cost_type?: CostItemDTO['cost_type']
  budget_amount?: number
  actual_amount?: number
  status?: CostItemDTO['status']
  vendor?: string
  note?: string
}

export type CostItemUpdatePayload = Partial<CostItemPayload>

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
    status: BidChapterDTO['status']
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
  status: 'draft' | 'generating' | 'editing' | 'in_review' | 'approved' | 'submitted' | 'archived'
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
  status: 'draft' | 'generated'
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
  status: 'pending' | 'generating' | 'generated' | 'accepted' | 'edited' | 'needs_fix'
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

export type BidParseResultDTO = {
  id: string
  bid_document_id: string
  file_asset_id: string | null
  status: 'queued' | 'processing' | 'ready' | 'confirmed' | 'failed'
  structured_result: Record<string, unknown>
  error_message: string | null
  confirmed_by: string | null
  confirmed_at: string | null
  created_at: string
  updated_at: string
}

export type BidRequirementItemDTO = {
  id: string
  bid_document_id: string
  parse_result_id: string | null
  external_id: string
  module: string
  type: string
  requirement: string
  priority: 'high' | 'medium' | 'low'
  mandatory: boolean
  score: number | null
  expected_response: string
  coverage_status: 'unmapped' | 'planned' | 'covered' | 'needs_review'
  source_ref: Record<string, unknown>
  needs_review: boolean
  sort_order: number
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type BidRequirementCoverageEventDTO = {
  id: string
  bid_document_id: string
  requirement_item_id: string
  requirement_external_id: string
  chapter_id: string | null
  actor_user_id: string | null
  source: 'model' | 'manual' | 'system'
  coverage_status: BidRequirementItemDTO['coverage_status']
  needs_review: boolean
  evidence: string
  source_refs: unknown[]
  metadata: Record<string, unknown>
  created_at: string
}

export type BidPipelineGateDTO = {
  id: string
  bid_document_id: string
  stage: 'interpret' | 'plan' | 'generate' | 'check' | 'format'
  status: 'pending' | 'needs_review' | 'passed' | 'blocked'
  reviewed_by: string | null
  reviewed_at: string | null
  reason: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type BidTenderFileDTO = {
  id: string
  bid_document_id: string
  file_asset_id: string
  object_key: string
  filename: string
  content_type: string
  size_bytes: number
  status: 'active' | 'superseded' | 'deleted'
  created_at: string
  updated_at: string
}

export type UploadBidTenderFileDTO = {
  file: BidTenderFileDTO
  parse_result: BidParseResultDTO
}

export type ParseBidTenderDTO = {
  task: AITaskDTO
  parse_result: BidParseResultDTO
}

export type BidPartOutlineDTO = {
  part: BidPartDTO
  chapters: BidChapterDTO[]
}

export type GenerateBidOutlineDTO = {
  task: AITaskDTO
  parts: BidPartDTO[]
  chapters: BidChapterDTO[]
}

export type BidMaterialSelectionDTO = {
  id: string
  bid_document_id: string
  selected_refs: unknown[]
  notes: string
  updated_by: string | null
  created_at: string
  updated_at: string
}

export type BidGenerationJobDTO = {
  id: string
  bid_document_id: string
  scope: 'full' | 'part' | 'chapter'
  status: 'queued' | 'running' | 'paused' | 'done' | 'failed' | 'cancelled'
  progress: number
  total_steps: number
  completed_steps: number
  failed_steps: number
  model_used: string
  prompt_tokens: number
  completion_tokens: number
  error_message: string | null
  trace_id: string
  created_by: string | null
  started_at: string | null
  completed_at: string | null
  created_at: string
  updated_at: string
}

export type BidGenerationStepDTO = {
  id: string
  job_id: string
  bid_document_id: string
  bid_part_id: string
  chapter_id: string
  chapter_title: string
  step_order: number
  status: 'queued' | 'running' | 'paused' | 'done' | 'failed' | 'cancelled'
  ai_task_id: string | null
  external_task_id: string | null
  error_message: string | null
  metadata: Record<string, unknown>
  started_at: string | null
  completed_at: string | null
  created_at: string
  updated_at: string
}

export type BidGenerationJobDetailDTO = {
  task_id: string
  job: BidGenerationJobDTO
  steps: BidGenerationStepDTO[]
}

export type BidGenerationCoverageSpecDTO = {
  name: string
  bid_document_id: string
  bid_title: string
  requirements: Array<{
    id: string
    database_id: string
    external_id: string
    module: string
    type: string
    requirement: string
    priority: BidRequirementItemDTO['priority']
    mandatory: boolean
    score?: number
    expected_response?: string
    coverage_status: BidRequirementItemDTO['coverage_status']
    source_ref?: Record<string, unknown>
    needs_review: boolean
    sort_order: number
    metadata: Record<string, unknown>
  }>
  chapters: Array<{
    id: string
    bid_part_id: string
    part_code: string
    part_title: string
    title: string
    status: BidChapterDTO['status']
    sort_order: number
    source_refs: unknown[]
    requirement_coverage?: unknown[]
    model_metadata: Record<string, unknown>
    needs_human_input: string[]
    version_no: number
    change_reason: string
    updated_at: string
  }>
  knowledge_chunks: Array<{
    chunk_id: string
    document_id: string
    title: string
    section_path: string
    page_start?: number
    page_end?: number
  }>
  thresholds: {
    min_mandatory_coverage_ratio: number
    min_source_ref_resolution_ratio: number
  }
  require_source_refs: boolean
  generated_at: string
}

export async function login(payload: {
  email: string
  password: string
  tenant_id?: string
}): Promise<LoginSessionPayload> {
  const { data } = await apiClient.post<LoginSessionPayload>('/auth/login', payload)
  return data
}

export async function registerTenant(payload: {
  tenant_name: string
  admin_name: string
  email: string
  password: string
}): Promise<LoginSessionPayload> {
  const { data } = await apiClient.post<LoginSessionPayload>('/auth/register', payload)
  return data
}

export async function refreshSession(): Promise<LoginSessionPayload> {
  return refreshStoredSession()
}

export async function logoutSession(): Promise<{ status: string }> {
  const { data } = await apiClient.post<{ status: string }>('/auth/logout')
  return data
}

export async function updateTenant(payload: { name: string }): Promise<Tenant> {
  const { data } = await apiClient.patch<Tenant>('/tenant', payload)
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
  initial_password: string
}): Promise<MemberDTO> {
  const { data } = await apiClient.post<MemberDTO>('/tenant/members/invite', payload)
  return data
}

export async function updateMember(
  memberId: string,
  payload: {
    name?: string
    status?: 'active' | 'invited' | 'disabled'
    role_codes?: string[]
  },
): Promise<MemberDTO> {
  const { data } = await apiClient.patch<MemberDTO>(`/tenant/members/${memberId}`, payload)
  return data
}

export async function deleteMember(memberId: string): Promise<void> {
  await apiClient.delete(`/tenant/members/${memberId}`)
}

export async function fetchRoles(): Promise<RoleDTO[]> {
  const { data } = await apiClient.get<{ items: RoleDTO[] }>('/roles')
  return data.items
}

export async function fetchExternalToolCatalog(): Promise<ExternalToolProviderPresetDTO[]> {
  const { data } = await apiClient.get<{ items: ExternalToolProviderPresetDTO[] }>('/external-tools/catalog')
  return data.items
}

export async function fetchExternalTools(): Promise<ExternalToolConfigDTO[]> {
  const { data } = await apiClient.get<{ items: ExternalToolConfigDTO[] }>('/external-tools')
  return data.items
}

export async function updateExternalToolConfig(
  providerKey: string,
  payload: {
    name?: string
    transport?: 'streamable_http'
    endpoint?: string
    command?: string
    enabled?: boolean
    allowed_tools?: string[]
    timeout_ms?: number
    monthly_budget?: number
    redaction_policy?: ExternalToolConfigDTO['redaction_policy']
    metadata?: Record<string, unknown>
  },
): Promise<ExternalToolConfigDTO> {
  const { data } = await apiClient.put<ExternalToolConfigDTO>(
    `/external-tools/${encodeURIComponent(providerKey)}`,
    payload,
  )
  return data
}

export async function invokeExternalTool(
  providerKey: string,
  payload: {
    tool_name: string
    arguments: Record<string, unknown>
    resource_type?: string
    resource_id?: string
  },
): Promise<ExternalToolInvokeResultDTO> {
  const { data } = await apiClient.post<ExternalToolInvokeResultDTO>(
    `/external-tools/${encodeURIComponent(providerKey)}/invoke`,
    payload,
  )
  return data
}

export async function fetchExternalToolAuditLogs(limit = 50): Promise<ExternalToolAuditLogDTO[]> {
  const { data } = await apiClient.get<{ items: ExternalToolAuditLogDTO[] }>('/external-tools/audit', {
    params: { limit },
  })
  return data.items
}

export async function fetchNotifications(): Promise<NotificationDTO[]> {
  const { data } = await apiClient.get<{ items: NotificationDTO[] }>('/notifications')
  return data.items
}

export async function fetchAICallLogs(limit = 50): Promise<AICallLogDTO[]> {
  const { data } = await apiClient.get<{ items: AICallLogDTO[] }>('/ai-call-logs', { params: { limit } })
  return data.items
}

export async function fetchAIConfig(): Promise<AIConfigOverviewDTO> {
  const { data } = await apiClient.get<AIConfigOverviewDTO>('/ai-config')
  return data
}

export async function updateAIConfig(payload: AIConfigPayload): Promise<AIConfigOverviewDTO> {
  const { data } = await apiClient.put<AIConfigOverviewDTO>('/ai-config', payload)
  return data
}

export async function checkAIConfig(): Promise<AIConfigCheckResultDTO> {
  const { data } = await apiClient.post<AIConfigCheckResultDTO>('/ai-config/health-check')
  return data
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

export async function fetchApprovals(params?: { status?: ApprovalInstanceDTO['status'] }): Promise<ApprovalInstanceDTO[]> {
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
  status?: TenderDTO['status']
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

export async function createTender(payload: TenderPayload): Promise<TenderDTO> {
  const { data } = await apiClient.post<TenderDTO>('/tenders', payload)
  return data
}

export async function updateTender(tenderId: string, payload: TenderUpdatePayload): Promise<TenderDTO> {
  const { data } = await apiClient.patch<TenderDTO>(`/tenders/${tenderId}`, payload)
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

export type PlatformTenderDTO = {
  id: string
  external_source: 'zbcg' | 'iccec' | string
  external_id: string
  title: string
  purchaser: string
  region: string
  notice_type_name: string
  publish_date: string | null
  deadline: string | null
  source_url: string
  budget_text: string
  budget_amount: number | null
  raw_content_preview: string
  requirement_dims: Record<string, unknown>
  timeline: Record<string, unknown>
  attachments: Record<string, unknown>
  review_result: Record<string, unknown>
  risk_flags: string[]
  status: TenderDTO['status']
  collected_at: string
  updated_at: string
}

export type PlatformCollectorRunDTO = {
  id: string
  external_source: string
  status: 'ok' | 'partial' | 'failed' | 'blocked' | string
  fetched_count: number
  ingested_count: number
  message: string
  started_at: string
  finished_at: string
}

export type PlatformTenderListDTO = {
  items: PlatformTenderDTO[]
  total: number
  limit: number
  offset: number
}

export async function fetchPlatformTenders(params?: {
  q?: string
  source?: string
  limit?: number
  offset?: number
}): Promise<PlatformTenderListDTO> {
  const { data } = await apiClient.get<PlatformTenderListDTO>('/platform/tenders', { params })
  return {
    items: data.items ?? [],
    total: data.total ?? 0,
    limit: data.limit ?? params?.limit ?? 50,
    offset: data.offset ?? params?.offset ?? 0,
  }
}

export async function fetchPlatformCollectorRuns(params?: {
  source?: string
  limit?: number
}): Promise<PlatformCollectorRunDTO[]> {
  const { data } = await apiClient.get<{ items: PlatformCollectorRunDTO[] }>('/platform/collector-runs', { params })
  return data.items ?? []
}

export async function fetchTenderSources(): Promise<TenderSourceDTO[]> {
  const { data } = await apiClient.get<{ items: TenderSourceDTO[] }>('/tender-sources')
  return data.items
}

export async function createTenderSource(payload: TenderSourcePayload): Promise<TenderSourceDTO> {
  const { data } = await apiClient.post<TenderSourceDTO>('/tender-sources', payload)
  return data
}

export async function updateTenderSource(sourceId: string, payload: TenderSourceUpdatePayload): Promise<TenderSourceDTO> {
  const { data } = await apiClient.patch<TenderSourceDTO>(`/tender-sources/${sourceId}`, payload)
  return data
}

export async function verifyTenderSource(sourceId: string): Promise<TenderSourceDTO> {
  const { data } = await apiClient.post<TenderSourceDTO>(`/tender-sources/${sourceId}/verify`)
  return data
}

export async function fetchProjects(params?: { status?: ProjectDTO['status'] }): Promise<ProjectDTO[]> {
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
  payload: ProjectMilestonePayload,
): Promise<ProjectMilestoneDTO> {
  const { data } = await apiClient.post<ProjectMilestoneDTO>(`/projects/${projectId}/milestones`, payload)
  return data
}

export async function updateProjectMilestone(
  projectId: string,
  milestoneId: string,
  payload: ProjectMilestoneUpdatePayload,
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

export async function archiveProjectCase(projectId: string): Promise<ArchiveProjectCaseDTO> {
  const { data } = await apiClient.post<ArchiveProjectCaseDTO>(`/projects/${projectId}/archive-case`)
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
  payload: CostItemPayload,
): Promise<CostItemDTO> {
  const { data } = await apiClient.post<CostItemDTO>(`/cost-projects/${costProjectId}/items`, payload)
  return data
}

export async function updateCostItem(
  costItemId: string,
  payload: CostItemUpdatePayload,
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

export async function createCostAdvice(costProjectId: string): Promise<AITaskDTO> {
  const { data } = await apiClient.post<AITaskDTO>(`/cost-projects/${costProjectId}/ai-advice`)
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

export async function fetchKnowledgeDocumentPreview(documentId: string): Promise<PresignedFileUrlDTO> {
  const { data } = await apiClient.get<PresignedFileUrlDTO>(`/knowledge/documents/${documentId}/preview`)
  return data
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
  const { data } = await apiClient.post<ConfirmUploadDTO | FileAssetDTO>(`/files/${fileId}/confirm`)
  if ('file' in data) {
    return data
  }
  return { file: data }
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

export async function updateBid(
  bidId: string,
  payload: {
    title?: string
    project_id?: string | null
    status?: 'draft' | 'generating' | 'editing' | 'in_review' | 'approved' | 'submitted' | 'archived'
  },
): Promise<BidDocumentDTO> {
  const { data } = await apiClient.patch<BidDocumentDTO>(`/bids/${bidId}`, payload)
  return data
}

export async function deleteBid(bidId: string): Promise<void> {
  await apiClient.delete(`/bids/${bidId}`)
}

export async function fetchBidParts(bidId: string): Promise<BidPartDTO[]> {
  const { data } = await apiClient.get<{ items: BidPartDTO[] }>(`/bids/${bidId}/parts`)
  return data.items
}

export async function fetchBidChapters(bidId: string): Promise<BidChapterDTO[]> {
  const { data } = await apiClient.get<{ items: BidChapterDTO[] }>(`/bids/${bidId}/chapters`)
  return data.items
}

export async function uploadBidTenderFile(
  bidId: string,
  payload: { file_id: string },
): Promise<UploadBidTenderFileDTO> {
  const { data } = await apiClient.post<UploadBidTenderFileDTO>(`/bids/${bidId}/upload-tender-file`, payload)
  return data
}

export async function parseBidTender(bidId: string): Promise<ParseBidTenderDTO> {
  const { data } = await apiClient.post<ParseBidTenderDTO>(`/bids/${bidId}/parse-tender`)
  return data
}

export async function fetchBidParseResult(bidId: string): Promise<BidParseResultDTO> {
  const { data } = await apiClient.get<BidParseResultDTO>(`/bids/${bidId}/parse-result`)
  return data
}

export async function fetchBidRequirements(bidId: string): Promise<BidRequirementItemDTO[]> {
  const { data } = await apiClient.get<{ items: BidRequirementItemDTO[] }>(`/bids/${bidId}/requirements`)
  return data.items
}

export type BidRequirementExportFormat = 'csv' | 'xlsx'

export async function exportBidRequirements(
  bidId: string,
  format: BidRequirementExportFormat = 'csv',
  fallbackTitle?: string,
): Promise<{ blob: Blob; filename: string }> {
  const response = await apiClient.get<Blob>(`/bids/${bidId}/requirements/export`, {
    params: format === 'xlsx' ? { format: 'xlsx' } : undefined,
    responseType: 'blob',
  })
  const fallbackFilename = bidRequirementFallbackFilename(fallbackTitle, format)
  return {
    blob: response.data,
    filename: safeDownloadFilename(filenameFromContentDisposition(response.headers['content-disposition']), fallbackFilename),
  }
}

export async function updateBidRequirementCoverage(
  bidId: string,
  requirementId: string,
  payload: {
    coverage_status: BidRequirementItemDTO['coverage_status']
    evidence?: string
    source_refs?: unknown[]
  },
): Promise<BidRequirementItemDTO> {
  const { data } = await apiClient.patch<BidRequirementItemDTO>(
    `/bids/${encodeURIComponent(bidId)}/requirements/${encodeURIComponent(requirementId)}`,
    payload,
  )
  return data
}

export async function batchUpdateBidRequirementCoverage(
  bidId: string,
  payload: {
    requirement_ids?: string[]
    apply_all?: boolean
    filter?: 'all' | 'mandatory' | 'review' | 'covered'
    evidence_filter?: 'all' | 'missing_evidence' | 'missing_source' | 'complete'
    coverage_status: BidRequirementItemDTO['coverage_status']
    evidence?: string
    source_refs?: unknown[]
  },
): Promise<BidRequirementItemDTO[]> {
  const { data } = await apiClient.patch<{ items: BidRequirementItemDTO[] }>(
    `/bids/${encodeURIComponent(bidId)}/requirements`,
    payload,
  )
  return data.items
}

export async function fetchBidRequirementCoverageHistory(
  bidId: string,
  requirementId: string,
): Promise<BidRequirementCoverageEventDTO[]> {
  const { data } = await apiClient.get<{ items: BidRequirementCoverageEventDTO[] }>(
    `/bids/${encodeURIComponent(bidId)}/requirements/${encodeURIComponent(requirementId)}/history`,
  )
  return data.items
}

function filenameFromContentDisposition(value: unknown): string {
  const text = Array.isArray(value) ? String(value[0] || '') : String(value || '')
  const encodedMatch = text.match(/filename\*=UTF-8''([^;]+)/i)
  if (encodedMatch?.[1]) {
    try {
      return decodeURIComponent(encodedMatch[1])
    } catch {
      return encodedMatch[1]
    }
  }
  const filenameMatch = text.match(/filename="?([^";]+)"?/i)
  return filenameMatch?.[1] ?? ''
}

const MAX_DOWNLOAD_FILENAME_LENGTH = 120
const WINDOWS_RESERVED_DOWNLOAD_NAMES = /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(\..*)?$/i

function safeDownloadFilename(value: string | undefined, fallback: string) {
  const fallbackFilename = normalizeDownloadFilename(fallback, 'download')
  return normalizeDownloadFilename(value, fallbackFilename)
}

function normalizeDownloadFilename(value: string | undefined, fallback: string) {
  const pathParts = String(value || '')
    .split(/[\\/]+/)
    .map((part) => part.trim())
    .filter(Boolean)
  const filename = pathParts[pathParts.length - 1] ?? ''
  const cleaned = downloadSafeFilenamePart(filename, fallback)
  const reservedSafe = WINDOWS_RESERVED_DOWNLOAD_NAMES.test(cleaned) ? `_${cleaned}` : cleaned
  return truncateDownloadFilename(reservedSafe || fallback)
}

function truncateDownloadFilename(value: string) {
  if (value.length <= MAX_DOWNLOAD_FILENAME_LENGTH) return value
  const dotIndex = value.lastIndexOf('.')
  const ext = dotIndex > 0 && value.length - dotIndex <= 16 ? value.slice(dotIndex) : ''
  const stemLength = Math.max(1, MAX_DOWNLOAD_FILENAME_LENGTH - ext.length)
  return `${value.slice(0, stemLength).trim().replace(/\.+$/g, '')}${ext}`
}

function bidRequirementFallbackFilename(title: string | undefined, format: BidRequirementExportFormat) {
  const ext = format === 'xlsx' ? 'xlsx' : 'csv'
  const prefix = format === 'xlsx' ? '响应矩阵-覆盖历史' : '响应矩阵'
  return `${prefix}-${downloadSafeFilenamePart(title, '标书')}-${filenameTimestamp()}.${ext}`
}

function filenameTimestamp() {
  const value = new Date()
  const pad = (input: number) => String(input).padStart(2, '0')
  return [
    value.getFullYear(),
    pad(value.getMonth() + 1),
    pad(value.getDate()),
    '-',
    pad(value.getHours()),
    pad(value.getMinutes()),
    pad(value.getSeconds()),
  ].join('')
}

function downloadSafeFilenamePart(value: string | undefined, fallback: string) {
  let cleaned = ''
  let lastWasSpace = false
  for (const char of String(value || '').trim()) {
    const code = char.charCodeAt(0)
    const unsafe = code < 32 || code === 127 || '\\/:*?"<>|'.includes(char)
    if (unsafe || /\s/.test(char)) {
      if (cleaned && !lastWasSpace) {
        cleaned += ' '
        lastWasSpace = true
      }
      continue
    }
    cleaned += char
    lastWasSpace = false
  }
  cleaned = cleaned.trim().replace(/^\.+|\.+$/g, '')
  return cleaned || fallback
}

export async function fetchBidPipelineGates(bidId: string): Promise<BidPipelineGateDTO[]> {
  const { data } = await apiClient.get<{ items: BidPipelineGateDTO[] }>(`/bids/${bidId}/pipeline-gates`)
  return data.items
}

export async function confirmBidParseResult(
  bidId: string,
  payload: { structured_result?: Record<string, unknown> },
): Promise<BidParseResultDTO> {
  const { data } = await apiClient.put<BidParseResultDTO>(`/bids/${bidId}/parse-result`, payload)
  return data
}

export async function generateBidOutline(bidId: string): Promise<GenerateBidOutlineDTO> {
  const { data } = await apiClient.post<GenerateBidOutlineDTO>(`/bids/${bidId}/outline/generate`)
  return data
}

export async function fetchBidPartOutline(
  bidId: string,
  partId: string,
): Promise<BidPartOutlineDTO> {
  const { data } = await apiClient.get<BidPartOutlineDTO>(`/bids/${bidId}/parts/${partId}/outline`)
  return data
}

export async function updateBidPartOutline(
  bidId: string,
  partId: string,
  payload: { chapters: Array<{ id?: string; title: string; plain_text?: string; sort_order?: number }> },
): Promise<BidPartOutlineDTO> {
  const { data } = await apiClient.put<BidPartOutlineDTO>(`/bids/${bidId}/parts/${partId}/outline`, payload)
  return data
}

export async function fetchBidMaterialSelection(bidId: string): Promise<BidMaterialSelectionDTO> {
  const { data } = await apiClient.get<BidMaterialSelectionDTO>(`/bids/${bidId}/material-selection`)
  return data
}

export async function updateBidMaterialSelection(
  bidId: string,
  payload: { selected_refs: unknown[]; notes?: string },
): Promise<BidMaterialSelectionDTO> {
  const { data } = await apiClient.put<BidMaterialSelectionDTO>(`/bids/${bidId}/material-selection`, payload)
  return data
}

export async function generateBid(
  bidId: string,
  payload: { scope?: 'full' | 'part' | 'chapter'; part_code?: string; chapter_ids?: string[] } = {},
): Promise<BidGenerationJobDetailDTO> {
  const { data } = await apiClient.post<BidGenerationJobDetailDTO>(`/bids/${bidId}/generate`, payload)
  return data
}

export async function fetchBidGenerationJobs(bidId: string): Promise<BidGenerationJobDTO[]> {
  const { data } = await apiClient.get<{ items: BidGenerationJobDTO[] }>(`/bids/${bidId}/generation-jobs`)
  return data.items
}

export async function fetchBidGenerationJob(jobId: string): Promise<BidGenerationJobDetailDTO> {
  const { data } = await apiClient.get<BidGenerationJobDetailDTO>(`/generation-jobs/${jobId}`)
  return data
}

export async function fetchBidGenerationCoverage(bidId: string): Promise<BidGenerationCoverageSpecDTO> {
  const { data } = await apiClient.get<BidGenerationCoverageSpecDTO>(`/bids/${bidId}/generation-coverage`)
  return data
}

export async function pauseBidGenerationJob(jobId: string): Promise<BidGenerationJobDetailDTO> {
  const { data } = await apiClient.post<BidGenerationJobDetailDTO>(`/generation-jobs/${jobId}/pause`)
  return data
}

export async function resumeBidGenerationJob(jobId: string): Promise<BidGenerationJobDetailDTO> {
  const { data } = await apiClient.post<BidGenerationJobDetailDTO>(`/generation-jobs/${jobId}/resume`)
  return data
}

export async function cancelBidGenerationJob(jobId: string): Promise<BidGenerationJobDetailDTO> {
  const { data } = await apiClient.post<BidGenerationJobDetailDTO>(`/generation-jobs/${jobId}/cancel`)
  return data
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

export async function chapterAiAction(
  chapterId: string,
  payload: { action: 'optimize' | 'expand' | 'shorten' | 'add_detail' | 'self_check'; instruction?: string },
): Promise<ChapterRegenerateDTO> {
  const { data } = await apiClient.post<ChapterRegenerateDTO>(`/chapters/${chapterId}/ai-action`, payload)
  return data
}

export async function fetchChapterVersions(chapterId: string): Promise<BidChapterVersionDTO[]> {
  const { data } = await apiClient.get<{ items: BidChapterVersionDTO[] }>(`/chapters/${chapterId}/versions`)
  return data.items
}

export type BidChapterDiffDTO = {
  current: BidChapterDTO
  previous: BidChapterVersionDTO | null
}

export async function fetchChapterDiff(chapterId: string): Promise<BidChapterDiffDTO> {
  const { data } = await apiClient.get<BidChapterDiffDTO>(`/chapters/${chapterId}/diff`)
  return data
}

export async function fetchBidExports(bidId: string): Promise<BidExportDTO[]> {
  const { data } = await apiClient.get<{ items: BidExportDTO[] }>(`/bids/${bidId}/exports`)
  return data.items
}

export async function createBidExport(
  bidId: string,
  payload: {
    export_type: 'docx' | 'pdf' | 'zip'
    part_code: string
    layout?: Record<string, unknown>
    attachments?: Array<Record<string, unknown>>
    boq_files?: Array<Record<string, unknown>>
  },
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

export type PlatformSummaryDTO = {
  stats: {
    active_projects: number
    monthly_bids: number
    compliance_pass_rate: number
    win_rate: number
    pending_tasks: number
    knowledge_docs: number
  }
  trends: Array<{ month: string; bids: number; win_rate: number }>
  recommended_tenders: Array<{
    id: string
    title: string
    region: string
    purchaser: string
    match_score: number
    deadline: string | null
  }>
  recent_projects: Array<{
    id: string
    name: string
    status: ProjectDTO['status']
    owner_name: string
    due_date: string | null
    updated_at: string
  }>
  pending_approvals: Array<{
    id: string
    title: string
    bid_title: string
    current_step: number
    created_at: string
  }>
  notifications: NotificationDTO[]
  generated_at: string
}

export async function fetchPlatformSummary(): Promise<PlatformSummaryDTO> {
  const { data } = await apiClient.get<PlatformSummaryDTO>('/dashboard/summary')
  return data
}
