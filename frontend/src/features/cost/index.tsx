import ReactECharts from 'echarts-for-react'
import {
  App as AntApp,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  createCostAdvice,
  createCostItem,
  createCostReport,
  fetchAITask,
  fetchCostAnalysis,
  fetchCostItems,
  fetchCostProject,
  fetchCostProjects,
  getApiErrorMessage,
  type CostItemDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { useCanAccess } from '../../shared/permissions/permissions'

const statusLabels: Record<string, string> = {
  draft: '草稿',
  active: '进行中',
  closed: '已结项',
  planned: '计划',
  committed: '已承诺',
  actual: '已发生',
}

const costTypeOptions = [
  { value: 'labor', label: '人力' },
  { value: 'material', label: '材料' },
  { value: 'equipment', label: '设备' },
  { value: 'service', label: '服务' },
  { value: 'other', label: '其他' },
]

function wan(value?: number | null) {
  return Number(((value || 0) / 10000).toFixed(2))
}

export function CostsPage() {
  const projects = useQuery({
    queryKey: ['cost-projects'],
    queryFn: fetchCostProjects,
  })

  return (
    <PageFrame
      module="企业管理"
      title="成本管理"
      subtitle="项目成本列表、预算实际对比和利润率"
      tags={['page-cost', '/costs']}
    >
      {projects.isLoading && <LoadingBlock />}
      {projects.isError && <ErrorBlock />}
      {!projects.isLoading && !projects.isError && !projects.data?.length && <EmptyBlock />}
      {!projects.isLoading && !projects.isError && Boolean(projects.data?.length) && (
        <Table
          rowKey="id"
          dataSource={projects.data}
          scroll={{ x: 900 }}
          columns={[
            {
              title: '项目',
              dataIndex: 'name',
              width: 220,
              render: (value, row) => <Link to={`/costs/${row.id}`}>{value}</Link>,
            },
            { title: '关联项目', dataIndex: 'project_name', width: 220 },
            {
              title: '预算(万)',
              dataIndex: 'total_budget',
              align: 'right',
              width: 120,
              render: (value) => <span className="data-mono">{wan(value)}</span>,
            },
            {
              title: '实际(万)',
              dataIndex: 'total_actual',
              align: 'right',
              width: 120,
              render: (value) => <span className="data-mono">{wan(value)}</span>,
            },
            {
              title: '利润率',
              dataIndex: 'margin_rate',
              align: 'right',
              width: 110,
              render: (value) => {
                const rate = Number(value || 0)
                const color = rate < 0 ? '#D43030' : rate > 0 ? '#2F9E63' : undefined
                return <span className="data-mono" style={{ color }}>{`${rate.toFixed(2)}%`}</span>
              },
            },
            { title: '成本项', dataIndex: 'item_count', align: 'right', width: 90 },
            { title: '状态', dataIndex: 'status', width: 90, render: (value) => <Tag>{statusLabels[value] || value}</Tag> },
          ]}
        />
      )}
    </PageFrame>
  )
}

export function CostDetailPage() {
  const { costProjectId = '' } = useParams()
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canWrite = useCanAccess('cost', 'full')
  const [open, setOpen] = useState(false)
  const [adviceTaskId, setAdviceTaskId] = useState('')
  const [form] = Form.useForm()
  const project = useQuery({
    queryKey: ['cost-project', costProjectId],
    queryFn: () => fetchCostProject(costProjectId),
    enabled: Boolean(costProjectId),
  })
  const items = useQuery({
    queryKey: ['cost-items', costProjectId],
    queryFn: () => fetchCostItems(costProjectId),
    enabled: Boolean(costProjectId),
  })
  const analysis = useQuery({
    queryKey: ['cost-analysis', costProjectId],
    queryFn: () => fetchCostAnalysis(costProjectId),
    enabled: Boolean(costProjectId),
  })
  const adviceTask = useQuery({
    queryKey: ['ai-task', adviceTaskId],
    queryFn: () => fetchAITask(adviceTaskId),
    enabled: Boolean(adviceTaskId),
    refetchInterval: (query) => {
      const task = query.state.data
      if (!task) return 1500
      return task.status === 'queued' || task.status === 'running' ? 1500 : false
    },
  })
  const itemMutation = useMutation({
    mutationFn: (payload: Partial<CostItemDTO> & { name: string }) => createCostItem(costProjectId, payload),
    onSuccess: () => {
      setOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['cost-items', costProjectId] })
      queryClient.invalidateQueries({ queryKey: ['cost-project', costProjectId] })
      queryClient.invalidateQueries({ queryKey: ['cost-analysis', costProjectId] })
      message.success('成本项已新增')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '成本项新增失败')),
  })
  const adviceMutation = useMutation({
    mutationFn: () => createCostAdvice(costProjectId),
    onSuccess: (task) => {
      setAdviceTaskId(task.id)
      message.success('已开始生成建议')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '生成建议失败')),
  })
  const reportMutation = useMutation({
    mutationFn: () => createCostReport(costProjectId),
    onSuccess: (report) => message.success(report.summary || '报告已生成'),
    onError: (error) => message.error(getApiErrorMessage(error, '报告生成失败')),
  })
  const adviceTaskStatus = adviceTask.data?.status
  useEffect(() => {
    if (!adviceTaskId || !adviceTaskStatus) return
    if (adviceTaskStatus === 'done') {
      message.success(adviceSummary(adviceTask.data?.result) || '建议已生成')
      void queryClient.invalidateQueries({ queryKey: ['cost-project', costProjectId] })
    }
    if (adviceTaskStatus === 'failed' || adviceTaskStatus === 'cancelled') {
      message.error('生成建议失败')
    }
  }, [adviceTaskId, adviceTaskStatus, adviceTask.data?.result, message, queryClient, costProjectId])
  const adviceBusy = adviceMutation.isPending || adviceTaskStatus === 'queued' || adviceTaskStatus === 'running'

  if (project.isLoading) return <LoadingBlock />
  if (project.isError) return <ErrorBlock />
  if (!project.data) return <EmptyBlock />

  const chartData =
    analysis.data?.category_totals.map((item) => ({
      name: item.category,
      value: wan(item.total_actual),
    })) || []

  return (
    <PageFrame
      module="成本管理"
      title={project.data.name}
      subtitle={project.data.project_name || '未关联项目'}
      tags={['/costs/:costProjectId']}
      bare
      actions={[
        canWrite ? <Button key="add" onClick={() => setOpen(true)}>
          新增成本项
        </Button> : null,
        canWrite ? <Button key="report" loading={reportMutation.isPending} onClick={() => reportMutation.mutate()}>
          导出成本报告
        </Button> : null,
        canWrite ? <Button key="ai" type="primary" loading={adviceBusy} onClick={() => adviceMutation.mutate()}>
          智能优化建议
        </Button> : null,
      ]}
    >
      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Statistic title="预算金额" value={wan(project.data.total_budget)} suffix="万" precision={2} />
        </Col>
        <Col xs={24} md={8}>
          <Statistic title="实际成本" value={wan(project.data.total_actual)} suffix="万" precision={2} />
        </Col>
        <Col xs={24} md={8}>
          <Statistic title="利润率" value={project.data.margin_rate || 0} suffix="%" precision={2} />
        </Col>
        <Col xs={24} xl={12}>
          <Card title="成本构成">
            {analysis.isLoading && <LoadingBlock />}
            {analysis.isError && <ErrorBlock />}
            {!analysis.isLoading && !analysis.isError && (
              <ReactECharts
                style={{ height: 260 }}
                option={{
                  tooltip: { trigger: 'item' },
                  series: [
                    {
                      type: 'pie',
                      radius: '65%',
                      data: chartData,
                    },
                  ],
                }}
              />
            )}
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title="成本分析">
            <Descriptions bordered column={1}>
              <Descriptions.Item label="预算 vs 实际">
                预算 {wan(project.data.total_budget)} 万，实际 {wan(project.data.total_actual)} 万，差额 {wan(project.data.margin_amount)} 万
              </Descriptions.Item>
              <Descriptions.Item label="超预算项">{analysis.data?.overrun_items.length || 0} 项</Descriptions.Item>
              <Descriptions.Item label="建议">
                <Space direction="vertical">
                  {analysis.data?.recommendations.map((item) => (
                    <span key={item}>{item}</span>
                  ))}
                </Space>
              </Descriptions.Item>
              {adviceTask.data ? (
                <Descriptions.Item label="智能建议">
                  <Space direction="vertical">
                    {adviceStatusTag(adviceTask.data.status)}
                    {adviceRecommendations(adviceTask.data.result).map((item) => (
                      <span key={item}>{item}</span>
                    ))}
                    {adviceRiskFlags(adviceTask.data.result).map((item) => (
                      <Tag color="orange" key={item}>
                        {item}
                      </Tag>
                    ))}
                  </Space>
                </Descriptions.Item>
              ) : null}
            </Descriptions>
          </Card>
        </Col>
        <Col xs={24}>
          <Card title="成本明细">
            {items.isLoading && <LoadingBlock />}
            {items.isError && <ErrorBlock />}
            {!items.isLoading && !items.isError && !items.data?.length && <EmptyBlock />}
            {!items.isLoading && !items.isError && Boolean(items.data?.length) && (
              <Table
                rowKey="id"
                dataSource={items.data}
                scroll={{ x: 820 }}
                columns={[
                  { title: '分类', dataIndex: 'category', width: 120 },
                  { title: '名称', dataIndex: 'name', width: 220 },
                  {
                    title: '预算(万)',
                    dataIndex: 'budget_amount',
                    align: 'right',
                    width: 120,
                    render: (value) => <span className="data-mono">{wan(value)}</span>,
                  },
                  {
                    title: '实际(万)',
                    dataIndex: 'actual_amount',
                    align: 'right',
                    width: 120,
                    render: (value) => <span className="data-mono">{wan(value)}</span>,
                  },
                  { title: '供应商', dataIndex: 'vendor', width: 160, render: (value) => value || '-' },
                  { title: '状态', dataIndex: 'status', width: 100, render: (value) => <Tag>{statusLabels[value] || value}</Tag> },
                ]}
              />
            )}
          </Card>
        </Col>
      </Row>
      <Modal open={open} title="新增成本项" onCancel={() => setOpen(false)} onOk={form.submit} confirmLoading={itemMutation.isPending}>
        <Form
          form={form}
          layout="vertical"
          initialValues={{ category: '其他', cost_type: 'other', status: 'planned', budget_amount: 0, actual_amount: 0 }}
          onFinish={itemMutation.mutate}
        >
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '名称必填' }]}>
            <Input placeholder="例如：实施顾问投入" />
          </Form.Item>
          <Form.Item name="category" label="分类">
            <Input placeholder="人力 / 材料 / 设备 / 服务 / 其他" />
          </Form.Item>
          <Form.Item name="cost_type" label="类型">
            <Select options={costTypeOptions} />
          </Form.Item>
          <Form.Item name="budget_amount" label="预算金额">
            <InputNumber min={0} className="full-width" />
          </Form.Item>
          <Form.Item name="actual_amount" label="实际金额">
            <InputNumber min={0} className="full-width" />
          </Form.Item>
          <Form.Item name="status" label="状态">
            <Select
              options={[
                { value: 'planned', label: '计划' },
                { value: 'committed', label: '已承诺' },
                { value: 'actual', label: '已发生' },
              ]}
            />
          </Form.Item>
          <Form.Item name="vendor" label="供应商">
            <Input />
          </Form.Item>
          <Form.Item name="note" label="备注">
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  )
}

function adviceRecommendations(result: Record<string, unknown> | undefined) {
  if (!result || !Array.isArray(result.recommendations)) return []
  return result.recommendations.filter((item): item is string => typeof item === 'string')
}

function adviceRiskFlags(result: Record<string, unknown> | undefined) {
  if (!result || !Array.isArray(result.risk_flags)) return []
  return result.risk_flags.filter((item): item is string => typeof item === 'string')
}

function adviceSummary(result: Record<string, unknown> | undefined) {
  if (!result || typeof result.summary !== 'string') return ''
  return result.summary
}

function adviceStatusTag(status: string) {
  const labels: Record<string, string> = {
    queued: '等待生成',
    running: '生成中',
    done: '已完成',
    failed: '生成失败',
    cancelled: '已取消',
  }
  const color = status === 'done' ? 'green' : status === 'failed' || status === 'cancelled' ? 'red' : 'blue'
  return <Tag color={color}>{labels[status] || '处理中'}</Tag>
}
