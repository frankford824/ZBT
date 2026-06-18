import {
  CheckOutlined,
  CompressOutlined,
  CopyOutlined,
  DeleteOutlined,
  DiffOutlined,
  DownloadOutlined,
  EditOutlined,
  ExpandAltOutlined,
  FilePdfOutlined,
  FileZipOutlined,
  PlusCircleOutlined,
  PlusOutlined,
  SaveOutlined,
  SafetyCertificateOutlined,
  SyncOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import { zodResolver } from '@hookform/resolvers/zod'
import { EditorContent, useEditor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  Form,
  Input,
  Modal,
  Popconfirm,
  Progress,
  Radio,
  Row,
  Segmented,
  Select,
  Space,
  Steps,
  Table,
  Tabs,
  Tag,
  Timeline,
  Tooltip,
  Typography,
  Upload,
  App as AntApp,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { type SetStateAction, useEffect, useMemo, useRef, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { z } from 'zod'
import {
  acceptChapter,
  batchUpdateBidRequirementCoverage,
  chapterAiAction,
  confirmBidParseResult,
  confirmFileUpload,
  createBid,
  createBidExport,
  createPresignedUpload,
  deleteBid,
  exportBidRequirements,
  fetchAITask,
  fetchBidChapters,
  fetchBid,
  fetchBidExport,
  fetchBidExports,
  fetchBidGenerationJobs,
  fetchBidMaterialSelection,
  fetchBidParseResult,
  fetchBidParts,
  fetchBidRequirementCoverageHistory,
  fetchBidRequirements,
  fetchBidTemplates,
  fetchBids,
  fetchChapterDiff,
  fetchChapterVersions,
  fetchFileURL,
  fetchKnowledgeDocumentPreview,
  getApiErrorMessage,
  generateBid,
  generateBidOutline,
  cancelBidGenerationJob,
  pauseBidGenerationJob,
  parseBidTender,
  regenerateChapter,
  resumeBidGenerationJob,
  submitBidForApproval,
  updateBidMaterialSelection,
  updateBidRequirementCoverage,
  updateBidPartOutline,
  updateChapterContent,
  uploadBidTenderFile,
  uploadToPresignedUrl,
  useBidTemplate,
  type AITaskDTO,
  type BidChapterDTO,
  type BidChapterVersionDTO,
  type BidDocumentDTO,
  type BidExportDTO,
  type BidGenerationJobDTO,
  type BidGenerationSnapshotDTO,
  type BidRequirementCoverageEventDTO,
  type BidRequirementExportFormat,
  type BidRequirementItemDTO,
  type BidTemplateDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { fileOpenErrorMessage, openFileUrl } from '../../shared/files/openFileUrl'
import { formatBytes, isUploadFileTooLarge, uploadSizeLimitMessage } from '../../shared/files/uploadLimits'
import { formatDateTime } from '../../shared/format/date'
import { useCanAccess } from '../../shared/permissions/permissions'
import { openSse } from '../../shared/sse/client'

const bidSchema = z.object({
  projectName: z.string().min(1, '项目名称必填'),
  tenderOrg: z.string().optional(),
  deadline: z.string().optional(),
  budget: z.string().optional(),
  bidType: z.enum(['combined', 'separated', 'custom']),
})

type BidFormValues = z.infer<typeof bidSchema>

type OutlineDraftChapter = {
  id?: string
  title: string
  plain_text: string
  sort_order: number
}

type DraftState<T> = {
  sourceKey: string
  value: T
}

type RequirementFilter = 'all' | 'mandatory' | 'review' | 'covered'
type RequirementEvidenceFilter = 'all' | 'missing_evidence' | 'missing_source' | 'complete'

type ParseConfirmDraft = {
  projectName: string
  deadline: string
  bidType: BidFormValues['bidType']
  qualificationRequirements: string
  scoringPoints: string
  invalidClauseRisks: string
}

export function BidNewPage() {
  const navigate = useNavigate()
  const { message } = AntApp.useApp()
  const canWrite = useCanAccess('bid', 'full')
  const mutation = useMutation({
    mutationFn: createBid,
    onSuccess: (bid) => {
      message.success('标书已创建')
      navigate(`/bids/${bid.id}/wizard?step=1`)
    },
    onError: (error) => message.error(getApiErrorMessage(error, '创建标书失败')),
  })
  const {
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<BidFormValues>({
    resolver: zodResolver(bidSchema),
    defaultValues: {
      projectName: '',
      tenderOrg: '',
      deadline: '',
      budget: '',
      bidType: 'separated',
    },
  })

  return (
    <PageFrame
      module="标书生成"
      title="新建标书"
      subtitle="创建综合标书、分离标书或自定义组合"
      tags={['新建标书']}
      permission={canWrite}
      bare
    >
      <Row gutter={16}>
        <Col xs={24} lg={15}>
          <Form
            layout="vertical"
            onFinish={handleSubmit((values) =>
              mutation.mutate({
                title: values.projectName,
                project_name: values.projectName,
                bid_type: values.bidType,
              }),
            )}
          >
            <Form.Item
              label="项目名称"
              htmlFor="bid-project-name"
              validateStatus={errors.projectName ? 'error' : undefined}
              help={errors.projectName?.message}
            >
              <Controller
                name="projectName"
                control={control}
                render={({ field }) => (
                  <Input {...field} id="bid-project-name" placeholder="某市智慧交通综合治理平台建设项目" />
                )}
              />
            </Form.Item>
            <Form.Item label="招标单位" htmlFor="bid-tender-org">
              <Controller
                name="tenderOrg"
                control={control}
                render={({ field }) => <Input {...field} id="bid-tender-org" placeholder="某市交通运输局" />}
              />
            </Form.Item>
            <Row gutter={12}>
              <Col span={12}>
                <Form.Item label="投标截止日期" htmlFor="bid-deadline">
                  <Controller
                    name="deadline"
                    control={control}
                    render={({ field }) => <Input {...field} id="bid-deadline" placeholder="2026-06-18" />}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="项目预算" htmlFor="bid-budget">
                  <Controller
                    name="budget"
                    control={control}
                    render={({ field }) => <Input {...field} id="bid-budget" placeholder="12800000" />}
                  />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item label="标书类型" htmlFor="bid-type">
              <Controller
                name="bidType"
                control={control}
                render={({ field }) => (
                  <Radio.Group {...field} id="bid-type">
                    <Radio.Button value="combined">综合标书</Radio.Button>
                    <Radio.Button value="separated">分离标书</Radio.Button>
                    <Radio.Button value="custom">自定义组合</Radio.Button>
                  </Radio.Group>
                )}
              />
            </Form.Item>
            <Space>
              <Button>
                <Link to="/bids">取消</Link>
              </Button>
              <Button type="primary" htmlType="submit" icon={<PlusOutlined />} loading={mutation.isPending}>
                开始生成
              </Button>
            </Space>
          </Form>
        </Col>
        <Col xs={24} lg={9}>
          <Card title="编制流程">
            <Timeline
              items={[
                { color: 'blue', children: '填写项目与标书类型' },
                { color: 'blue', children: '准备技术标、商务标或综合标' },
                { color: 'gray', children: '上传招标文件并解读重点' },
                { color: 'gray', children: '进入编制流程' },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </PageFrame>
  )
}

const bidStatusFilters: Record<string, string[]> = {
  全部: [],
  编制中: ['draft', 'editing'],
  审批中: ['in_review'],
  已完成: ['approved', 'submitted'],
}

export function BidListPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState('全部')
  const canWrite = useCanAccess('bid', 'full')
  const bids = useQuery({
    queryKey: ['bids'],
    queryFn: fetchBids,
  })
  const approvalMutation = useMutation({
    mutationFn: submitBidForApproval,
    onSuccess: async () => {
      message.success('已提交审批')
      await queryClient.invalidateQueries({ queryKey: ['bids'] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '提交审批失败')),
  })
  const archiveMutation = useMutation({
    mutationFn: deleteBid,
    onSuccess: async () => {
      message.success('标书已归档')
      await queryClient.invalidateQueries({ queryKey: ['bids'] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '标书归档失败')),
  })
  if (bids.isLoading) return <LoadingBlock />
  if (bids.isError) return <ErrorBlock />

  const activeStatuses = bidStatusFilters[statusFilter] ?? []
  const visibleBids = (bids.data ?? []).filter(
    (bid) => activeStatuses.length === 0 || activeStatuses.includes(bid.status),
  )

  return (
    <PageFrame
      module="标书生成"
      title="我的标书"
      subtitle="状态筛选、审批和编辑入口"
      tags={['标书列表']}
      actions={[
        canWrite ? <Button key="new" type="primary" icon={<PlusOutlined />}>
          <Link to="/bids/new">新建标书</Link>
        </Button> : null,
      ]}
    >
      <Space direction="vertical" size={16} className="full-width">
        <Segmented
          options={Object.keys(bidStatusFilters)}
          value={statusFilter}
          onChange={(value) => setStatusFilter(String(value))}
        />
        {visibleBids.length === 0 ? (
          <EmptyBlock
            description={statusFilter === '全部' ? '还没有标书，点击右上角新建' : `没有「${statusFilter}」状态的标书`}
          />
        ) : (
          <Table
            rowKey="id"
            dataSource={visibleBids}
            scroll={{ x: 1120 }}
            columns={[
              {
                title: '标书名称',
                dataIndex: 'title',
                width: 260,
                render: (value: string, row: BidDocumentDTO) => <Link to={`/bids/${row.id}/editor`}>{value}</Link>,
              },
              {
                title: '项目',
                dataIndex: 'project_name',
                width: 280,
                render: (value: string) => value || '未关联项目',
              },
              { title: '类型', dataIndex: 'bid_type', width: 92, render: bidTypeLabel },
              { title: '状态', dataIndex: 'status', width: 96, render: bidStatusLabel },
              {
                title: '更新时间',
                dataIndex: 'updated_at',
                width: 146,
                render: (value: string) => <span className="data-mono">{formatDateTime(value)}</span>,
              },
              {
                title: '操作',
                width: 246,
                render: (_, row: BidDocumentDTO) => (
                  <Space size={10} wrap={false}>
                    {canWrite ? <Link to={`/bids/${row.id}/wizard?step=5`}>生成</Link> : null}
                    <Link to={`/bids/${row.id}/editor`}>{canWrite ? '编辑' : '查看'}</Link>
                    {canWrite ? <Button
                      type="link"
                      size="small"
                      disabled={row.status === 'in_review' || row.status === 'approved'}
                      loading={approvalMutation.isPending && approvalMutation.variables === row.id}
                      onClick={() => approvalMutation.mutate(row.id)}
                    >
                      提交审批
                    </Button> : null}
                    {canWrite ? <Popconfirm title="归档该标书" onConfirm={() => archiveMutation.mutate(row.id)}>
                      <Button
                        type="link"
                        size="small"
                        danger
                        icon={<DeleteOutlined />}
                        disabled={row.status === 'in_review' || row.status === 'approved'}
                        loading={archiveMutation.isPending && archiveMutation.variables === row.id}
                      >
                        归档
                      </Button>
                    </Popconfirm> : null}
                  </Space>
                ),
              },
            ]}
          />
        )}
      </Space>
    </PageFrame>
  )
}

export function BidTemplatesPage() {
  const navigate = useNavigate()
  const { message } = AntApp.useApp()
  const canWrite = useCanAccess('bid', 'full')
  const templates = useQuery({
    queryKey: ['bid-templates'],
    queryFn: fetchBidTemplates,
  })
  const mutation = useMutation({
    mutationFn: useBidTemplate,
    onSuccess: ({ bid }) => {
      message.success('已按模板创建标书')
      navigate(`/bids/${bid.id}/wizard?step=1`)
    },
    onError: (error) => message.error(getApiErrorMessage(error, '模板使用失败')),
  })

  const renderSections = (template: BidTemplateDTO) => {
    const sections = template.content.sections
    if (!Array.isArray(sections)) {
      return null
    }
    return sections.slice(0, 4).map((section) => (
      <Tag key={String(section)} color="geekblue">
        {String(section)}
      </Tag>
    ))
  }

  return (
    <PageFrame
      module="标书生成"
      title="标书模板"
      subtitle="行业分类、预览和使用"
      tags={['模板管理']}
      bare
    >
      {templates.isLoading && <LoadingBlock />}
      {templates.isError && <ErrorBlock />}
      {!templates.isLoading && !templates.isError && (templates.data?.length ?? 0) === 0 && <EmptyBlock />}
      {!templates.isLoading && !templates.isError && Boolean(templates.data?.length) && (
        <Row gutter={[16, 16]}>
          {templates.data?.map((template) => (
            <Col xs={24} md={12} xl={6} key={template.id}>
              <Card
                title={template.name}
                actions={
                  canWrite
                    ? [
                        <Button
                          key="use"
                          type="link"
                          loading={mutation.isPending && mutation.variables?.templateId === template.id}
                          onClick={() =>
                            mutation.mutate({
                              templateId: template.id,
                              title: `${template.name}生成标书`,
                            })
                          }
                        >
                          使用模板
                        </Button>,
                        <Link key="blank" to="/bids/new">
                          新建空白
                        </Link>,
                      ]
                    : undefined
                }
              >
                <Space direction="vertical" size={8}>
                  <Space wrap>
                    <Tag color={template.bid_type === 'combined' ? 'green' : 'blue'}>
                      {template.bid_type === 'combined' ? '综合标' : template.bid_type === 'separated' ? '分册标' : '自定义'}
                    </Tag>
                    <Tag>{template.category}</Tag>
                    <Typography.Text type="secondary">{template.version}</Typography.Text>
                  </Space>
                  <Typography.Paragraph type="secondary" ellipsis={{ rows: 2 }}>
                    {template.description}
                  </Typography.Paragraph>
                  <Space wrap>{renderSections(template)}</Space>
                  <Typography.Text type="secondary">使用 {template.usage_count} 次</Typography.Text>
                </Space>
              </Card>
            </Col>
          ))}
        </Row>
      )}
    </PageFrame>
  )
}

export function BidWizardPage() {
  const { bidId = '' } = useParams()
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canWrite = useCanAccess('bid', 'full')
  const [searchParams, setSearchParams] = useSearchParams()
  const step = Number(searchParams.get('step') || '1')
  const current = Math.min(Math.max(step, 1), 7) - 1
  const steps = ['上传招标文件', '文件解读', '目录大纲', '素材选择', '生成正文', '标书编辑', '定稿导出']
  const [tenderFile, setTenderFile] = useState<File | null>(null)
  const [outlineDraftState, setOutlineDraftState] = useState<DraftState<Record<string, OutlineDraftChapter[]>> | null>(null)
  const [materialDraftState, setMaterialDraftState] = useState<
    DraftState<{ selectedRefs: unknown[]; notes: string }> | null
  >(null)
  const [parseConfirmDraftState, setParseConfirmDraftState] = useState<DraftState<ParseConfirmDraft> | null>(null)
  const [requirementFilter, setRequirementFilter] = useState<RequirementFilter>('all')
  const [requirementEvidenceFilter, setRequirementEvidenceFilter] = useState<RequirementEvidenceFilter>('all')
  const [selectedRequirementKeys, setSelectedRequirementKeys] = useState<string[]>([])
  const [historyRequirement, setHistoryRequirement] = useState<ParseRequirementRow | null>(null)
  const bid = useQuery({
    queryKey: ['bid', bidId],
    queryFn: () => fetchBid(bidId),
    enabled: Boolean(bidId),
  })
  const parts = useQuery({
    queryKey: ['bid-parts', bidId],
    queryFn: () => fetchBidParts(bidId),
    enabled: Boolean(bidId),
  })
  const chapters = useQuery({
    queryKey: ['bid-chapters', bidId],
    queryFn: () => fetchBidChapters(bidId),
    enabled: Boolean(bidId),
  })
  const parseResult = useQuery({
    queryKey: ['bid-parse-result', bidId],
    queryFn: () => fetchBidParseResult(bidId),
    enabled: Boolean(bidId),
    refetchInterval: (query) => {
      const status = query.state.data?.status
      return status === 'queued' || status === 'processing' ? 2000 : false
    },
  })
  const parseConfirmSourceKey = parseResult.data
    ? `${parseResult.data.id}:${parseResult.data.updated_at}:${parseResult.data.status}`
    : ''
  const parseConfirmServerDraft = useMemo(
    () => parseConfirmDraftFromStructuredResult(parseResult.data?.structured_result),
    [parseResult.data?.structured_result],
  )
  const parseConfirmDraft =
    parseConfirmDraftState?.sourceKey === parseConfirmSourceKey
      ? parseConfirmDraftState.value
      : parseConfirmServerDraft
  const parseConfirmEditedFields = parseConfirmChangedFieldLabels(parseConfirmServerDraft, parseConfirmDraft)
  const canEditParseConfirm =
    canWrite && Boolean(parseResult.data) && ['ready', 'confirmed'].includes(parseResult.data?.status ?? 'queued')
  const setParseConfirmDraftField = (field: keyof ParseConfirmDraft, value: ParseConfirmDraft[keyof ParseConfirmDraft]) => {
    setParseConfirmDraftState((currentDraft) => {
      const currentValue =
        currentDraft?.sourceKey === parseConfirmSourceKey ? currentDraft.value : parseConfirmServerDraft
      return { sourceKey: parseConfirmSourceKey, value: { ...currentValue, [field]: value } }
    })
  }
  const bidRequirements = useQuery({
    queryKey: ['bid-requirements', bidId],
    queryFn: () => fetchBidRequirements(bidId),
    enabled: Boolean(bidId),
    refetchInterval: () =>
      parseResult.data?.status === 'queued' || parseResult.data?.status === 'processing' ? 2000 : false,
  })
  const materialSelection = useQuery({
    queryKey: ['bid-material-selection', bidId],
    queryFn: () => fetchBidMaterialSelection(bidId),
    enabled: Boolean(bidId),
  })
  const outlineSourceKey = useMemo(() => {
    if (!parts.data || !chapters.data) return ''
    return [
      ...parts.data.map((part) => `${part.id}:${part.updated_at ?? ''}:${part.title}`),
      ...chapters.data.map((chapter) => `${chapter.id}:${chapter.updated_at}:${chapter.title}:${chapter.sort_order}`),
    ].join('|')
  }, [parts.data, chapters.data])
  const outlineServerDrafts = useMemo(() => {
    if (!parts.data || !chapters.data) return {}
    const next: Record<string, OutlineDraftChapter[]> = {}
    for (const part of parts.data) {
      next[part.id] = chapters.data
        .filter((chapter) => chapter.bid_part_id === part.id)
        .map((chapter) => ({
          id: chapter.id,
          title: chapter.title,
          plain_text: chapter.plain_text,
          sort_order: chapter.sort_order,
        }))
    }
    return next
  }, [parts.data, chapters.data])
  const outlineDrafts =
    outlineDraftState?.sourceKey === outlineSourceKey ? outlineDraftState.value : outlineServerDrafts
  const setOutlineDrafts = (updater: SetStateAction<Record<string, OutlineDraftChapter[]>>) => {
    setOutlineDraftState((currentDraft) => {
      const currentValue = currentDraft?.sourceKey === outlineSourceKey ? currentDraft.value : outlineServerDrafts
      const value = typeof updater === 'function' ? updater(currentValue) : updater
      return { sourceKey: outlineSourceKey, value }
    })
  }
  const materialSourceKey = materialSelection.data ? `${materialSelection.data.id}:${materialSelection.data.updated_at}` : ''
  const materialServerDraft = useMemo(
    () => ({
      selectedRefs: materialSelection.data?.selected_refs ?? [],
      notes: materialSelection.data?.notes ?? '',
    }),
    [materialSelection.data],
  )
  const materialDraft =
    materialDraftState?.sourceKey === materialSourceKey ? materialDraftState.value.selectedRefs : materialServerDraft.selectedRefs
  const materialNotes =
    materialDraftState?.sourceKey === materialSourceKey ? materialDraftState.value.notes : materialServerDraft.notes
  const setMaterialDraft = (updater: SetStateAction<unknown[]>) => {
    setMaterialDraftState((currentDraft) => {
      const currentValue = currentDraft?.sourceKey === materialSourceKey ? currentDraft.value : materialServerDraft
      const selectedRefs = typeof updater === 'function' ? updater(currentValue.selectedRefs) : updater
      return { sourceKey: materialSourceKey, value: { ...currentValue, selectedRefs } }
    })
  }
  const setMaterialNotes = (notes: string) => {
    setMaterialDraftState((currentDraft) => {
      const currentValue = currentDraft?.sourceKey === materialSourceKey ? currentDraft.value : materialServerDraft
      return { sourceKey: materialSourceKey, value: { ...currentValue, notes } }
    })
  }
  const exportsQuery = useQuery({
    queryKey: ['bid-exports', bidId],
    queryFn: () => fetchBidExports(bidId),
    enabled: Boolean(bidId),
    refetchInterval: (query) => {
      const items = query.state.data ?? []
      return items.some((item) => item.status === 'queued' || item.status === 'running') ? 2000 : false
    },
  })
  const generationJobs = useQuery({
    queryKey: ['bid-generation-jobs', bidId],
    queryFn: () => fetchBidGenerationJobs(bidId),
    enabled: Boolean(bidId),
    refetchInterval: (query) => {
      const items = query.state.data ?? []
      return items.some((item) => item.status === 'queued' || item.status === 'running') ? 2000 : false
    },
  })
  useEffect(() => {
    const active = (generationJobs.data ?? []).some((item) => item.status === 'queued' || item.status === 'running')
    if (!active) return
    void queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] })
  }, [generationJobs.data, queryClient, bidId])
  const uploadTenderMutation = useMutation({
    mutationFn: async () => {
      if (!tenderFile) {
        throw new Error('请选择招标文件')
      }
      const upload = await createPresignedUpload({
        filename: tenderFile.name,
        content_type: tenderFile.type || 'application/octet-stream',
        size_bytes: tenderFile.size,
        biz_type: 'bid_tender',
        biz_id: bidId,
      })
      await uploadToPresignedUrl(upload, tenderFile)
      const confirmed = await confirmFileUpload(upload.file.id)
      return uploadBidTenderFile(bidId, { file_id: confirmed.file.id })
    },
    onSuccess: async () => {
      message.success('招标文件已上传')
      setTenderFile(null)
      await queryClient.invalidateQueries({ queryKey: ['bid-parse-result', bidId] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '上传招标文件失败')),
  })
  const parseTenderMutation = useMutation({
    mutationFn: () => parseBidTender(bidId),
    onSuccess: async () => {
      message.success('已开始解读招标文件')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['bid-parse-result', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-requirements', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-material-selection', bidId] }),
      ])
    },
    onError: (error) => message.error(getApiErrorMessage(error, '解析招标文件失败')),
  })
  const confirmParseMutation = useMutation({
    mutationFn: () => {
      const structuredResult = applyParseConfirmDraft(
        parseResult.data?.structured_result,
        parseConfirmServerDraft,
        parseConfirmDraft,
      )
      return confirmBidParseResult(bidId, { structured_result: structuredResult })
    },
    onSuccess: async () => {
      message.success('解析结果已确认')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['bid-parse-result', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-requirements', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-material-selection', bidId] }),
      ])
    },
    onError: (error) => message.error(getApiErrorMessage(error, '确认文件信息失败，请重新解读后再试')),
  })
  const generateOutlineMutation = useMutation({
    mutationFn: () => generateBidOutline(bidId),
    onSuccess: async () => {
      message.success('目录大纲已生成')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['bid-parts', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] }),
      ])
    },
    onError: (error) => message.error(getApiErrorMessage(error, '生成目录大纲失败')),
  })
  const saveOutlineMutation = useMutation({
    mutationFn: (partId: string) =>
      updateBidPartOutline(bidId, partId, {
        chapters: outlineDrafts[partId] ?? [],
      }),
    onSuccess: async () => {
      message.success('目录已保存')
      await queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '保存目录失败')),
  })
  const saveMaterialMutation = useMutation({
    mutationFn: () =>
      updateBidMaterialSelection(bidId, {
        selected_refs: materialDraft,
        notes: materialNotes,
      }),
    onSuccess: async () => {
      message.success('素材选择已保存')
      await queryClient.invalidateQueries({ queryKey: ['bid-material-selection', bidId] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '保存素材选择失败')),
  })
  const generateBidMutation = useMutation({
    mutationFn: (partCode?: string) => generateBid(bidId, partCode ? { scope: 'part', part_code: partCode } : { scope: 'full' }),
    onSuccess: async () => {
      message.success('已开始生成正文')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['bid-generation-jobs', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] }),
      ])
    },
    onError: (error) => message.error(getApiErrorMessage(error, '生成正文失败')),
  })
  const pauseJobMutation = useMutation({
    mutationFn: pauseBidGenerationJob,
    onSuccess: async () => {
      message.success('生成已暂停')
      await queryClient.invalidateQueries({ queryKey: ['bid-generation-jobs', bidId] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '暂停生成失败')),
  })
  const resumeJobMutation = useMutation({
    mutationFn: resumeBidGenerationJob,
    onSuccess: async () => {
      message.success('生成已继续')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['bid-generation-jobs', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] }),
      ])
    },
    onError: (error) => message.error(getApiErrorMessage(error, '继续生成失败')),
  })
  const cancelJobMutation = useMutation({
    mutationFn: cancelBidGenerationJob,
    onSuccess: async () => {
      message.success('生成已取消')
      await queryClient.invalidateQueries({ queryKey: ['bid-generation-jobs', bidId] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '取消生成失败')),
  })
  const exportMutation = useMutation({
    mutationFn: (payload: { export_type: 'docx' | 'pdf' | 'zip'; part_code: string }) => createBidExport(bidId, payload),
    onSuccess: async () => {
      message.success('已开始生成导出文件')
      await queryClient.invalidateQueries({ queryKey: ['bid-exports', bidId] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '生成导出文件失败')),
  })
  const downloadMutation = useMutation({
    mutationFn: fetchBidExport,
    onSuccess: (detail) => {
      if (!detail.download?.url) {
        message.info('导出文件仍在生成')
        return
      }
      const openError = fileOpenErrorMessage(openFileUrl(detail.download.url))
      if (openError) {
        message.error(openError)
      }
    },
    onError: (error) => message.error(getApiErrorMessage(error, '获取下载链接失败')),
  })
  const requirementExportMutation = useMutation({
    mutationFn: (format: BidRequirementExportFormat) => exportBidRequirements(bidId, format),
    onSuccess: ({ blob, filename }) => {
      saveBlobFile(blob, filename)
      message.success('响应矩阵已导出')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '导出响应矩阵失败')),
  })
  const requirementCoverageMutation = useMutation({
    mutationFn: (payload: {
      requirementId: string
      coverageStatus: BidRequirementItemDTO['coverage_status']
      evidence?: string
      sourceRefs?: unknown[]
    }) =>
      updateBidRequirementCoverage(bidId, payload.requirementId, {
        coverage_status: payload.coverageStatus,
        evidence: payload.evidence,
        source_refs: payload.sourceRefs,
      }),
    onSuccess: async () => {
      message.success('响应状态已更新')
      await queryClient.invalidateQueries({ queryKey: ['bid-requirements', bidId] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '更新响应状态失败')),
  })
  const requirementBatchCoverageMutation = useMutation({
    mutationFn: (payload: {
      requirementIds: string[]
      coverageStatus: BidRequirementItemDTO['coverage_status']
      evidence?: string
      sourceRefs?: unknown[]
    }) =>
      batchUpdateBidRequirementCoverage(bidId, {
        requirement_ids: payload.requirementIds,
        coverage_status: payload.coverageStatus,
        evidence: payload.evidence,
        source_refs: payload.sourceRefs,
      }),
    onSuccess: async (items, variables) => {
      message.success(
        variables.evidence || variables.sourceRefs?.length ? `已更新 ${items.length} 项响应依据` : `已更新 ${items.length} 项响应状态`,
      )
      setSelectedRequirementKeys([])
      await queryClient.invalidateQueries({ queryKey: ['bid-requirements', bidId] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '批量更新响应信息失败')),
  })
  const requirementHistoryMutation = useMutation({
    mutationFn: (requirementId: string) => fetchBidRequirementCoverageHistory(bidId, requirementId),
    onError: (error) => message.error(getApiErrorMessage(error, '获取响应历史失败')),
  })
  const updateBatchRequirementStatus = (coverageStatus: BidRequirementItemDTO['coverage_status']) => {
    if (!selectedRequirementRows.length || requirementBatchCoverageMutation.isPending) return
    requirementBatchCoverageMutation.mutate({
      requirementIds: selectedRequirementRows.map((row) => row.id),
      coverageStatus,
    })
  }
  const updateRequirementStatus = (row: ParseRequirementRow, coverageStatus: BidRequirementItemDTO['coverage_status']) => {
    if (!row.canUpdate) return
    requirementCoverageMutation.mutate({ requirementId: row.id, coverageStatus })
  }
  const openRequirementHistory = (row: ParseRequirementRow) => {
    if (!row.canUpdate) return
    setHistoryRequirement(row)
    requirementHistoryMutation.reset()
    requirementHistoryMutation.mutate(row.id)
  }
  const openRequirementSourcePreview = async (sourceRef: unknown) => {
    const target = requirementSourcePreviewTarget(sourceRef)
    if (!target) {
      message.info('该来源暂未关联可预览文件')
      return
    }
    try {
      const result =
        target.type === 'file' ? await fetchFileURL(target.id, 'preview') : await fetchKnowledgeDocumentPreview(target.id)
      const openError = fileOpenErrorMessage(openFileUrl(withPreviewPageAnchor(result.url, requirementSourcePage(sourceRef))))
      if (openError) {
        message.error(openError)
      }
    } catch (error) {
      message.error(getApiErrorMessage(error, '获取来源预览失败'))
    }
  }
  const copyRequirementSourceLocator = async (sourceRef: unknown) => {
    const locator = requirementSourceLocatorText(sourceRef)
    if (!locator) {
      message.info('该来源暂无可复制定位')
      return
    }
    try {
      await navigator.clipboard.writeText(locator)
      message.success('来源定位已复制')
    } catch {
      message.error('复制失败，请手动选择定位信息')
    }
  }
  const openRequirementSourcesModal = (row: ParseRequirementRow) => {
    const sourceRefs = row.coverageSourceRefs
    if (!sourceRefs.length) {
      message.info('暂未关联响应来源')
      return
    }
    Modal.info({
      title: '响应来源',
      width: 720,
      okText: '关闭',
      content: (
        <Space direction="vertical" size={10} className="full-width">
          {sourceRefs.map((sourceRef, index) => {
            const target = requirementSourcePreviewTarget(sourceRef)
            return (
              <div className="requirement-source-preview-row" key={index}>
                <div>
                  <Typography.Text strong>{requirementSourceTitle(sourceRef)}</Typography.Text>
                  <Typography.Paragraph className="requirement-source-preview-text">
                    {requirementSourceRefsSummary([sourceRef]) || '未填写来源摘录'}
                  </Typography.Paragraph>
                  <div className="requirement-source-locator">
                    {requirementSourceLocatorParts(sourceRef).map((part) => (
                      <Tag key={`${part.label}-${part.value}`}>
                        {part.label}：{part.value}
                      </Tag>
                    ))}
                  </div>
                </div>
                <Space size={6} wrap={false}>
                  <Tooltip title="复制来源定位">
                    <Button
                      size="small"
                      icon={<CopyOutlined />}
                      disabled={!requirementSourceLocatorText(sourceRef)}
                      onClick={() => void copyRequirementSourceLocator(sourceRef)}
                    />
                  </Tooltip>
                  <Button size="small" disabled={!target} onClick={() => void openRequirementSourcePreview(sourceRef)}>
                    查看原文
                  </Button>
                </Space>
              </div>
            )
          })}
        </Space>
      ),
    })
  }
  const openRequirementEvidenceModal = (row: ParseRequirementRow) => {
    if (!row.canUpdate) return
    let evidence = row.coverageEvidence
    let sourceRefs = sourceRefDraftsFromRefs(row.coverageSourceRefs)
    Modal.confirm({
      title: '补充响应证据',
      okText: '保存',
      cancelText: '取消',
      content: (
        <Space direction="vertical" size={10} className="full-width">
          <Input.TextArea
            defaultValue={row.coverageEvidence}
            rows={4}
            maxLength={1000}
            showCount
            autoFocus
            placeholder="填写响应证据"
            onChange={(event) => {
              evidence = event.target.value
            }}
          />
          <RequirementSourceRefsEditor initialRefs={sourceRefs} onChange={(nextRefs) => (sourceRefs = nextRefs)} />
        </Space>
      ),
      onOk: () => {
        const nextEvidence = evidence.trim()
        const nextSourceRefs = sourceRefsFromDrafts(sourceRefs)
        if (!nextEvidence && !nextSourceRefs.length) {
          message.warning('请填写响应证据或来源')
          return Promise.reject(new Error('empty evidence or source'))
        }
        return requirementCoverageMutation.mutateAsync({
          requirementId: row.id,
          coverageStatus: row.coverageStatus === 'unmapped' ? 'planned' : row.coverageStatus,
          evidence: nextEvidence,
          sourceRefs: nextSourceRefs,
        })
      },
    })
  }
  const openBatchRequirementEvidenceModal = () => {
    if (!selectedRequirementRows.length) return
    let coverageStatus = inferBatchCoverageStatus(selectedRequirementRows)
    let evidence = ''
    let sourceRefs = [] as RequirementSourceRefDraft[]
    Modal.confirm({
      title: '批量补充响应证据',
      okText: '保存',
      cancelText: '取消',
      content: (
        <Space direction="vertical" size={10} className="full-width">
          <Typography.Text type="secondary">已选 {selectedRequirementRows.length} 项</Typography.Text>
          <Select
            defaultValue={coverageStatus}
            className="full-width"
            options={requirementCoverageOptions}
            onChange={(value) => {
              coverageStatus = value
            }}
          />
          <Input.TextArea
            rows={4}
            maxLength={1000}
            showCount
            autoFocus
            placeholder="填写这些响应要点共同适用的依据"
            onChange={(event) => {
              evidence = event.target.value
            }}
          />
          <RequirementSourceRefsEditor initialRefs={sourceRefs} onChange={(nextRefs) => (sourceRefs = nextRefs)} />
        </Space>
      ),
      onOk: () => {
        const nextEvidence = evidence.trim()
        const nextSourceRefs = sourceRefsFromDrafts(sourceRefs)
        if (!nextEvidence && !nextSourceRefs.length) {
          message.warning('请填写响应证据或来源')
          return Promise.reject(new Error('empty evidence or source'))
        }
        return requirementBatchCoverageMutation.mutateAsync({
          requirementIds: selectedRequirementRows.map((row) => row.id),
          coverageStatus,
          evidence: nextEvidence,
          sourceRefs: nextSourceRefs,
        })
      },
    })
  }
  const exportableParts = (parts.data ?? []).filter((part) => ['combined_body', 'tech', 'business'].includes(part.code))
  const primaryPartCode = exportableParts[0]?.code
  const parseRows = structuredResultRows(parseResult.data?.structured_result)
  const parseModuleRows = structuredModuleRows(parseResult.data?.structured_result)
  const syncedRequirementRows = requirementRowsFromItems(bidRequirements.data ?? [])
  const parseRequirementRows = syncedRequirementRows.length
    ? syncedRequirementRows
    : structuredRequirementRows(parseResult.data?.structured_result)
  const visibleRequirementRows = filterRequirementRows(parseRequirementRows, requirementFilter, requirementEvidenceFilter)
  const selectedRequirementRows = parseRequirementRows.filter((row) => row.canUpdate && selectedRequirementKeys.includes(row.id))
  const isRequirementUpdating = requirementCoverageMutation.isPending || requirementBatchCoverageMutation.isPending
  const parseFailureMessage =
    parseResult.data?.status === 'failed'
      ? parseResult.data.error_message?.trim() || '文件解读失败，请重新上传或重新解读'
      : ''
  const setOutlineDraft = (partId: string, rowIndex: number, patch: Partial<OutlineDraftChapter>) => {
    setOutlineDrafts((current) => ({
      ...current,
      [partId]: (current[partId] ?? []).map((item, index) => (index === rowIndex ? { ...item, ...patch } : item)),
    }))
  }
  const addOutlineDraft = (partId: string) => {
    setOutlineDrafts((current) => {
      const rows = current[partId] ?? []
      return {
        ...current,
        [partId]: [
          ...rows,
          {
            title: '新增章节',
            plain_text: '请补充新增章节内容。',
            sort_order: (rows.length + 1) * 10,
          },
        ],
      }
    })
  }
  const toggleMaterialRef = (index: number, selected: boolean) => {
    setMaterialDraft((current) =>
      current.map((item, itemIndex) => (itemIndex === index ? { ...materialRefRecord(item), selected } : item)),
    )
  }

  return (
    <PageFrame
      module="标书生成"
      title="标书编制流程"
      subtitle={bid.data?.title ?? '分离标书支持技术标和商务标独立生成'}
      tags={['编制流程', '分册生成']}
      permission={canWrite}
      bare
    >
      <Space direction="vertical" size={20} className="full-width">
        <Steps
          current={current}
          items={steps.map((title) => ({ title }))}
          onChange={(next) => setSearchParams({ step: String(next + 1) })}
        />
        <Card title={steps[current]}>
          {current === 0 ? (
            <Space direction="vertical" size={16} className="full-width">
              <Space wrap>
                <Upload
                  accept=".pdf,.doc,.docx,.txt,.xlsx,.xlsm,.xls,.pptx,.pptm,.ppt,.png,.jpg,.jpeg,.webp,.tif,.tiff"
                  maxCount={1}
                  beforeUpload={(file) => {
                    if (isUploadFileTooLarge(file)) {
                      message.error(uploadSizeLimitMessage())
                      setTenderFile(null)
                      return Upload.LIST_IGNORE
                    }
                    setTenderFile(file)
                    return false
                  }}
                  onRemove={() => {
                    setTenderFile(null)
                  }}
                >
                  <Button icon={<UploadOutlined />}>选择招标文件</Button>
                </Upload>
                <Button
                  type="primary"
                  icon={<UploadOutlined />}
                  disabled={!tenderFile}
                  loading={uploadTenderMutation.isPending}
                  onClick={() => uploadTenderMutation.mutate()}
                >
                  上传招标文件
                </Button>
                <Tag color={parseResult.data?.file_asset_id ? 'green' : 'default'}>
                  {parseResult.data?.file_asset_id ? '已绑定' : '未绑定'}
                </Tag>
              </Space>
              <Typography.Text type="secondary">
                {tenderFile ? `${tenderFile.name} · ${formatBytes(tenderFile.size)}` : '支持招标文件、清单表格、演示文稿或扫描图片'}
              </Typography.Text>
            </Space>
          ) : null}
          {current === 1 ? (
            <Space direction="vertical" size={16} className="full-width">
              <Space wrap>
                <Button
                  type="primary"
                  icon={<SyncOutlined />}
                  loading={parseTenderMutation.isPending}
                  onClick={() => parseTenderMutation.mutate()}
                >
                  开始解读
                </Button>
                <Button
                  icon={<CheckOutlined />}
                  loading={confirmParseMutation.isPending}
                  disabled={!canEditParseConfirm}
                  onClick={() => confirmParseMutation.mutate()}
                >
                  确认文件信息
                </Button>
                <Tag color={parseStatusColor(parseResult.data?.status ?? 'queued')}>
                  {parseStatusLabel(parseResult.data?.status ?? 'queued')}
                </Tag>
              </Space>
              {parseFailureMessage ? <Alert type="error" showIcon message={parseFailureMessage} /> : null}
              <div className="parse-confirm-panel">
                <div className="parse-confirm-head">
                  <Typography.Title level={5}>确认信息</Typography.Title>
                  {parseConfirmEditedFields.length ? (
                    <Tag color="gold">已调整 {parseConfirmEditedFields.join('、')}</Tag>
                  ) : null}
                </div>
                <div className="parse-confirm-grid">
                  <label className="parse-confirm-field">
                    <span>项目名称</span>
                    <Input
                      value={parseConfirmDraft.projectName}
                      disabled={!canEditParseConfirm}
                      placeholder="项目名称"
                      onChange={(event) => setParseConfirmDraftField('projectName', event.target.value)}
                    />
                  </label>
                  <label className="parse-confirm-field">
                    <span>投标截止</span>
                    <Input
                      value={parseConfirmDraft.deadline}
                      disabled={!canEditParseConfirm}
                      placeholder="投标截止"
                      onChange={(event) => setParseConfirmDraftField('deadline', event.target.value)}
                    />
                  </label>
                  <label className="parse-confirm-field">
                    <span>标书类型</span>
                    <Select
                      value={parseConfirmDraft.bidType}
                      disabled={!canEditParseConfirm}
                      onChange={(value) => setParseConfirmDraftField('bidType', value)}
                      options={[
                        { label: '综合标书', value: 'combined' },
                        { label: '分离标书', value: 'separated' },
                        { label: '自定义组合', value: 'custom' },
                      ]}
                    />
                  </label>
                  <label className="parse-confirm-field parse-confirm-field-wide">
                    <span>资格要求</span>
                    <Input.TextArea
                      value={parseConfirmDraft.qualificationRequirements}
                      disabled={!canEditParseConfirm}
                      placeholder="资格要求"
                      autoSize={{ minRows: 2, maxRows: 5 }}
                      onChange={(event) => setParseConfirmDraftField('qualificationRequirements', event.target.value)}
                    />
                  </label>
                  <label className="parse-confirm-field parse-confirm-field-wide">
                    <span>评分要点</span>
                    <Input.TextArea
                      value={parseConfirmDraft.scoringPoints}
                      disabled={!canEditParseConfirm}
                      placeholder="评分要点"
                      autoSize={{ minRows: 2, maxRows: 5 }}
                      onChange={(event) => setParseConfirmDraftField('scoringPoints', event.target.value)}
                    />
                  </label>
                  <label className="parse-confirm-field parse-confirm-field-wide">
                    <span>否决风险</span>
                    <Input.TextArea
                      value={parseConfirmDraft.invalidClauseRisks}
                      disabled={!canEditParseConfirm}
                      placeholder="否决风险"
                      autoSize={{ minRows: 2, maxRows: 5 }}
                      onChange={(event) => setParseConfirmDraftField('invalidClauseRisks', event.target.value)}
                    />
                  </label>
                </div>
              </div>
              <Table
                size="small"
                pagination={false}
                rowKey="name"
                dataSource={parseRows}
                columns={[
                  { title: '字段', dataIndex: 'name', width: 180 },
                  { title: '文件信息', dataIndex: 'value' },
                ]}
              />
              {parseModuleRows.length || parseRequirementRows.length ? (
                <Tabs
                  size="small"
                  items={[
                    {
                      key: 'groups',
                      label: '信息分组',
                      children: (
                        <Table
                          size="small"
                          pagination={false}
                          rowKey="key"
                          dataSource={parseModuleRows}
                          columns={[
                            { title: '分组', dataIndex: 'title', width: 120 },
                            {
                              title: '状态',
                              dataIndex: 'status',
                              width: 110,
                              render: (_, row) => parseReviewTag(row.needsReview),
                            },
                            { title: '提取内容', dataIndex: 'summary', ellipsis: true },
                            { title: '响应要点', dataIndex: 'requirementCount', width: 110 },
                          ]}
                        />
                      ),
                    },
                    {
                      key: 'requirements',
                      label: '响应要点',
                      children: (
                        <Space direction="vertical" size={12} className="full-width">
                          <Space wrap align="center">
                            <Segmented
                              size="small"
                              value={requirementFilter}
                              onChange={(value) => setRequirementFilter(value as RequirementFilter)}
                              options={[
                                { label: '全部', value: 'all' },
                                { label: '必须', value: 'mandatory' },
                                { label: '待确认', value: 'review' },
                                { label: '已覆盖', value: 'covered' },
                              ]}
                            />
                            <Select
                              size="small"
                              className="requirement-evidence-filter-select"
                              value={requirementEvidenceFilter}
                              onChange={(value) => setRequirementEvidenceFilter(value)}
                              options={requirementEvidenceFilterOptions}
                            />
                            {canWrite ? (
                              <>
                                <Select
                                  size="small"
                                  className="requirement-batch-status-select"
                                  value={undefined}
                                  placeholder="批量标记"
                                  disabled={!selectedRequirementRows.length || isRequirementUpdating}
                                  loading={requirementBatchCoverageMutation.isPending}
                                  onChange={updateBatchRequirementStatus}
                                  options={requirementCoverageOptions}
                                />
                                <Button
                                  size="small"
                                  icon={<EditOutlined />}
                                  disabled={!selectedRequirementRows.length || isRequirementUpdating}
                                  loading={requirementBatchCoverageMutation.isPending}
                                  onClick={openBatchRequirementEvidenceModal}
                                >
                                  批量补证据
                                </Button>
                                {selectedRequirementRows.length ? (
                                  <Typography.Text type="secondary">已选 {selectedRequirementRows.length} 项</Typography.Text>
                                ) : null}
                              </>
                            ) : null}
                            <Button
                              size="small"
                              icon={<DownloadOutlined />}
                              loading={requirementExportMutation.isPending && requirementExportMutation.variables === 'csv'}
                              disabled={!syncedRequirementRows.length}
                              onClick={() => requirementExportMutation.mutate('csv')}
                            >
                              导出矩阵
                            </Button>
                            <Button
                              size="small"
                              icon={<DownloadOutlined />}
                              loading={requirementExportMutation.isPending && requirementExportMutation.variables === 'xlsx'}
                              disabled={!syncedRequirementRows.length}
                              onClick={() => requirementExportMutation.mutate('xlsx')}
                            >
                              导出历史
                            </Button>
                          </Space>
                          <Table
                            size="small"
                            pagination={{ pageSize: 6, size: 'small' }}
                            rowKey="id"
                            rowSelection={
                              canWrite
                                ? {
                                    columnWidth: 42,
                                    selectedRowKeys: selectedRequirementKeys,
                                    onChange: (keys) => setSelectedRequirementKeys(keys.map(String)),
                                    getCheckboxProps: (row: ParseRequirementRow) => ({
                                      disabled: !row.canUpdate || isRequirementUpdating,
                                    }),
                                  }
                                : undefined
                            }
                            dataSource={visibleRequirementRows}
                            columns={[
                              { title: '来源分组', dataIndex: 'module', width: 120 },
                              {
                                title: '要求',
                                dataIndex: 'requirement',
                                ellipsis: true,
                              },
                              {
                                title: '状态',
                                width: 128,
                                render: (_, row) =>
                                  row.canUpdate && canWrite ? (
                                    <Select
                                      size="small"
                                      className="requirement-status-select"
                                      value={row.coverageStatus}
                                      disabled={isRequirementUpdating}
                                      onChange={(value) => updateRequirementStatus(row, value)}
                                      options={requirementCoverageOptions}
                                    />
                                  ) : (
                                    requirementCoverageTag(row.coverageStatus)
                                  ),
                              },
                              {
                                title: '响应证据',
                                width: 260,
                                render: (_, row) =>
                                  requirementEvidenceCell(
                                    row,
                                    canWrite,
                                    isRequirementUpdating,
                                    openRequirementEvidenceModal,
                                    openRequirementSourcesModal,
                                  ),
                              },
                              {
                                title: '属性',
                                width: 120,
                                render: (_, row) => (
                                  <Space size={4} wrap>
                                    {row.mandatory ? <Tag color="red">必须响应</Tag> : <Tag>建议覆盖</Tag>}
                                    {row.needsReview ? <Tag color="gold">待确认</Tag> : null}
                                  </Space>
                                ),
                              },
                              {
                                title: '历史',
                                width: 80,
                                render: (_, row) =>
                                  row.canUpdate ? (
                                    <Button
                                      size="small"
                                      type="link"
                                      loading={requirementHistoryMutation.isPending && historyRequirement?.id === row.id}
                                      onClick={() => openRequirementHistory(row)}
                                    >
                                      查看
                                    </Button>
                                  ) : (
                                    <Typography.Text type="secondary">-</Typography.Text>
                                  ),
                              },
                            ]}
                          />
                        </Space>
                      ),
                    },
                  ]}
                />
              ) : null}
            </Space>
          ) : null}
          {current === 2 ? (
            <Space direction="vertical" size={16} className="full-width">
              <Space wrap>
                <Button
                  type="primary"
                  icon={<SyncOutlined />}
                  loading={generateOutlineMutation.isPending}
                  onClick={() => generateOutlineMutation.mutate()}
                >
                  生成目录大纲
                </Button>
                <Typography.Text type="secondary">目录保存后会同步到标书编辑器章节树</Typography.Text>
              </Space>
              <Tabs
                items={(parts.data ?? []).map((part) => ({
                  key: part.id,
                  label: part.title,
                  children: (
                    <Space direction="vertical" size={12} className="full-width">
                      <Space wrap>
                        <Button icon={<PlusOutlined />} onClick={() => addOutlineDraft(part.id)}>
                          新增章节
                        </Button>
                        <Button
                          type="primary"
                          icon={<SaveOutlined />}
                          loading={saveOutlineMutation.isPending && saveOutlineMutation.variables === part.id}
                          onClick={() => saveOutlineMutation.mutate(part.id)}
                        >
                          保存目录
                        </Button>
                      </Space>
                      <Table
                        size="small"
                        pagination={false}
                        rowKey={(row) => row.id ?? `${part.id}-${row.sort_order}-${row.title}`}
                        dataSource={outlineDrafts[part.id] ?? []}
                        columns={[
                          {
                            title: '排序',
                            dataIndex: 'sort_order',
                            width: 96,
                            render: (value: number, _row: OutlineDraftChapter, index: number) => (
                              <Input
                                value={value}
                                onChange={(event) =>
                                  setOutlineDraft(part.id, index, { sort_order: Number(event.target.value) || (index + 1) * 10 })
                                }
                              />
                            ),
                          },
                          {
                            title: '章节标题',
                            dataIndex: 'title',
                            width: 240,
                            render: (value: string, _row: OutlineDraftChapter, index: number) => (
                              <Input value={value} onChange={(event) => setOutlineDraft(part.id, index, { title: event.target.value })} />
                            ),
                          },
                          {
                            title: '生成提示',
                            dataIndex: 'plain_text',
                            render: (value: string, _row: OutlineDraftChapter, index: number) => (
                              <Input.TextArea
                                rows={2}
                                value={value}
                                onChange={(event) => setOutlineDraft(part.id, index, { plain_text: event.target.value })}
                              />
                            ),
                          },
                        ]}
                      />
                    </Space>
                  ),
                }))}
              />
            </Space>
          ) : null}
          {current === 3 ? (
            <Space direction="vertical" size={16} className="full-width">
              <Table
                size="small"
                pagination={false}
                rowKey={(_row, index) => `material-${index}`}
                dataSource={materialDraft}
                columns={[
                  {
                    title: '选择',
                    width: 80,
                    render: (_value, row: unknown, index: number) => (
                      <Checkbox
                        checked={materialRefSelected(row)}
                        onChange={(event) => toggleMaterialRef(index, event.target.checked)}
                      />
                    ),
                  },
                  { title: '素材', render: (_value, row: unknown) => materialRefTitle(row) },
                  { title: '原因', render: (_value, row: unknown) => materialRefReason(row) },
                ]}
              />
              <Input.TextArea
                rows={4}
                value={materialNotes}
                placeholder="素材筛选备注"
                onChange={(event) => setMaterialNotes(event.target.value)}
              />
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={saveMaterialMutation.isPending}
                onClick={() => saveMaterialMutation.mutate()}
              >
                保存素材选择
              </Button>
            </Space>
          ) : null}
          {current === 4 ? (
            <Space direction="vertical" size={16} className="full-width">
              <Space wrap>
                <Button
                  type="primary"
                  icon={<SyncOutlined />}
                  loading={generateBidMutation.isPending && !generateBidMutation.variables}
                  onClick={() => generateBidMutation.mutate(undefined)}
                >
                  启动整标逐章生成
                </Button>
                {exportableParts.map((part) => (
                  <Button
                    key={part.id}
                    icon={<SyncOutlined />}
                    loading={generateBidMutation.isPending && generateBidMutation.variables === part.code}
                    onClick={() => generateBidMutation.mutate(part.code)}
                  >
                    生成{part.title}
                  </Button>
                ))}
              </Space>
              <Tabs
                items={exportableParts.map((part) => {
                  const partChapters = (chapters.data ?? []).filter((chapter) => chapter.bid_part_id === part.id)
                  const readyCount = partChapters.filter((chapter) => ['generated', 'accepted', 'edited'].includes(chapter.status)).length
                  const percent = partChapters.length ? Math.round((readyCount / partChapters.length) * 100) : 0
                  return {
                    key: part.id,
                    label: part.title,
                    children: (
                      <Space direction="vertical" className="full-width">
                        <Progress percent={percent} />
                        <Timeline
                          items={partChapters.map((chapter) => ({
                            color: chapter.status === 'generated' || chapter.status === 'accepted' ? 'green' : chapter.status === 'generating' ? 'blue' : 'gray',
                            children: (
                              <Space>
                                <span>{chapter.title}</span>
                                <Tag color={chapterStatusColor(chapter.status)}>
                                  {chapterStatusText(chapter.status)}
                                </Tag>
                              </Space>
                            ),
                          }))}
                        />
                      </Space>
                    ),
                  }
                })}
              />
              <Table
                size="small"
                rowKey="id"
                pagination={false}
                loading={generationJobs.isLoading}
                locale={{ emptyText: <EmptyBlock /> }}
                dataSource={generationJobs.data ?? []}
                columns={[
                  { title: '范围', dataIndex: 'scope', render: generationScopeLabel },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    render: (_, row: BidGenerationJobDTO) => generationJobStatusCell(row),
                  },
                  { title: '进度', dataIndex: 'progress', render: (value: number) => <Progress percent={value} size="small" /> },
                  {
                    title: '章节',
                    render: (_, row: BidGenerationJobDTO) => `${row.completed_steps}/${row.total_steps}`,
                  },
                  {
                    title: '操作',
                    render: (_, row: BidGenerationJobDTO) => (
                      <Space>
                        <Button
                          size="small"
                          disabled={row.status !== 'running' && row.status !== 'queued'}
                          loading={pauseJobMutation.isPending && pauseJobMutation.variables === row.id}
                          onClick={() => pauseJobMutation.mutate(row.id)}
                        >
                          暂停
                        </Button>
                        <Button
                          size="small"
                          disabled={row.status !== 'paused'}
                          loading={resumeJobMutation.isPending && resumeJobMutation.variables === row.id}
                          onClick={() => resumeJobMutation.mutate(row.id)}
                        >
                          继续
                        </Button>
                        <Button
                          size="small"
                          danger
                          disabled={!['queued', 'running', 'paused'].includes(row.status)}
                          loading={cancelJobMutation.isPending && cancelJobMutation.variables === row.id}
                          onClick={() => cancelJobMutation.mutate(row.id)}
                        >
                          取消
                        </Button>
                      </Space>
                    ),
                  },
                ]}
              />
            </Space>
          ) : null}
          {current === 5 ? (
            <Button type="primary">
              <Link to={`/bids/${bidId}/editor${primaryPartCode ? `?part=${primaryPartCode}` : ''}`}>进入编辑器</Link>
            </Button>
          ) : null}
          {current === 6 ? (
            <Space direction="vertical" className="full-width">
              <Space wrap>
                {exportableParts.map((part) => (
                  <Space.Compact key={part.id}>
                    <Button
                      icon={<DownloadOutlined />}
                      loading={exportMutation.isPending}
                      onClick={() => exportMutation.mutate({ export_type: 'docx', part_code: part.code })}
                    >
                      {part.title}.docx
                    </Button>
                    <Button
                      icon={<FilePdfOutlined />}
                      loading={exportMutation.isPending}
                      onClick={() => exportMutation.mutate({ export_type: 'pdf', part_code: part.code })}
                    >
                      {part.title}.pdf
                    </Button>
                  </Space.Compact>
                ))}
                <Button
                  icon={<FileZipOutlined />}
                  loading={exportMutation.isPending}
                  disabled={exportableParts.length < 2}
                  onClick={() => exportMutation.mutate({ export_type: 'zip', part_code: 'all' })}
                >
                  打包全套文件
                </Button>
              </Space>
              <Table
                size="small"
                rowKey="id"
                pagination={false}
                locale={{ emptyText: <EmptyBlock /> }}
                dataSource={exportsQuery.data ?? []}
                columns={[
                  { title: '文件名', dataIndex: 'filename' },
                  {
                    title: '类型',
                    dataIndex: 'part_code',
                    render: (value: string, row: BidExportDTO) => exportTypeLabel(row, value),
                  },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    render: (_, row: BidExportDTO) => exportStatusCell(row),
                  },
                  {
                    title: '操作',
                    render: (_, row: BidExportDTO) => (
                      <Button
                        size="small"
                        icon={<DownloadOutlined />}
                        disabled={row.status !== 'done'}
                        loading={downloadMutation.isPending}
                        onClick={() => downloadMutation.mutate(row.id)}
                      >
                        下载
                      </Button>
                    ),
                  },
                ]}
              />
            </Space>
          ) : null}
        </Card>
      </Space>
      <Modal
        title="响应历史"
        open={Boolean(historyRequirement)}
        footer={null}
        width={720}
        onCancel={() => {
          setHistoryRequirement(null)
          requirementHistoryMutation.reset()
        }}
      >
        {historyRequirement ? (
          <Space direction="vertical" size={12} className="full-width">
            <Typography.Text strong>{historyRequirement.requirement}</Typography.Text>
            {requirementHistoryMutation.isPending ? (
              <LoadingBlock />
            ) : requirementHistoryMutation.data?.length ? (
              <Timeline
                items={requirementHistoryMutation.data.map((event) => ({
                  color: requirementCoverageTimelineColor(event.coverage_status),
                  children: requirementHistoryTimelineItem(event),
                }))}
              />
            ) : (
              <EmptyBlock description="暂无响应历史" />
            )}
          </Space>
        ) : null}
      </Modal>
    </PageFrame>
  )
}

