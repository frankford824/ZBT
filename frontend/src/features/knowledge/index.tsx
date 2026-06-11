import {
  CloudUploadOutlined,
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
import { App as AntApp, Button, Card, Col, Drawer, Form, Input, List, Modal, Popconfirm, Row, Select, Space, Statistic, Table, Tag, Upload } from 'antd'
import type { UploadProps } from 'antd'
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  confirmFileUpload,
  createKnowledgeCategory,
  createKnowledgeTemplate,
  createKnowledgeTag,
  createPresignedUpload,
  deleteKnowledgeCategory,
  deleteKnowledgeTag,
  fetchFileURL,
  fetchKnowledgeCategories,
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
    } catch {
      message.error('搜索失败')
    } finally {
      setSearching(false)
    }
  }

  return (
    <PageFrame
      module="企业管理"
      title="知识库"
      subtitle="分类卡、语义搜索、引用统计和中标案例回流"
      tags={['page-knowledge', '/knowledge']}
    >
      <Space direction="vertical" size={16} className="full-width">
        <Input.Search
          placeholder="搜索企业资质、案例、技术方案"
          enterButton="搜索"
          loading={searching}
          onSearch={(value) => void runSearch(value)}
        />
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
        {searchResult ? (
          <Card title="搜索结果">
            <List
              dataSource={searchResult.items}
              locale={{ emptyText: '暂无命中' }}
              renderItem={(item) => (
                <List.Item
                  actions={[
                    <Tag key="ref" color="blue">
                      {item.source_ref.chunk_id.slice(0, 8)}
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
  const [referenceDocument, setReferenceDocument] = useState<KnowledgeDocumentDTO | null>(null)
  const [categoryModalOpen, setCategoryModalOpen] = useState(false)
  const [editingCategory, setEditingCategory] = useState<KnowledgeCategoryDTO | null>(null)
  const [categoryForm] = Form.useForm<{ name: string; description?: string }>()
  const [editingDocument, setEditingDocument] = useState<KnowledgeDocumentDTO | null>(null)
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
    onError: () => message.error('分类创建失败'),
  })
  const updateCategory = useMutation({
    mutationFn: ({ id, values }: { id: string; values: { name?: string; description?: string } }) =>
      updateKnowledgeCategory(id, values),
    onSuccess: async () => {
      await refreshKnowledgeLists()
      message.success('分类已更新')
      closeCategoryModal()
    },
    onError: () => message.error('分类更新失败'),
  })
  const deleteCategory = useMutation({
    mutationFn: deleteKnowledgeCategory,
    onSuccess: async () => {
      await refreshKnowledgeLists()
      message.success('分类已删除')
    },
    onError: () => message.error('分类删除失败'),
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
    onError: () => message.error('文档信息更新失败'),
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
          <Card
            title={`文档分类 ${documents.data?.length ?? 0}`}
            extra={<Button size="small" icon={<PlusOutlined />} onClick={openCreateCategory}>新建</Button>}
          >
            <List
              loading={categories.isLoading}
              dataSource={categories.data ?? []}
              locale={{ emptyText: '暂无分类' }}
              renderItem={(item) => (
                <List.Item
                  actions={[
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
                  ]}
                >
                  <List.Item.Meta title={item.name} description={item.description || '无说明'} />
                </List.Item>
              )}
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
                        icon={<EditOutlined />}
                        onClick={() => openEditDocument(row)}
                      >
                        编辑
                      </Button>
                      <Button
                        size="small"
                        icon={<LinkOutlined />}
                        onClick={() => setReferenceDocument(row)}
                      >
                        引用
                      </Button>
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
              options={[
                { value: 'general', label: '通用' },
                { value: 'pdf', label: 'PDF' },
                { value: 'word', label: 'Word' },
                { value: 'spreadsheet', label: '表格' },
                { value: 'presentation', label: '演示文稿' },
                { value: 'image', label: '图片' },
              ]}
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
      title={document ? `${document.title} 的引用追踪` : '引用追踪'}
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
            title: 'Chunk',
            render: (_, row) => row.chunk_id ? <Tag color="blue">{row.chunk_id.slice(0, 8)}</Tag> : <Tag>未解析</Tag>,
          },
          {
            title: '来源状态',
            render: (_, row) => {
              const sourceRef = row.metadata.source_ref as { resolved?: boolean; resolved_by?: string } | undefined
              if (sourceRef?.resolved) {
                return <Tag color="green">{sourceRef.resolved_by ?? 'resolved'}</Tag>
              }
              return <Tag color="orange">待确认</Tag>
            },
          },
          {
            title: '引用时间',
            render: (_, row) => new Date(row.created_at).toLocaleString(),
          },
        ]}
      />
    </Drawer>
  )
}

export function KnowledgeTemplatesPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
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
    onError: () => {
      message.error('创建模板失败')
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
      tags={['page-knowledge-templates', '/knowledge/templates']}
      actions={[
        <Button key="new" type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          新建模板
        </Button>,
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
            render: (_, row) => new Date(row.created_at).toLocaleDateString(),
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
  const [open, setOpen] = useState(false)
  const [editingTag, setEditingTag] = useState<KnowledgeTagDTO | null>(null)
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
    onError: () => message.error('标签创建失败'),
  })
  const updateTag = useMutation({
    mutationFn: ({ id, values }: { id: string; values: { name?: string; color?: string } }) =>
      updateKnowledgeTag(id, values),
    onSuccess: async () => {
      await refreshTags()
      message.success('标签已更新')
      closeModal()
    },
    onError: () => message.error('标签更新失败'),
  })
  const deleteTag = useMutation({
    mutationFn: deleteKnowledgeTag,
    onSuccess: async () => {
      await refreshTags()
      message.success('标签已删除')
    },
    onError: () => message.error('标签删除失败'),
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
      tags={['page-knowledge-tags', '/knowledge/tags']}
      actions={[
        <Button key="new" type="primary" icon={<TagsOutlined />} onClick={openCreate}>
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
              locale={{ emptyText: '暂无标签' }}
              renderItem={(item) => (
                <List.Item
                  actions={[
                    <Button key="edit" type="text" size="small" icon={<EditOutlined />} onClick={() => openEdit(item)} />,
                    <Popconfirm
                      key="delete"
                      title="删除标签"
                      description="关联文档会移除此标签。"
                      okText="删除"
                      cancelText="取消"
                      onConfirm={() => deleteTag.mutate(item.id)}
                    >
                      <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                    </Popconfirm>,
                  ]}
                >
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
