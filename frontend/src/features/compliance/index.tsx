import {
  AimOutlined,
  CheckCircleOutlined,
  DeleteOutlined,
  PlusOutlined,
  ReloadOutlined,
  ToolOutlined,
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  App as AntApp,
  Button,
  Card,
  Checkbox,
  Col,
  Form,
  Input,
  Modal,
  Popconfirm,
  Progress,
  Row,
  Select,
  Space,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  autofixComplianceIssue,
  confirmFailComplianceIssue,
  createComplianceCheck,
  createComplianceReport,
  createComplianceRule,
  deleteComplianceRule,
  fetchComplianceCheck,
  fetchComplianceChecks,
  fetchComplianceIssues,
  fetchComplianceRules,
  getApiErrorMessage,
  ignoreComplianceIssue,
  updateComplianceRule,
  type ComplianceCheckDTO,
  type ComplianceIssueDTO,
  type ComplianceRuleDTO,
  type ComplianceSeverity,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { useCanAccess } from '../../shared/permissions/permissions'

const levelOptions = ['L1', 'L2', 'L3', 'L4'].map((value) => ({ label: value, value }))

const severityOptions: Array<{ label: string; value: ComplianceSeverity }> = [
  { label: '通过', value: 'pass' },
  { label: '警告', value: 'warn' },
  { label: '待人工确认', value: 'fail_candidate' },
  { label: '废标项', value: 'fail' },
]

const severityLabels: Record<string, string> = {
  pass: '通过',
  warn: '警告',
  fail_candidate: '待确认',
  fail: '废标项',
}

const statusLabels: Record<string, string> = {
  queued: '排队中',
  running: '检查中',
  done: '完成',
  failed: '失败',
  open: '未处理',
  fixed: '已修复',
  ignored: '已忽略',
  confirmed_fail: '已判废标',
}

const verdictText: Record<string, string> = {
  pass: '可以提交，未发现废标风险',
  warn: '可以提交，但有警告项建议先处理',
  fail_candidate: '存在待人工确认的风险项，确认前不建议提交',
  fail: '存在废标项，必须修复后再提交',
}

function verdictColor(result: string) {
  if (result === 'fail') return '#DC2626'
  if (result === 'fail_candidate' || result === 'warn') return '#D97706'
  return '#16A34A'
}

function severityTag(severity: ComplianceSeverity) {
  const color = severity === 'fail' ? 'red' : severity === 'fail_candidate' ? 'orange' : severity === 'warn' ? 'gold' : 'green'
  return <Tag color={color}>{severityLabels[severity] || severity}</Tag>
}

function statusTag(status: string) {
  const color = status === 'done' || status === 'fixed' ? 'green' : status === 'failed' || status === 'confirmed_fail' ? 'red' : 'blue'
  return <Tag color={color}>{statusLabels[status] || status}</Tag>
}

function formatTime(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function issueCounts(checks: ComplianceCheckDTO[]) {
  return checks.reduce(
    (acc, check) => {
      acc.total += check.issue_count
      if (check.result_status === 'fail') acc.fail += 1
      if (check.result_status === 'fail_candidate') acc.pending += 1
      acc.score += check.score
      return acc
    },
    { total: 0, fail: 0, pending: 0, score: 0 },
  )
}

function stringLocationValue(location: Record<string, unknown>, key: string) {
  const value = location[key]
  return typeof value === 'string' ? value.trim() : ''
}

function editorLocationPath(checkBidId: string | null, issue: ComplianceIssueDTO) {
  const bidId = stringLocationValue(issue.location, 'bid_document_id') || checkBidId || ''
  if (!bidId) return ''

  const locationPath = stringLocationValue(issue.location, 'path')
  if (locationPath.startsWith(`/bids/${bidId}/editor`)) {
    return locationPath
  }

  const params = new URLSearchParams()
  const partCode = stringLocationValue(issue.location, 'part_code')
  const chapterId = stringLocationValue(issue.location, 'chapter_id')
  if (partCode) params.set('part', partCode)
  if (chapterId) params.set('chapter', chapterId)
  const query = params.toString()
  return `/bids/${bidId}/editor${query ? `?${query}` : ''}`
}

export function CompliancePage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [checkOpen, setCheckOpen] = useState(false)
  const [ruleOpen, setRuleOpen] = useState(false)
  const canWrite = useCanAccess('compliance', 'full')
  const [checkForm] = Form.useForm<{ name: string; bid_document_id?: string; levels: string[] }>()
  const [ruleForm] = Form.useForm<{
    code: string
    name: string
    category: string
    level: ComplianceRuleDTO['level']
    severity: ComplianceSeverity
    description?: string
    enabled: boolean
  }>()

  const checks = useQuery({ queryKey: ['compliance-checks'], queryFn: fetchComplianceChecks })
  const rules = useQuery({ queryKey: ['compliance-rules'], queryFn: fetchComplianceRules })
  const counts = issueCounts(checks.data || [])

  const createCheckMutation = useMutation({
    mutationFn: createComplianceCheck,
    onSuccess: (snapshot) => {
      setCheckOpen(false)
      checkForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['compliance-checks'] })
      message.success('合规检查已完成')
      navigate(`/compliance/${snapshot.check.id}`)
    },
    onError: (error) => message.error(getApiErrorMessage(error, '合规检查创建失败')),
  })

  const createRuleMutation = useMutation({
    mutationFn: createComplianceRule,
    onSuccess: () => {
      setRuleOpen(false)
      ruleForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['compliance-rules'] })
      message.success('规则已创建')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '规则创建失败')),
  })

  const updateRuleMutation = useMutation({
    mutationFn: (rule: ComplianceRuleDTO) =>
      updateComplianceRule(rule.id, {
        code: rule.code,
        name: rule.name,
        category: rule.category,
        level: rule.level,
        severity: rule.severity,
        description: rule.description,
        enabled: rule.enabled,
        metadata: rule.metadata,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['compliance-rules'] })
      message.success('规则已更新')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '规则更新失败')),
  })

  const deleteRuleMutation = useMutation({
    mutationFn: deleteComplianceRule,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['compliance-rules'] })
      message.success('规则已删除')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '规则删除失败')),
  })

  return (
    <PageFrame
      module="投标管控"
      title="合规检查"
      subtitle="四层规则检查、人工确认和修复闭环"
      tags={['page-compliance', '/compliance']}
      bare
      actions={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={() => checks.refetch()}>
          刷新
        </Button>,
        canWrite ? <Button key="rule" icon={<PlusOutlined />} onClick={() => setRuleOpen(true)}>
          新增规则
        </Button> : null,
        canWrite ? <Button key="start" type="primary" icon={<CheckCircleOutlined />} onClick={() => setCheckOpen(true)}>
          开始检查
        </Button> : null,
      ]}
    >
      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Card size="small" className="stat-card">
            <Statistic title="检查任务" value={checks.data?.length || 0} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card size="small" className="stat-card">
            <Statistic title="平均得分" value={checks.data?.length ? counts.score / checks.data.length : 0} precision={1} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card size="small" className="stat-card">
            <Statistic title="待处理风险" value={counts.fail + counts.pending} />
          </Card>
        </Col>
        <Col span={24}>
          <Tabs
            items={[
              {
                key: 'checks',
                label: '检查历史',
                children: (
                  <>
                    {checks.isLoading && <LoadingBlock />}
                    {checks.isError && <ErrorBlock />}
                    {!checks.isLoading && !checks.isError && !checks.data?.length && <EmptyBlock />}
                    {!checks.isLoading && !checks.isError && Boolean(checks.data?.length) && (
                      <Table
                        rowKey="id"
                        dataSource={checks.data}
                        scroll={{ x: 920 }}
                        columns={[
                          {
                            title: '检查名称',
                            dataIndex: 'name',
                            width: 220,
                            render: (value, row) => <Link to={`/compliance/${row.id}`}>{value}</Link>,
                          },
                          { title: '标书', dataIndex: 'bid_title', width: 220, render: (value) => value || '-' },
                          { title: '结果', dataIndex: 'result_status', width: 110, render: severityTag },
                          { title: '得分', dataIndex: 'score', width: 150, render: (value) => <Progress percent={value} size="small" /> },
                          { title: '问题数', dataIndex: 'issue_count', width: 86 },
                          { title: '状态', dataIndex: 'status', width: 96, render: statusTag },
                          { title: '创建时间', dataIndex: 'created_at', width: 190, render: formatTime },
                        ]}
                      />
                    )}
                  </>
                ),
              },
              {
                key: 'rules',
                label: '规则库',
                children: (
                  <>
                    {rules.isLoading && <LoadingBlock />}
                    {rules.isError && <ErrorBlock />}
                    {!rules.isLoading && !rules.isError && !rules.data?.length && <EmptyBlock />}
                    {!rules.isLoading && !rules.isError && Boolean(rules.data?.length) && (
                      <Table
                        rowKey="id"
                        dataSource={rules.data}
                        scroll={{ x: 820 }}
                        columns={[
                          { title: '编码', dataIndex: 'code', width: 170 },
                          { title: '名称', dataIndex: 'name', width: 180 },
                          { title: '分类', dataIndex: 'category', width: 130 },
                          { title: '层级', dataIndex: 'level', width: 80, render: (value) => <Tag>{value}</Tag> },
                          { title: '严重度', dataIndex: 'severity', width: 120, render: severityTag },
                          {
                            title: '启用',
                            dataIndex: 'enabled',
                            width: 80,
                            render: (enabled, row) => (
                              <Switch
                                checked={enabled}
                                size="small"
                                disabled={!canWrite}
                                onChange={(checked) => updateRuleMutation.mutate({ ...row, enabled: checked })}
                              />
                            ),
                          },
                          {
                            title: '操作',
                            width: 110,
                            render: (_, row) => canWrite ? (
                              <Popconfirm title="删除规则" description="确认删除该合规规则？" onConfirm={() => deleteRuleMutation.mutate(row.id)}>
                                <Button size="small" danger icon={<DeleteOutlined />}>
                                  删除
                                </Button>
                              </Popconfirm>
                            ) : '-',
                          },
                        ]}
                      />
                    )}
                  </>
                ),
              },
            ]}
          />
        </Col>
      </Row>
      <Modal
        open={checkOpen}
        title="开始合规检查"
        onCancel={() => setCheckOpen(false)}
        onOk={checkForm.submit}
        confirmLoading={createCheckMutation.isPending}
      >
        <Form
          form={checkForm}
          layout="vertical"
          initialValues={{ name: '投标文件合规检查', levels: ['L1', 'L2', 'L3'] }}
          onFinish={createCheckMutation.mutate}
        >
          <Form.Item name="name" label="检查名称" rules={[{ required: true, message: '检查名称必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="bid_document_id" label="关联标书">
            <Input placeholder="可选，填写标书编号；留空则执行独立检查" />
          </Form.Item>
          <Form.Item name="levels" label="检查层级">
            <Checkbox.Group options={levelOptions} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        open={ruleOpen}
        title="新增合规规则"
        onCancel={() => setRuleOpen(false)}
        onOk={ruleForm.submit}
        confirmLoading={createRuleMutation.isPending}
      >
        <Form
          form={ruleForm}
          layout="vertical"
          initialValues={{ level: 'L1', severity: 'warn', enabled: true }}
          onFinish={(values) => createRuleMutation.mutate({ ...values, metadata: {} })}
        >
          <Form.Item name="code" label="规则编号" rules={[{ required: true, message: '规则编号必填' }]}>
            <Input placeholder="例如：交付时间要求" />
          </Form.Item>
          <Form.Item name="name" label="规则名称" rules={[{ required: true, message: '规则名称必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="category" label="分类" rules={[{ required: true, message: '分类必填' }]}>
            <Input placeholder="废标条款 / 格式规范 / 评分标准" />
          </Form.Item>
          <Form.Item name="level" label="层级">
            <Select options={levelOptions} />
          </Form.Item>
          <Form.Item name="severity" label="严重度">
            <Select options={severityOptions} />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  )
}

export function ComplianceDetailPage() {
  const { checkId = '' } = useParams()
  const navigate = useNavigate()
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canWrite = useCanAccess('compliance', 'full')
  const check = useQuery({
    queryKey: ['compliance-check', checkId],
    queryFn: () => fetchComplianceCheck(checkId),
    enabled: Boolean(checkId),
  })
  const issues = useQuery({
    queryKey: ['compliance-issues', checkId],
    queryFn: () => fetchComplianceIssues(checkId),
    enabled: Boolean(checkId),
  })

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['compliance-check', checkId] })
    queryClient.invalidateQueries({ queryKey: ['compliance-issues', checkId] })
    queryClient.invalidateQueries({ queryKey: ['compliance-checks'] })
  }

  const actionMutation = useMutation({
    mutationFn: ({ action, issueId }: { action: 'autofix' | 'ignore' | 'confirm'; issueId: string }) => {
      if (action === 'autofix') return autofixComplianceIssue(issueId)
      if (action === 'ignore') return ignoreComplianceIssue(issueId)
      return confirmFailComplianceIssue(issueId)
    },
    onSuccess: () => {
      refresh()
      message.success('问题状态已更新')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '问题状态更新失败')),
  })

  const reportMutation = useMutation({
    mutationFn: () => createComplianceReport(checkId),
    onSuccess: (report) => message.success(report.summary || '报告已生成'),
    onError: (error) => message.error(getApiErrorMessage(error, '报告生成失败')),
  })

  const categories = useMemo(() => {
    const names = Array.from(new Set((issues.data || []).map((issue) => issue.category)))
    return ['全部', ...names]
  }, [issues.data])

  if (check.isLoading || issues.isLoading) return <LoadingBlock />
  if (check.isError || issues.isError) return <ErrorBlock />
  if (!check.data) return <EmptyBlock />

  const issueRows = issues.data || []
  const openIssues = issueRows.filter((issue) => issue.status === 'open' || issue.status === 'confirmed_fail').length
  const severityCounts = issueRows.reduce(
    (acc, issue) => {
      acc[issue.severity] = (acc[issue.severity] ?? 0) + 1
      return acc
    },
    {} as Record<string, number>,
  )
  const resultStatus = check.data.result_status
  const scoreColor = verdictColor(resultStatus)

  const renderIssueTable = (rows: ComplianceIssueDTO[]) => (
    <Table
      rowKey="id"
      dataSource={rows}
      scroll={{ x: 960 }}
      columns={[
        { title: '问题', dataIndex: 'title', width: 220, ellipsis: true },
        { title: '分类', dataIndex: 'category', width: 110 },
        { title: '严重度', dataIndex: 'severity', width: 96, render: severityTag },
        { title: '状态', dataIndex: 'status', width: 96, render: statusTag },
        {
          title: '证据',
          dataIndex: 'evidence',
          ellipsis: { showTitle: false },
          render: (value: string) => (
            <Tooltip title={value} placement="topLeft">
              <span>{value || '-'}</span>
            </Tooltip>
          ),
        },
        {
          title: '修复建议',
          dataIndex: 'suggestion',
          ellipsis: { showTitle: false },
          render: (value: string) => (
            <Tooltip title={value} placement="topLeft">
              <span>{value || '-'}</span>
            </Tooltip>
          ),
        },
        {
          title: '操作',
          width: 280,
          render: (_, row) => {
            const closed = row.status === 'fixed' || row.status === 'ignored'
            const editorPath = editorLocationPath(check.data.bid_document_id, row)
            return (
              <Space wrap>
                <Button size="small" icon={<AimOutlined />} disabled={!editorPath} onClick={() => navigate(editorPath)}>
                  定位
                </Button>
                <Button
                    size="small"
                    icon={<ToolOutlined />}
                    disabled={!canWrite || closed}
                    loading={actionMutation.isPending}
                  onClick={() => actionMutation.mutate({ action: 'autofix', issueId: row.id })}
                >
                  一键修复
                </Button>
                  <Button
                    size="small"
                    disabled={!canWrite || closed}
                    loading={actionMutation.isPending}
                  onClick={() => actionMutation.mutate({ action: 'ignore', issueId: row.id })}
                >
                  忽略
                </Button>
                {row.severity === 'fail_candidate' && (
                  <Button
                      size="small"
                      danger
                      disabled={!canWrite || closed}
                      loading={actionMutation.isPending}
                    onClick={() => actionMutation.mutate({ action: 'confirm', issueId: row.id })}
                  >
                    判定废标
                  </Button>
                )}
              </Space>
            )
          },
        },
      ]}
      locale={{ emptyText: <EmptyBlock description="该分类下没有发现问题" /> }}
    />
  )

  return (
    <PageFrame
      module="合规检查"
      title={check.data.name}
      subtitle={check.data.bid_title || '未关联标书'}
      tags={['/compliance/:checkId']}
      actions={[
        canWrite ? <Button key="report" icon={<CheckCircleOutlined />} loading={reportMutation.isPending} onClick={() => reportMutation.mutate()}>
          导出报告
        </Button> : null,
      ]}
    >
      <div className="report-head">
        <div className="report-score">
          <Progress
            type="circle"
            size={108}
            percent={check.data.score}
            strokeColor={scoreColor}
            format={(value) => (
              <span className="data-mono" style={{ fontSize: 26, fontWeight: 600 }}>
                {value}
              </span>
            )}
          />
        </div>
        <div className="report-verdict">
          <p className="report-verdict-title" style={{ color: scoreColor }}>
            {verdictText[resultStatus] || severityLabels[resultStatus] || resultStatus}
          </p>
          <div className="report-meta">
            <span>检查状态 {statusTag(check.data.status)}</span>
            <span>结果 {severityTag(resultStatus)}</span>
            <span>
              完成时间 <span className="data-mono">{formatTime(check.data.completed_at)}</span>
            </span>
          </div>
        </div>
        <div className="severity-strip">
          <span className="severity-pill fail">
            <span className="data-mono">{severityCounts.fail ?? 0}</span>废标项
          </span>
          <span className="severity-pill pending">
            <span className="data-mono">{severityCounts.fail_candidate ?? 0}</span>待确认
          </span>
          <span className="severity-pill warn">
            <span className="data-mono">{severityCounts.warn ?? 0}</span>警告
          </span>
          <span className="severity-pill pass">
            <span className="data-mono">{severityCounts.pass ?? 0}</span>通过
          </span>
        </div>
      </div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        问题清单
        <Typography.Text type="secondary" style={{ fontSize: 13, marginLeft: 10 }}>
          未处理 {openIssues} 项
        </Typography.Text>
      </Typography.Title>
      <Tabs
        items={categories.map((category) => ({
          key: category,
          label: category,
          children: renderIssueTable(category === '全部' ? issueRows : issueRows.filter((issue) => issue.category === category)),
        }))}
      />
    </PageFrame>
  )
}