function bidTypeLabel(value: BidDocumentDTO['bid_type']) {
  return { combined: '综合标书', separated: '分离标书', custom: '自定义组合' }[value] ?? '自定义组合'
}

function bidStatusLabel(value: string) {
  const labels: Record<string, string> = {
    draft: '草稿',
    editing: '编制中',
    in_review: '审批中',
    approved: '已通过',
    submitted: '已提交',
    archived: '已归档',
  }
  const color = value === 'approved' || value === 'submitted' ? 'green' : value === 'in_review' ? 'orange' : 'blue'
  return <Tag color={color}>{labels[value] || '编制中'}</Tag>
}

function partCodeLabel(value: string) {
  return { combined_body: '综合标书', tech: '技术标', business: '商务标', all: '全套文件' }[value] ?? '其他文件'
}

function exportTypeLabel(row: BidExportDTO, partCode: string) {
  if (row.export_type === 'zip') {
    return <Tag color="blue">全套文件</Tag>
  }
  return (
    <Space size={4}>
      <Tag>{partCodeLabel(partCode)}</Tag>
      <Tag color={row.export_type === 'pdf' ? 'red' : 'green'}>
        {row.export_type === 'docx' ? 'Word' : 'PDF'}
      </Tag>
    </Space>
  )
}

function exportStatusTag(value: BidExportDTO['status']) {
  const color = value === 'done' ? 'green' : value === 'failed' || value === 'cancelled' ? 'red' : 'blue'
  const labels: Record<BidExportDTO['status'], string> = {
    queued: '等待生成',
    running: '生成中',
    done: '可下载',
    failed: '生成失败',
    cancelled: '已取消',
  }
  return <Tag color={color}>{labels[value] ?? '状态未知'}</Tag>
}

