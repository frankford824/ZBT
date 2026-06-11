import {
  CheckOutlined,
  DiffOutlined,
  DownloadOutlined,
  FilePdfOutlined,
  FileZipOutlined,
  PlusOutlined,
  SaveOutlined,
  SyncOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import { zodResolver } from '@hookform/resolvers/zod'
import { EditorContent, useEditor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import {
  Button,
  Card,
  Col,
  Form,
  Input,
  Progress,
  Radio,
  Row,
  Segmented,
  Space,
  Steps,
  Table,
  Tabs,
  Tag,
  Timeline,
  Typography,
  Upload,
  App as AntApp,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { z } from 'zod'
import {
  acceptChapter,
  confirmBidParseResult,
  confirmFileUpload,
  createBid,
  createBidExport,
  createPresignedUpload,
  fetchAITask,
  fetchBidChapters,
  fetchBid,
  fetchBidExport,
  fetchBidExports,
  fetchBidGenerationJobs,
  fetchBidMaterialSelection,
  fetchBidParseResult,
  fetchBidParts,
  fetchBidTemplates,
  fetchBids,
  fetchChapterVersions,
  generateBid,
  generateBidOutline,
  cancelBidGenerationJob,
  pauseBidGenerationJob,
  parseBidTender,
  regenerateChapter,
  resumeBidGenerationJob,
  submitBidForApproval,
  updateBidMaterialSelection,
  updateBidPartOutline,
  updateChapterContent,
  uploadBidTenderFile,
  uploadToPresignedUrl,
  useBidTemplate,
  type BidChapterDTO,
  type BidDocumentDTO,
  type BidExportDTO,
  type BidGenerationJobDTO,
  type BidGenerationSnapshotDTO,
  type BidTemplateDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
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

export function BidNewPage() {
  const navigate = useNavigate()
  const { message } = AntApp.useApp()
  const mutation = useMutation({
    mutationFn: createBid,
    onSuccess: (bid) => {
      message.success('标书已创建')
      navigate(`/bids/${bid.id}/wizard?step=1`)
    },
    onError: () => message.error('创建标书失败'),
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
      tags={['page-generate-new', '/bids/new']}
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
              validateStatus={errors.projectName ? 'error' : undefined}
              help={errors.projectName?.message}
            >
              <Controller
                name="projectName"
                control={control}
                render={({ field }) => <Input {...field} placeholder="某市智慧交通综合治理平台建设项目" />}
              />
            </Form.Item>
            <Form.Item label="招标单位">
              <Controller
                name="tenderOrg"
                control={control}
                render={({ field }) => <Input {...field} placeholder="某市交通运输局" />}
              />
            </Form.Item>
            <Row gutter={12}>
              <Col span={12}>
                <Form.Item label="投标截止日期">
                  <Controller
                    name="deadline"
                    control={control}
                    render={({ field }) => <Input {...field} placeholder="2026-06-18" />}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="项目预算">
                  <Controller
                    name="budget"
                    control={control}
                    render={({ field }) => <Input {...field} placeholder="12800000" />}
                  />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item label="标书类型">
              <Controller
                name="bidType"
                control={control}
                render={({ field }) => (
                  <Radio.Group {...field}>
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
          <Card title="生成队列">
            <Timeline
              items={[
                { color: 'blue', children: '创建 bid_document 草稿' },
                { color: 'blue', children: '按类型创建 bid_parts' },
                { color: 'gray', children: '上传招标文件并触发解析' },
                { color: 'gray', children: '进入 7 步向导' },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </PageFrame>
  )
}

export function BidListPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
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
    onError: () => message.error('提交审批失败'),
  })
  if (bids.isLoading) return <LoadingBlock />
  if (bids.isError) return <ErrorBlock />

  return (
    <PageFrame
      module="标书生成"
      title="我的标书"
      subtitle="状态筛选、批量导出、审批和编辑入口"
      tags={['page-generate-list', '/bids']}
      actions={[
        <Button key="new" type="primary" icon={<PlusOutlined />}>
          <Link to="/bids/new">新建标书</Link>
        </Button>,
      ]}
    >
      <Space direction="vertical" size={16} className="full-width">
        <Segmented options={['全部', '编制中', '审核中', '已完成']} />
        <Table
          rowKey="id"
          rowSelection={{}}
          dataSource={bids.data ?? []}
          columns={[
            {
              title: '标书名称',
              dataIndex: 'title',
              render: (value: string, row: BidDocumentDTO) => <Link to={`/bids/${row.id}/editor`}>{value}</Link>,
            },
            { title: '项目', dataIndex: 'project_name', render: (value: string) => value || '未关联项目' },
            { title: '类型', dataIndex: 'bid_type', render: bidTypeLabel },
            { title: '状态', dataIndex: 'status', render: bidStatusLabel },
            {
              title: '进度',
              render: (_, row: BidDocumentDTO) => <Progress percent={row.status === 'editing' ? 72 : 18} size="small" />,
            },
            {
              title: '操作',
              render: (_, row: BidDocumentDTO) => (
                <Space>
                  <Link to={`/bids/${row.id}/wizard?step=5`}>生成</Link>
                  <Link to={`/bids/${row.id}/editor`}>编辑</Link>
                  <Button
                    type="link"
                    size="small"
                    disabled={row.status === 'in_review' || row.status === 'approved'}
                    loading={approvalMutation.isPending && approvalMutation.variables === row.id}
                    onClick={() => approvalMutation.mutate(row.id)}
                  >
                    提交审批
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Space>
    </PageFrame>
  )
}

export function BidTemplatesPage() {
  const navigate = useNavigate()
  const { message } = AntApp.useApp()
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
    onError: () => message.error('模板使用失败'),
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
      tags={['page-generate-templates', '/bids/templates']}
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
                actions={[
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
                ]}
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
  const [searchParams, setSearchParams] = useSearchParams()
  const step = Number(searchParams.get('step') || '1')
  const current = Math.min(Math.max(step, 1), 7) - 1
  const steps = ['项目信息', 'AI解析', '目录大纲', '知识库配置', '逐章生成', '标书编辑器', '导出提交']
  const [tenderFile, setTenderFile] = useState<File | null>(null)
  const [parseDraftText, setParseDraftText] = useState('')
  const [outlineDrafts, setOutlineDrafts] = useState<Record<string, OutlineDraftChapter[]>>({})
  const [materialDraft, setMaterialDraft] = useState<unknown[]>([])
  const [materialNotes, setMaterialNotes] = useState('')
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
  })
  const materialSelection = useQuery({
    queryKey: ['bid-material-selection', bidId],
    queryFn: () => fetchBidMaterialSelection(bidId),
    enabled: Boolean(bidId),
  })
  useEffect(() => {
    if (!parseResult.data) return
    setParseDraftText(JSON.stringify(parseResult.data.structured_result ?? {}, null, 2))
  }, [parseResult.data?.updated_at])
  useEffect(() => {
    if (!parts.data || !chapters.data) return
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
    setOutlineDrafts(next)
  }, [parts.data, chapters.data])
  useEffect(() => {
    if (!materialSelection.data) return
    setMaterialDraft(materialSelection.data.selected_refs ?? [])
    setMaterialNotes(materialSelection.data.notes ?? '')
  }, [materialSelection.data?.updated_at])
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
      return items.some((item) => item.status === 'queued' || item.status === 'running' || item.status === 'paused') ? 2000 : false
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
    onError: () => message.error('上传招标文件失败'),
  })
  const parseTenderMutation = useMutation({
    mutationFn: () => parseBidTender(bidId),
    onSuccess: async () => {
      message.success('解析任务已完成')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['bid-parse-result', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-material-selection', bidId] }),
      ])
    },
    onError: () => message.error('解析招标文件失败'),
  })
  const confirmParseMutation = useMutation({
    mutationFn: () => {
      const structured = parseDraftText.trim() ? (JSON.parse(parseDraftText) as Record<string, unknown>) : undefined
      return confirmBidParseResult(bidId, { structured_result: structured })
    },
    onSuccess: async () => {
      message.success('解析结果已确认')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['bid-parse-result', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-material-selection', bidId] }),
      ])
    },
    onError: () => message.error('确认解析结果失败，请检查 JSON 格式'),
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
    onError: () => message.error('生成目录大纲失败'),
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
    onError: () => message.error('保存目录失败'),
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
    onError: () => message.error('保存素材选择失败'),
  })
  const generateBidMutation = useMutation({
    mutationFn: (partCode?: string) => generateBid(bidId, partCode ? { scope: 'part', part_code: partCode } : { scope: 'full' }),
    onSuccess: async () => {
      message.success('逐章生成任务已启动')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['bid-generation-jobs', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] }),
      ])
    },
    onError: () => message.error('启动逐章生成失败'),
  })
  const pauseJobMutation = useMutation({
    mutationFn: pauseBidGenerationJob,
    onSuccess: async () => {
      message.success('生成任务已暂停')
      await queryClient.invalidateQueries({ queryKey: ['bid-generation-jobs', bidId] })
    },
    onError: () => message.error('暂停生成任务失败'),
  })
  const resumeJobMutation = useMutation({
    mutationFn: resumeBidGenerationJob,
    onSuccess: async () => {
      message.success('生成任务已继续')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['bid-generation-jobs', bidId] }),
        queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] }),
      ])
    },
    onError: () => message.error('继续生成任务失败'),
  })
  const cancelJobMutation = useMutation({
    mutationFn: cancelBidGenerationJob,
    onSuccess: async () => {
      message.success('生成任务已取消')
      await queryClient.invalidateQueries({ queryKey: ['bid-generation-jobs', bidId] })
    },
    onError: () => message.error('取消生成任务失败'),
  })
  const exportMutation = useMutation({
    mutationFn: (payload: { export_type: 'docx' | 'pdf' | 'zip'; part_code: string }) => createBidExport(bidId, payload),
    onSuccess: async () => {
      message.success('导出任务已创建')
      await queryClient.invalidateQueries({ queryKey: ['bid-exports', bidId] })
    },
    onError: () => message.error('创建导出任务失败'),
  })
  const downloadMutation = useMutation({
    mutationFn: fetchBidExport,
    onSuccess: (detail) => {
      if (!detail.download?.url) {
        message.info('导出文件仍在生成')
        return
      }
      window.open(detail.download.url, '_blank', 'noopener,noreferrer')
    },
    onError: () => message.error('获取下载链接失败'),
  })
  const exportableParts = (parts.data ?? []).filter((part) => ['combined_body', 'tech', 'business'].includes(part.code))
  const primaryPartCode = exportableParts[0]?.code
  const parseRows = structuredResultRows(parseResult.data?.structured_result)
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
      title="标书生成 7 步向导"
      subtitle={bid.data?.title ?? '分离标书支持技术标和商务标独立生成'}
      tags={['page-generate', '/bids/:bidId/wizard?step=1..7']}
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
                  accept=".pdf,.doc,.docx,.txt"
                  maxCount={1}
                  beforeUpload={(file) => {
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
                  上传并绑定
                </Button>
                <Tag color={parseResult.data?.file_asset_id ? 'green' : 'default'}>
                  {parseResult.data?.file_asset_id ? '已绑定' : '未绑定'}
                </Tag>
              </Space>
              <Typography.Text type="secondary">
                {tenderFile ? `${tenderFile.name} · ${Math.ceil(tenderFile.size / 1024)} KB` : '支持 PDF、Word 或文本文件'}
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
                  触发解析
                </Button>
                <Button
                  icon={<CheckOutlined />}
                  loading={confirmParseMutation.isPending}
                  disabled={!parseResult.data}
                  onClick={() => confirmParseMutation.mutate()}
                >
                  确认解析结果
                </Button>
                <Tag color={parseResult.data?.status === 'confirmed' ? 'green' : parseResult.data?.status === 'ready' ? 'blue' : 'default'}>
                  {parseResult.data?.status ?? 'queued'}
                </Tag>
              </Space>
              <Table
                size="small"
                pagination={false}
                rowKey="name"
                dataSource={parseRows}
                columns={[
                  { title: '字段', dataIndex: 'name', width: 180 },
                  { title: '解析结果', dataIndex: 'value' },
                ]}
              />
              <Input.TextArea
                rows={10}
                value={parseDraftText}
                onChange={(event) => setParseDraftText(event.target.value)}
              />
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
                      <input
                        type="checkbox"
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
                                <Tag color={chapterStatusColor(chapter.status)}>{chapter.status}</Tag>
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
                  { title: '任务', dataIndex: 'id', render: (value: string) => value.slice(0, 8) },
                  { title: '范围', dataIndex: 'scope' },
                  { title: '状态', dataIndex: 'status', render: generationJobStatusTag },
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
                  打包全套 ZIP
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
                  { title: '状态', dataIndex: 'status', render: exportStatusTag },
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
    </PageFrame>
  )
}

