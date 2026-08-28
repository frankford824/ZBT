import { CloudSyncOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Alert,
  App as AntApp,
  AutoComplete,
  Button,
  Card,
  Drawer,
  Form,
  Input,
  Popconfirm,
  Select,
  Space,
  Statistic,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import { useEffect, useState, type ReactNode } from 'react'
import {
  fetchCompanyCertificates,
  fetchCompanyPersonnel,
  fetchQualificationSource,
  getApiErrorMessage,
  reviewCompanyCertificate,
  reviewCompanyPersonnel,
  syncQualificationFromZizhi,
  type CompanyCertificateDTO,
  type CompanyPersonnelDTO,
} from '../../shared/api/client'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { useCanAccess } from '../../shared/permissions/permissions'
import { formatDateOnly } from '../../shared/format/date'

const pageSize = 20

const verifyStatusMeta: Record<string, { color: string; label: string }> = {
  pending_review: { color: 'gold', label: '待确认' },
  confirmed: { color: 'green', label: '已确认' },
  rejected: { color: 'default', label: '已驳回' },
}

const extractedByLabels: Record<string, string> = {
  manual: '人工录入',
  zizhi: '资质库同步',
  ocr_llm: '本地抽取',
  case_archive: '项目回流',
}

const statusFilterOptions = [
  { key: 'pending_review', label: '待确认' },
  { key: 'confirmed', label: '已确认' },
  { key: '', label: '全部' },
]

// 两套术语并列，因为它们各自对应不同的资质体系，不是同义词的重复：
// 公路养护资质分甲乙丙级，建筑业企业资质分一二三级。填哪个按原件来。
const levelSuggestions = ['特级', '一级', '二级', '三级', '甲级', '乙级', '丙级']

function evidenceText(evidence: Record<string, unknown>, key: string) {
  const value = evidence?.[key]
  return typeof value === 'string' ? value : ''
}

// 置信度只反映「上游抽出了多少字段」，不代表这条记录是对的。
// 低分是常态而非异常：多数扫描件只有分类和正文，没有证号与有效期。
function ConfidenceTag({ value }: { value: number | null }) {
  if (value == null) return <Tag>未评估</Tag>
  const color = value >= 0.6 ? 'green' : value >= 0.35 ? 'gold' : 'red'
  return <Tag color={color}>{value.toFixed(2)}</Tag>
}

function ExpiryCell({ date, expired }: { date: string | null; expired: boolean }) {
  if (!date) return <Typography.Text type="secondary">未抽出</Typography.Text>
  return expired ? <Tag color="red">{formatDateOnly(date)} 已过期</Tag> : <span>{formatDateOnly(date)}</span>
}

function EvidencePanel({
  evidence,
  sourceRef,
  extractedBy,
}: {
  evidence: Record<string, unknown>
  sourceRef: string
  extractedBy: string
}) {
  const warning = evidenceText(evidence, 'warning')
  const path = evidenceText(evidence, 'unc_path') || evidenceText(evidence, 'path')
  const snippet = evidenceText(evidence, 'snippet')
  const contentState = evidenceText(evidence, 'content_state')
  return (
    <Space direction="vertical" size={8} className="full-width">
      {warning ? <Alert type="warning" showIcon message={warning} /> : null}
      <Space wrap size={4}>
        <Tag>{extractedByLabels[extractedBy] || extractedBy}</Tag>
        {sourceRef ? <Tag>{sourceRef}</Tag> : null}
        {contentState ? <Tag>正文 {contentState}</Tag> : null}
      </Space>
      {path ? (
        <Typography.Paragraph copyable={{ text: path }} style={{ marginBottom: 0 }}>
          <Typography.Text type="secondary">原件位置：</Typography.Text>
          {path}
        </Typography.Paragraph>
      ) : null}
      {snippet ? (
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
          {snippet}
        </Typography.Paragraph>
      ) : null}
    </Space>
  )
}

// 接口返回的是带时区的 ISO 串，表单里要填的是 YYYY-MM-DD。
function dateInputValue(value: string | null) {
  return value ? value.slice(0, 10) : ''
}

function ReviewDrawerShell({
  open,
  title,
  evidence,
  sourceRef,
  extractedBy,
  submitting,
  onClose,
  onConfirm,
  onReject,
  children,
}: {
  open: boolean
  title: string
  evidence: Record<string, unknown>
  sourceRef: string
  extractedBy: string
  submitting: boolean
  onClose: () => void
  onConfirm: () => void
  onReject: () => void
  children: ReactNode
}) {
  return (
    <Drawer
      open={open}
      title={title}
      width={640}
      onClose={onClose}
      destroyOnClose
      footer={
        <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <Popconfirm
            title="驳回这条记录？"
            description="驳回后它不再计入企业资质，也不会出现在到期预警里。"
            okText="驳回"
            cancelText="取消"
            onConfirm={onReject}
          >
            <Button danger loading={submitting}>
              驳回
            </Button>
          </Popconfirm>
          <Button type="primary" loading={submitting} onClick={onConfirm}>
            确认入档
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size={16} className="full-width">
        <Alert
          type="info"
          showIcon
          message="请对照 NAS 上的原件核对字段"
          description="确认后这条记录才会作为企业资质生效，并纳入到期预警。抽错的字段可以直接在下面改，留空表示该项原件上没有。"
        />
        <EvidencePanel evidence={evidence} sourceRef={sourceRef} extractedBy={extractedBy} />
        {children}
      </Space>
    </Drawer>
  )
}

function CertificateReviewDrawer({
  record,
  onClose,
}: {
  record: CompanyCertificateDTO | null
  onClose: () => void
}) {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const [form] = Form.useForm()

  useEffect(() => {
    if (!record) return
    form.setFieldsValue({
      cert_name: record.cert_name,
      cert_category: record.cert_category,
      cert_level: record.cert_level,
      cert_no: record.cert_no,
      issuer: record.issuer,
      issued_at: dateInputValue(record.issued_at),
      expires_at: dateInputValue(record.expires_at),
    })
  }, [record, form])

  const mutation = useMutation({
    mutationFn: (status: 'confirmed' | 'rejected') => {
      if (!record) throw new Error('no record')
      if (status === 'rejected') {
        return reviewCompanyCertificate(record.id, { verify_status: 'rejected' })
      }
      return reviewCompanyCertificate(record.id, { verify_status: 'confirmed', ...form.getFieldsValue() })
    },
    onSuccess: (_, status) => {
      message.success(status === 'confirmed' ? '已确认入档' : '已驳回')
      queryClient.invalidateQueries({ queryKey: ['company-certificates'] })
      onClose()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '提交失败')),
  })

  return (
    <ReviewDrawerShell
      open={Boolean(record)}
      title={record?.cert_name || '审核资质'}
      evidence={record?.extract_evidence ?? {}}
      sourceRef={record?.source_ref ?? ''}
      extractedBy={record?.extracted_by ?? ''}
      submitting={mutation.isPending}
      onClose={onClose}
      onConfirm={() => mutation.mutate('confirmed')}
      onReject={() => mutation.mutate('rejected')}
    >
      <Form form={form} layout="vertical">
        <Form.Item name="cert_name" label="资质名称">
          <Input />
        </Form.Item>
        <Form.Item name="cert_category" label="分类">
          <Input placeholder="例如：安全生产许可证" />
        </Form.Item>
        <Form.Item
          name="cert_level"
          label="等级"
          extra="按原件上的写法填，不用换算：公路养护资质写甲/乙/丙级，建筑业企业资质写一/二/三级。「一级及以上」这类要求由后端按等级高低判断，甲级与一级同级。"
        >
          <AutoComplete
            allowClear
            placeholder="原件上没有等级则留空"
            options={levelSuggestions.map((value) => ({ value }))}
          />
        </Form.Item>
        <Form.Item name="cert_no" label="证书编号">
          <Input placeholder="抽错了就直接改，原件上没有就清空" />
        </Form.Item>
        <Form.Item name="issuer" label="发证机关">
          <Input />
        </Form.Item>
        <Form.Item name="issued_at" label="发证日期">
          <Input placeholder="YYYY-MM-DD" />
        </Form.Item>
        <Form.Item name="expires_at" label="有效期至" extra="填了才会进入到期预警。">
          <Input placeholder="YYYY-MM-DD" />
        </Form.Item>
      </Form>
    </ReviewDrawerShell>
  )
}

function PersonnelReviewDrawer({
  record,
  onClose,
}: {
  record: CompanyPersonnelDTO | null
  onClose: () => void
}) {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const [form] = Form.useForm()

  useEffect(() => {
    if (!record) return
    form.setFieldsValue({
      person_name: record.person_name,
      cert_type: record.cert_type,
      cert_level: record.cert_level,
      major: record.major,
      reg_no: record.reg_no,
      expires_at: dateInputValue(record.expires_at),
      in_service: record.in_service,
    })
  }, [record, form])

  const mutation = useMutation({
    mutationFn: (status: 'confirmed' | 'rejected') => {
      if (!record) throw new Error('no record')
      if (status === 'rejected') {
        return reviewCompanyPersonnel(record.id, { verify_status: 'rejected' })
      }
      return reviewCompanyPersonnel(record.id, { verify_status: 'confirmed', ...form.getFieldsValue() })
    },
    onSuccess: (_, status) => {
      message.success(status === 'confirmed' ? '已确认入档' : '已驳回')
      queryClient.invalidateQueries({ queryKey: ['company-personnel'] })
      onClose()
    },
    onError: (error) => message.error(getApiErrorMessage(error, '提交失败')),
  })

  return (
    <ReviewDrawerShell
      open={Boolean(record)}
      title={record?.person_name || '审核人员'}
      evidence={record?.extract_evidence ?? {}}
      sourceRef={record?.source_ref ?? ''}
      extractedBy={record?.extracted_by ?? ''}
      submitting={mutation.isPending}
      onClose={onClose}
      onConfirm={() => mutation.mutate('confirmed')}
      onReject={() => mutation.mutate('rejected')}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="person_name"
          label="姓名"
          extra="上游是从文件名切出来的，可能把「经办人」这类前缀粘在了姓名前。"
        >
          <Input />
        </Form.Item>
        <Form.Item name="cert_type" label="持有证件" extra="多个证件用空格分隔。">
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} />
        </Form.Item>
        <Form.Item name="cert_level" label="等级">
          <AutoComplete
            allowClear
            placeholder="如注册建造师的一级 / 二级"
            options={levelSuggestions.map((value) => ({ value }))}
          />
        </Form.Item>
        <Form.Item name="major" label="专业">
          <Input placeholder="例如：市政公用 / 建筑工程 / 机电" />
        </Form.Item>
        <Form.Item name="reg_no" label="注册编号">
          <Input />
        </Form.Item>
        <Form.Item name="expires_at" label="最近到期" extra="多本证件时填最早到期的那本。">
          <Input placeholder="YYYY-MM-DD" />
        </Form.Item>
        <Form.Item name="in_service" label="在职状态">
          <Select
            options={[
              { value: true, label: '在职' },
              { value: false, label: '离职' },
            ]}
          />
        </Form.Item>
      </Form>
    </ReviewDrawerShell>
  )
}