function exportStatusCell(row: BidExportDTO) {
  return (
    <Space direction="vertical" size={2}>
      {exportStatusTag(row.status)}
      {row.status === 'failed' || row.status === 'cancelled' ? (
        <Typography.Text type="danger">{taskFailureMessage(row, '导出文件生成失败')}</Typography.Text>
      ) : null}
    </Space>
  )
}

function generationJobStatusTag(value: BidGenerationJobDTO['status']) {
  const color = value === 'done' ? 'green' : value === 'failed' || value === 'cancelled' ? 'red' : value === 'paused' ? 'orange' : 'blue'
  const labels: Record<BidGenerationJobDTO['status'], string> = {
    queued: '等待中',
    running: '生成中',
    paused: '已暂停',
    done: '已完成',
    failed: '生成失败',
    cancelled: '已取消',
  }
  return <Tag color={color}>{labels[value] ?? '状态未知'}</Tag>
}

function generationJobStatusCell(row: BidGenerationJobDTO) {
  return (
    <Space direction="vertical" size={2}>
      {generationJobStatusTag(row.status)}
      {row.status === 'failed' || row.status === 'cancelled' ? (
        <Typography.Text type="danger">{taskFailureMessage(row, '正文生成失败')}</Typography.Text>
      ) : null}
    </Space>
  )
}

