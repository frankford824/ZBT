import {
  ApiOutlined,
  CheckOutlined,
  CloseOutlined,
  CloudServerOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  RobotOutlined,
  SettingOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import {
  App as AntApp,
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
} from 'antd'
import { useEffect, useMemo, useRef, useState } from 'react'
import {
  approveApproval,
  checkAIConfig,
  createApprovalChain,
  deleteApprovalChain,
  deleteMember,
  fetchAICallLogs,
  fetchAIConfig,
  fetchApprovalChains,
  fetchApprovals,
  fetchExternalToolAuditLogs,
  fetchExternalToolCatalog,
  fetchExternalTools,
  fetchMembers,
  fetchNotifications,
  fetchRoles,
  getApiErrorMessage,
  inviteMember,
  markNotificationsRead,
  rejectApproval,
  updateAIConfig,
  updateExternalToolConfig,
  updateMember,
  updateApprovalChain,
  type AIConfigCheckResultDTO,
  type AIConfigDTO,
  type AIConfigOverviewDTO,
  type AIModelPricingRateDTO,
  type ApprovalChainDTO,
  type ApprovalStepDTO,
  type ExternalToolAuditLogDTO,
  type ExternalToolConfigDTO,
  type ExternalToolProviderPresetDTO,
  type MemberDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { formatDateTime } from '../../shared/format/date'
import { useCanAccess } from '../../shared/permissions/permissions'

const teamTabs = ['members', 'approvals', 'external-tools', 'ai-config', 'logs', 'notifications'] as const
type TeamTab = (typeof teamTabs)[number]
const MAX_EXTERNAL_TOOL_MONTHLY_BUDGET = 10000000
const MAX_EXTERNAL_TOOL_COST_PER_CALL = 100000
const MAX_AI_MONTHLY_BUDGET = 10000000
const MAX_AI_MODEL_RATE = 1000000

function normalizeTeamTab(value: string | null): TeamTab {
  return teamTabs.includes(value as TeamTab) ? (value as TeamTab) : 'members'
}

function statusTag(status: string) {
  const color =
    status === 'approved' || status === 'done' || status === 'success'
      ? 'green'
      : status === 'rejected' || status === 'failed' || status === 'disabled'
        ? 'red'
        : status === 'blocked'
          ? 'gold'
          : status === 'pending' || status === 'running' || status === 'queued' || status === 'invited'
          ? 'blue'
          : 'default'
  const label: Record<string, string> = {
    pending: '审批中',
    approved: '已通过',
    rejected: '已驳回',
    cancelled: '已取消',
    done: '完成',
    success: '成功',
    failed: '失败',
    blocked: '已阻断',
    running: '运行中',
    queued: '排队中',
    active: '正常',
    invited: '已邀请',
    disabled: '已禁用',
  }
  return <Tag color={color}>{label[status] || '状态未知'}</Tag>
}

const taskTypeLabels: Record<string, string> = {
  tender_parse: '招标文件解读',
  knowledge_process: '知识库处理',
  knowledge_embedding: '知识库整理',
  knowledge_rerank: '知识库匹配',
  outline_generate: '目录大纲生成',
  chapter_generate: '章节生成',
  chapter_ai_action: '章节处理',
  document_export: '文档导出',
  compliance_check: '合规检查',
  cost_advice: '成本建议',
}

const resourceLabels: Record<string, string> = {
  bid: '标书',
  bid_export: '导出文件',
  bid_chapter: '标书章节',
  chapter: '章节',
  knowledge: '知识库',
  knowledge_document: '知识文档',
  document: '文档',
  tender: '标讯',
  cost: '成本',
  cost_project: '成本测算',
}

const externalToolNameLabels: Record<string, string> = {
  analyze_gaps: '差距分析',
  bid_bigdata_bid_search: '标讯搜索',
  bid_bigdata_bid_win_stats: '中标统计',
  bid_bigdata_bidding_info: '招标信息',
  bid_bigdata_fuzzy_search: '模糊检索',
  bid_bigdata_planned_projects: '拟建项目',
  bid_bigdata_procurement_stats: '采购统计',
  bid_bigdata_tender_stats: '招标统计',
  bid_no_bid_decision: '投标决策',
  extract_requirements: '要求抽取',
  get_deal_snapshot: '机会快照',
  get_intelligence_summary: '情报摘要',
  get_library_entry: '答案详情',
  get_project: '项目详情',
  get_q_routing_state: '问题分派',
  get_requirement: '需求详情',
  get_tender_detail: '标讯详情',
  list_competitors: '竞品列表',
  list_deals: '机会列表',
  list_library_entries: '答案库列表',
  list_projects: '项目列表',
  list_questions: '问题列表',
  list_requirements: '需求列表',
  list_tags: '标签列表',
  map_sections: '章节映射',
  outline_response: '响应提纲',
  score_win_probability: '中标评分',
  search_compliance_items: '合规项搜索',
  search_content_library: '内容库搜索',
  search_library: '答案库搜索',
  search_tenders: '标讯检索',
  search_tenders_for_my_company: '适配标讯',
  usage_analytics: '使用分析',
}

type ExternalToolRow = {
  preset: ExternalToolProviderPresetDTO
  config?: ExternalToolConfigDTO
}

type AIConfigPricingRow = {
  key?: string
  display_name?: string
  input_per_1m?: number
  output_per_1m?: number
  currency?: string
}

type AIConfigFormValues = {
  enabled?: boolean
  llm_provider: string
  llm_model: string
  embedding_provider: string
  embedding_model: string
  rerank_provider: string
  rerank_model: string
  ocr_provider: string
  ocr_endpoint?: string
  monthly_budget?: number
  mock_fallback_allowed?: boolean
  pricing_items?: AIConfigPricingRow[]
}

function formatTime(value?: string | null) {
  return formatDateTime(value)
}

function formatBizRef(value: Record<string, unknown>) {
  const resourceType = typeof value.resource_type === 'string' ? value.resource_type : '-'
  return resourceLabels[resourceType] || '相关内容'
}

function formatProcessingVolume(input: number, output: number) {
  const total = Number(input || 0) + Number(output || 0)
  if (!total) return '-'
  if (total < 2000) return '少量内容'
  if (total < 10000) return '中等内容'
  if (total < 30000) return '较多内容'
  return '大量内容'
}

function formatEstimatedCost(value: number) {
  const cost = Number(value || 0)
  if (!cost) return '-'
  return `¥${cost < 0.01 ? cost.toFixed(4) : cost.toFixed(2)}`
}

function formatProviderModel(provider: string, model: string, overview?: AIConfigOverviewDTO) {
  const providerName = formatAIProvider(provider, overview)
  const modelName = String(model || '').trim()
  return modelName ? `${providerName} / ${modelName}` : providerName
}

function formatAIProvider(provider: string, overview?: AIConfigOverviewDTO) {
  return overview?.provider_catalog.find((item) => item.provider_key === provider)?.name || provider || '-'
}

function formatOCRProvider(provider: string, overview?: AIConfigOverviewDTO) {
  return overview?.ocr_catalog.find((item) => item.provider_key === provider)?.name || formatAIProvider(provider, overview)
}

function aiCheckStatusTag(status: string) {
  if (status === 'passed') return <Tag color="green">通过</Tag>
  if (status === 'failed') return <Tag color="red">需处理</Tag>
  return <Tag color="gold">需关注</Tag>
}

function aiRouteStatusTag(row: { active_provider: string; active_model: string; saved_provider: string; saved_model: string }, enabled: boolean) {
  if (!enabled) return <Tag>未启用</Tag>
  if (row.active_provider === row.saved_provider && row.active_model === row.saved_model) {
    return <Tag color="green">一致</Tag>
  }
  return <Tag color="gold">待切换</Tag>
}

function aiConfigStatusTag(overview: AIConfigOverviewDTO) {
  if (!overview.config.enabled) return <Tag>未启用</Tag>
  if (overview.runtime.saved_config_active) return <Tag color="green">已生效</Tag>
  return <Tag color="gold">待切换</Tag>
}

function aiServiceStatusTag(overview: AIConfigOverviewDTO) {
  return overview.runtime.service_reachable ? <Tag color="green">服务可达</Tag> : <Tag color="red">服务不可达</Tag>
}

function aiFallbackStatusTag(overview: AIConfigOverviewDTO) {
  return overview.runtime.mock_fallback_allowed || overview.runtime.mock_providers_enabled || overview.config.mock_fallback_allowed ? (
    <Tag color="gold">允许演练回退</Tag>
  ) : (
    <Tag color="green">生产回退关闭</Tag>
  )
}

function pricingRowsFromConfig(config: AIConfigDTO): AIConfigPricingRow[] {
  const rows = Object.entries(config.pricing || {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, rate]) => ({
      key,
      display_name: rate.display_name || '',
      input_per_1m: rate.input_per_1m || (rate.input_per_1k ? rate.input_per_1k * 1000 : undefined),
      output_per_1m: rate.output_per_1m || (rate.output_per_1k ? rate.output_per_1k * 1000 : undefined),
      currency: rate.currency || 'CNY',
    }))
  if (rows.length) return rows
  return [
    {
      key: `${config.llm_provider}/*`,
      display_name: '通用模型',
      currency: 'CNY',
    },
  ]
}

function aiConfigFormValues(overview: AIConfigOverviewDTO): AIConfigFormValues {
  const config = overview.config
  return {
    enabled: config.enabled,
    llm_provider: config.llm_provider,
    llm_model: config.llm_model,
    embedding_provider: config.embedding_provider,
    embedding_model: config.embedding_model,
    rerank_provider: config.rerank_provider,
    rerank_model: config.rerank_model,
    ocr_provider: config.ocr_provider,
    ocr_endpoint: config.ocr_endpoint,
    monthly_budget: config.monthly_budget || undefined,
    mock_fallback_allowed: config.mock_fallback_allowed,
    pricing_items: pricingRowsFromConfig(config),
  }
}

function pricingFromRows(rows: AIConfigPricingRow[] | undefined): Record<string, AIModelPricingRateDTO> {
  const pricing: Record<string, AIModelPricingRateDTO> = {}
  for (const row of rows ?? []) {
    const key = String(row.key || '').trim()
    if (!key) continue
    const input = Number(row.input_per_1m || 0)
    const output = Number(row.output_per_1m || 0)
    if (input <= 0 && output <= 0) continue
    pricing[key] = {
      input_per_1m: input > 0 ? input : undefined,
      output_per_1m: output > 0 ? output : undefined,
      currency: row.currency || 'CNY',
      display_name: row.display_name?.trim() || undefined,
    }
  }
  return pricing
}

function aiConfigPayloadFromForm(values: AIConfigFormValues) {
  return {
    enabled: Boolean(values.enabled),
    llm_provider: values.llm_provider,
    llm_model: values.llm_model?.trim() || 'gpt-4o-mini',
    embedding_provider: values.embedding_provider,
    embedding_model: values.embedding_model?.trim() || 'text-embedding-3-large',
    rerank_provider: values.rerank_provider,
    rerank_model: values.rerank_model?.trim() || 'gpt-4o-mini',
    ocr_provider: values.ocr_provider,
    ocr_endpoint: values.ocr_endpoint?.trim() || '',
    monthly_budget: values.monthly_budget || 0,
    mock_fallback_allowed: Boolean(values.mock_fallback_allowed),
    pricing: pricingFromRows(values.pricing_items),
    metadata: {},
  }
}

function formatExternalToolName(value: string) {
  return externalToolNameLabels[value] || '外部能力'
}

function formatExternalToolProviderName(value: string, displayNameMap: Map<string, string>) {
  return displayNameMap.get(value) || '外部数据源'
}

function formatExternalToolAuditResult(row: ExternalToolAuditLogDTO) {
  if (row.status === 'success') {
    return formatExternalToolSuccessSummary(row.response_summary)
  }
  const message = row.error_message || row.response_summary
  return externalToolBusinessErrorMessage(message, row.status)
}

function formatExternalToolAuditRequest(value: string) {
  const summary = value.trim()
  if (!summary) return '-'
  const itemCount = summary.split(',').map((item) => item.trim()).filter(Boolean).length
  if (!itemCount) return '已提交条件'
  return itemCount === 1 ? '提交 1 项条件' : `提交 ${itemCount} 项条件`
}

function formatExternalToolSuccessSummary(value: string) {
  const summary = value.trim()
  if (!summary) return '调用成功'
  const parsed = parseExternalToolSummary(summary)
  if (parsed === null) return '调用成功'
  const itemCount = countExternalToolResultItems(parsed)
  if (itemCount > 0) return `返回 ${itemCount} 条结果`
  return '已返回结果'
}

function parseExternalToolSummary(value: string): unknown | null {
  try {
    return JSON.parse(value)
  } catch {
    return null
  }
}

function countExternalToolResultItems(value: unknown): number {
  if (Array.isArray(value)) return value.length
  if (!value || typeof value !== 'object') return 0
  const record = value as Record<string, unknown>
  const structuralCount = structuralArrayCount(record)
  if (structuralCount > 0) return structuralCount
  for (const key of ['items', 'results', 'data', 'documents', 'questions', 'tenders']) {
    const item = record[key]
    if (Array.isArray(item)) return item.length
    if (item && typeof item === 'object') {
      const count = structuralArrayCount(item as Record<string, unknown>)
      if (count > 0) return count
    }
  }
  return 0
}

function structuralArrayCount(value: Record<string, unknown>) {
  const count = value.type === 'array' ? Number(value.count) : 0
  return Number.isFinite(count) && count > 0 ? count : 0
}

function externalToolBusinessErrorMessage(value: string, status: ExternalToolAuditLogDTO['status']) {
  const message = value.trim()
  if (/[\u4e00-\u9fff]/.test(message)) {
    return message
  }
  const normalized = message.toLowerCase()
  if (normalized.includes('provider is disabled')) {
    return '数据源未启用'
  }
  if (normalized.includes('tenant allowlist')) {
    return '该能力未启用'
  }
  if (normalized.includes('monthly budget exceeded')) {
    return '本月调用预算已用完'
  }
  if (normalized.includes('http') || normalized.includes('timeout') || normalized.includes('connection')) {
    return '数据源暂时不可用'
  }
  if (normalized.includes('external tool error')) {
    return '数据源返回失败'
  }
  return status === 'blocked' ? '调用已拦截，请检查配置' : '调用失败，请稍后重试'
}

function externalToolAuthorizationLabel(preset: ExternalToolProviderPresetDTO) {
  return preset.requires_token ? '需要授权' : '无需单独授权'
}

function externalToolAuthorizationHint(preset: ExternalToolProviderPresetDTO) {
  if (!preset.requires_token) {
    return '该数据源当前不需要单独授权。'
  }
  return '请使用服务商提供的授权信息完成连接。'
}

function numericMetadata(value: Record<string, unknown> | undefined, key: string) {
  const raw = value?.[key]
  const numberValue = typeof raw === 'number' ? raw : Number(raw)
  return Number.isFinite(numberValue) && numberValue > 0 ? numberValue : 0
}

function formatLatency(value: number) {
  if (!value) return '-'
  return value >= 1000 ? `${(value / 1000).toFixed(1)} 秒` : `${value} 毫秒`
}

function externalToolFormValues(preset: ExternalToolProviderPresetDTO, config?: ExternalToolConfigDTO) {
  return {
    name: config?.name || preset.name,
    enabled: Boolean(config?.enabled),
    endpoint: config?.endpoint || '',
    allowed_tools: config?.allowed_tools?.length ? config.allowed_tools : preset.default_allowed_tools,
    timeout_ms: config?.timeout_ms || 5000,
    monthly_budget: config?.monthly_budget || 0,
    redaction_policy: config?.redaction_policy === 'disabled' ? 'summary_only' : config?.redaction_policy || 'summary_only',
    cost_per_call: numericMetadata(config?.metadata, 'cost_per_call'),
  }
}

export function TeamPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = normalizeTeamTab(searchParams.get('tab'))
  const canWrite = useCanAccess('team', 'full')
  const [inviteOpen, setInviteOpen] = useState(false)
  const [chainOpen, setChainOpen] = useState(false)
  const [memberOpen, setMemberOpen] = useState(false)
  const [editingMember, setEditingMember] = useState<MemberDTO | null>(null)
  const [externalToolOpen, setExternalToolOpen] = useState(false)
  const [editingExternalTool, setEditingExternalTool] = useState<ExternalToolRow | null>(null)
  const [aiConfigCheckResult, setAIConfigCheckResult] = useState<AIConfigCheckResultDTO | null>(null)
  const aiConfigFormInitialized = useRef(false)
  const [inviteForm] = Form.useForm()
  const [chainForm] = Form.useForm()
  const [memberForm] = Form.useForm()
  const [externalToolForm] = Form.useForm()
  const [aiConfigForm] = Form.useForm<AIConfigFormValues>()

  const membersQuery = useQuery({ queryKey: ['team', 'members'], queryFn: fetchMembers })
  const rolesQuery = useQuery({ queryKey: ['team', 'roles'], queryFn: fetchRoles })
  const notificationsQuery = useQuery({ queryKey: ['team', 'notifications'], queryFn: fetchNotifications })
  const approvalsQuery = useQuery({ queryKey: ['team', 'approvals'], queryFn: () => fetchApprovals() })
  const chainsQuery = useQuery({ queryKey: ['team', 'approval-chains'], queryFn: fetchApprovalChains })
  const aiLogsQuery = useQuery({ queryKey: ['team', 'ai-call-logs'], queryFn: () => fetchAICallLogs(50) })
  const aiConfigQuery = useQuery({ queryKey: ['team', 'ai-config'], queryFn: fetchAIConfig })
  const externalCatalogQuery = useQuery({ queryKey: ['team', 'external-tools', 'catalog'], queryFn: fetchExternalToolCatalog })
  const externalToolsQuery = useQuery({ queryKey: ['team', 'external-tools'], queryFn: fetchExternalTools })
  const externalToolAuditQuery = useQuery({
    queryKey: ['team', 'external-tools', 'audit'],
    queryFn: () => fetchExternalToolAuditLogs(50),
  })

  const roleOptions = rolesQuery.data?.map((role) => ({ value: role.code, label: role.name })) ?? []
  const externalToolConfigMap = useMemo(() => {
    const map = new Map<string, ExternalToolConfigDTO>()
    for (const config of externalToolsQuery.data ?? []) {
      map.set(config.provider_key, config)
    }
    return map
  }, [externalToolsQuery.data])
  const externalToolRows: ExternalToolRow[] = useMemo(
    () =>
      (externalCatalogQuery.data ?? []).map((preset) => ({
        preset,
        config: externalToolConfigMap.get(preset.provider_key),
      })),
    [externalCatalogQuery.data, externalToolConfigMap],
  )
  const externalToolDisplayNameMap = useMemo(() => {
    const map = new Map<string, string>()
    for (const preset of externalCatalogQuery.data ?? []) {
      map.set(preset.provider_key, preset.name)
    }
    for (const config of externalToolsQuery.data ?? []) {
      if (!map.has(config.provider_key)) {
        map.set(config.provider_key, config.name)
      }
    }
    return map
  }, [externalCatalogQuery.data, externalToolsQuery.data])
  const aiProviderOptions = useMemo(
    () => (aiConfigQuery.data?.provider_catalog ?? []).map((item) => ({ value: item.provider_key, label: item.name })),
    [aiConfigQuery.data?.provider_catalog],
  )
  const aiOCROptions = useMemo(
    () => (aiConfigQuery.data?.ocr_catalog ?? []).map((item) => ({ value: item.provider_key, label: item.name })),
    [aiConfigQuery.data?.ocr_catalog],
  )

  useEffect(() => {
    if (aiConfigQuery.data && (!aiConfigFormInitialized.current || !aiConfigForm.isFieldsTouched(true))) {
      aiConfigForm.setFieldsValue(aiConfigFormValues(aiConfigQuery.data))
      aiConfigFormInitialized.current = true
    }
  }, [aiConfigForm, aiConfigQuery.data])

  const inviteMutation = useMutation({
    mutationFn: inviteMember,
    onSuccess: () => {
      setInviteOpen(false)
      inviteForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['team', 'members'] })
      message.success('成员已邀请')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '邀请成员失败')),
  })

  const updateMemberMutation = useMutation({
    mutationFn: (values: { name?: string; status?: 'active' | 'invited' | 'disabled'; role_codes?: string[] }) => {
      if (!editingMember) throw new Error('missing member')
      return updateMember(editingMember.id, values)
    },
    onSuccess: () => {
      setMemberOpen(false)
      setEditingMember(null)
      memberForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['team', 'members'] })
      message.success('成员已更新')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '成员更新失败')),
  })

  const deleteMemberMutation = useMutation({
    mutationFn: deleteMember,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'members'] })
      message.success('成员已禁用')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '成员禁用失败')),
  })

  const createChainMutation = useMutation({
    mutationFn: createApprovalChain,
    onSuccess: () => {
      setChainOpen(false)
      chainForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['team', 'approval-chains'] })
      message.success('审批流程已创建')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '审批流程创建失败')),
  })

  const updateChainMutation = useMutation({
    mutationFn: (chain: ApprovalChainDTO) =>
      updateApprovalChain(chain.id, {
        name: chain.name,
        description: chain.description,
        resource_type: chain.resource_type,
        steps: chain.steps,
        enabled: chain.enabled,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'approval-chains'] })
      message.success('审批流程已更新')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '审批流程更新失败')),
  })

  const deleteChainMutation = useMutation({
    mutationFn: deleteApprovalChain,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'approval-chains'] })
      message.success('审批流程已删除')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '审批流程删除失败')),
  })

  const approvalMutation = useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'approve' | 'reject' }) =>
      action === 'approve' ? approveApproval(id, '审批通过') : rejectApproval(id, '审批驳回'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'approvals'] })
      queryClient.invalidateQueries({ queryKey: ['team', 'notifications'] })
      message.success('审批状态已更新')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '审批操作失败')),
  })

  const readMutation = useMutation({
    mutationFn: () => markNotificationsRead(),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['team', 'notifications'] })
      message.success(`已读 ${result.updated} 条通知`)
    },
    onError: (error) => message.error(getApiErrorMessage(error, '通知状态更新失败')),
  })

  const externalToolMutation = useMutation({
    mutationFn: (values: {
      name: string
      enabled?: boolean
      endpoint?: string
      allowed_tools?: string[]
      timeout_ms?: number
      monthly_budget?: number
      redaction_policy?: ExternalToolConfigDTO['redaction_policy']
      cost_per_call?: number
    }) => {
      if (!editingExternalTool) throw new Error('missing external tool')
      const costPerCall = Number(values.cost_per_call || 0)
      return updateExternalToolConfig(editingExternalTool.preset.provider_key, {
        name: values.name || editingExternalTool.preset.name,
        transport: 'streamable_http',
        endpoint: values.endpoint?.trim() || '',
        enabled: Boolean(values.enabled),
        allowed_tools: values.allowed_tools?.length ? values.allowed_tools : editingExternalTool.preset.default_allowed_tools,
        timeout_ms: values.timeout_ms || 5000,
        monthly_budget: values.monthly_budget || 0,
        redaction_policy: values.redaction_policy || 'summary_only',
        metadata: costPerCall > 0 ? { cost_per_call: costPerCall } : {},
      })
    },
    onSuccess: () => {
      setExternalToolOpen(false)
      setEditingExternalTool(null)
      externalToolForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['team', 'external-tools'] })
      message.success('外部数据源已保存')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '外部数据源保存失败')),
  })

  const aiConfigMutation = useMutation({
    mutationFn: (values: AIConfigFormValues) => updateAIConfig(aiConfigPayloadFromForm(values)),
    onSuccess: (result) => {
      queryClient.setQueryData(['team', 'ai-config'], result)
      aiConfigForm.setFieldsValue(aiConfigFormValues(result))
      aiConfigFormInitialized.current = true
      setAIConfigCheckResult(null)
      message.success('智能配置已保存')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '智能配置保存失败')),
  })

  const aiConfigCheckMutation = useMutation({
    mutationFn: checkAIConfig,
    onSuccess: (result) => {
      setAIConfigCheckResult(result)
      message.success('检查完成')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '检查失败')),
  })

  const createChain = (values: {
    name: string
    description?: string
    first_role: string
    second_role: string
    executive_enabled?: boolean
  }) => {
    const steps: ApprovalStepDTO[] = [
      { order: 1, name: '部门主管审批', role_code: values.first_role, required: true, condition: '' },
      { order: 2, name: '项目经理审批', role_code: values.second_role, required: true, condition: '' },
      {
        order: 3,
        name: '总经理审批',
        role_code: 'company_admin',
        required: Boolean(values.executive_enabled),
        condition: '金额 > 100 万时启用',
      },
    ]
    createChainMutation.mutate({
      name: values.name,
      description: values.description,
      resource_type: 'bid',
      steps,
      enabled: true,
    })
  }

  const openMemberEditor = (member: MemberDTO) => {
    setEditingMember(member)
    memberForm.setFieldsValue({
      name: member.user.name,
      status: member.status,
      role_codes: member.roles.map((role) => role.code),
    })
    setMemberOpen(true)
  }

  const openExternalToolEditor = (row: ExternalToolRow) => {
    setEditingExternalTool(row)
    externalToolForm.setFieldsValue(externalToolFormValues(row.preset, row.config))
    setExternalToolOpen(true)
  }

  const renderAIConfigCenter = () => {
    if (aiConfigQuery.isLoading) return <LoadingBlock />
    if (aiConfigQuery.isError || !aiConfigQuery.data) return <ErrorBlock />
    const overview = aiConfigQuery.data
    return (
      <Space direction="vertical" size={16} className="full-width">
        <div className="ai-config-summary-grid">
          <div className="ai-config-summary-item">
            <Space size={8}>
              <RobotOutlined />
              <Typography.Text strong>运行连接</Typography.Text>
            </Space>
            <div>{aiServiceStatusTag(overview)}</div>
            <Typography.Text type="secondary">
              {overview.runtime.service_reachable ? '智能服务响应正常' : overview.runtime.error_message || '智能服务暂不可达'}
            </Typography.Text>
          </div>
          <div className="ai-config-summary-item">
            <Typography.Text strong>保存配置</Typography.Text>
            <div>{aiConfigStatusTag(overview)}</div>
            <Typography.Text type="secondary">
              {overview.runtime.pending_deploy_fields.length
                ? `待切换 ${overview.runtime.pending_deploy_fields.length} 项`
                : overview.config.enabled
                  ? '当前配置一致'
                  : '保存后可统一切换'}
            </Typography.Text>
          </div>
          <div className="ai-config-summary-item">
            <Typography.Text strong>本月费用</Typography.Text>
            <Typography.Text className="data-mono">{formatEstimatedCost(overview.summary.estimated_cost)}</Typography.Text>
            <Typography.Text type="secondary">
              {overview.summary.monthly_budget ? `预算 ${formatEstimatedCost(overview.summary.monthly_budget)}` : '未设置预算'}
            </Typography.Text>
          </div>
          <div className="ai-config-summary-item">
            <Typography.Text strong>回退策略</Typography.Text>
            <div>{aiFallbackStatusTag(overview)}</div>
            <Typography.Text type="secondary">生产环境建议关闭演练回退</Typography.Text>
          </div>
        </div>

        <Form
          form={aiConfigForm}
          layout="vertical"
          onFinish={aiConfigMutation.mutate}
          onValuesChange={() => setAIConfigCheckResult(null)}
        >
          <Space direction="vertical" size={16} className="full-width">
            <Typography.Title level={4}>能力选择</Typography.Title>
            <Row gutter={[16, 0]}>
              <Col xs={24} md={8}>
                <Form.Item label="启用保存配置" name="enabled" valuePropName="checked">
                  <Switch disabled={!canWrite} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label="月度预算" name="monthly_budget">
                  <InputNumber
                    min={0}
                    max={MAX_AI_MONTHLY_BUDGET}
                    step={10}
                    addonAfter="元"
                    className="full-width"
                    disabled={!canWrite}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label="允许演练回退" name="mock_fallback_allowed" valuePropName="checked">
                  <Switch disabled={!canWrite} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={[16, 0]}>
              <Col xs={24} md={12}>
                <Form.Item label="文件理解服务" name="llm_provider" rules={[{ required: true, message: '请选择服务' }]}>
                  <Select options={aiProviderOptions} disabled={!canWrite} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label="文件理解模型" name="llm_model" rules={[{ required: true, message: '请输入模型名称' }]}>
                  <Input placeholder="gpt-4o-mini" disabled={!canWrite} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label="资料整理服务" name="embedding_provider" rules={[{ required: true, message: '请选择服务' }]}>
                  <Select options={aiProviderOptions} disabled={!canWrite} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label="资料整理模型" name="embedding_model" rules={[{ required: true, message: '请输入模型名称' }]}>
                  <Input placeholder="text-embedding-3-large" disabled={!canWrite} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label="资料匹配服务" name="rerank_provider" rules={[{ required: true, message: '请选择服务' }]}>
                  <Select options={aiProviderOptions} disabled={!canWrite} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label="资料匹配模型" name="rerank_model" rules={[{ required: true, message: '请输入模型名称' }]}>
                  <Input placeholder="gpt-4o-mini" disabled={!canWrite} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label="扫描件识别服务" name="ocr_provider" rules={[{ required: true, message: '请选择服务' }]}>
                  <Select options={aiOCROptions} disabled={!canWrite} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label="扫描件识别地址" name="ocr_endpoint">
                  <Input placeholder="https://ocr.example.com" disabled={!canWrite} />
                </Form.Item>
              </Col>
            </Row>

            <Typography.Title level={4}>能力路由</Typography.Title>
            <Table
              rowKey="task_type"
              dataSource={overview.runtime.routes}
              pagination={false}
              scroll={{ x: 920 }}
              columns={[
                { title: '能力', dataIndex: 'name', width: 150 },
                { title: '用途', dataIndex: 'capability', width: 120 },
                {
                  title: '当前使用',
                  width: 260,
                  render: (_, row) =>
                    row.track === 'ocr'
                      ? formatOCRProvider(row.active_provider, overview)
                      : formatProviderModel(row.active_provider, row.active_model, overview),
                },
                {
                  title: '保存配置',
                  width: 260,
                  render: (_, row) =>
                    row.track === 'ocr'
                      ? formatOCRProvider(row.saved_provider, overview)
                      : formatProviderModel(row.saved_provider, row.saved_model, overview),
                },
                {
                  title: '状态',
                  width: 100,
                  render: (_, row) => aiRouteStatusTag(row, overview.config.enabled),
                },
              ]}
            />

            <Typography.Title level={4}>费用估算</Typography.Title>
            <Form.List name="pricing_items">
              {(fields, { add, remove }) => (
                <Space direction="vertical" size={8} className="full-width">
                  {fields.map((field) => (
                    <Row key={field.key} gutter={[8, 0]} className="ai-config-pricing-row">
                      <Col xs={24} md={7}>
                        <Form.Item
                          {...field}
                          label="计费对象"
                          name={[field.name, 'key']}
                          rules={[{ required: true, message: '请输入计费对象' }]}
                        >
                          <Input placeholder="provider/model 或 provider/*" disabled={!canWrite} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={5}>
                        <Form.Item {...field} label="显示名称" name={[field.name, 'display_name']}>
                          <Input placeholder="模型名称" disabled={!canWrite} />
                        </Form.Item>
                      </Col>
                      <Col xs={12} md={4}>
                        <Form.Item {...field} label="输入/百万" name={[field.name, 'input_per_1m']}>
                          <InputNumber
                            min={0}
                            max={MAX_AI_MODEL_RATE}
                            step={0.01}
                            className="full-width"
                            disabled={!canWrite}
                          />
                        </Form.Item>
                      </Col>
                      <Col xs={12} md={4}>
                        <Form.Item {...field} label="输出/百万" name={[field.name, 'output_per_1m']}>
                          <InputNumber
                            min={0}
                            max={MAX_AI_MODEL_RATE}
                            step={0.01}
                            className="full-width"
                            disabled={!canWrite}
                          />
                        </Form.Item>
                      </Col>
                      <Col xs={16} md={2}>
                        <Form.Item {...field} label="币种" name={[field.name, 'currency']}>
                          <Select
                            disabled={!canWrite}
                            options={[
                              { value: 'CNY', label: 'CNY' },
                              { value: 'USD', label: 'USD' },
                            ]}
                          />
                        </Form.Item>
                      </Col>
                      <Col xs={8} md={2}>
                        <Form.Item label="操作">
                          <Button danger disabled={!canWrite} onClick={() => remove(field.name)}>
                            删除
                          </Button>
                        </Form.Item>
                      </Col>
                    </Row>
                  ))}
                  <Button disabled={!canWrite} icon={<PlusOutlined />} onClick={() => add({ currency: 'CNY' })}>
                    添加单价
                  </Button>
                </Space>
              )}
            </Form.List>

            <Typography.Title level={4}>授权状态</Typography.Title>
            <Table
              rowKey="key"
              dataSource={overview.runtime.secrets}
              pagination={false}
              scroll={{ x: 620 }}
              columns={[
                { title: '授权项', dataIndex: 'name', width: 220 },
                {
                  title: '状态',
                  dataIndex: 'configured',
                  width: 100,
                  render: (configured) => (configured ? <Tag color="green">已配置</Tag> : <Tag>未配置</Tag>),
                },
                {
                  title: '适用服务',
                  dataIndex: 'provider',
                  width: 200,
                  render: (provider) => (provider === 'ocr' ? '扫描件识别' : formatAIProvider(provider, overview)),
                },
              ]}
            />

            {aiConfigCheckResult ? (
              <>
                <Typography.Title level={4}>检查结果</Typography.Title>
                <Table
                  rowKey="key"
                  dataSource={aiConfigCheckResult.checks}
                  pagination={false}
                  scroll={{ x: 720 }}
                  columns={[
                    { title: '检查项', dataIndex: 'name', width: 180 },
                    { title: '结果', dataIndex: 'status', width: 100, render: aiCheckStatusTag },
                    { title: '说明', dataIndex: 'message', width: 320 },
                  ]}
                />
              </>
            ) : null}

            <Typography.Title level={4}>本月用量</Typography.Title>
            <Table
              rowKey={(row) => `${row.provider}/${row.model}`}
              dataSource={overview.summary.provider_usage}
              pagination={false}
              locale={{ emptyText: <EmptyBlock /> }}
              scroll={{ x: 720 }}
              columns={[
                {
                  title: '服务',
                  width: 240,
                  render: (_, row) => formatProviderModel(row.provider, row.model, overview),
                },
                { title: '调用次数', dataIndex: 'calls', width: 100 },
                { title: '预估费用', dataIndex: 'estimated_cost', width: 120, render: formatEstimatedCost },
              ]}
            />

            <Space wrap>
              <Button
                icon={<RobotOutlined />}
                onClick={() => aiConfigCheckMutation.mutate()}
                loading={aiConfigCheckMutation.isPending}
              >
                检查配置
              </Button>
              <Button type="primary" htmlType="submit" disabled={!canWrite} loading={aiConfigMutation.isPending}>
                保存配置
              </Button>
            </Space>
          </Space>
        </Form>
      </Space>
    )
  }

  return (
    <PageFrame
      module="企业管理"
      title="团队协作"
      subtitle="成员、审批流程、外部数据源、智能配置、使用记录和通知"
      tags={['团队协作', '智能配置']}
      actions={[
        canWrite ? <Button key="chain" icon={<PlusOutlined />} onClick={() => setChainOpen(true)}>
          审批流程
        </Button> : null,
        canWrite ? <Button key="invite" type="primary" icon={<TeamOutlined />} onClick={() => setInviteOpen(true)}>
          邀请成员
        </Button> : null,
      ]}
    >
      <Tabs
        activeKey={activeTab}
        onChange={(tab) => setSearchParams({ tab })}
        items={[
          {
            key: 'members',
            label: '成员',
            children: membersQuery.isLoading ? (
              <LoadingBlock />
            ) : membersQuery.isError ? (
              <ErrorBlock />
            ) : membersQuery.data?.length ? (
              <Table
                rowKey="id"
                dataSource={membersQuery.data}
                scroll={{ x: 760 }}
                columns={[
                  { title: '姓名', dataIndex: ['user', 'name'], width: 120 },
                  { title: '邮箱', dataIndex: ['user', 'email'], width: 220 },
                  {
                    title: '角色',
                    width: 220,
                    render: (_, record) => (
                      <Space wrap size={[4, 4]}>
                        {record.roles.map((role) => <Tag key={role.id}>{role.name}</Tag>)}
                      </Space>
                    ),
                  },
                  { title: '状态', dataIndex: 'status', width: 100, render: statusTag },
                  {
                    title: '操作',
                    width: 150,
                    render: (_, record) => canWrite ? (
                      <Space size={8} wrap={false}>
                        <Button size="small" icon={<EditOutlined />} onClick={() => openMemberEditor(record)}>
                          编辑
                        </Button>
                        <Popconfirm title="禁用该成员" onConfirm={() => deleteMemberMutation.mutate(record.id)}>
                          <Button size="small" danger icon={<DeleteOutlined />} loading={deleteMemberMutation.isPending}>
                            禁用
                          </Button>
                        </Popconfirm>
                      </Space>
                    ) : '-',
                  },
                ]}
              />
            ) : (
              <EmptyBlock />
            ),
          },
          {
            key: 'approvals',
            label: '审批',
            children: (
              <Space direction="vertical" size={16} className="full-width">
                <Row gutter={[16, 16]}>
                  <Col xs={24} xl={14}>
                    <Typography.Title level={4}>待办审批</Typography.Title>
                    {approvalsQuery.isLoading && <LoadingBlock />}
                    {approvalsQuery.isError && <ErrorBlock />}
                    {!approvalsQuery.isLoading && !approvalsQuery.isError && !approvalsQuery.data?.length && <EmptyBlock />}
                    {!approvalsQuery.isLoading && !approvalsQuery.isError && Boolean(approvalsQuery.data?.length) && (
                      <Table
                        rowKey="id"
                        dataSource={approvalsQuery.data}
                        scroll={{ x: 840 }}
                        columns={[
                          { title: '审批标题', dataIndex: 'title', width: 220 },
                          { title: '标书', dataIndex: 'bid_title', width: 220 },
                          { title: '提交人', dataIndex: 'submitted_by_name', width: 110, render: (value) => value || '-' },
                          { title: '当前环节', dataIndex: 'current_step', width: 110, render: (value: number) => `第 ${value} 级` },
                          { title: '状态', dataIndex: 'status', width: 100, render: statusTag },
                          {
                            title: '操作',
                            width: 150,
                            render: (_, row) =>
                              row.status === 'pending' && canWrite ? (
                                <Space size={8} wrap={false}>
                                  <Button
                                    size="small"
                                    icon={<CheckOutlined />}
                                    loading={approvalMutation.isPending}
                                    onClick={() => approvalMutation.mutate({ id: row.id, action: 'approve' })}
                                  >
                                    通过
                                  </Button>
                                  <Button
                                    size="small"
                                    danger
                                    icon={<CloseOutlined />}
                                    loading={approvalMutation.isPending}
                                    onClick={() => approvalMutation.mutate({ id: row.id, action: 'reject' })}
                                  >
                                    驳回
                                  </Button>
                                </Space>
                              ) : (
                                '-'
                              ),
                          },
                        ]}
                      />
                    )}
                  </Col>
                  <Col xs={24} xl={10}>
                    <Typography.Title level={4}>审批流程</Typography.Title>
                    {chainsQuery.isLoading && <LoadingBlock />}
                    {chainsQuery.isError && <ErrorBlock />}
                    {!chainsQuery.isLoading && !chainsQuery.isError && !chainsQuery.data?.length && <EmptyBlock />}
                    {!chainsQuery.isLoading && !chainsQuery.isError && Boolean(chainsQuery.data?.length) && (
                      <Table
                        rowKey="id"
                        dataSource={chainsQuery.data}
                        pagination={false}
                        scroll={{ x: 640 }}
                        columns={[
                          { title: '名称', dataIndex: 'name', width: 160 },
                          {
                            title: '流程',
                            width: 300,
                            render: (_, row) =>
                              (
                                <Space wrap size={[4, 4]}>
                                  {row.steps.map((step) => (
                                    <Tag key={step.order} color={step.required ? 'blue' : 'default'}>
                                      {step.order}.{step.name}
                                    </Tag>
                                  ))}
                                </Space>
                              ),
                          },
                          {
                            title: '启用',
                            dataIndex: 'enabled',
                            width: 80,
                            render: (enabled, row) => (
                              <Switch size="small" checked={enabled} disabled={!canWrite} onChange={(checked) => updateChainMutation.mutate({ ...row, enabled: checked })} />
                            ),
                          },
                          {
                            title: '操作',
                            width: 80,
                            render: (_, row) => canWrite ? (
                              <Popconfirm title="删除审批流程" onConfirm={() => deleteChainMutation.mutate(row.id)}>
                                <Button size="small" danger icon={<DeleteOutlined />} />
                              </Popconfirm>
                            ) : '-',
                          },
                        ]}
                      />
                    )}
                  </Col>
                </Row>
              </Space>
            ),
          },
          {
            key: 'external-tools',
            label: '外部数据源',
            children:
              externalCatalogQuery.isLoading || externalToolsQuery.isLoading ? (
                <LoadingBlock />
              ) : externalCatalogQuery.isError || externalToolsQuery.isError ? (
                <ErrorBlock />
              ) : (
                <Space direction="vertical" size={16} className="full-width">
                  <Table
                    rowKey={(row) => row.preset.provider_key}
                    dataSource={externalToolRows}
                    scroll={{ x: 1180 }}
                    pagination={false}
                    locale={{ emptyText: <EmptyBlock /> }}
                    columns={[
                      {
                        title: '数据源',
                        width: 260,
                        render: (_, row) => (
                          <Space direction="vertical" size={4} className="full-width">
                            <Space size={8} wrap={false}>
                              <CloudServerOutlined />
                              <Typography.Text strong>{row.preset.name}</Typography.Text>
                            </Space>
                            <Typography.Text type="secondary">{row.preset.category}</Typography.Text>
                            <Typography.Paragraph className="external-tool-description">
                              {row.preset.description}
                            </Typography.Paragraph>
                          </Space>
                        ),
                      },
                      {
                        title: '用途',
                        width: 260,
                        render: (_, row) => (
                          <Space wrap size={[4, 4]}>
                            {row.preset.recommended_use.map((item) => (
                              <Tag key={item}>{item}</Tag>
                            ))}
                          </Space>
                        ),
                      },
                      {
                        title: '数据边界',
                        width: 300,
                        render: (_, row) => (
                          <Space direction="vertical" size={4} className="full-width">
                            {row.preset.data_boundary.map((item) => (
                              <Typography.Text key={item} type="secondary">
                                {item}
                              </Typography.Text>
                            ))}
                          </Space>
                        ),
                      },
                      {
                        title: '配置状态',
                        width: 230,
                        render: (_, row) => (
                          <Space direction="vertical" size={6} className="full-width">
                            <Space size={6} wrap>
                              {row.config?.enabled ? <Tag color="green">已启用</Tag> : <Tag>未启用</Tag>}
                              {row.preset.read_only ? <Tag color="blue">只读</Tag> : null}
                              {row.preset.strict_allowed_tools ? <Tag color="gold">受控调用</Tag> : null}
                            </Space>
                            <Typography.Text className="external-tool-muted-text" type="secondary">
                              授权状态 {externalToolAuthorizationLabel(row.preset)}
                            </Typography.Text>
                            <Typography.Text type="secondary">
                              已选能力 {row.config?.allowed_tools?.length || row.preset.default_allowed_tools.length} 项
                            </Typography.Text>
                          </Space>
                        ),
                      },
                      {
                        title: '操作',
                        width: 170,
                        render: (_, row) => (
                          <Space size={8} wrap={false}>
                            <Button
                              size="small"
                              icon={<SettingOutlined />}
                              disabled={!canWrite}
                              onClick={() => openExternalToolEditor(row)}
                            >
                              配置
                            </Button>
                            <Button
                              size="small"
                              icon={<ApiOutlined />}
                              href={row.preset.source_url}
                              target="_blank"
                              rel="noreferrer"
                            >
                              来源
                            </Button>
                          </Space>
                        ),
                      },
                    ]}
                  />
                  <Typography.Title level={4}>调用记录</Typography.Title>
                  {externalToolAuditQuery.isLoading ? (
                    <LoadingBlock />
                  ) : externalToolAuditQuery.isError ? (
                    <ErrorBlock />
                  ) : (
                    <Table
                      rowKey="id"
                      dataSource={externalToolAuditQuery.data ?? []}
                      locale={{ emptyText: <EmptyBlock /> }}
                      scroll={{ x: 960 }}
                      columns={[
                        {
                          title: '数据源',
                          dataIndex: 'tool_provider',
                          width: 150,
                          render: (value) => formatExternalToolProviderName(value, externalToolDisplayNameMap),
                        },
                        { title: '能力', dataIndex: 'tool_name', width: 180, render: formatExternalToolName },
                        { title: '状态', dataIndex: 'status', width: 90, render: statusTag },
                        { title: '费用', dataIndex: 'estimated_cost', width: 90, render: formatEstimatedCost },
                        { title: '耗时', dataIndex: 'latency_ms', width: 100, render: formatLatency },
                        {
                          title: '提交内容',
                          dataIndex: 'request_summary',
                          width: 140,
                          render: formatExternalToolAuditRequest,
                        },
                        {
                          title: '结果摘要',
                          width: 220,
                          ellipsis: true,
                          render: (_, row) => formatExternalToolAuditResult(row),
                        },
                        { title: '时间', dataIndex: 'created_at', width: 180, render: formatTime },
                      ]}
                    />
                  )}
                </Space>
              ),
          },
          {
            key: 'ai-config',
            label: '智能配置',
            children: renderAIConfigCenter(),
          },
          {
            key: 'logs',
            label: '使用记录',
            children: aiLogsQuery.isLoading ? (
              <LoadingBlock />
            ) : aiLogsQuery.isError ? (
              <ErrorBlock />
            ) : aiLogsQuery.data?.length ? (
              <Table
                rowKey="id"
                dataSource={aiLogsQuery.data}
                scroll={{ x: 920 }}
                columns={[
                  { title: '事项', dataIndex: 'task_type', width: 150, render: (value) => taskTypeLabels[value] || '智能处理' },
                  { title: '处理方式', width: 130, render: () => '平台智能处理' },
                  { title: '调用人', dataIndex: 'user_name', width: 120, render: (value) => value || '系统' },
                  {
                    title: '处理量',
                    width: 110,
                    render: (_, row) => formatProcessingVolume(row.input_tokens, row.output_tokens),
                  },
                  {
                    title: '预估费用',
                    dataIndex: 'estimated_cost',
                    width: 110,
                    render: formatEstimatedCost,
                  },
                  { title: '处理时长', dataIndex: 'latency_ms', width: 110, render: formatLatency },
                  { title: '状态', dataIndex: 'status', width: 90, render: statusTag },
                  {
                    title: '关联内容',
                    width: 120,
                    render: (_, row) => formatBizRef(row.biz_ref),
                  },
                  { title: '时间', dataIndex: 'created_at', width: 190, render: formatTime },
                ]}
              />
            ) : (
              <EmptyBlock />
            ),
          },
          {
            key: 'notifications',
            label: '通知',
            children: notificationsQuery.isLoading ? (
              <LoadingBlock />
            ) : notificationsQuery.isError ? (
              <ErrorBlock />
            ) : (
              <Space direction="vertical" size={16} className="full-width">
                <Button loading={readMutation.isPending} onClick={() => readMutation.mutate()}>
                  全部标记已读
                </Button>
                <Table
                  rowKey="id"
                  dataSource={notificationsQuery.data ?? []}
                  locale={{ emptyText: <EmptyBlock /> }}
                  scroll={{ x: 820 }}
                  columns={[
                    { title: '标题', dataIndex: 'title', width: 220, render: (value, row) => <Tag color={row.read_at ? 'default' : 'blue'}>{value}</Tag> },
                    { title: '内容', dataIndex: 'body', width: 360, ellipsis: true },
                    { title: '时间', dataIndex: 'created_at', width: 160, render: formatTime },
                    { title: '状态', dataIndex: 'read_at', width: 80, render: (value) => (value ? <Tag>已读</Tag> : <Tag color="blue">未读</Tag>) },
                  ]}
                />
              </Space>
            ),
          },
        ]}
      />
      <Modal
        open={externalToolOpen}
        title={editingExternalTool ? `配置${editingExternalTool.preset.name}` : '外部数据源配置'}
        width={720}
        onCancel={() => {
          setExternalToolOpen(false)
          setEditingExternalTool(null)
          externalToolForm.resetFields()
        }}
        onOk={externalToolForm.submit}
        confirmLoading={externalToolMutation.isPending}
      >
        {editingExternalTool ? (
          <Form form={externalToolForm} layout="vertical" onFinish={externalToolMutation.mutate}>
            <Row gutter={16}>
              <Col xs={24} md={12}>
                <Form.Item label="名称" name="name" rules={[{ required: true, message: '名称必填' }]}>
                  <Input />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label="启用" name="enabled" valuePropName="checked">
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item
              label="访问地址"
              name="endpoint"
              rules={[
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!getFieldValue('enabled') || String(value || '').trim()) {
                      return Promise.resolve()
                    }
                    return Promise.reject(new Error('启用时必须填写访问地址'))
                  },
                }),
              ]}
            >
              <Input placeholder="粘贴服务商提供的访问地址" />
            </Form.Item>
            <Form.Item
              label="启用能力"
              name="allowed_tools"
              rules={[{ required: true, type: 'array', min: 1, message: '至少选择一个能力' }]}
            >
              <Select
                mode={editingExternalTool.preset.strict_allowed_tools ? 'multiple' : 'tags'}
                options={editingExternalTool.preset.default_allowed_tools.map((tool) => ({
                  value: tool,
                  label: formatExternalToolName(tool),
                }))}
              />
            </Form.Item>
            <Row gutter={16}>
              <Col xs={24} md={8}>
                <Form.Item label="超时时间" name="timeout_ms" rules={[{ required: true, message: '超时时间必填' }]}>
                  <InputNumber min={500} max={60000} step={500} addonAfter="毫秒" className="full-width" />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label="月度预算" name="monthly_budget">
                  <InputNumber
                    min={0}
                    max={MAX_EXTERNAL_TOOL_MONTHLY_BUDGET}
                    step={10}
                    addonAfter="元"
                    className="full-width"
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label="单次估算" name="cost_per_call">
                  <InputNumber
                    min={0}
                    max={MAX_EXTERNAL_TOOL_COST_PER_CALL}
                    step={0.01}
                    addonAfter="元"
                    className="full-width"
                  />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item label="脱敏策略" name="redaction_policy" rules={[{ required: true, message: '脱敏策略必填' }]}>
              <Select
                options={[
                  { value: 'summary_only', label: '仅保存摘要' },
                  { value: 'no_sensitive', label: '过滤敏感内容' },
                ]}
              />
            </Form.Item>
            <Space direction="vertical" size={6} className="full-width">
              <Typography.Text className="external-tool-muted-text" type="secondary">
                授权说明：{externalToolAuthorizationHint(editingExternalTool.preset)}
              </Typography.Text>
              {editingExternalTool.preset.data_boundary.map((item) => (
                <Typography.Text key={item} type="secondary">
                  {item}
                </Typography.Text>
              ))}
            </Space>
          </Form>
        ) : null}
      </Modal>
      <Modal open={inviteOpen} title="邀请成员" onCancel={() => setInviteOpen(false)} onOk={inviteForm.submit} confirmLoading={inviteMutation.isPending}>
        <Form form={inviteForm} layout="vertical" initialValues={{ role_code: 'viewer' }} onFinish={inviteMutation.mutate}>
          <Form.Item label="姓名" name="name" rules={[{ required: true, message: '姓名必填' }]}>
            <Input placeholder="新成员姓名" />
          </Form.Item>
          <Form.Item label="邮箱" name="email" rules={[{ required: true, message: '邮箱必填' }]}>
            <Input placeholder="member@example.com" />
          </Form.Item>
          <Form.Item label="初始密码" name="initial_password" rules={[{ required: true, min: 8, message: '密码至少 8 位' }]}>
            <Input.Password placeholder="至少 8 位" />
          </Form.Item>
          <Form.Item label="角色" name="role_code">
            <Select options={roleOptions} loading={rolesQuery.isLoading} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        open={memberOpen}
        title="成员设置"
        onCancel={() => {
          setMemberOpen(false)
          setEditingMember(null)
        }}
        onOk={memberForm.submit}
        confirmLoading={updateMemberMutation.isPending}
      >
        <Form form={memberForm} layout="vertical" onFinish={updateMemberMutation.mutate}>
          <Form.Item label="姓名" name="name" rules={[{ required: true, message: '姓名必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="状态" name="status" rules={[{ required: true, message: '状态必填' }]}>
            <Select
              options={[
                { value: 'active', label: '正常' },
                { value: 'invited', label: '已邀请' },
                { value: 'disabled', label: '已禁用' },
              ]}
            />
          </Form.Item>
          <Form.Item label="角色" name="role_codes" rules={[{ required: true, message: '至少选择一个角色' }]}>
            <Select mode="multiple" options={roleOptions} loading={rolesQuery.isLoading} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal open={chainOpen} title="审批流程配置" onCancel={() => setChainOpen(false)} onOk={chainForm.submit} confirmLoading={createChainMutation.isPending}>
        <Form
          form={chainForm}
          layout="vertical"
          initialValues={{ name: '标书两级审批流程', first_role: 'department_admin', second_role: 'project_manager', executive_enabled: false }}
          onFinish={createChain}
        >
          <Form.Item label="流程名称" name="name" rules={[{ required: true, message: '名称必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="说明" name="description">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item label="第一级审批角色" name="first_role">
            <Select options={roleOptions} loading={rolesQuery.isLoading} />
          </Form.Item>
          <Form.Item label="第二级审批角色" name="second_role">
            <Select options={roleOptions} loading={rolesQuery.isLoading} />
          </Form.Item>
          <Form.Item label="启用总经理审批" name="executive_enabled" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  )
}