function bidTypeLabel(value: BidDocumentDTO['bid_type']) {
  return { combined: '综合标书', separated: '分离标书', custom: '自定义组合' }[value] ?? value
}

function bidStatusLabel(value: string) {
  return <Tag color={value === 'editing' ? 'blue' : 'default'}>{value}</Tag>
}

function partCodeLabel(value: string) {
  return { combined_body: '综合标书', tech: '技术标', business: '商务标', all: '全套 ZIP' }[value] ?? value
}

function exportTypeLabel(row: BidExportDTO, partCode: string) {
  if (row.export_type === 'zip') {
    return <Tag color="blue">全套 ZIP</Tag>
  }
  return (
    <Space size={4}>
      <Tag>{partCodeLabel(partCode)}</Tag>
      <Tag color={row.export_type === 'pdf' ? 'red' : 'green'}>{row.export_type.toUpperCase()}</Tag>
    </Space>
  )
}

function exportStatusTag(value: BidExportDTO['status']) {
  const color = value === 'done' ? 'green' : value === 'failed' ? 'red' : 'blue'
  return <Tag color={color}>{value}</Tag>
}

function generationJobStatusTag(value: BidGenerationJobDTO['status']) {
  const color = value === 'done' ? 'green' : value === 'failed' || value === 'cancelled' ? 'red' : value === 'paused' ? 'orange' : 'blue'
  return <Tag color={color}>{value}</Tag>
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

function formatStructuredValue(value: unknown) {
  if (Array.isArray(value)) {
    return value.map((item) => String(item)).join('、')
  }
  if (value && typeof value === 'object') {
    return JSON.stringify(value)
  }
  return value ? String(value) : ''
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
  const [regenerateTaskId, setRegenerateTaskId] = useState('')
  const [generationSnapshot, setGenerationSnapshot] = useState<BidGenerationSnapshotDTO | null>(null)
  const [generationStreamStatus, setGenerationStreamStatus] = useState<'connecting' | 'open' | 'error'>('connecting')
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
  const editor = useEditor({
    extensions: [StarterKit],
    content: '<p>请选择章节</p>',
  })
  useEffect(() => {
    if (!editor || !currentChapter) return
    editor.commands.setContent(contentForEditor(currentChapter))
  }, [editor, currentChapter?.id, currentChapter?.updated_at])
  useEffect(() => {
    if (!regenerateTaskId || !regenerateTaskStatus) return
    if (regenerateTaskStatus === 'done') {
      message.success('AI 重新生成完成')
      setRegenerateTaskId('')
      void queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] })
      void queryClient.invalidateQueries({ queryKey: ['chapter-versions', currentChapter?.id] })
    }
    if (regenerateTaskStatus === 'failed' || regenerateTaskStatus === 'cancelled') {
      message.error('AI 重新生成失败')
      setRegenerateTaskId('')
      void queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] })
    }
  }, [regenerateTaskId, regenerateTaskStatus, message, queryClient, bidId, currentChapter?.id])
  useEffect(() => {
    if (!bidId) return
    setGenerationStreamStatus('connecting')
    return openSse<BidGenerationSnapshotDTO>(`/bids/${bidId}/generation/stream`, {
      onOpen: () => setGenerationStreamStatus('open'),
      onMessage: (snapshot, event) => {
        if (event !== 'generation') return
        setGenerationSnapshot(snapshot)
        void queryClient.invalidateQueries({ queryKey: ['bid-chapters', bidId] })
        if (currentChapter?.id) {
          void queryClient.invalidateQueries({ queryKey: ['chapter-versions', currentChapter.id] })
        }
      },
      onError: () => setGenerationStreamStatus('error'),
    })
  }, [bidId, queryClient, currentChapter?.id])

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
    onError: () => message.error('保存章节失败'),
  })
  const acceptMutation = useMutation({
    mutationFn: () => acceptChapter(currentChapter?.id ?? ''),
    onSuccess: async () => {
      message.success('章节已采纳')
      await invalidateChapterState()
    },
    onError: () => message.error('采纳章节失败'),
  })
  const regenerateMutation = useMutation({
    mutationFn: () => regenerateChapter(currentChapter?.id ?? ''),
    onSuccess: async (result) => {
      setRegenerateTaskId(result.task.id)
      message.success(`AI 重新生成任务已创建：${result.task.external_task_id ?? result.task.id}`)
      await invalidateChapterState()
    },
    onError: () => message.error('AI 重新生成失败'),
  })
  const isRegenerating = Boolean(
    regenerateMutation.isPending || isRegenerateTaskActive(regenerateTaskStatus, regenerateTaskId) || currentChapter?.status === 'generating',
  )

  if (bid.isLoading || parts.isLoading || chapters.isLoading) return <LoadingBlock />
  if (bid.isError || parts.isError || chapters.isError) return <ErrorBlock />

  return (
    <PageFrame
      module="标书生成"
      title="标书编辑器"
      subtitle={bid.data?.title ?? bidId}
      tags={['/bids/:bidId/editor', 'Tiptap']}
      actions={[
        <Button key="save" type="primary" icon={<SaveOutlined />} loading={saveMutation.isPending} disabled={!currentChapter} onClick={() => saveMutation.mutate()}>
          保存
        </Button>,
        <Button key="accept" icon={<CheckOutlined />} loading={acceptMutation.isPending} disabled={!currentChapter} onClick={() => acceptMutation.mutate()}>
          采纳
        </Button>,
        <Button key="regen" icon={<SyncOutlined />} loading={isRegenerating} disabled={!currentChapter || isRegenerating} onClick={() => regenerateMutation.mutate()}>
          重新生成
        </Button>,
        <Button key="export" icon={<DownloadOutlined />}>
          <Link to={`/bids/${bidId}/wizard?step=7`}>导出</Link>
        </Button>,
      ]}
    >
      <Row gutter={16}>
        <Col xs={24} xl={5}>
          <Card title="章节大纲">
            {visibleChapters.length ? (
              <Timeline
                items={visibleChapters.map((chapter) => ({
                  color: chapter.id === currentChapter?.id ? 'blue' : chapter.status === 'accepted' ? 'green' : 'gray',
                  children: (
                    <Button
                      type="link"
                      onClick={() => setSearchParams({ ...(partParam ? { part: partParam } : {}), chapter: chapter.id })}
                    >
                      {chapter.title}
                    </Button>
                  ),
                }))}
              />
            ) : (
              <EmptyBlock />
            )}
          </Card>
        </Col>
        <Col xs={24} xl={13}>
          <Card
            title={currentChapter?.title ?? '内容编辑'}
            extra={currentChapter ? <Tag color={chapterStatusColor(currentChapter.status)}>{currentChapter.status}</Tag> : null}
          >
            <EditorContent editor={editor} className="editor-surface" />
          </Card>
          <Card title="版本记录" className="mt-16">
            <Table
              size="small"
              rowKey="id"
              pagination={false}
              loading={versions.isLoading}
              locale={{ emptyText: <EmptyBlock /> }}
              dataSource={versions.data ?? []}
              columns={[
                { title: '版本', dataIndex: 'version_no', render: (value) => `v${value}` },
                { title: '原因', dataIndex: 'change_reason' },
                { title: '状态', dataIndex: 'status', render: (value) => <Tag>{value}</Tag> },
                { title: '时间', dataIndex: 'created_at', render: (value) => new Date(value).toLocaleString() },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} xl={6}>
          <Card title="AI 助手">
            <Space direction="vertical">
              <Tag color={generationStreamColor(generationStreamStatus)}>SSE: {generationStreamStatus}</Tag>
              <Progress size="small" percent={generationPercent} />
              <Typography.Text type="secondary">
                章节 {completedChapterCount}/{generationSummary?.total_chapters ?? visibleChapters.length} · task done {generationSummary?.done_tasks ?? 0}
              </Typography.Text>
              <Tag color="purple">source_refs: {currentChapter?.source_refs.length ?? 0}</Tag>
              <Tag color="orange">needs_human_input: {currentChapter?.needs_human_input.length ?? 0}</Tag>
              {(currentChapter?.needs_human_input ?? []).map((item) => (
                <Typography.Text key={item} type="warning">
                  {item}
                </Typography.Text>
              ))}
              {regenerateTask.data ? <Tag color="blue">task: {regenerateTask.data.status}</Tag> : null}
              {latestChapterTask ? <Tag color="cyan">latest: {latestChapterTask.status}</Tag> : null}
              <Button icon={<SyncOutlined />} loading={isRegenerating} disabled={!currentChapter || isRegenerating} onClick={() => regenerateMutation.mutate()}>
                查找素材并重新生成
              </Button>
              <Button icon={<DiffOutlined />} disabled={!currentChapter}>
                查看版本差异
              </Button>
            </Space>
          </Card>
        </Col>
      </Row>
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

function chapterStatusColor(status: BidChapterDTO['status']) {
  if (status === 'accepted') return 'green'
  if (status === 'edited') return 'orange'
  if (status === 'generated') return 'blue'
  if (status === 'generating') return 'purple'
  return 'default'
}

function isRegenerateTaskActive(status: string | undefined, taskId: string) {
  if (!taskId) return false
  return !status || status === 'queued' || status === 'running'
}

function generationStreamColor(status: 'connecting' | 'open' | 'error') {
  if (status === 'open') return 'green'
  if (status === 'error') return 'red'
  return 'blue'
}