function generationScopeLabel(value: BidGenerationJobDTO['scope']) {
  return { full: '整份标书', part: '单个分册', chapter: '单个章节' }[value] ?? '标书内容'
}

function parseStatusLabel(value: 'queued' | 'processing' | 'ready' | 'confirmed' | 'failed') {
  const labels = {
    queued: '等待解读',
    processing: '解读中',
    ready: '可确认',
    confirmed: '已确认',
    failed: '解读失败',
  }
  return labels[value] ?? '状态未知'
}

function parseStatusColor(value: 'queued' | 'processing' | 'ready' | 'confirmed' | 'failed') {
  const colors = {
    queued: 'default',
    processing: 'blue',
    ready: 'cyan',
    confirmed: 'green',
    failed: 'red',
  }
  return colors[value]
}

function isTerminalTaskStatus(status: AITaskDTO['status']) {
  return status === 'done' || status === 'failed' || status === 'cancelled'
}

function taskFailureMessage(
  task:
    | Pick<AITaskDTO, 'status' | 'error_message'>
    | Pick<BidGenerationSnapshotDTO['tasks'][number], 'status' | 'error_message'>
    | Pick<BidExportDTO, 'status' | 'error_message'>
    | Pick<BidGenerationJobDTO, 'status' | 'error_message'>
    | undefined,
  fallback: string,
) {
  if (!task || (task.status !== 'failed' && task.status !== 'cancelled')) return ''
  if (task.error_message?.trim()) return task.error_message.trim()
  return task.status === 'cancelled' ? '任务已取消' : fallback
}

