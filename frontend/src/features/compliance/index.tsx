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
  Checkbox,
  Col,
  Descriptions,
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
  ignoreComplianceIssue,
  updateComplianceRule,
  type ComplianceCheckDTO,
  type ComplianceIssueDTO,
  type ComplianceRuleDTO,
  type ComplianceSeverity,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'

const levelOptions = ['L1', 'L2', 'L3', 'L4'].map((value) => ({ label: value, value }))

const severityOptions: Array<{ label: string; value: ComplianceSeverity }> = [
  { label: '通过', value: 'pass' },
  { label: '警告', value: 'warn' },
  { label: '待人工确认', value: 'fail_candidate' },
  { label: '确定 fail', value: 'fail' },
]

const severityLabels: Record<string, string> = {
  pass: '通过',
  warn: '警告',
  fail_candidate: '待确认',
  fail: 'Fail',
}

const statusLabels: Record<string, string> = {
  queued: '排队中',
  running: '检查中',
  done: '完成',
  failed: '失败',
  open: '未处理',
  fixed: '已修复',
  ignored: '已忽略',
  confirmed_fail: '确认 fail',
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
    onError: () => message.error('合规检查创建失败'),
  })

  const createRuleMutation = useMutation({
    mutationFn: createComplianceRule,
    onSuccess: () => {
      setRuleOpen(false)
      ruleForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['compliance-rules'] })
      message.success('规则已创建')
    },
    onError: () => message.error('规则创建失败'),
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
    onError: () => message.error('规则更新失败'),
  })

  const deleteRuleMutation = useMutation({
    mutationFn: deleteComplianceRule,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['compliance-rules'] })
      message.success('规则已删除')
    },
    onError: () => message.error('规则删除失败'),
  })

  return (
    <PageFrame
      module="投标管控"
      title="合规检查"
      subtitle="四层规则检查、人工确认和修复闭环"
      tags={['page-compliance', '/compliance']}
      actions={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={() => checks.refetch()}>
          刷新
        </Button>,
        <Button key="rule" icon={<PlusOutlined />} onClick={() => setRuleOpen(true)}>
          新增规则
        </Button>,
        <Button key="start" type="primary" icon={<CheckCircleOutlined />} onClick={() => setCheckOpen(true)}>
          开始检查
        </Button>,
      ]}
    >
      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Statistic title="检查任务" value={checks.data?.length || 0} />
        </Col>
        <Col xs={24} md={8}>
          <Statistic title="平均得分" value={checks.data?.length ? counts.score / checks.data.length : 0} precision={1} />
        </Col>
        <Col xs={24} md={8}>
          <Statistic title="待处理风险" value={counts.fail + counts.pending} />
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
                        columns={[
                          {
                            title: '检查名称',
                            dataIndex: 'name',
                            render: (value, row) => <Link to={`/compliance/${row.id}`}>{value}</Link>,
                          },
                          { title: '标书', dataIndex: 'bid_title', render: (value) => value || '-' },
                          { title: '结果', dataIndex: 'result_status', render: severityTag },
                          { title: '得分', dataIndex: 'score', render: (value) => <Progress percent={value} size="small" /> },
                          { title: '问题数', dataIndex: 'issue_count' },
                          { title: '状态', dataIndex: 'status', render: statusTag },
                          { title: '创建时间', dataIndex: 'created_at', render: formatTime },
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
                        columns={[
                          { title: '编码', dataIndex: 'code' },
                          { title: '名称', dataIndex: 'name' },
                          { title: '分类', dataIndex: 'category' },
                          { title: '层级', dataIndex: 'level', render: (value) => <Tag>{value}</Tag> },
                          { title: '严重度', dataIndex: 'severity', render: severityTag },
                          {
                            title: '启用',
                            dataIndex: 'enabled',
                            render: (enabled, row) => (
                              <Switch
                                checked={enabled}
                                size="small"
                                onChange={(checked) => updateRuleMutation.mutate({ ...row, enabled: checked })}
                              />
                            ),
                          },
                          {
                            title: '操作',
                            render: (_, row) => (
                              <Popconfirm title="删除规则" description="确认删除该合规规则？" onConfirm={() => deleteRuleMutation.mutate(row.id)}>
                                <Button size="small" danger icon={<DeleteOutlined />}>
                                  删除
                                </Button>
                              </Popconfirm>
                            ),
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
          <Form.Item name="bid_document_id" label="关联标书 ID">
            <Input placeholder="可选，留空则执行独立检查" />
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
          <Form.Item name="code" label="规则编码" rules={[{ required: true, message: '规则编码必填' }]}>
            <Input placeholder="例如：delivery_clause" />
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
    onError: () => message.error('问题状态更新失败'),
  })

  const reportMutation = useMutation({
    mutationFn: () => createComplianceReport(checkId),
    onSuccess: (report) => message.success(report.summary || '报告已生成'),
    onError: () => message.error('报告生成失败'),
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

  const renderIssueTable = (rows: ComplianceIssueDTO[]) => (
    <Table
      rowKey="id"
      dataSource={rows}
      columns={[
        { title: '问题', dataIndex: 'title' },
        { title: '分类', dataIndex: 'category' },
        { title: '严重度', dataIndex: 'severity', render: severityTag },
        { title: '状态', dataIndex: 'status', render: statusTag },
        { title: '证据', dataIndex: 'evidence' },
        { title: '修复建议', dataIndex: 'suggestion' },
        {
          title: '操作',
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
                  disabled={closed}
                  loading={actionMutation.isPending}
                  onClick={() => actionMutation.mutate({ action: 'autofix', issueId: row.id })}
                >
                  一键修复
                </Button>
                <Button
                  size="small"
                  disabled={closed}
                  loading={actionMutation.isPending}
                  onClick={() => actionMutation.mutate({ action: 'ignore', issueId: row.id })}
                >
                  忽略
                </Button>
                {row.severity === 'fail_candidate' && (
                  <Button
                    size="small"
                    danger
                    disabled={closed}
                    loading={actionMutation.isPending}
                    onClick={() => actionMutation.mutate({ action: 'confirm', issueId: row.id })}
                  >
                    确认 fail
                  </Button>
                )}
              </Space>
            )
          },
        },
      ]}
      locale={{ emptyText: <EmptyBlock /> }}
    />
  )

  return (
    <PageFrame
      module="合规检查"
      title={check.data.name}
      subtitle={check.data.bid_title || check.data.id}
      tags={['/compliance/:checkId']}
      actions={[
        <Button key="report" icon={<CheckCircleOutlined />} loading={reportMutation.isPending} onClick={() => reportMutation.mutate()}>
          导出报告
        </Button>,
      ]}
    >
      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Statistic title="合规得分" value={check.data.score} suffix="分" />
        </Col>
        <Col xs={24} md={8}>
          <Statistic title="风险项" value={openIssues} />
        </Col>
        <Col xs={24} md={8}>
          <Statistic title="结果" value={severityLabels[check.data.result_status] || check.data.result_status} />
        </Col>
        <Col span={24}>
          <Descriptions bordered column={{ xs: 1, md: 2 }}>
            <Descriptions.Item label="检查状态">{statusTag(check.data.status)}</Descriptions.Item>
            <Descriptions.Item label="结果">{severityTag(check.data.result_status)}</Descriptions.Item>
            <Descriptions.Item label="Task ID">{check.data.task_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="完成时间">{formatTime(check.data.completed_at)}</Descriptions.Item>
          </Descriptions>
        </Col>
        <Col span={24}>
          <Typography.Title level={4}>问题清单</Typography.Title>
          <Tabs
            items={categories.map((category) => ({
              key: category,
              label: category,
              children: renderIssueTable(category === '全部' ? issueRows : issueRows.filter((issue) => issue.category === category)),
            }))}
          />
        </Col>
      </Row>
    </PageFrame>
  )
}
