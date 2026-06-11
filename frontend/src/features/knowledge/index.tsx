import {
  CloudUploadOutlined,
  DownloadOutlined,
  FileSearchOutlined,
  TagsOutlined,
} from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { App as AntApp, Button, Card, Col, Input, List, Row, Space, Table, Tag, Tree, Upload } from 'antd'
import type { UploadProps } from 'antd'
import { useParams } from 'react-router-dom'
import {
  confirmFileUpload,
  createPresignedUpload,
  fetchFileURL,
  fetchKnowledgeDocuments,
  uploadToPresignedUrl,
  type FileAssetDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'

const docs = [
  { id: 'file-demo', name: 'CMMI 资质证书.pdf', category: '资质证书', size: '2.5M', refs: 12 },
  { id: 'file-2', name: '智慧交通实施案例.docx', category: '项目案例', size: '1.2M', refs: 18 },
  { id: 'file-3', name: '数据安全技术方案.docx', category: '技术方案', size: '860K', refs: 9 },
]

export function KnowledgeHomePage() {
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
          {['资质证书', '项目案例', '技术方案', '合同模板', '人员信息', '公共模板库'].map((name, index) => (
            <Col xs={24} md={8} xl={4} key={name}>
              <Card title={name}>
                <p>{36 + index * 11} 个文档</p>
                <Tag color="blue">引用 {12 + index * 5}</Tag>
              </Card>
            </Col>
          ))}
        </Row>
        <Card title="被引用排行">
          <List
            dataSource={docs}
            renderItem={(item) => (
              <List.Item actions={[<Tag key="refs">{item.refs} 次引用</Tag>]}>
                <List.Item.Meta title={item.name} description={item.category} />
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
                  title: '全部文档 156',
                  key: 'all',
                  children: [
                    { title: '资质证书 23', key: 'cert' },
                    { title: '项目案例 45', key: 'case' },
                    { title: '技术方案 67', key: 'solution' },
                    { title: '合同模板 21', key: 'contract' },
                  ],
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
            <Table<FileAssetDTO>
              rowKey="id"
              loading={documents.isLoading}
              dataSource={documents.data ?? []}
              columns={[
                { title: '文档名', dataIndex: 'filename' },
                { title: '分类', dataIndex: 'biz_type', render: categoryLabel },
                { title: '大小', dataIndex: 'size_bytes', render: formatBytes },
                {
                  title: '状态',
                  dataIndex: 'status',
                  render: (status: FileAssetDTO['status']) => (
                    <Tag color={status === 'ready' ? 'green' : 'orange'}>{statusLabel(status)}</Tag>
                  ),
                },
                {
                  title: '操作',
                  render: (_, row) => (
                    <Space>
                      <a href={`/files/${row.id}/preview`}>预览</a>
                      <Button
                        size="small"
                        icon={<DownloadOutlined />}
                        disabled={row.status !== 'ready'}
                        onClick={() => void openFile(row.id, 'download')}
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
              dataSource={[
                ['资质证书', 23, 'blue'],
                ['技术方案', 67, 'green'],
                ['项目案例', 45, 'orange'],
              ]}
              renderItem={(item) => (
                <List.Item>
                  <Tag color={item[2] as string}>{item[0]}</Tag>
                  <span>{item[1]} 个文档</span>
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col xs={24} md={16}>
          <Card title="标签详情：资质证书">
            <Table
              rowKey="name"
              pagination={false}
              dataSource={docs.filter((doc) => doc.category === '资质证书')}
              columns={[
                { title: '关联文档', dataIndex: 'name' },
                { title: '引用次数', dataIndex: 'refs' },
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

function categoryLabel(bizType: string) {
  if (bizType === 'knowledge') {
    return '知识库'
  }
  return bizType
}

function statusLabel(status: FileAssetDTO['status']) {
  const labels: Record<FileAssetDTO['status'], string> = {
    pending: '待确认',
    ready: '可用',
    failed: '失败',
    deleted: '已删除',
  }
  return labels[status]
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