const parseConfirmFieldLabels: Record<keyof ParseConfirmDraft, string> = {
  projectName: '项目名称',
  deadline: '投标截止',
  bidType: '标书类型',
  qualificationRequirements: '资格要求',
  scoringPoints: '评分要点',
  invalidClauseRisks: '否决风险',
}

function parseConfirmDraftFromStructuredResult(result: Record<string, unknown> | undefined): ParseConfirmDraft {
  const modules = objectRecord(result?.modules)
  const basicFields = parseModuleFields(modules, 'basic')
  const qualificationFields = parseModuleFields(modules, 'qualification')
  const evaluationFields = parseModuleFields(modules, 'evaluation')
  const invalidRiskFields = parseModuleFields(modules, 'invalid_risk')
  return {
    projectName: parseConfirmText(result?.project_name ?? basicFields?.project_name),
    deadline: parseConfirmText(result?.deadline ?? basicFields?.deadline),
    bidType: normalizeParseConfirmBidType(result?.bid_type ?? basicFields?.bid_type),
    qualificationRequirements: parseConfirmListText(
      result?.qualification_requirements ?? qualificationFields?.qualification_requirements,
    ),
    scoringPoints: parseConfirmListText(result?.scoring_points ?? evaluationFields?.scoring_points),
    invalidClauseRisks: parseConfirmListText(result?.invalid_clause_risks ?? invalidRiskFields?.invalid_clause_risks),
  }
}

function applyParseConfirmDraft(
  structuredResult: Record<string, unknown> | undefined,
  originalDraft: ParseConfirmDraft,
  draft: ParseConfirmDraft,
): Record<string, unknown> {
  const next = cloneStructuredResult(structuredResult)
  const qualificationRequirements = parseConfirmTextList(draft.qualificationRequirements)
  const scoringPoints = parseConfirmTextList(draft.scoringPoints)
  const invalidClauseRisks = parseConfirmTextList(draft.invalidClauseRisks)
  next.project_name = draft.projectName.trim()
  next.deadline = draft.deadline.trim()
  next.bid_type = draft.bidType
  next.qualification_requirements = qualificationRequirements
  next.scoring_points = scoringPoints
  next.invalid_clause_risks = invalidClauseRisks
  setParseModuleFields(next, 'basic', '基础信息', {
    project_name: next.project_name,
    deadline: next.deadline,
    bid_type: next.bid_type,
  })
  setParseModuleFields(next, 'qualification', '资格要求', {
    qualification_requirements: qualificationRequirements,
  })
  setParseModuleFields(next, 'evaluation', '评审办法', {
    scoring_points: scoringPoints,
  })
  setParseModuleFields(next, 'invalid_risk', '否决风险', {
    invalid_clause_risks: invalidClauseRisks,
  })
  syncParseConfirmRequirementItems(next, originalDraft, draft)
  next.parse_metadata = {
    ...(objectRecord(next.parse_metadata) ?? {}),
    confirm_overrides: {
      version: 'parse-confirm-draft-v1',
      edited_fields: parseConfirmChangedFields(originalDraft, draft),
      confirmed_at: new Date().toISOString(),
    },
  }
  return next
}

const parseConfirmRequirementFields: Array<{
  field: keyof Pick<ParseConfirmDraft, 'qualificationRequirements' | 'scoringPoints' | 'invalidClauseRisks'>
  moduleKey: string
  requirementType: string
  mandatory: boolean
  priority: 'high' | 'medium' | 'low'
}> = [
  {
    field: 'qualificationRequirements',
    moduleKey: 'qualification',
    requirementType: 'qualification',
    mandatory: true,
    priority: 'high',
  },
  {
    field: 'scoringPoints',
    moduleKey: 'evaluation',
    requirementType: 'scoring',
    mandatory: false,
    priority: 'high',
  },
  {
    field: 'invalidClauseRisks',
    moduleKey: 'invalid_risk',
    requirementType: 'invalid_risk',
    mandatory: true,
    priority: 'high',
  },
]

function syncParseConfirmRequirementItems(
  result: Record<string, unknown>,
  originalDraft: ParseConfirmDraft,
  draft: ParseConfirmDraft,
) {
  for (const config of parseConfirmRequirementFields) {
    if (parseConfirmComparableText(originalDraft[config.field]) === parseConfirmComparableText(draft[config.field])) {
      continue
    }
    replaceParseRequirementItems(result, {
      ...config,
      previousValues: parseConfirmTextList(originalDraft[config.field]),
      nextValues: parseConfirmTextList(draft[config.field]),
    })
  }
}

function replaceParseRequirementItems(
  result: Record<string, unknown>,
  config: {
    moduleKey: string
    requirementType: string
    mandatory: boolean
    priority: 'high' | 'medium' | 'low'
    previousValues: string[]
    nextValues: string[]
  },
) {
  const markers = new Set([...config.previousValues, ...config.nextValues].map(normalizeParseRequirementText))
  const nextItems = buildParseConfirmRequirementItems(config)
  const currentItems = arrayValue(result.requirement_items).map(objectRecord).filter(Boolean)
  result.requirement_items = [
    ...currentItems.filter((item) => !isReplaceableParseRequirement(item, config.moduleKey, markers)),
    ...nextItems,
  ]

  const modules = { ...(objectRecord(result.modules) ?? {}) }
  const currentModule = { ...(objectRecord(modules[config.moduleKey]) ?? {}) }
  const moduleItems = arrayValue(currentModule.requirement_items).map(objectRecord).filter(Boolean)
  modules[config.moduleKey] = {
    ...currentModule,
    requirement_items: [
      ...moduleItems.filter((item) => !isReplaceableParseRequirement(item, config.moduleKey, markers)),
      ...nextItems,
    ],
  }
  result.modules = modules
}

function buildParseConfirmRequirementItems(config: {
  moduleKey: string
  requirementType: string
  mandatory: boolean
  priority: 'high' | 'medium' | 'low'
  nextValues: string[]
}) {
  return config.nextValues.map((requirement, index) => ({
    id: `${config.moduleKey}-confirm-${index + 1}`,
    module: config.moduleKey,
    type: config.requirementType,
    requirement,
    priority: config.priority,
    mandatory: config.mandatory,
    status: 'needs_review',
    needs_review: true,
    expected_response: '按确认信息准备响应',
  }))
}