function SourceCard() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canSync = useCanAccess('team', 'full')
  const source = useQuery({ queryKey: ['qualification-source'], queryFn: fetchQualificationSource })

  const sync = useMutation({
    mutationFn: syncQualificationFromZizhi,
    onSuccess: (result) => {
      const { certificates, personnel } = result
      message.success(
        `同步完成：证书新增 ${certificates.inserted} / 更新 ${certificates.updated}，` +
          `人员新增 ${personnel.inserted} / 更新 ${personnel.updated}，待确认 ${result.pending_review} 条`,
      )
      queryClient.invalidateQueries({ queryKey: ['company-certificates'] })
      queryClient.invalidateQueries({ queryKey: ['company-personnel'] })
      queryClient.invalidateQueries({ queryKey: ['qualification-source'] })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '同步失败')),
  })

  if (source.isLoading) return <LoadingBlock />
  const data = source.data
  if (!data?.configured) {
    return (
      <Alert
        type="warning"
        showIcon
        message="资质库未接入"
        description={data?.reason || '需要后端配置 ZIZHI_API_URL 与 ZIZHI_API_KEY 后才能同步企业资质。'}
      />
    )
  }
  if (!data.reachable) {
    return (
      <Alert
        type="error"
        showIcon
        message="资质库暂时不可达"
        description="已配置但连不上局域网资质库，档案仍可查看，但无法同步最新内容。"
        action={<Button onClick={() => source.refetch()}>重试</Button>}
      />
    )
  }

  return (
    <Card size="small">
      <Space size={48} wrap>
        <Statistic title="资质库文件" value={data.total_files ?? 0} suffix="份" />
        <Statistic title="人员档案" value={data.people ?? 0} suffix="人" />
        <Statistic
          title="证面已过期"
          value={data.expired_files ?? 0}
          suffix="份"
          valueStyle={{ color: (data.expired_files ?? 0) > 0 ? '#cf1322' : undefined }}
        />
        <Space direction="vertical" size={4}>
          <Tooltip title={canSync ? '从局域网资质库拉取最新资质与人员，已确认的记录不会被覆盖' : '需要完整权限'}>
            <Button
              type="primary"
              icon={<CloudSyncOutlined />}
              loading={sync.isPending}
              disabled={!canSync || data.indexing || data.extracting}
              onClick={() => sync.mutate()}
            >
              同步资质库
            </Button>
          </Tooltip>
          {data.indexing || data.extracting ? (
            <Typography.Text type="secondary">资质库正在{data.indexing ? '建索引' : '抽取正文'}，稍后再同步</Typography.Text>
          ) : null}
        </Space>
      </Space>
    </Card>
  )
}

