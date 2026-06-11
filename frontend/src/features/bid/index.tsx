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
  App as AntApp,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { z } from 'zod'
import {
  acceptChapter,
  createBid,
  createBidExport,
  fetchAITask,
  fetchBidChapters,
  fetchBid,
  fetchBidExport,
  fetchBidExports,
  fetchBidParts,
  fetchBidTemplates,
  fetchBids,
  fetchChapterVersions,
  regenerateChapter,
  submitBidForApproval,
  updateChapterContent,
  useBidTemplate,
  type BidChapterDTO,
  type BidDocumentDTO,
  type BidExportDTO,
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
  const exportsQuery = useQuery({
    queryKey: ['bid-exports', bidId],
    queryFn: () => fetchBidExports(bidId),
    enabled: Boolean(bidId),
    refetchInterval: (query) => {
      const items = query.state.data ?? []
      return items.some((item) => item.status === 'queued' || item.status === 'running') ? 2000 : false
    },
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
          {current === 0 ? <Button icon={<UploadOutlined />}>上传招标文件</Button> : null}
          {current === 1 ? (
            <Table
              pagination={false}
              rowKey="name"
              dataSource={[
                { name: '项目名称', value: '智慧交通综合治理平台建设' },
                { name: '废标条款', value: '签章、有效期、报价一致性' },
                { name: '评分重点', value: '实施方案、数据安全、运维能力' },
              ]}
              columns={[
                { title: '字段', dataIndex: 'name' },
                { title: '解析结果', dataIndex: 'value' },
              ]}
            />
          ) : null}
          {current === 4 ? (
            <Tabs
              items={[
                { key: 'tech', label: '技术标', children: <Progress percent={66} /> },
                { key: 'business', label: '商务标', children: <Progress percent={42} /> },
              ]}
            />
          ) : null}
          {current === 5 ? (
            <Button type="primary">
              <Link to="/bids/bid-demo/editor?part=tech">进入编辑器</Link>
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