function isReplaceableParseRequirement(
  item: Record<string, unknown> | null,
  moduleKey: string,
  markers: Set<string>,
) {
  if (!item || String(item.module ?? '') !== moduleKey) return false
  const id = String(item.id ?? '')
  if (id.startsWith(`${moduleKey}-confirm-`)) return true
  return markers.has(normalizeParseRequirementText(formatStructuredValue(item.requirement)))
}

function normalizeParseRequirementText(value: string) {
  return value.trim().replace(/\s+/g, ' ')
}

function cloneStructuredResult(value: Record<string, unknown> | undefined): Record<string, unknown> {
  if (!value) return {}
  return JSON.parse(JSON.stringify(value)) as Record<string, unknown>
}

function parseModuleFields(modules: Record<string, unknown> | null, moduleKey: string) {
  return objectRecord(objectRecord(modules?.[moduleKey])?.fields)
}

function setParseModuleFields(
  result: Record<string, unknown>,
  moduleKey: string,
  title: string,
  fields: Record<string, unknown>,
) {
  const modules = { ...(objectRecord(result.modules) ?? {}) }
  const currentModule = { ...(objectRecord(modules[moduleKey]) ?? {}) }
  const currentFields = { ...(objectRecord(currentModule.fields) ?? {}) }
  modules[moduleKey] = {
    ...currentModule,
    module: String(currentModule.module || moduleKey),
    title: String(currentModule.title || title),
    fields: { ...currentFields, ...fields },
  }
  result.modules = modules
}

function parseConfirmChangedFieldLabels(originalDraft: ParseConfirmDraft, draft: ParseConfirmDraft) {
  return parseConfirmChangedFields(originalDraft, draft).map((field) => parseConfirmFieldLabels[field])
}

function parseConfirmChangedFields(originalDraft: ParseConfirmDraft, draft: ParseConfirmDraft) {
  return (Object.keys(parseConfirmFieldLabels) as Array<keyof ParseConfirmDraft>).filter((field) => {
    if (field === 'bidType') return originalDraft[field] !== draft[field]
    return parseConfirmComparableText(originalDraft[field]) !== parseConfirmComparableText(draft[field])
  })
}

function parseConfirmComparableText(value: string) {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .join('\n')
}

function parseConfirmText(value: unknown): string {
  return formatStructuredValue(value).trim()
}

function parseConfirmListText(value: unknown): string {
  if (Array.isArray(value)) {
    return value
      .map((item) => formatStructuredValue(item).trim())
      .filter(Boolean)
      .join('\n')
  }
  return parseConfirmText(value)
}

function parseConfirmTextList(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
}

function normalizeParseConfirmBidType(value: unknown): BidFormValues['bidType'] {
  const text = String(value ?? '').trim()
  if (text === 'combined' || text === 'separated' || text === 'custom') return text
  if (text.includes('综合')) return 'combined'
  if (text.includes('分离') || text.includes('分册')) return 'separated'
  return 'custom'
}

function structuredResultRows(result: Record<string, unknown> | undefined) {
  const rows = [
    { name: '项目名称', value: formatStructuredValue(result?.project_name) },
    { name: '投标截止', value: formatStructuredValue(result?.deadline) },
    { name: '资格要求', value: formatStructuredValue(result?.qualification_requirements) },
    { name: '废标风险', value: formatStructuredValue(result?.invalid_clause_risks) },
    { name: '评分重点', value: formatStructuredValue(result?.scoring_points) },
  ]
  return rows.filter((row) => row.value)
}

function structuredModuleRows(result: Record<string, unknown> | undefined) {
  const modules = objectRecord(result?.modules)
  if (!modules) return []
  return ['basic', 'qualification', 'evaluation', 'submission', 'invalid_risk', 'annex']
    .map((key) => {
      const module = objectRecord(modules[key])
      if (!module) return null
      const fields = objectRecord(module.fields)
      const evidence = arrayValue(module.evidence)
      const requirementItems = arrayValue(module.requirement_items)
      const summary = fields
        ? Object.values(fields)
            .map((value) => formatStructuredValue(value))
            .filter(Boolean)
            .slice(0, 5)
            .join('、')
        : ''
      return {
        key,
        title: String(module.title || parseModuleLabel(key)),
        status: String(module.status || ''),
        summary,
        requirementCount: requirementItems.length,
        needsReview:
          module.status === 'needs_review' ||
          evidence.some((item) => Boolean(objectRecord(item)?.needs_review)) ||
          requirementItems.some((item) => Boolean(objectRecord(item)?.needs_review)),
      }
    })
    .filter((row): row is NonNullable<typeof row> => Boolean(row))
}

function structuredRequirementRows(result: Record<string, unknown> | undefined) {
  return arrayValue(result?.requirement_items)
    .map((item, index) => {
      const record = objectRecord(item)
      if (!record) return null
      return {
        id: String(record.id || `requirement-${index}`),
        module: parseModuleLabel(String(record.module || '')),
        requirement: formatStructuredValue(record.requirement),
        mandatory: Boolean(record.mandatory),
        needsReview: Boolean(record.needs_review || record.status === 'needs_review'),
        coverageStatus: requirementCoverageStatusValue(record.status),
        coverageEvidence: '',
        coverageSourceCount: 0,
        coverageSourceRefs: [] as unknown[],
        canUpdate: false,
      }
    })
    .filter((row): row is NonNullable<typeof row> => Boolean(row?.requirement))
}

function requirementRowsFromItems(items: BidRequirementItemDTO[]) {
  return items
    .map((item) => {
      const latestCoverage = latestCoverageFromMetadata(item.metadata)
      const sourceRefs = arrayValue(latestCoverage?.source_refs)
      return {
        id: item.id || item.external_id,
        module: parseModuleLabel(item.module),
        requirement: item.requirement,
        mandatory: item.mandatory,
        needsReview: item.needs_review || item.coverage_status === 'needs_review',
        coverageStatus: item.coverage_status,
        coverageEvidence: formatStructuredValue(latestCoverage?.evidence),
        coverageSourceCount: sourceRefs.length,
        coverageSourceRefs: sourceRefs,
        canUpdate: true,
      }
    })
    .filter((row) => Boolean(row.requirement.trim()))
}

type ParseRequirementRow = ReturnType<typeof structuredRequirementRows>[number]

function filterRequirementRows(rows: ParseRequirementRow[], filter: RequirementFilter, evidenceFilter: RequirementEvidenceFilter) {
  const statusRows = (() => {
    switch (filter) {
      case 'mandatory':
        return rows.filter((row) => row.mandatory)
      case 'review':
        return rows.filter((row) => row.needsReview || row.coverageStatus === 'needs_review')
      case 'covered':
        return rows.filter((row) => row.coverageStatus === 'covered')
      default:
        return rows
    }
  })()
  switch (evidenceFilter) {
    case 'missing_evidence':
      return statusRows.filter((row) => row.coverageStatus === 'covered' && !row.coverageEvidence.trim())
    case 'missing_source':
      return statusRows.filter((row) => row.coverageStatus === 'covered' && row.coverageSourceCount === 0)
    case 'complete':
      return statusRows.filter((row) => row.coverageStatus === 'covered' && row.coverageEvidence.trim() && row.coverageSourceCount > 0)
    default:
      return statusRows
  }
}

const requirementEvidenceFilterOptions = [
  { label: '全部依据', value: 'all' },
  { label: '待补证据', value: 'missing_evidence' },
  { label: '待补来源', value: 'missing_source' },
  { label: '依据完整', value: 'complete' },
] satisfies Array<{ label: string; value: RequirementEvidenceFilter }>

function requirementCoverageStatusValue(value: unknown): BidRequirementItemDTO['coverage_status'] {
  const text = String(value || '').trim().toLowerCase()
  if (text === 'planned' || text === 'covered' || text === 'needs_review') return text
  if (text === 'done' || text === 'satisfied' || text === 'passed') return 'covered'
  if (text === 'partial' || text === 'review' || text === 'needs_check') return 'needs_review'
  return 'unmapped'
}

function requirementCoverageTag(value: BidRequirementItemDTO['coverage_status']) {
  if (value === 'covered') return <Tag color="green">已覆盖</Tag>
  if (value === 'planned') return <Tag color="blue">已规划</Tag>
  if (value === 'needs_review') return <Tag color="gold">待复核</Tag>
  return <Tag>未确认</Tag>
}

function requirementCoverageTimelineColor(value: BidRequirementItemDTO['coverage_status']) {
  if (value === 'covered') return 'green'
  if (value === 'planned') return 'blue'
  if (value === 'needs_review') return 'orange'
  return 'gray'
}

function requirementHistoryTimelineItem(event: BidRequirementCoverageEventDTO) {
  const sourceRefs = arrayValue(event.source_refs)
  const sourceSummary = requirementSourceRefsSummary(sourceRefs)
  return (
    <Space direction="vertical" size={4} className="full-width">
      <Space size={6} wrap>
        {requirementCoverageTag(event.coverage_status)}
        <Tag>{requirementEventSourceLabel(event.source)}</Tag>
        {event.needs_review ? <Tag color="gold">待确认</Tag> : null}
        <Typography.Text type="secondary">{formatDateTime(event.created_at)}</Typography.Text>
      </Space>
      {event.evidence.trim() ? (
        <Typography.Paragraph className="requirement-history-evidence">{event.evidence}</Typography.Paragraph>
      ) : (
        <Typography.Text type="secondary">未填写响应证据</Typography.Text>
      )}
      {sourceRefs.length ? (
        <Typography.Text type="secondary">
          来源 {sourceRefs.length} 处{sourceSummary ? `：${sourceSummary}` : ''}
        </Typography.Text>
      ) : (
        <Typography.Text type="secondary">未关联响应来源</Typography.Text>
      )}
    </Space>
  )
}

function requirementEventSourceLabel(value: BidRequirementCoverageEventDTO['source']) {
  if (value === 'manual') return '人工调整'
  if (value === 'model') return '自动生成'
  return '系统记录'
}

function requirementSourceRefsSummary(sourceRefs: unknown[]) {
  return sourceRefs
    .map((ref) => {
      const record = objectRecord(ref)
      if (!record) return ''
      const title = String(record.title || record.filename || record.document_title || '响应来源')
      const page = String(record.page || record.page_start || '').trim()
      const excerpt = formatStructuredValue(record.source_text || record.excerpt || record.text)
      const locator = firstSourceRefString(record, ['reference_id', 'referenceId', 'citation_id', 'citationId', 'chunk_id', 'chunkId'])
      return [title, page ? `第${page}页` : '', locator ? `定位：${locator}` : '', excerpt ? `摘录：${excerpt}` : '']
        .filter(Boolean)
        .join('，')
    })
    .filter(Boolean)
    .join('；')
}

function requirementSourceTitle(sourceRef: unknown) {
  const record = objectRecord(sourceRef)
  if (!record) return '响应来源'
  return String(record.title || record.filename || record.document_title || '响应来源')
}

function requirementSourcePreviewTarget(sourceRef: unknown): { type: 'file' | 'document'; id: string } | null {
  const record = objectRecord(sourceRef)
  if (!record) return null
  const fileID = firstSourceRefString(record, ['file_id', 'fileId', 'file_asset_id', 'fileAssetId'])
  if (fileID) {
    return { type: 'file', id: fileID }
  }
  const documentID = firstSourceRefString(record, ['source_document_id', 'sourceDocumentId', 'document_id', 'documentId'])
  if (documentID) {
    return { type: 'document', id: documentID }
  }
  return null
}

function firstSourceRefString(record: Record<string, unknown>, keys: string[]) {
  for (const key of keys) {
    const value = String(record[key] || '').trim()
    if (value) return value
  }
  return ''
}

function requirementSourcePage(sourceRef: unknown) {
  const record = objectRecord(sourceRef)
  if (!record) return null
  for (const key of ['page', 'page_start', 'pageStart']) {
    const value = Number(record[key])
    if (Number.isInteger(value) && value > 0) return value
  }
  return null
}

function requirementSourceLocatorParts(sourceRef: unknown) {
  const record = objectRecord(sourceRef)
  if (!record) return []
  const parts: Array<{ label: string; value: string }> = []
  const page = requirementSourcePage(sourceRef)
  if (page) {
    parts.push({ label: '页码', value: String(page) })
  }
  const section = firstSourceRefString(record, ['section_path', 'sectionPath', 'title_path', 'titlePath'])
  if (section) {
    parts.push({ label: '章节', value: section })
  }
  const referenceID = firstSourceRefString(record, ['reference_id', 'referenceId', 'citation_id', 'citationId'])
  if (referenceID) {
    parts.push({ label: '引用号', value: referenceID })
  }
  const chunkID = firstSourceRefString(record, ['chunk_id', 'chunkId'])
  if (chunkID) {
    parts.push({ label: '定位码', value: chunkID })
  }
  return parts
}

function requirementSourceLocatorText(sourceRef: unknown) {
  const record = objectRecord(sourceRef)
  if (!record) return ''
  const title = requirementSourceTitle(sourceRef)
  const parts = requirementSourceLocatorParts(sourceRef).map((part) => `${part.label}: ${part.value}`)
  const excerpt = formatStructuredValue(record.source_text || record.excerpt || record.text).trim()
  return [title, ...parts, excerpt ? `摘录: ${excerpt}` : ''].filter(Boolean).join('\n')
}

function withPreviewPageAnchor(url: string, page: number | null) {
  if (!page) return url
  try {
    const nextURL = new URL(url, window.location.href)
    nextURL.hash = `page=${page}`
    return nextURL.toString()
  } catch {
    return url
  }
}

const requirementCoverageOptions = [
  { label: '未确认', value: 'unmapped' },
  { label: '已规划', value: 'planned' },
  { label: '已覆盖', value: 'covered' },
  { label: '待复核', value: 'needs_review' },
] satisfies Array<{ label: string; value: BidRequirementItemDTO['coverage_status'] }>

type RequirementSourceRefDraft = {
  title: string
  page: string
  excerpt: string
}