function CertificatesTable() {
  const [status, setStatus] = useState('pending_review')
  const [page, setPage] = useState(1)
  const [reviewing, setReviewing] = useState<CompanyCertificateDTO | null>(null)
  const canReview = useCanAccess('team', 'full')
  const query = useQuery({
    queryKey: ['company-certificates', status, page],
    queryFn: () =>
      fetchCompanyCertificates({
        verify_status: status || undefined,
        limit: pageSize,
        offset: (page - 1) * pageSize,
      }),
  })

  return (
    <Space direction="vertical" size={12} className="full-width">
      <Tabs
        size="small"
        activeKey={status}
        onChange={(key) => {
          setStatus(key)
          setPage(1)
        }}
        items={statusFilterOptions.map((option) => ({ key: option.key, label: option.label }))}
      />
      {query.isLoading ? <LoadingBlock /> : null}
      {query.isError ? (
        <ErrorBlock description={getApiErrorMessage(query.error, '资质档案加载失败')} onRetry={() => query.refetch()} />
      ) : null}
      {!query.isLoading && !query.isError && !query.data?.items.length ? (
        <EmptyBlock description={status === 'confirmed' ? '还没有已确认的资质' : '资质档案为空，先同步资质库'} />
      ) : null}
      {query.data?.items.length ? (
        <Table<CompanyCertificateDTO>
          rowKey="id"
          dataSource={query.data.items}
          scroll={{ x: 1000 }}
          pagination={{
            current: page,
            pageSize,
            total: query.data.total,
            showSizeChanger: false,
            showTotal: (total) => `共 ${total} 条`,
            onChange: setPage,
          }}
          expandable={{
            expandedRowRender: (row) => (
              <EvidencePanel
                evidence={row.extract_evidence}
                sourceRef={row.source_ref}
                extractedBy={row.extracted_by}
              />
            ),
          }}
          columns={[
            { title: '资质名称', dataIndex: 'cert_name', width: 200 },
            { title: '分类', dataIndex: 'cert_category', width: 140, render: (value: string) => value || '-' },
            {
              title: '等级',
              dataIndex: 'cert_level',
              width: 90,
              render: (value: string) => (value ? <Tag color="blue">{value}</Tag> : <Typography.Text type="secondary">未抽出</Typography.Text>),
            },
            { title: '证号', dataIndex: 'cert_no', width: 200, ellipsis: true, render: (value: string) => value || '-' },
            {
              title: '有效期至',
              dataIndex: 'expires_at',
              width: 150,
              render: (value: string | null, row) => <ExpiryCell date={value} expired={row.expired} />,
            },
            {
              title: '完整度',
              dataIndex: 'extract_confidence',
              width: 90,
              render: (value: number | null) => <ConfidenceTag value={value} />,
            },
            {
              title: '状态',
              dataIndex: 'verify_status',
              width: 90,
              render: (value: string) => {
                const meta = verifyStatusMeta[value] || { color: 'default', label: value }
                return <Tag color={meta.color}>{meta.label}</Tag>
              },
            },
            {
              title: '操作',
              key: 'action',
              width: 80,
              fixed: 'right',
              render: (_: unknown, row: CompanyCertificateDTO) => (
                <Button type="link" size="small" disabled={!canReview} onClick={() => setReviewing(row)}>
                  {row.verify_status === 'pending_review' ? '审核' : '修订'}
                </Button>
              ),
            },
          ]}
        />
      ) : null}
      <CertificateReviewDrawer record={reviewing} onClose={() => setReviewing(null)} />
    </Space>
  )
}

