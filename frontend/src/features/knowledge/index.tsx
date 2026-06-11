import {
  CloudUploadOutlined,
  DownloadOutlined,
  FileSearchOutlined,
  PlayCircleOutlined,
  TagsOutlined,
} from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { App as AntApp, Button, Card, Col, Input, List, Row, Space, Statistic, Table, Tag, Tree, Upload } from 'antd'
import type { UploadProps } from 'antd'
import { useParams } from 'react-router-dom'
import {
  confirmFileUpload,
  createPresignedUpload,
  fetchFileURL,
  fetchKnowledgeCategories,
  fetchKnowledgeDocuments,
  fetchKnowledgeStats,
  fetchKnowledgeTags,
  processKnowledgeDocument,
  uploadToPresignedUrl,
  type KnowledgeDocumentDTO,
  type KnowledgeTagDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'

export function KnowledgeHomePage() {
  const stats = useQuery({
    queryKey: ['knowledge-stats'],
    queryFn: fetchKnowledgeStats,
  })
  const categories = Object.entries(stats.data?.category_counts ?? {})

  return (
    <PageFrame
      module="企业管理"
      title="知识库"
      subtitle="分类卡、语义搜索、引用统计和中标案例回流"
      tags={['page-knowledge', '/knowledge']}
    >
      <Space direction="vertical" size={16} className="full-width">
        <Input.Search placeholder="语义搜索企业资质、案例、技术方案" enterButton="搜索" />
        <Row gutter={[16, 16]}>
          {[
            ['总文档', stats.data?.document_count ?? 0],
            ['待处理', (stats.data?.ready_count ?? 0) + (stats.data?.queued_count ?? 0)],
            ['已处理', stats.data?.processed_count ?? 0],
            ['失败', stats.data?.failed_count ?? 0],
          ].map(([name, value]) => (
            <Col xs={24} md={12} xl={6} key={name}>
              <Card title={name}>
                <Statistic value={value} suffix="个" />
              </Card>
            </Col>
          ))}
        </Row>
        <Card title="分类统计">
          <List
            loading={stats.isLoading}
            dataSource={categories.length > 0 ? categories : [['未分类', 0]]}
            renderItem={(item) => (
              <List.Item actions={[<Tag key="count">{item[1]} 个文档</Tag>]}>
                <List.Item.Meta title={item[0]} description="当前租户知识库" />
              </List.Item>
            )}
          />
        </Card>
      </Space>
    </PageFrame>
  )
}

export function KnowledgeDocsPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const documents = useQuery({
    queryKey: ['knowledge-documents'],
    queryFn: fetchKnowledgeDocuments,
  })
  const categories = useQuery({
    queryKey: ['knowledge-categories'],
    queryFn: fetchKnowledgeCategories,
  })

  const uploadProps: UploadProps = {
    multiple: false,
    showUploadList: false,
    customRequest: async ({ file, onError, onSuccess }) => {
      const uploadFile = file as File
      try {
        const presigned = await createPresignedUpload({
          filename: uploadFile.name,
          content_type: uploadFile.type || 'application/octet-stream',
          size_bytes: uploadFile.size,
        })
        await uploadToPresignedUrl(presigned, uploadFile)
        await confirmFileUpload(presigned.file.id)
        await queryClient.invalidateQueries({ queryKey: ['knowledge-documents'] })
        await queryClient.invalidateQueries({ queryKey: ['knowledge-stats'] })
        message.success('上传完成')
        onSuccess?.(presigned.file)
      } catch (error) {
        message.error('上传失败')
        onError?.(error as Error)
      }
    },
  }

  const openFile = async (fileId: string, mode: 'download' | 'preview') => {
    const result = await fetchFileURL(fileId, mode)
    window.open(result.url, '_blank', 'noopener,noreferrer')
  }

  const startProcess = async (documentId: string) => {
    const task = await processKnowledgeDocument(documentId)
    await queryClient.invalidateQueries({ queryKey: ['knowledge-documents'] })
    await queryClient.invalidateQueries({ queryKey: ['knowledge-stats'] })
    message.success(`处理任务已创建：${task.external_task_id ?? task.id}`)
  }

  return (
    <PageFrame
      module="知识库"
      title="文档库"
      subtitle="分类树、上传、预览、下载和引用追踪"
      tags={['page-knowledge-docs', '/knowledge/docs']}
      actions={[
        <Button key="upload" type="primary" icon={<CloudUploadOutlined />}>
          上传文档
        </Button>,
      ]}
    >
      <Row gutter={16}>
        <Col xs={24} xl={6}>
          <Card title="文档分类">
            <Tree
              defaultExpandAll
              treeData={[
                {
                  title: `全部文档 ${documents.data?.length ?? 0}`,
                  key: 'all',
                  children: (categories.data ?? []).map((item) => ({
                    title: item.name,
                    key: item.id,
                  })),
                },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} xl={18}>
          <Space direction="vertical" className="full-width">
            <Upload.Dragger {...uploadProps}>
              <CloudUploadOutlined />
              <p>拖拽上传 PDF、Word、Excel、PPT、图片或压缩包</p>
            </Upload.Dragger>
            {documents.isLoading ? <LoadingBlock /> : null}
            {documents.isError ? <ErrorBlock /> : null}
            {!documents.isLoading && !documents.isError && documents.data?.length === 0 ? (
              <EmptyBlock />
            ) : null}
            <Table<KnowledgeDocumentDTO>
              rowKey="id"
              loading={documents.isLoading}
              dataSource={documents.data ?? []}
              columns={[
                { title: '文档名', dataIndex: 'title' },
                { title: '分类', render: (_, row) => row.category?.name ?? '未分类' },
                { title: '大小', render: (_, row) => formatBytes(row.file.size_bytes) },
                { title: '类型', dataIndex: 'doc_type' },
                {
                  title: '状态',
                  dataIndex: 'parse_status',
                  render: (status: KnowledgeDocumentDTO['parse_status']) => (
                    <Tag color={statusColor(status)}>{statusLabel(status)}</Tag>
                  ),
                },
                {
                  title: '标签',
                  render: (_, row) => (
                    <Space wrap>
                      {row.tags.map((tag) => (
                        <Tag key={tag.id} color={tag.color}>
                          {tag.name}
                        </Tag>
                      ))}
                    </Space>
                  ),
                },
                {
                  title: '操作',
                  render: (_, row) => (
                    <Space>
                      <a href={`/files/${row.file.id}/preview`}>预览</a>
                      <Button
                        size="small"
                        icon={<PlayCircleOutlined />}
                        disabled={row.parse_status === 'queued' || row.parse_status === 'processing'}
                        onClick={() => void startProcess(row.id)}
                      >
                        处理
                      </Button>
                      <Button
                        size="small"
                        icon={<DownloadOutlined />}
                        disabled={row.file.status !== 'ready'}
                        onClick={() => void openFile(row.file.id, 'download')}
                      >
                        下载
                      </Button>
                    </Space>
                  ),
                },
              ]}
            />
          </Space>
        </Col>
      </Row>
    </PageFrame>
  )
}

export function KnowledgeTemplatesPage() {
  return (
    <PageFrame
      module="知识库"
      title="文档模板"
      subtitle="企业内部方案、报告、合同、制度模板"
      tags={['page-knowledge-templates', '/knowledge/templates']}
    >
      <Table
        rowKey="name"
        dataSource={[
          { name: '项目实施方案模板.docx', category: '方案模板', version: 'v3.0', used: 89 },
          { name: '售后服务承诺模板.docx', category: '报告模板', version: 'v2.1', used: 42 },
          { name: '数据安全响应模板.docx', category: '制度模板', version: 'v1.4', used: 37 },
        ]}
        columns={[
          { title: '模板名称', dataIndex: 'name' },
          { title: '分类', dataIndex: 'category' },
          { title: '版本', dataIndex: 'version' },
          { title: '使用次数', dataIndex: 'used' },
          { title: '操作', render: () => <Space><a>预览</a><a>编辑</a><a>下载</a></Space> },
        ]}
      />
    </PageFrame>
  )
}

export function KnowledgeTagsPage() {
  const tags = useQuery({
    queryKey: ['knowledge-tags'],
    queryFn: fetchKnowledgeTags,
  })
  const documents = useQuery({
    queryKey: ['knowledge-documents'],
    queryFn: fetchKnowledgeDocuments,
  })
  const documentsByTag = (tag: KnowledgeTagDTO) =>
    (documents.data ?? []).filter((document) => document.tags.some((item) => item.id === tag.id))

  return (
    <PageFrame
      module="知识库"
      title="标签管理"
      subtitle="标签颜色、关联文档和删除确认"
      tags={['page-knowledge-tags', '/knowledge/tags']}
      actions={[
        <Button key="new" type="primary" icon={<TagsOutlined />}>
          新建标签
        </Button>,
      ]}
    >
      <Row gutter={16}>
        <Col xs={24} md={8}>
          <Card title="标签列表">
            <List
              loading={tags.isLoading}
              dataSource={tags.data ?? []}
              renderItem={(item) => (
                <List.Item>
                  <Tag color={item.color}>{item.name}</Tag>
                  <span>{documentsByTag(item).length} 个文档</span>
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col xs={24} md={16}>
          <Card title="关联文档">
            <Table
              rowKey="id"
              pagination={false}
              loading={documents.isLoading}
              dataSource={documents.data ?? []}
              columns={[
                { title: '关联文档', dataIndex: 'title' },
                {
                  title: '标签',
                  render: (_, row: KnowledgeDocumentDTO) => row.tags.map((tag) => <Tag key={tag.id} color={tag.color}>{tag.name}</Tag>),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </PageFrame>
  )
}

export function FilePreviewPage() {
  const { fileId } = useParams()
  const preview = useQuery({
    queryKey: ['file-preview', fileId],
    queryFn: () => fetchFileURL(fileId!, 'preview'),
    enabled: Boolean(fileId),
  })
  const openDownload = async () => {
    if (!fileId) {
      return
    }
    const result = await fetchFileURL(fileId, 'download')
    window.open(result.url, '_blank', 'noopener,noreferrer')
  }

  return (
    <PageFrame
      module="知识库"
      title={preview.data?.file.filename ?? '文件预览'}
      subtitle={fileId}
      tags={['/files/:fileId/preview']}
      actions={[
        <Button key="download" icon={<DownloadOutlined />} onClick={() => void openDownload()}>
          下载
        </Button>,
      ]}
    >
      {preview.isLoading ? <LoadingBlock /> : null}
      {preview.isError ? <ErrorBlock /> : null}
      {preview.data ? (
        <Card>
          <FileSearchOutlined className="preview-icon" />
          <iframe title={preview.data.file.filename} src={preview.data.url} className="file-preview-frame" />
        </Card>
      ) : null}
    </PageFrame>
  )
}

function statusLabel(status: KnowledgeDocumentDTO['parse_status']) {
  const labels: Record<KnowledgeDocumentDTO['parse_status'], string> = {
    ready: '待处理',
    queued: '排队中',
    processing: '处理中',
    processed: '已处理',
    failed: '失败',
  }
  return labels[status]
}

function statusColor(status: KnowledgeDocumentDTO['parse_status']) {
  const colors: Record<KnowledgeDocumentDTO['parse_status'], string> = {
    ready: 'blue',
    queued: 'orange',
    processing: 'cyan',
    processed: 'green',
    failed: 'red',
  }
  return colors[status]
}

function formatBytes(value: number) {
  if (value < 1024) {
    return `${value} B`
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`
  }
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}
