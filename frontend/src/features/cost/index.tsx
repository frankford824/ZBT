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
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  createCostAdvice,
  createCostItem,
  createCostReport,
  fetchCostAnalysis,
  fetchCostItems,
  fetchCostProject,
  fetchCostProjects,
  type CostItemDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'

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
          columns={[
            {
              title: '项目',
              dataIndex: 'name',
              render: (value, row) => <Link to={`/costs/${row.id}`}>{value}</Link>,
            },
            { title: '关联项目', dataIndex: 'project_name' },
            { title: '预算(万)', dataIndex: 'total_budget', render: wan },
            { title: '实际(万)', dataIndex: 'total_actual', render: wan },
            { title: '利润率', dataIndex: 'margin_rate', render: (value) => `${Number(value || 0).toFixed(2)}%` },
            { title: '成本项', dataIndex: 'item_count' },
            { title: '状态', dataIndex: 'status', render: (value) => <Tag>{statusLabels[value] || value}</Tag> },
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
  const [open, setOpen] = useState(false)
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
    onError: () => message.error('成本项新增失败'),
  })
  const adviceMutation = useMutation({
    mutationFn: () => createCostAdvice(costProjectId),
    onSuccess: (result) => message.success(result.recommendations[0] || '建议已生成'),
    onError: () => message.error('AI 建议生成失败'),
  })
  const reportMutation = useMutation({
    mutationFn: () => createCostReport(costProjectId),
    onSuccess: (report) => message.success(report.summary || '报告已生成'),
    onError: () => message.error('报告生成失败'),
  })

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
      subtitle={project.data.project_name || project.data.project_id}
      tags={['/costs/:costProjectId']}
      actions={[
        <Button key="add" onClick={() => setOpen(true)}>
          新增成本项
        </Button>,
        <Button key="report" loading={reportMutation.isPending} onClick={() => reportMutation.mutate()}>
          导出成本报告
        </Button>,
        <Button key="ai" type="primary" loading={adviceMutation.isPending} onClick={() => adviceMutation.mutate()}>
          AI 优化建议
        </Button>,
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
                columns={[
                  { title: '分类', dataIndex: 'category' },
                  { title: '名称', dataIndex: 'name' },
                  { title: '预算(万)', dataIndex: 'budget_amount', render: wan },
                  { title: '实际(万)', dataIndex: 'actual_amount', render: wan },
                  { title: '供应商', dataIndex: 'vendor', render: (value) => value || '-' },
                  { title: '状态', dataIndex: 'status', render: (value) => <Tag>{statusLabels[value] || value}</Tag> },
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
