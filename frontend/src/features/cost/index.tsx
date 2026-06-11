import ReactECharts from 'echarts-for-react'
import { Button, Card, Col, Descriptions, Row, Statistic, Table, Tag } from 'antd'
import { Link, useParams } from 'react-router-dom'
import { PageFrame } from '../../shared/components/PageFrame'

const costRows = [
  { id: 'cost-demo', project: '智慧交通综合治理', revenue: 1280, cost: 860, margin: 32.8, status: '进行中' },
  { id: 'cost-2', project: '政务云迁移服务', revenue: 860, cost: 590, margin: 31.4, status: '已结项' },
]

export function CostsPage() {
  return (
    <PageFrame
      module="企业管理"
      title="成本管理"
      subtitle="项目成本列表、预算实际对比和利润率"
      tags={['page-cost', '/costs']}
    >
      <Table
        rowKey="id"
        dataSource={costRows}
        columns={[
          {
            title: '项目',
            dataIndex: 'project',
            render: (value, row) => <Link to={`/costs/${row.id}`}>{value}</Link>,
          },
          { title: '中标金额(万)', dataIndex: 'revenue' },
          { title: '实际成本(万)', dataIndex: 'cost' },
          { title: '利润率', dataIndex: 'margin', render: (value) => `${value}%` },
          { title: '状态', dataIndex: 'status', render: (value) => <Tag>{value}</Tag> },
        ]}
      />
    </PageFrame>
  )
}

export function CostDetailPage() {
  const { costProjectId } = useParams()
  return (
    <PageFrame
      module="成本管理"
      title="单项目成本分析"
      subtitle={costProjectId}
      tags={['/costs/:costProjectId']}
      actions={[<Button key="report">导出成本报告</Button>, <Button key="ai">AI 优化建议</Button>]}
    >
      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Statistic title="中标金额" value={1280} suffix="万" />
        </Col>
        <Col xs={24} md={8}>
          <Statistic title="实际成本" value={860} suffix="万" />
        </Col>
        <Col xs={24} md={8}>
          <Statistic title="利润率" value={32.8} suffix="%" />
        </Col>
        <Col xs={24} xl={12}>
          <Card title="成本构成">
            <ReactECharts
              style={{ height: 260 }}
              option={{
                tooltip: { trigger: 'item' },
                series: [
                  {
                    type: 'pie',
                    radius: '65%',
                    data: [
                      { name: '人力', value: 360 },
                      { name: '材料', value: 210 },
                      { name: '设备', value: 160 },
                      { name: '其他', value: 130 },
                    ],
                  },
                ],
              }}
            />
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Descriptions bordered column={1}>
            <Descriptions.Item label="预算 vs 实际">预算 920 万，实际 860 万，节约 60 万</Descriptions.Item>
            <Descriptions.Item label="行业对比">利润率高于行业均值 4.2%</Descriptions.Item>
            <Descriptions.Item label="建议">人力成本占比偏高，建议复用历史交付资产。</Descriptions.Item>
          </Descriptions>
        </Col>
      </Row>
    </PageFrame>
  )
}
