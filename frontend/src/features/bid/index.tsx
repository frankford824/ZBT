import {
  CheckOutlined,
  DiffOutlined,
  DownloadOutlined,
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
import { useEffect } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { z } from 'zod'
import {
  acceptChapter,
  createBid,
  createBidExport,
  fetchBidChapters,
  fetchBid,
  fetchBidExport,
  fetchBidExports,
  fetchBidParts,
  fetchBids,
  fetchChapterVersions,
  regenerateChapter,
  updateChapterContent,
  type BidChapterDTO,
  type BidDocumentDTO,
  type BidExportDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'

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
  const bids = useQuery({
    queryKey: ['bids'],
    queryFn: fetchBids,
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
  return (
    <PageFrame
      module="标书生成"
      title="标书模板"
      subtitle="行业分类、预览和使用"
      tags={['page-generate-templates', '/bids/templates']}
    >
      <Row gutter={[16, 16]}>
        {['IT信息化综合标', '政府采购商务标', '工程建设技术标', '我的实施方案模板'].map((name, index) => (
          <Col xs={24} md={12} xl={6} key={name}>
            <Card title={name} actions={[<Link to="/bids/new">使用</Link>, <a>预览</a>]}>
              <Space direction="vertical">
                <Tag color="blue">{index === 3 ? '我的模板' : '行业模板'}</Tag>
                <Typography.Text type="secondary">使用 {120 - index * 17} 次</Typography.Text>
                <Typography.Text>评分 {4.8 - index * 0.2}</Typography.Text>
              </Space>
            </Card>
          </Col>
        ))}
      </Row>
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
    mutationFn: (payload: { export_type: 'docx' | 'zip'; part_code: string }) => createBidExport(bidId, payload),
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
                  <Button
                    key={part.id}
                    icon={<DownloadOutlined />}
                    loading={exportMutation.isPending}
                    onClick={() => exportMutation.mutate({ export_type: 'docx', part_code: part.code })}
                  >
                    导出{part.title}.docx
                  </Button>
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
                    render: (value: string, row: BidExportDTO) => partCodeLabel(row.export_type === 'zip' ? 'all' : value),
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

function exportStatusTag(value: BidExportDTO['status']) {
  const color = value === 'done' ? 'green' : value === 'failed' ? 'red' : 'blue'
  return <Tag color={color}>{value}</Tag>
}

export function BidEditorPage() {
  const { bidId = '' } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
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
  const editor = useEditor({
    extensions: [StarterKit],
    content: '<p>请选择章节</p>',
  })
  useEffect(() => {
    if (!editor || !currentChapter) return
    editor.commands.setContent(contentForEditor(currentChapter))
  }, [editor, currentChapter?.id, currentChapter?.updated_at])

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
    onSuccess: async () => {
      message.success('AI 重新生成完成')
      await invalidateChapterState()
    },
    onError: () => message.error('AI 重新生成失败'),
  })

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
        <Button key="regen" icon={<SyncOutlined />} loading={regenerateMutation.isPending} disabled={!currentChapter} onClick={() => regenerateMutation.mutate()}>
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
              <Tag color="purple">source_refs: {currentChapter?.source_refs.length ?? 0}</Tag>
              <Tag color="orange">needs_human_input: {currentChapter?.needs_human_input.length ?? 0}</Tag>
              {(currentChapter?.needs_human_input ?? []).map((item) => (
                <Typography.Text key={item} type="warning">
                  {item}
                </Typography.Text>
              ))}
              <Button icon={<SyncOutlined />} loading={regenerateMutation.isPending} disabled={!currentChapter} onClick={() => regenerateMutation.mutate()}>
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
  return 'default'
}
