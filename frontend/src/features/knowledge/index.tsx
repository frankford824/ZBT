import {
  CloudUploadOutlined,
  CopyOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  FileSearchOutlined,
  LinkOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  TagsOutlined,
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App as AntApp, Button, Card, Col, Drawer, Form, Input, List, Modal, Popconfirm, Row, Select, Space, Statistic, Table, Tag, Typography, Upload } from 'antd'
import type { UploadProps } from 'antd'
import { useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import {
  confirmFileUpload,
  createKnowledgeCategory,
  createKnowledgeTemplate,
  createKnowledgeTag,
  createPresignedUpload,
  deleteKnowledgeCategory,
  deleteKnowledgeTag,
  fetchFileURL,
  getApiErrorMessage,
  fetchKnowledgeCategories,
  fetchKnowledgeDocumentPreview,
  fetchKnowledgeDocumentReferences,
  fetchKnowledgeDocuments,
  fetchKnowledgeStats,
  fetchKnowledgeTags,
  fetchKnowledgeTemplates,
  processKnowledgeDocument,
  searchKnowledge,
  updateKnowledgeCategory,
  updateKnowledgeDocument,
  updateKnowledgeTag,
  uploadToPresignedUrl,
  type KnowledgeCategoryDTO,
  type KnowledgeDocumentTemplateDTO,
  type KnowledgeDocumentReferenceDTO,
  type KnowledgeSearchResponseDTO,
  type KnowledgeDocumentDTO,
  type KnowledgeTagDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { fileOpenErrorMessage, openFileUrl, safeFileUrlString } from '../../shared/files/openFileUrl'
import { formatBytes, isUploadFileTooLarge, uploadSizeLimitMessage } from '../../shared/files/uploadLimits'
import { formatDateOnly, formatDateTime } from '../../shared/format/date'
import { useCanAccess } from '../../shared/permissions/permissions'

export function KnowledgeHomePage() {
  const { message } = AntApp.useApp()
  const [searchResult, setSearchResult] = useState<KnowledgeSearchResponseDTO | null>(null)
  const [searching, setSearching] = useState(false)
  const stats = useQuery({
    queryKey: ['knowledge-stats'],
    queryFn: fetchKnowledgeStats,
  })
  const categories = Object.entries(stats.data?.category_counts ?? {})
  const runSearch = async (query: string) => {
    if (!query.trim()) {
      setSearchResult(null)
      return
    }
    setSearching(true)
    try {
      setSearchResult(await searchKnowledge({ query, limit: 8 }))
    } catch (error) {
      message.error(getApiErrorMessage(error, '搜索失败'))
    } finally {
      setSearching(false)
    }
  }

  return (
    <PageFrame
      module="企业管理"
      title="知识库"
      subtitle="资料检索、引用统计和中标案例回流"
      tags={['资料检索']}
      bare
    >
      <Space direction="vertical" size={16} className="full-width">
        <Input.Search
          placeholder="搜索企业资质、案例、技术方案"
          enterButton="搜索"
          loading={searching}
          onSearch={(value) => void runSearch(value)}
        />
        {stats.isError ? (
          <ErrorBlock description="知识库统计加载失败，请刷新重试" />
        ) : (
          <>
            <Row gutter={[16, 16]}>
              {[
                ['总文档', stats.data?.document_count ?? 0],
                ['待整理', (stats.data?.ready_count ?? 0) + (stats.data?.queued_count ?? 0)],
                ['已整理', stats.data?.processed_count ?? 0],
                ['失败', stats.data?.failed_count ?? 0],
              ].map(([name, value]) => (
                <Col xs={24} md={12} xl={6} key={name}>
                  <Card title={name} className="stat-card">
                    <Statistic value={value} suffix="个" />
                  </Card>
                </Col>
              ))}
            </Row>
            <Card title="分类统计">
              <List
                loading={stats.isLoading}
                dataSource={categories}
                locale={{ emptyText: '暂无分类统计' }}
                renderItem={(item) => (
                  <List.Item actions={[<Tag key="count">{item[1]} 个文档</Tag>]}>
                    <List.Item.Meta title={item[0]} description="当前企业资料" />
                  </List.Item>
                )}
              />
            </Card>
          </>
        )}
        {searchResult ? (
          <Card title="搜索结果">
            <List
              dataSource={searchResult.items}
              locale={{ emptyText: '暂无命中' }}
              renderItem={(item) => (
                <List.Item
                  actions={[
                    <Tag key="ref" color="blue">
                      {item.page_start ? `第 ${item.page_start} 页` : '相关片段'}
                    </Tag>,
                  ]}
                >
                  <List.Item.Meta
                    title={item.title}
                    description={`${item.document.title} / ${item.section_path}`}
                  />
                  <div className="knowledge-snippet">{item.content.slice(0, 180)}</div>
                </List.Item>
              )}
            />
          </Card>
        ) : null}
      </Space>
    </PageFrame>
  )
}

export function KnowledgeDocsPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canWrite = useCanAccess('knowledge', 'full')
  const [referenceDocument, setReferenceDocument] = useState<KnowledgeDocumentDTO | null>(null)
  const [categoryModalOpen, setCategoryModalOpen] = useState(false)
  const [editingCategory, setEditingCategory] = useState<KnowledgeCategoryDTO | null>(null)
  const [categoryForm] = Form.useForm<{ name: string; description?: string }>()
  const [editingDocument, setEditingDocument] = useState<KnowledgeDocumentDTO | null>(null)
  const [startingProcessId, setStartingProcessId] = useState<string | null>(null)
  const [documentForm] = Form.useForm<{
    title: string
    doc_type: string
    category_id?: string | null
    tag_ids: string[]
    summary?: string
  }>()
  const documents = useQuery({
    queryKey: ['knowledge-documents'],
    queryFn: fetchKnowledgeDocuments,
    refetchInterval: (query) => {
      const items = query.state.data ?? []
      return items.some((item) => item.parse_status === 'queued' || item.parse_status === 'processing') ? 2000 : false
    },
  })
  const categories = useQuery({
    queryKey: ['knowledge-categories'],
    queryFn: fetchKnowledgeCategories,
  })
  const tags = useQuery({
    queryKey: ['knowledge-tags'],
    queryFn: fetchKnowledgeTags,
  })
  const refreshKnowledgeLists = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['knowledge-categories'] }),
      queryClient.invalidateQueries({ queryKey: ['knowledge-documents'] }),
      queryClient.invalidateQueries({ queryKey: ['knowledge-stats'] }),
    ])
  }
  const createCategory = useMutation({
    mutationFn: createKnowledgeCategory,
    onSuccess: async () => {
      await refreshKnowledgeLists()
      message.success('分类已创建')
      closeCategoryModal()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '分类创建失败')),
  })
  const updateCategory = useMutation({
    mutationFn: ({ id, values }: { id: string; values: { name?: string; description?: string } }) =>
      updateKnowledgeCategory(id, values),
    onSuccess: async () => {
      await refreshKnowledgeLists()
      message.success('分类已更新')
      closeCategoryModal()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '分类更新失败')),
  })
  const deleteCategory = useMutation({
    mutationFn: deleteKnowledgeCategory,
    onSuccess: async () => {
      await refreshKnowledgeLists()
      message.success('分类已删除')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '分类删除失败')),
  })
  const updateDocument = useMutation({
    mutationFn: ({
      id,
      values,
    }: {
      id: string
      values: {
        title?: string
        doc_type?: string
        category_id?: string | null
        tag_ids?: string[]
        summary?: string
      }
    }) => updateKnowledgeDocument(id, values),
    onSuccess: async () => {
      await refreshKnowledgeLists()
      message.success('文档信息已更新')
      closeDocumentModal()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '文档信息更新失败')),
  })

  const uploadProps: UploadProps = {
    multiple: false,
    showUploadList: false,
    customRequest: async ({ file, onError, onSuccess }) => {
      const uploadFile = file as File
      if (isUploadFileTooLarge(uploadFile)) {
        const error = new Error(uploadSizeLimitMessage())
        message.error(error.message)
        onError?.(error)
        return
      }
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
        message.error(getApiErrorMessage(error, '上传失败'))
        onError?.(error as Error)
      }
    },
  }

  const openFile = async (fileId: string, mode: 'download' | 'preview') => {
    try {
      const result = await fetchFileURL(fileId, mode)
      const openError = fileOpenErrorMessage(openFileUrl(result.url))
      if (openError) {
        message.error(openError)
      }
    } catch (error) {
      message.error(getApiErrorMessage(error, mode === 'download' ? '获取下载链接失败' : '获取预览链接失败'))
    }
  }

  const startProcess = async (documentId: string) => {
    setStartingProcessId(documentId)
    try {
      await processKnowledgeDocument(documentId)
      await queryClient.invalidateQueries({ queryKey: ['knowledge-documents'] })
      await queryClient.invalidateQueries({ queryKey: ['knowledge-stats'] })
      message.success('已开始整理文档')
    } catch (error) {
      message.error(getApiErrorMessage(error, '文档整理启动失败'))
    } finally {
      setStartingProcessId((current) => (current === documentId ? null : current))
    }
  }
  const openCreateCategory = () => {
    setEditingCategory(null)
    categoryForm.resetFields()
    setCategoryModalOpen(true)
  }
  const openEditCategory = (category: KnowledgeCategoryDTO) => {
    setEditingCategory(category)
    categoryForm.setFieldsValue({
      name: category.name,
      description: category.description,
    })
    setCategoryModalOpen(true)
  }
  const closeCategoryModal = () => {
    setCategoryModalOpen(false)
    setEditingCategory(null)
    categoryForm.resetFields()
  }
  const submitCategory = async () => {
    const values = await categoryForm.validateFields()
    if (editingCategory) {
      await updateCategory.mutateAsync({ id: editingCategory.id, values })
      return
    }
    await createCategory.mutateAsync(values)
  }
  const openEditDocument = (document: KnowledgeDocumentDTO) => {
    setEditingDocument(document)
    documentForm.setFieldsValue({
      title: document.title,
      doc_type: document.doc_type,
      category_id: document.category?.id ?? null,
      tag_ids: document.tags.map((tag) => tag.id),
      summary: document.summary,
    })
  }
  const closeDocumentModal = () => {
    setEditingDocument(null)
    documentForm.resetFields()
  }
  const submitDocument = async () => {
    if (!editingDocument) {
      return
    }
    const values = await documentForm.validateFields()
    await updateDocument.mutateAsync({
      id: editingDocument.id,
      values: {
        title: values.title,
        doc_type: values.doc_type,
        category_id: values.category_id ?? null,
        tag_ids: values.tag_ids ?? [],
        summary: values.summary ?? '',
      },
    })
  }

  return (
    <PageFrame
      module="知识库"
      title="文档库"
      subtitle="分类、上传、预览、下载和引用记录"
      tags={['文档管理']}
      bare
      actions={[
        canWrite ? <Upload key="upload" {...uploadProps}>
          <Button type="primary" icon={<CloudUploadOutlined />}>
            上传文档
          </Button>
        </Upload> : null,
      ]}
    >
      <Row gutter={16}>
        <Col xs={24} xl={6}>
            <Card
              title={`文档分类 ${categories.data?.length ?? 0}`}
              extra={canWrite ? <Button size="small" icon={<PlusOutlined />} onClick={openCreateCategory}>新建</Button> : null}
            >
            <List
              loading={categories.isLoading}
              dataSource={categories.data ?? []}
              locale={{ emptyText: '暂无分类' }}
              renderItem={(item) => (
                <List.Item
                    actions={canWrite ? [
                      <Button key="edit" type="text" size="small" icon={<EditOutlined />} onClick={() => openEditCategory(item)} />,
                      <Popconfirm
                        key="delete"
                      title="删除分类"
                      description="关联文档会变为未分类。"
                      okText="删除"
                      cancelText="取消"
                      onConfirm={() => deleteCategory.mutate(item.id)}
                      >
                        <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                      </Popconfirm>,
                    ] : []}
                >
                  <List.Item.Meta title={item.name} description={item.description || '无说明'} />
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col xs={24} xl={18}>
          <Space direction="vertical" className="full-width">
              {canWrite ? (
                <Upload.Dragger {...uploadProps}>
                  <CloudUploadOutlined />
                  <p>拖拽上传 PDF、Word、Excel、PPT、图片或压缩包</p>
                </Upload.Dragger>
              ) : null}
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
                {
                  title: '大小',
                  align: 'right',
                  render: (_, row) => <span className="data-mono">{formatBytes(row.file.size_bytes)}</span>,
                },
                {
                  title: '类型',
                  dataIndex: 'doc_type',
                  render: (value: string) => docTypeLabels[value] ?? '其他',
                },
                {
                  title: '状态',
                  dataIndex: 'parse_status',
                  render: (_, row: KnowledgeDocumentDTO) => documentStatusCell(row),
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
                  width: 320,
                  render: (_, row) => (
                    <Space wrap size={[8, 6]}>
                      <Button
                        size="small"
                        icon={<FileSearchOutlined />}
                        disabled={row.file.status !== 'ready'}
                        onClick={() => void openFile(row.file.id, 'preview')}
                      >
                        预览
                      </Button>
                      {canWrite ? (
                        <Button
                          size="small"
                          icon={<EditOutlined />}
                          onClick={() => openEditDocument(row)}
                        >
                          编辑
                        </Button>
                      ) : null}
                      <Button
                        size="small"
                        icon={<LinkOutlined />}
                        onClick={() => setReferenceDocument(row)}
                      >
                        引用
                      </Button>
                      {canWrite ? (
                        <Button
                          size="small"
                          icon={<PlayCircleOutlined />}
                          loading={startingProcessId === row.id}
                          disabled={row.parse_status === 'queued' || row.parse_status === 'processing'}
                          onClick={() => void startProcess(row.id)}
                        >
                          整理
                        </Button>
                      ) : null}
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
      <DocumentReferenceDrawer
        document={referenceDocument}
        onClose={() => setReferenceDocument(null)}
      />
      <Modal
        title={editingDocument ? `编辑文档：${editingDocument.title}` : '编辑文档'}
        open={Boolean(editingDocument)}
        onCancel={closeDocumentModal}
        onOk={() => void submitDocument()}
        confirmLoading={updateDocument.isPending}
        destroyOnHidden
      >
        <Form form={documentForm} layout="vertical">
          <Form.Item label="文档标题" name="title" rules={[{ required: true, message: '请输入文档标题' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="文档类型" name="doc_type" rules={[{ required: true, message: '请选择文档类型' }]}>
            <Select
              options={Object.entries(docTypeLabels).map(([value, label]) => ({ value, label }))}
            />
          </Form.Item>
          <Form.Item label="分类" name="category_id">
            <Select
              allowClear
              placeholder="未分类"
              options={(categories.data ?? []).map((category) => ({
                value: category.id,
                label: category.name,
              }))}
            />
          </Form.Item>
          <Form.Item label="标签" name="tag_ids">
            <Select
              mode="multiple"
              placeholder="选择标签"
              options={(tags.data ?? []).map((tag) => ({
                value: tag.id,
                label: tag.name,
              }))}
            />
          </Form.Item>
          <Form.Item label="摘要" name="summary">
            <Input.TextArea rows={4} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title={editingCategory ? '编辑分类' : '新建分类'}
        open={categoryModalOpen}
        onCancel={closeCategoryModal}
        onOk={() => void submitCategory()}
        confirmLoading={createCategory.isPending || updateCategory.isPending}
        destroyOnHidden
      >
        <Form form={categoryForm} layout="vertical">
          <Form.Item label="分类名称" name="name" rules={[{ required: true, message: '请输入分类名称' }]}>
            <Input placeholder="例如：技术方案" />
          </Form.Item>
          <Form.Item label="说明" name="description">
            <Input.TextArea rows={3} placeholder="分类适用范围或归档规则" />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  )
}

function DocumentReferenceDrawer({
  document,
  onClose,
}: {
  document: KnowledgeDocumentDTO | null
  onClose: () => void
}) {
  const references = useQuery({
    queryKey: ['knowledge-document-references', document?.id],
    queryFn: () => fetchKnowledgeDocumentReferences(document!.id),
    enabled: Boolean(document),
  })

  return (
    <Drawer
      width={720}
      title={document ? `${document.title} 的引用记录` : '引用记录'}
      open={Boolean(document)}
      onClose={onClose}
      destroyOnHidden
    >
      {references.isLoading ? <LoadingBlock /> : null}
      {references.isError ? <ErrorBlock /> : null}
      {!references.isLoading && !references.isError && references.data?.length === 0 ? (
        <EmptyBlock />
      ) : null}
      <Table<KnowledgeDocumentReferenceDTO>
        rowKey="id"
        loading={references.isLoading}
        dataSource={references.data ?? []}
        pagination={false}
        columns={[
          {
            title: '标书',
            render: (_, row) => row.bid_title || '未关联标书',
          },
          {
            title: '章节',
            render: (_, row) => row.chapter_title || row.title,
          },
          {
            title: '资料片段',
            render: (_, row) => row.chunk_id ? <Tag color="blue">已定位</Tag> : <Tag>未整理</Tag>,
          },
          {
            title: '确认状态',
            render: (_, row) => {
              const sourceRef = row.metadata.source_ref as { resolved?: boolean; resolved_by?: string } | undefined
              if (sourceRef?.resolved) {
                return <Tag color="green">已确认</Tag>
              }
              return <Tag color="orange">待确认</Tag>
            },
          },
          {
            title: '引用时间',
            render: (_, row) => formatDateTime(row.created_at),
          },
        ]}
      />
    </Drawer>
  )
}

export function KnowledgeTemplatesPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canWrite = useCanAccess('knowledge', 'full')
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm<{
    name: string
    category?: string
    description?: string
    version?: string
    sections?: string
  }>()
  const templates = useQuery({
    queryKey: ['knowledge-templates'],
    queryFn: fetchKnowledgeTemplates,
  })
  const createTemplate = useMutation({
    mutationFn: createKnowledgeTemplate,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['knowledge-templates'] })
      message.success('模板已创建')
      setOpen(false)
      form.resetFields()
    },
    onError: (error) => {
      message.error(getApiErrorMessage(error, '创建模板失败'))
    },
  })

  const submitTemplate = async () => {
    const values = await form.validateFields()
    const sections = (values.sections ?? '')
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean)
    await createTemplate.mutateAsync({
      name: values.name,
      category: values.category,
      description: values.description,
      version: values.version,
      content: { sections },
    })
  }

  return (
    <PageFrame
      module="知识库"
      title="文档模板"
      subtitle="企业内部方案、报告、合同、制度模板"
      tags={['模板管理']}
      actions={[
        canWrite ? <Button key="new" type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          新建模板
        </Button> : null,
      ]}
    >
      {templates.isError ? <ErrorBlock /> : null}
      <Table<KnowledgeDocumentTemplateDTO>
        rowKey="id"
        loading={templates.isLoading}
        dataSource={templates.data ?? []}
        locale={{ emptyText: '暂无模板' }}
        columns={[
          { title: '模板名称', dataIndex: 'name' },
          { title: '分类', dataIndex: 'category' },
          { title: '版本', dataIndex: 'version' },
          { title: '说明', dataIndex: 'description' },
          { title: '使用次数', dataIndex: 'usage_count' },
          {
            title: '章节结构',
            render: (_, row) => {
              const sections = Array.isArray(row.content.sections) ? row.content.sections : []
              return (
                <Space wrap>
                  {sections.slice(0, 4).map((section) => (
                    <Tag key={String(section)}>{String(section)}</Tag>
                  ))}
                </Space>
              )
            },
          },
          {
            title: '创建时间',
            render: (_, row) => formatDateOnly(row.created_at),
          },
        ]}
      />
      <Modal
        title="新建文档模板"
        open={open}
        onCancel={() => setOpen(false)}
        onOk={() => void submitTemplate()}
        confirmLoading={createTemplate.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={{ category: '方案模板', version: 'v1.0' }}>
          <Form.Item label="模板名称" name="name" rules={[{ required: true, message: '请输入模板名称' }]}>
            <Input placeholder="例如：项目实施方案模板.docx" />
          </Form.Item>
          <Form.Item label="分类" name="category">
            <Input placeholder="方案模板 / 服务模板 / 制度模板" />
          </Form.Item>
          <Form.Item label="版本" name="version">
            <Input placeholder="v1.0" />
          </Form.Item>
          <Form.Item label="说明" name="description">
            <Input.TextArea rows={3} placeholder="模板用途、适用场景或审核要求" />
          </Form.Item>
          <Form.Item label="章节结构" name="sections">
            <Input.TextArea rows={5} placeholder={'每行一个章节\n项目理解\n总体架构\n实施计划'} />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  )
}

export function KnowledgeTagsPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canWrite = useCanAccess('knowledge', 'full')
  const [open, setOpen] = useState(false)
  const [editingTag, setEditingTag] = useState<KnowledgeTagDTO | null>(null)
  const [selectedTagId, setSelectedTagId] = useState<string | null>(null)
  const [form] = Form.useForm<{ name: string; color?: string }>()
  const tags = useQuery({
    queryKey: ['knowledge-tags'],
    queryFn: fetchKnowledgeTags,
  })
  const documents = useQuery({
    queryKey: ['knowledge-documents'],
    queryFn: fetchKnowledgeDocuments,
  })
  const refreshTags = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['knowledge-tags'] }),
      queryClient.invalidateQueries({ queryKey: ['knowledge-documents'] }),
      queryClient.invalidateQueries({ queryKey: ['knowledge-stats'] }),
    ])
  }
  const createTag = useMutation({
    mutationFn: createKnowledgeTag,
    onSuccess: async () => {
      await refreshTags()
      message.success('标签已创建')
      closeModal()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '标签创建失败')),
  })
  const updateTag = useMutation({
    mutationFn: ({ id, values }: { id: string; values: { name?: string; color?: string } }) =>
      updateKnowledgeTag(id, values),
    onSuccess: async () => {
      await refreshTags()
      message.success('标签已更新')
      closeModal()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '标签更新失败')),
  })
  const deleteTag = useMutation({
    mutationFn: deleteKnowledgeTag,
    onSuccess: async () => {
      await refreshTags()
      message.success('标签已删除')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '标签删除失败')),
  })
  const documentsByTag = (tag: KnowledgeTagDTO) =>
    (documents.data ?? []).filter((document) => document.tags.some((item) => item.id === tag.id))
  const openCreate = () => {
    setEditingTag(null)
    form.setFieldsValue({ name: '', color: 'blue' })
    setOpen(true)
  }
  const openEdit = (tag: KnowledgeTagDTO) => {
    setEditingTag(tag)
    form.setFieldsValue({ name: tag.name, color: tag.color })
    setOpen(true)
  }
  const closeModal = () => {
    setOpen(false)
    setEditingTag(null)
    form.resetFields()
  }
  const submitTag = async () => {
    const values = await form.validateFields()
    if (editingTag) {
      await updateTag.mutateAsync({ id: editingTag.id, values })
      return
    }
    await createTag.mutateAsync(values)
  }

  return (
    <PageFrame
      module="知识库"
      title="标签管理"
      subtitle="标签颜色、关联文档和删除确认"
      tags={['标签管理']}
      bare
      actions={[
        canWrite ? <Button key="new" type="primary" icon={<TagsOutlined />} onClick={openCreate}>
          新建标签
        </Button> : null,
      ]}
    >
      <Row gutter={16}>
        <Col xs={24} md={8}>
          <Card title="标签列表">
            <List
              loading={tags.isLoading}
              dataSource={tags.data ?? []}
              locale={{ emptyText: '暂无标签' }}
              renderItem={(item) => (
                <List.Item
                  style={{
                    cursor: 'pointer',
                    background: selectedTagId === item.id ? 'var(--primary-wash)' : undefined,
                    borderRadius: 8,
                    paddingInline: 8,
                  }}
                  onClick={() => setSelectedTagId(selectedTagId === item.id ? null : item.id)}
                  actions={canWrite ? [
                    <Button key="edit" type="text" size="small" icon={<EditOutlined />} onClick={(event) => { event.stopPropagation(); openEdit(item) }} />,
                    <Popconfirm
                      key="delete"
                      title="删除标签"
                      description="关联文档会移除此标签。"
                      okText="删除"
                      cancelText="取消"
                      onConfirm={() => deleteTag.mutate(item.id)}
                    >
                      <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={(event) => event.stopPropagation()} />
                    </Popconfirm>,
                  ] : []}
                >
                  <Tag color={item.color}>{item.name}</Tag>
                  <span>{documentsByTag(item).length} 个文档</span>
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col xs={24} md={16}>
          <Card
            title={
              selectedTagId
                ? `关联文档 · ${(tags.data ?? []).find((tag) => tag.id === selectedTagId)?.name ?? ''}`
                : '关联文档（点击左侧标签筛选）'
            }
            extra={
              selectedTagId ? (
                <Button size="small" type="link" onClick={() => setSelectedTagId(null)}>
                  清除筛选
                </Button>
              ) : null
            }
          >
            <Table
              rowKey="id"
              pagination={false}
              loading={documents.isLoading}
              locale={{ emptyText: selectedTagId ? '该标签下暂无文档' : '暂无文档' }}
              scroll={{ x: 620 }}
              dataSource={(documents.data ?? []).filter(
                (document) => !selectedTagId || document.tags.some((tag) => tag.id === selectedTagId),
              )}
              columns={[
                { title: '关联文档', dataIndex: 'title', width: 260 },
                {
                  title: '标签',
                  width: 360,
                  render: (_, row: KnowledgeDocumentDTO) => (
                    <Space wrap>
                      {row.tags.map((tag) => <Tag key={tag.id} color={tag.color}>{tag.name}</Tag>)}
                    </Space>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>
      <Modal
        title={editingTag ? '编辑标签' : '新建标签'}
        open={open}
        onCancel={closeModal}
        onOk={() => void submitTag()}
        confirmLoading={createTag.isPending || updateTag.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={{ color: 'blue' }}>
          <Form.Item label="标签名称" name="name" rules={[{ required: true, message: '请输入标签名称' }]}>
            <Input placeholder="例如：资质证书" />
          </Form.Item>
          <Form.Item label="颜色" name="color">
            <Select
              options={[
                { value: 'blue', label: '蓝色' },
                { value: 'green', label: '绿色' },
                { value: 'orange', label: '橙色' },
                { value: 'red', label: '红色' },
                { value: 'purple', label: '紫色' },
                { value: 'cyan', label: '青色' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  )
}

type FilePreviewSourceType = 'file' | 'knowledge'

export function FilePreviewPage({ sourceType = 'file' }: { sourceType?: FilePreviewSourceType }) {
  const { message } = AntApp.useApp()
  const { fileId, documentId } = useParams()
  const [searchParams] = useSearchParams()
  const sourceId = sourceType === 'knowledge' ? documentId : fileId
  const sourceLocator = filePreviewSourceLocator(searchParams)
  const preview = useQuery({
    queryKey: [sourceType === 'knowledge' ? 'knowledge-document-preview' : 'file-preview', sourceId],
    queryFn: () =>
      sourceType === 'knowledge'
        ? fetchKnowledgeDocumentPreview(sourceId!)
        : fetchFileURL(sourceId!, 'preview'),
    enabled: Boolean(sourceId),
  })
  const openDownload = async () => {
    const downloadFileId = preview.data?.file.id ?? (sourceType === 'file' ? sourceId : null)
    if (!downloadFileId) {
      return
    }
    try {
      const result = await fetchFileURL(downloadFileId, 'download')
      const openError = fileOpenErrorMessage(openFileUrl(result.url))
      if (openError) {
        message.error(openError)
      }
    } catch (error) {
      message.error(getApiErrorMessage(error, '获取下载链接失败'))
    }
  }
  const previewUrl = preview.data
    ? safeFileUrlString(
        withFilePreviewAnchor(preview.data.url, {
          page: sourceLocator.page,
          searchText: sourceLocator.searchText,
        }),
      )
    : null
  const copySourceLocator = async () => {
    if (!sourceLocator.locatorText) {
      message.info('暂无可复制定位')
      return
    }
    try {
      await navigator.clipboard.writeText(sourceLocator.locatorText)
      message.success('定位已复制')
    } catch {
      message.error('复制失败，请手动选择定位信息')
    }
  }

  return (
    <PageFrame
      module={fileModuleLabel(preview.data?.file.biz_type)}
      title={preview.data?.file.filename ?? '文件预览'}
      subtitle="文档预览"
      tags={['在线预览']}
      actions={[
        <Button key="download" icon={<DownloadOutlined />} disabled={!sourceId} onClick={() => void openDownload()}>
          下载
        </Button>,
      ]}
    >
      {preview.isLoading ? <LoadingBlock /> : null}
      {preview.isError ? <ErrorBlock /> : null}
      {preview.data && !previewUrl ? <ErrorBlock description="文件链接不可用，请重新获取" /> : null}
      {preview.data && previewUrl ? (
        <div className="file-preview-shell">
          {sourceLocator.hasLocator ? (
            <div className="file-preview-source-bar">
              <div className="file-preview-source-main">
                <Space size={6} wrap>
                  <FileSearchOutlined className="file-preview-source-icon" />
                  <Typography.Text strong>{sourceLocator.title || '定位来源'}</Typography.Text>
                  {sourceLocator.page ? <Tag>页码：{sourceLocator.page}</Tag> : null}
                </Space>
                {sourceLocator.searchText ? (
                  <Typography.Paragraph className="file-preview-source-excerpt">
                    {sourceLocator.searchText}
                  </Typography.Paragraph>
                ) : null}
              </div>
              <Button size="small" icon={<CopyOutlined />} disabled={!sourceLocator.locatorText} onClick={() => void copySourceLocator()}>
                复制定位
              </Button>
            </div>
          ) : null}
          <iframe
            title={preview.data.file.filename}
            src={previewUrl}
            className="file-preview-frame"
            referrerPolicy="no-referrer"
            sandbox="allow-downloads"
          />
        </div>
      ) : null}
    </PageFrame>
  )
}

function filePreviewSourceLocator(searchParams: URLSearchParams) {
  const page = positiveIntegerParam(searchParams.get('page'))
  const searchText = normalizePreviewParam(searchParams.get('search'), 120)
  const title = normalizePreviewParam(searchParams.get('source_title'), 120)
  const locatorText = normalizePreviewParam(searchParams.get('source_locator'), 800)
  return {
    page,
    searchText,
    title,
    locatorText,
    hasLocator: Boolean(page || searchText || title || locatorText),
  }
}

function positiveIntegerParam(value: string | null) {
  const numberValue = Number(value)
  return Number.isInteger(numberValue) && numberValue > 0 ? numberValue : null
}

function normalizePreviewParam(value: string | null, limit: number) {
  const normalized = (value || '').replace(/\s+/g, ' ').trim()
  if (!normalized) return ''
  return normalized.length > limit ? normalized.slice(0, limit) : normalized
}

function withFilePreviewAnchor(rawUrl: string, { page, searchText }: { page: number | null; searchText: string }) {
  if (!page && !searchText) return rawUrl
  try {
    const nextURL = new URL(rawUrl, window.location.href)
    const params = new URLSearchParams(nextURL.hash.replace(/^#/, '').replace(/^:/, ''))
    if (page) {
      params.set('page', String(page))
    }
    if (searchText) {
      params.set('search', searchText)
    }
    nextURL.hash = params.toString()
    return nextURL.toString()
  } catch {
    return rawUrl
  }
}

function fileModuleLabel(bizType?: string) {
  switch (bizType) {
    case 'bid_tender':
    case 'bid_export':
      return '标书管理'
    case 'knowledge':
    case 'knowledge_case':
    case 'generated':
      return '知识库'
    default:
      return '文件'
  }
}

const docTypeLabels: Record<string, string> = {
  general: '通用',
  won_case: '中标案例',
  pdf: 'PDF',
  word: 'Word',
  spreadsheet: '表格',
  presentation: '演示文稿',
  image: '图片',
}

function statusLabel(status: KnowledgeDocumentDTO['parse_status']) {
  const labels: Record<KnowledgeDocumentDTO['parse_status'], string> = {
    ready: '待整理',
    queued: '排队中',
    processing: '整理中',
    processed: '已整理',
    failed: '失败',
  }
  return labels[status] ?? '状态未知'
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

function documentStatusCell(document: KnowledgeDocumentDTO) {
  const failureMessage = documentFailureMessage(document)
  return (
    <Space direction="vertical" size={2}>
      <Tag color={statusColor(document.parse_status)}>{statusLabel(document.parse_status)}</Tag>
      {failureMessage ? <Typography.Text type="danger">{failureMessage}</Typography.Text> : null}
    </Space>
  )
}

function documentFailureMessage(document: KnowledgeDocumentDTO) {
  if (document.parse_status !== 'failed') return ''
  return document.error_message?.trim() || '文档整理失败，请重新整理'
}