function RequirementSourceRefsEditor({
  initialRefs,
  onChange,
}: {
  initialRefs: RequirementSourceRefDraft[]
  onChange: (refs: RequirementSourceRefDraft[]) => void
}) {
  const [drafts, setDrafts] = useState<RequirementSourceRefDraft[]>(initialRefs)
  const updateDrafts = (nextDrafts: RequirementSourceRefDraft[]) => {
    setDrafts(nextDrafts)
    onChange(nextDrafts)
  }
  const updateDraft = (index: number, patch: Partial<RequirementSourceRefDraft>) => {
    updateDrafts(drafts.map((draft, draftIndex) => (draftIndex === index ? { ...draft, ...patch } : draft)))
  }
  return (
    <div className="requirement-source-editor">
      <Space align="center" className="full-width requirement-source-editor-head">
        <Typography.Text strong>响应来源</Typography.Text>
        <Button size="small" icon={<PlusOutlined />} onClick={() => updateDrafts([...drafts, { title: '', page: '', excerpt: '' }])}>
          添加来源
        </Button>
      </Space>
      {drafts.length ? (
        <Space direction="vertical" size={8} className="full-width">
          {drafts.map((draft, index) => (
            <div className="requirement-source-row" key={index}>
              <Input
                value={draft.title}
                placeholder="文件或章节"
                maxLength={120}
                onChange={(event) => updateDraft(index, { title: event.target.value })}
              />
              <Input
                value={draft.page}
                placeholder="页码"
                maxLength={24}
                onChange={(event) => updateDraft(index, { page: event.target.value })}
              />
              <Input.TextArea
                value={draft.excerpt}
                placeholder="原文摘录"
                rows={2}
                maxLength={500}
                onChange={(event) => updateDraft(index, { excerpt: event.target.value })}
              />
              <Button
                size="small"
                type="text"
                danger
                icon={<DeleteOutlined />}
                onClick={() => updateDrafts(drafts.filter((_, draftIndex) => draftIndex !== index))}
              />
            </div>
          ))}
        </Space>
      ) : (
        <Typography.Text type="secondary">可添加文件名、页码和原文摘录</Typography.Text>
      )}
    </div>
  )
}

function sourceRefDraftsFromRefs(sourceRefs: unknown[]): RequirementSourceRefDraft[] {
  return sourceRefs
    .map((ref) => {
      const record = objectRecord(ref)
      if (!record) return null
      return {
        title: String(record.title || record.filename || record.document_title || '').trim(),
        page: String(record.page || record.page_start || '').trim(),
        excerpt: formatStructuredValue(record.source_text || record.excerpt || record.text).trim(),
      }
    })
    .filter((draft): draft is RequirementSourceRefDraft => Boolean(draft && (draft.title || draft.page || draft.excerpt)))
}

function sourceRefsFromDrafts(drafts: RequirementSourceRefDraft[]): unknown[] {
  const refs: Record<string, string>[] = []
  for (const draft of drafts) {
    const title = draft.title.trim()
    const page = draft.page.trim()
    const excerpt = draft.excerpt.trim()
    if (!title && !page && !excerpt) continue
    refs.push({
      title: title || '响应来源',
      ...(page ? { page } : {}),
      ...(excerpt ? { excerpt, source_text: excerpt } : {}),
    })
  }
  return refs
}

function inferBatchCoverageStatus(rows: ParseRequirementRow[]): BidRequirementItemDTO['coverage_status'] {
  if (!rows.length) return 'covered'
  const firstStatus = rows[0].coverageStatus
  if (firstStatus !== 'unmapped' && rows.every((row) => row.coverageStatus === firstStatus)) {
    return firstStatus
  }
  return 'covered'
}

function requirementEvidenceCell(
  row: ParseRequirementRow,
  canWrite: boolean,
  isUpdating: boolean,
  onEdit: (row: ParseRequirementRow) => void,
  onOpenSources: (row: ParseRequirementRow) => void,
) {
  const evidence = row.coverageEvidence.trim()
  const sourceCount = row.coverageSourceCount
  const editButton =
    canWrite && row.canUpdate ? (
      <Button size="small" type="link" loading={isUpdating} onClick={() => onEdit(row)}>
        补证据
      </Button>
    ) : null
  if (!evidence && sourceCount === 0) {
    return (
      <Space size={4}>
        {row.coverageStatus === 'covered' ? <Tag color="gold">待补证据</Tag> : <Typography.Text type="secondary">-</Typography.Text>}
        {editButton}
      </Space>
    )
  }
  return (
    <div className="requirement-evidence-cell">
      <Tooltip title={evidence || '已关联响应来源'}>
        <Typography.Text className="requirement-evidence-text">{evidence || '已关联响应来源'}</Typography.Text>
      </Tooltip>
      {sourceCount > 0 ? (
        <Button size="small" type="link" onClick={() => onOpenSources(row)}>
          来源 {sourceCount} 处
        </Button>
      ) : (
        <Tag color="gold">待补来源</Tag>
      )}
      {editButton}
    </div>
  )
}

function latestCoverageFromMetadata(metadata: Record<string, unknown> | undefined) {
  return objectRecord(objectRecord(metadata)?.latest_coverage)
}

function saveBlobFile(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

function parseModuleLabel(value: string) {
  const labels: Record<string, string> = {
    basic: '基础信息',
    qualification: '资格要求',
    evaluation: '评审办法',
    submission: '递交要求',
    invalid_risk: '否决风险',
    annex: '附件格式',
  }
  return labels[value] ?? '其他信息'
}

function parseReviewTag(needsReview: boolean) {
  return needsReview ? <Tag color="gold">待确认</Tag> : <Tag color="green">可确认</Tag>
}

function formatStructuredValue(value: unknown): string {
  if (Array.isArray(value)) {
    return value.map((item) => String(item)).join('、')
  }
  if (value && typeof value === 'object') {
    return Object.values(value as Record<string, unknown>)
      .map((item) => formatStructuredValue(item))
      .filter(Boolean)
      .join('、')
  }
  return value ? String(value) : ''
}

function objectRecord(value: unknown): Record<string, unknown> | null {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>
  }
  return null
}

function arrayValue(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

type RequirementCoverageRow = {
  id: string
  requirement: string
  evidence: string
  label: string
  color: 'green' | 'gold' | 'default'
}

function latestRequirementCoverageRows(versions: BidChapterVersionDTO[] | undefined): RequirementCoverageRow[] {
  for (const version of versions ?? []) {
    const rows = requirementCoverageRows(version.model_metadata)
    if (rows.length) return rows
  }
  return []
}

function requirementCoverageRows(metadata: Record<string, unknown> | undefined): RequirementCoverageRow[] {
  const record = objectRecord(metadata)
  if (!record) return []
  const selfCheck = objectRecord(record.self_check)
  const coverageItems = arrayValue(record.requirement_coverage).length
    ? arrayValue(record.requirement_coverage)
    : arrayValue(selfCheck?.requirement_coverage)
  return coverageItems
    .map((item, index) => {
      const coverage = objectRecord(item)
      if (!coverage) return null
      const requirement = formatStructuredValue(coverage.requirement)
      if (!requirement) return null
      const status = requirementCoverageStatus(coverage)
      const sourceCount = arrayValue(coverage.source_refs).length
      const evidence = formatStructuredValue(coverage.evidence)
      return {
        id: String(coverage.requirement_id || `coverage-${index}`),
        requirement,
        evidence: evidence || (sourceCount ? `已关联 ${sourceCount} 处来源` : ''),
        label: status.label,
        color: status.color,
      }
    })
    .filter((row): row is RequirementCoverageRow => Boolean(row))
}

function requirementCoverageStatus(record: Record<string, unknown>): Pick<RequirementCoverageRow, 'label' | 'color'> {
  if (record.satisfied === true && record.needs_review !== true) {
    return { label: '已覆盖', color: 'green' }
  }
  if (record.needs_review === true || record.satisfied === false) {
    return { label: '待复核', color: 'gold' }
  }
  return { label: '未确认', color: 'default' }
}

function summarizeRequirementCoverage(rows: RequirementCoverageRow[]) {
  return rows.reduce(
    (summary, row) => {
      if (row.label === '已覆盖') summary.done += 1
      else if (row.label === '待复核') summary.review += 1
      else summary.pending += 1
      return summary
    },
    { done: 0, review: 0, pending: 0 },
  )
}

function materialRefRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>
  }
  return { title: formatStructuredValue(value), selected: true }
}

function materialRefTitle(value: unknown) {
  const record = materialRefRecord(value)
  return String(record.title ?? record.name ?? record.ref_type ?? '素材')
}

function materialRefReason(value: unknown) {
  const record = materialRefRecord(value)
  return String(record.reason ?? record.description ?? '')
}

function materialRefSelected(value: unknown) {
  return materialRefRecord(value).selected !== false
}