function PersonnelTable() {
  const [status, setStatus] = useState('pending_review')
  const [page, setPage] = useState(1)
  const [reviewing, setReviewing] = useState<CompanyPersonnelDTO | null>(null)
  const canReview = useCanAccess('team', 'full')
  const query = useQuery({
    queryKey: ['company-personnel', status, page],
    queryFn: () =>
      fetchCompanyPersonnel({
        verify_status: status || undefined,
        limit: pageSize,
        offset: (page - 1) * pageSize,
      }),
  })

  return (
    <Space direction="vertical" size={12} className="full-width">
      <Tabs
        size="small"
        activeKey={status}
        onChange={(key) => {
          setStatus(key)
          setPage(1)
        }}
        items={statusFilterOptions.map((option) => ({ key: option.key, label: option.label }))}
      />
      {query.isLoading ? <LoadingBlock /> : null}
      {query.isError ? (
        <ErrorBlock description={getApiErrorMessage(query.error, '人员档案加载失败')} onRetry={() => query.refetch()} />
      ) : null}
      {!query.isLoading && !query.isError && !query.data?.items.length ? (
        <EmptyBlock description={status === 'confirmed' ? '还没有已确认的人员' : '人员档案为空，先同步资质库'} />
      ) : null}
      {query.data?.items.length ? (
        <Table<CompanyPersonnelDTO>
          rowKey="id"
          dataSource={query.data.items}
          scroll={{ x: 1000 }}
          pagination={{
            current: page,
            pageSize,
            total: query.data.total,
            showSizeChanger: false,
            showTotal: (total) => `共 ${total} 人`,
            onChange: setPage,
          }}
          expandable={{
            expandedRowRender: (row) => (
              <EvidencePanel
                evidence={row.extract_evidence}
                sourceRef={row.source_ref}
                extractedBy={row.extracted_by}
              />
            ),
          }}
          columns={[
            { title: '姓名', dataIndex: 'person_name', width: 120 },
            {
              title: '持有证件',
              dataIndex: 'cert_type',
              ellipsis: true,
              render: (value: string) => value || <Typography.Text type="secondary">未识别</Typography.Text>,
            },
            {
              title: '等级',
              dataIndex: 'cert_level',
              width: 90,
              render: (value: string) => (value ? <Tag color="blue">{value}</Tag> : '-'),
            },
            {
              title: '最近到期',
              dataIndex: 'expires_at',
              width: 150,
              render: (value: string | null, row) => <ExpiryCell date={value} expired={row.expired} />,
            },
            {
              title: '在职',
              dataIndex: 'in_service',
              width: 80,
              render: (value: boolean) => (value ? <Tag color="green">在职</Tag> : <Tag>离职</Tag>),
            },
            {
              title: '完整度',
              dataIndex: 'extract_confidence',
              width: 90,
              render: (value: number | null) => <ConfidenceTag value={value} />,
            },
            {
              title: '状态',
              dataIndex: 'verify_status',
              width: 90,
              render: (value: string) => {
                const meta = verifyStatusMeta[value] || { color: 'default', label: value }
                return <Tag color={meta.color}>{meta.label}</Tag>
              },
            },
            {
              title: '操作',
              key: 'action',
              width: 80,
              fixed: 'right',
              render: (_: unknown, row: CompanyPersonnelDTO) => (
                <Button type="link" size="small" disabled={!canReview} onClick={() => setReviewing(row)}>
                  {row.verify_status === 'pending_review' ? '审核' : '修订'}
                </Button>
              ),
            },
          ]}
        />
      ) : null}
      <PersonnelReviewDrawer record={reviewing} onClose={() => setReviewing(null)} />
    </Space>
  )
}

export function QualificationPage() {
  return (
    <Space direction="vertical" size={16} className="full-width">
      <div>
        <Typography.Title level={4} style={{ marginBottom: 4 }}>
          企业资质
        </Typography.Title>
        <Typography.Text type="secondary">
          档案同步自公司局域网资质库，原件仍存放在 NAS 上，这里只保留检索凭证与抽取出的字段。
        </Typography.Text>
      </div>
      <SourceCard />
      <Alert
        type="info"
        showIcon
        message="同步进来的记录一律为「待确认」"
        description="上游是扫描件 OCR 的结果，证号、有效期、持证人都可能抽错或抽不全，确认之前不作为投标资格依据。点右侧「审核」逐条核对：确认后才会计入企业资质并纳入到期预警，明显是模板或噪声的直接驳回。"
      />
      <Card>
        <Tabs
          items={[
            { key: 'certificates', label: '企业证书', children: <CertificatesTable /> },
            { key: 'personnel', label: '人员班子', children: <PersonnelTable /> },
          ]}
        />
      </Card>
    </Space>
  )
}