export function BidEditorPage() {
  const { bidId = '' } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canWrite = useCanAccess('bid', 'full')
  const [regenerateTaskId, setRegenerateTaskId] = useState('')
  const regenerateTaskNoticeRef = useRef('')
  const [generationSnapshot, setGenerationSnapshot] = useState<BidGenerationSnapshotDTO | null>(null)
  const [generationStreamState, setGenerationStreamState] = useState<{
    bidId: string
    status: GenerationStreamStatus
  }>({ bidId: '', status: 'connecting' })
  const generationStreamStatus = generationStreamState.bidId === bidId ? generationStreamState.status : 'connecting'
  const [diffOpen, setDiffOpen] = useState(false)
  const partParam = searchParams.get('part') ?? ''
  const chapterParam = searchParams.get('chapter') ?? ''
  const bid = useQuery({
    queryKey: ['bid', bidId],
    queryFn: () => fetchBid(bidId),
    enabled: Boolean(bidId),
  })
  const parts = useQuery({
    queryKey: ['bid-parts', bidId],
    queryFn: () => fetchBidParts(bidId),
    enabled: Boolean(bidId),
  })
  const chapters = useQuery({
    queryKey: ['bid-chapters', bidId],
    queryFn: () => fetchBidChapters(bidId),
    enabled: Boolean(bidId),
  })
  const activePart = (parts.data ?? []).find((part) => part.code === partParam)
  const visibleChapters = (chapters.data ?? []).filter((chapter) => !activePart || chapter.bid_part_id === activePart.id)
  const currentChapter = visibleChapters.find((chapter) => chapter.id === chapterParam) ?? visibleChapters[0]
  const versions = useQuery({
    queryKey: ['chapter-versions', currentChapter?.id],
    queryFn: () => fetchChapterVersions(currentChapter?.id ?? ''),
    enabled: Boolean(currentChapter?.id),
  })
  const diffQuery = useQuery({
    queryKey: ['chapter-diff', currentChapter?.id],
    queryFn: () => fetchChapterDiff(currentChapter?.id ?? ''),
    enabled: diffOpen && Boolean(currentChapter?.id),
  })
  const regenerateTask = useQuery({
    queryKey: ['ai-task', regenerateTaskId],
    queryFn: () => fetchAITask(regenerateTaskId),
    enabled: Boolean(regenerateTaskId),
    refetchInterval: (query) => {
      const task = query.state.data
      if (!task) return 1500
      return task.status === 'queued' || task.status === 'running' ? 1500 : false
    },
  })
  const regenerateTaskStatus = regenerateTask.data?.status
  const generationSummary = generationSnapshot?.summary
  const completedChapterCount = (generationSummary?.generated_chapters ?? 0) + (generationSummary?.accepted_chapters ?? 0)
  const generationPercent = generationSummary?.total_chapters
    ? Math.round((completedChapterCount / generationSummary.total_chapters) * 100)
    : 0
  const latestChapterTask = generationSnapshot?.tasks.find((task) => task.chapter_id === currentChapter?.id)
  const editorDirtyRef = useRef(false)
  const loadedEditorSourceRef = useRef('')
  const editorSourceKey = currentChapter ? `${currentChapter.id}:${currentChapter.updated_at}` : ''
  const editor = useEditor({
    extensions: [StarterKit],
    content: '<p>请选择章节</p>',
    editable: canWrite,
    onUpdate: () => {
      editorDirtyRef.current = true
    },
  })
  useEffect(() => {
    if (!editor || !currentChapter) return
    if (loadedEditorSourceRef.current === editorSourceKey) return
    const chapterChanged = !loadedEditorSourceRef.current.startsWith(`${currentChapter.id}:`)
    if (!chapterChanged && editorDirtyRef.current) return
    editor.commands.setContent(contentForEditor(currentChapter))
    loadedEditorSourceRef.current = editorSourceKey
    editorDirtyRef.current = false
  }, [editor, currentChapter, editorSourceKey])
  useEffect(() => {
    if (!regenerateTaskId || !regenerateTaskStatus) return
    if (!isTerminalTaskStatus(regenerateTaskStatus)) return
    const noticeKey = `${regenerateTaskId}:${regenerateTaskStatus}`
    if (regenerateTaskNoticeRef.current === noticeKey) return
    regenerateTaskNoticeRef.current = noticeKey
    if (regenerateTaskStatus === 'done') {
      message.success('本章已重新生成')
      void queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] })
      void queryClient.invalidateQueries({ queryKey: ['chapter-versions', currentChapter?.id] })
    }
    if (regenerateTaskStatus === 'failed' || regenerateTaskStatus === 'cancelled') {
      message.error(taskFailureMessage(regenerateTask.data, '重新生成失败'))
      void queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] })
    }
  }, [regenerateTaskId, regenerateTaskStatus, regenerateTask.data, message, queryClient, bidId, currentChapter?.id])
  useEffect(() => {
    if (!bidId) return
    return openSse<BidGenerationSnapshotDTO>(`/bids/${bidId}/generation/stream`, {
      onOpen: () => setGenerationStreamState({ bidId, status: 'open' }),
      onMessage: (snapshot, event) => {
        if (event !== 'generation') return
        setGenerationSnapshot(snapshot)
        void queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] })
        if (currentChapter?.id) {
          void queryClient.invalidateQueries({ queryKey: ['chapter-versions', currentChapter.id] })
        }
      },
      onError: (error) => {
        setGenerationStreamState({ bidId, status: 'error' })
        message.error(error.message)
      },
    })
  }, [bidId, queryClient, currentChapter?.id, message])

  const invalidateChapterState = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] }),
      queryClient.invalidateQueries({ queryKey: ['chapter-versions', currentChapter?.id] }),
    ])
  }
  const saveMutation = useMutation({
    mutationFn: () =>
      updateChapterContent(currentChapter?.id ?? '', {
        title: currentChapter?.title,
        content: editor?.getJSON() as Record<string, unknown>,
        plain_text: editor?.getText() ?? '',
      }),
    onSuccess: async () => {
      message.success('章节已保存')
      await invalidateChapterState()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '保存章节失败')),
  })
  const acceptMutation = useMutation({
    mutationFn: () => acceptChapter(currentChapter?.id ?? ''),
    onSuccess: async () => {
      message.success('章节已采纳')
      await invalidateChapterState()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '采纳章节失败')),
  })
  const regenerateMutation = useMutation({
    mutationFn: () => regenerateChapter(currentChapter?.id ?? ''),
    onSuccess: async (result) => {
      setRegenerateTaskId(result.task.id)
      message.success('已开始重新生成本章')
      await invalidateChapterState()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '重新生成失败')),
  })
  const aiActionMutation = useMutation({
    mutationFn: (action: 'optimize' | 'expand' | 'shorten' | 'add_detail' | 'self_check') =>
      chapterAiAction(currentChapter?.id ?? '', { action }),
    onSuccess: async (result) => {
      setRegenerateTaskId(result.task.id)
      message.success('已开始处理本章')
      await invalidateChapterState()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '处理本章失败')),
  })
  const isRegenerating = Boolean(
    regenerateMutation.isPending ||
      aiActionMutation.isPending ||
      isRegenerateTaskActive(regenerateTaskStatus, regenerateTaskId) ||
      currentChapter?.status === 'generating',
  )

  if (bid.isLoading || parts.isLoading || chapters.isLoading) return <LoadingBlock />
  if (bid.isError || parts.isError || chapters.isError) return <ErrorBlock />

  const acceptedCount = visibleChapters.filter((chapter) => chapter.status === 'accepted').length
  const acceptedPercent = visibleChapters.length
    ? Math.round((acceptedCount / visibleChapters.length) * 100)
    : 0
  const humanInputItems = currentChapter?.needs_human_input ?? []
  const latestChapterTaskFailure = taskFailureMessage(latestChapterTask, '本章处理失败')
  const requirementCoverageRows = latestRequirementCoverageRows(versions.data)
  const requirementCoverageSummary = summarizeRequirementCoverage(requirementCoverageRows)

  const switchPart = (code: string) => {
    setSearchParams(code === 'all' ? {} : { part: code })
  }

  return (
    <PageFrame
      module="标书生成"
      title="标书编辑器"
      subtitle={bid.data?.title ?? bidId}
      tags={['在线编辑', '章节协同']}
      bare
      actions={[
        <Button key="save" type="primary" icon={<SaveOutlined />} loading={saveMutation.isPending} disabled={!canWrite || !currentChapter} onClick={() => saveMutation.mutate()}>
          保存
        </Button>,
        <Button key="accept" icon={<CheckOutlined />} loading={acceptMutation.isPending} disabled={!canWrite || !currentChapter} onClick={() => acceptMutation.mutate()}>
          采纳
        </Button>,
        <Button key="regen" icon={<SyncOutlined />} loading={isRegenerating} disabled={!canWrite || !currentChapter || isRegenerating} onClick={() => regenerateMutation.mutate()}>
          重新生成
        </Button>,
        <Button key="export" icon={<DownloadOutlined />} disabled={!canWrite}>
          {canWrite ? <Link to={`/bids/${bidId}/wizard?step=7`}>导出</Link> : '导出'}
        </Button>,
      ]}
    >
      <div className="workbench">
        <Card title="章节大纲" size="small">
          {(parts.data?.length ?? 0) > 1 ? (
            <Segmented
              block
              size="small"
              style={{ marginBottom: 12 }}
              value={activePart?.code ?? 'all'}
              options={[
                { label: '全部', value: 'all' },
                ...(parts.data ?? []).map((part) => ({ label: part.title || part.code, value: part.code })),
              ]}
              onChange={(value) => switchPart(String(value))}
            />
          ) : null}
          <div className="outline-progress">
            <div className="outline-progress-label">
              <span>已采纳 {acceptedCount}/{visibleChapters.length} 章</span>
              <span className="data-mono">{acceptedPercent}%</span>
            </div>
            <Progress percent={acceptedPercent} size="small" showInfo={false} strokeColor="#16A34A" />
          </div>
          {visibleChapters.length ? (
            <ul className="outline-list">
              {visibleChapters.map((chapter, index) => (
                <li key={chapter.id}>
                  <button
                    type="button"
                    className={`outline-item${chapter.id === currentChapter?.id ? ' active' : ''}`}
                    title={chapterStatusText(chapter.status)}
                    onClick={() =>
                      setSearchParams({ ...(partParam ? { part: partParam } : {}), chapter: chapter.id })
                    }
                  >
                    <span className="outline-no">{String(index + 1).padStart(2, '0')}</span>
                    <span className="outline-title">{chapter.title}</span>
                    <span className={`outline-dot ${chapter.status}`} />
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <EmptyBlock description="本部分还没有章节，先在生成向导中生成大纲" />
          )}
        </Card>
        <div>
          <Card className="editor-zone">
            <div className="editor-zone-head">
              <Typography.Title level={4} className="editor-zone-title">
                {currentChapter?.title ?? '内容编辑'}
              </Typography.Title>
              {currentChapter ? (
                <Tag color={chapterStatusColor(currentChapter.status)}>
                  {chapterStatusText(currentChapter.status)}
                </Tag>
              ) : null}
              {currentChapter ? (
                <Typography.Text type="secondary" className="data-mono" style={{ fontSize: 12 }}>
                  {(currentChapter.plain_text ?? '').length} 字
                </Typography.Text>
              ) : null}
            </div>
            <div className="editor-paper-wrap">
              <div className="editor-paper">
                <EditorContent editor={editor} />
              </div>
            </div>
          </Card>
          <Card title="版本记录" size="small" style={{ marginTop: 16 }}>
            <Table
              size="small"
              rowKey="id"
              pagination={false}
              loading={versions.isLoading}
              locale={{ emptyText: <EmptyBlock description="保存或生成后会在这里留下版本" /> }}
              dataSource={versions.data ?? []}
              columns={[
                {
                  title: '版本',
                  dataIndex: 'version_no',
                  width: 80,
                  render: (value) => <span className="data-mono">v{value}</span>,
                },
                {
                  title: '原因',
                  dataIndex: 'change_reason',
                  ellipsis: true,
                  render: (value: string) => changeReasonLabels[value] || '内容更新',
                },
                {
                  title: '状态',
                  dataIndex: 'status',
                  width: 100,
                  render: (value) => <Tag>{chapterStatusText(value)}</Tag>,
                },
                {
                  title: '时间',
                  dataIndex: 'created_at',
                  width: 180,
                  render: (value) => <span className="data-mono">{formatDateTime(value)}</span>,
                },
              ]}
            />
          </Card>
        </div>
        <Card title="智能助手" className="ai-card" size="small">
          <Space direction="vertical" size={14} style={{ width: '100%' }}>
            <div className="ai-panel-row">
              <span>
                <span className={`conn-dot ${generationStreamStatus}`} />
                协作状态
              </span>
              <span>{streamStatusLabels[generationStreamStatus]}</span>
            </div>
            <div>
              <div className="ai-panel-row" style={{ marginBottom: 4 }}>
                <span>生成进度</span>
                <span className="ai-panel-value">
                  {completedChapterCount}/{generationSummary?.total_chapters ?? visibleChapters.length} 章
                </span>
              </div>
              <Progress
                percent={generationPercent}
                size="small"
                showInfo={false}
                strokeColor={{ '0%': '#7C3AED', '100%': '#2C5FA8' }}
              />
            </div>
            <div className="ai-panel-row">
              <span>本章引用素材</span>
              <span className="ai-panel-value">{currentChapter?.source_refs.length ?? 0} 处</span>
            </div>
            <div className="ai-panel-row">
              <span>待补充信息</span>
              <span className="ai-panel-value">{humanInputItems.length} 项</span>
            </div>
            {humanInputItems.length ? (
              <ul className="human-input-list">
                {humanInputItems.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            ) : null}
            {requirementCoverageRows.length ? (
              <div className="requirement-coverage">
                <div className="requirement-coverage-head">
                  <span>响应覆盖</span>
                  <Space size={4} wrap>
                    <Tag color="green">已覆盖 {requirementCoverageSummary.done}</Tag>
                    <Tag color="gold">待复核 {requirementCoverageSummary.review}</Tag>
                    <Tag>未确认 {requirementCoverageSummary.pending}</Tag>
                  </Space>
                </div>
                <ul className="requirement-coverage-list">
                  {requirementCoverageRows.slice(0, 5).map((row) => (
                    <li key={row.id}>
                      <div className="requirement-coverage-line">
                        <span>{row.requirement}</span>
                        <Tag color={row.color}>{row.label}</Tag>
                      </div>
                      {row.evidence ? <p>{row.evidence}</p> : null}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {isRegenerating || latestChapterTask?.status === 'running' ? (
              <Alert
                type="info"
                showIcon
                icon={<SyncOutlined spin />}
                message="正在处理本章，完成后自动刷新"
              />
            ) : null}
            {latestChapterTaskFailure ? <Alert type="error" showIcon message={latestChapterTaskFailure} /> : null}
            <Button
              block
              icon={<SyncOutlined />}
              loading={isRegenerating}
              disabled={!canWrite || !currentChapter || isRegenerating}
              onClick={() => regenerateMutation.mutate()}
            >
              查找素材并重新生成
            </Button>
            <Space wrap size={[6, 6]}>
              <Button size="small" icon={<EditOutlined />} disabled={!canWrite || !currentChapter || isRegenerating} onClick={() => aiActionMutation.mutate('optimize')}>
                优化
              </Button>
              <Button size="small" icon={<ExpandAltOutlined />} disabled={!canWrite || !currentChapter || isRegenerating} onClick={() => aiActionMutation.mutate('expand')}>
                扩写
              </Button>
              <Button size="small" icon={<CompressOutlined />} disabled={!canWrite || !currentChapter || isRegenerating} onClick={() => aiActionMutation.mutate('shorten')}>
                缩写
              </Button>
              <Button size="small" icon={<PlusCircleOutlined />} disabled={!canWrite || !currentChapter || isRegenerating} onClick={() => aiActionMutation.mutate('add_detail')}>
                加细节
              </Button>
              <Button size="small" icon={<SafetyCertificateOutlined />} disabled={!canWrite || !currentChapter || isRegenerating} onClick={() => aiActionMutation.mutate('self_check')}>
                自检
              </Button>
            </Space>
            <Button block icon={<DiffOutlined />} disabled={!currentChapter} onClick={() => setDiffOpen(true)}>
              对比上一版本
            </Button>
          </Space>
        </Card>
      </div>
      <Modal
        open={diffOpen}
        onCancel={() => setDiffOpen(false)}
        footer={null}
        width={920}
        title={`版本对比 · ${currentChapter?.title ?? ''}`}
      >
        {diffQuery.isLoading ? (
          <LoadingBlock />
        ) : diffQuery.isError ? (
          <ErrorBlock />
        ) : diffQuery.data?.previous ? (
          <div className="diff-grid">
            <div className="diff-pane previous">
              <div className="diff-pane-head">
                上一版本
                <Tag>v{diffQuery.data.previous.version_no}</Tag>
              </div>
              <pre className="diff-pane-body">{diffQuery.data.previous.plain_text || '（无正文）'}</pre>
            </div>
            <div className="diff-pane">
              <div className="diff-pane-head">
                当前版本
                <Tag color="blue">
                  {chapterStatusText(diffQuery.data.current.status)}
                </Tag>
              </div>
              <pre className="diff-pane-body">{diffQuery.data.current.plain_text || '（无正文）'}</pre>
            </div>
          </div>
        ) : (
          <EmptyBlock description="本章还没有历史版本，生成或保存后即可对比" />
        )}
      </Modal>
    </PageFrame>
  )
}

function contentForEditor(chapter: BidChapterDTO) {
  if (chapter.content?.type === 'doc') return chapter.content
  return {
    type: 'doc',
    content: [
      {
        type: 'paragraph',
        content: [{ type: 'text', text: chapter.plain_text || '请补充本章节内容。' }],
      },
    ],
  }
}

const chapterStatusLabels: Record<string, string> = {
  pending: '待生成',
  queued: '排队中',
  generating: '生成中',
  generated: '已生成',
  edited: '已编辑',
  accepted: '已采纳',
  needs_fix: '需完善',
}

function chapterStatusText(status: string) {
  return chapterStatusLabels[status] || '状态未知'
}

const changeReasonLabels: Record<string, string> = {
  ai_regenerate: '重新生成',
  ai_action: '智能改写',
  manual_edit: '手动编辑',
  accepted: '采纳定稿',
}

type GenerationStreamStatus = 'connecting' | 'open' | 'error'

const streamStatusLabels: Record<GenerationStreamStatus, string> = {
  connecting: '同步中…',
  open: '同步正常',
  error: '同步中断，刷新页面可恢复',
}

function chapterStatusColor(status: BidChapterDTO['status']) {
  if (status === 'accepted') return 'green'
  if (status === 'edited') return 'orange'
  if (status === 'generated') return 'blue'
  if (status === 'generating') return 'purple'
  if (status === 'needs_fix') return 'red'
  return 'default'
}

function isRegenerateTaskActive(status: string | undefined, taskId: string) {
  if (!taskId) return false
  return !status || status === 'queued' || status === 'running'
}
