import { useQuery } from '@tanstack/react-query'
import ReactECharts from 'echarts-for-react'
import { Button, Card, Col, List, Row, Skeleton, Space, Statistic, Table, Tag } from 'antd'
import { Link } from 'react-router-dom'
import { fetchPlatformSummary } from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { ErrorBlock } from '../../shared/components/StateBlocks'

const statusLabels: Record<string, string> = {
  opportunity: '商机评估',
  bidding: '标书制作',
  compliance_review: '合规审核',
  submitted: '投标中',
  closed: '已结果',
}

function formatDate(value?: string | null) {
  if (!value) return '-'
  return value.length <= 10 ? value : new Date(value).toLocaleString()
}

export function DashboardPage() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['dashboard-summary'],
    queryFn: fetchPlatformSummary,
  })

  const stats = data?.stats

  const statItems = stats
    ? [
        { title: '进行中项目', value: stats.active_projects },
        { title: '本月标书', value: stats.monthly_bids },
        { title: '合规通过率', value: stats.compliance_pass_rate, suffix: '%' },
        { title: '中标率', value: stats.win_rate, suffix: '%' },
        { title: '待办', value: stats.pending_tasks },
        { title: '知识库文档', value: stats.knowledge_docs },
      ]
    : []

  return (
    <PageFrame
      module="概览"
      title="工作台"
      subtitle="标讯、项目、审批和知识资产集中视图"
      tags={['page-dashboard', '/dashboard']}
      actions={[
        <Button key="bid" type="primary">
          <Link to="/bids/new">新建标书</Link>
        </Button>,
        <Button key="project">
          <Link to="/projects">新建项目</Link>
        </Button>,
      ]}
    >
      {isLoading ? (
        <Skeleton active />
      ) : isError || !data || !stats ? (
        <ErrorBlock />
      ) : (
        <Row gutter={[16, 16]}>
          {statItems.map((item) => (
            <Col key={item.title} xs={12} md={8} xl={4}>
              <Card size="small" className="stat-card">
                <Statistic title={item.title} value={item.value} suffix={item.suffix} />
              </Card>
            </Col>
          ))}
          <Col xs={24} xl={14}>
            <Card title="投标趋势">
              <ReactECharts
                style={{ height: 260 }}
                option={{
                  color: ['#2C5FA8', '#B08530'],
                  tooltip: {},
                  legend: { top: 0, left: 'center', data: ['标书数', '中标率'] },
                  grid: { left: 40, right: 24, top: 40, bottom: 28 },
                  xAxis: {
                    type: 'category',
                    data: data.trends.map((item) => item.month),
                    axisLine: { lineStyle: { color: '#DDDBD2' } },
                    axisLabel: { color: '#5B616E' },
                  },
                  yAxis: {
                    type: 'value',
                    splitLine: { lineStyle: { color: '#ECEAE4' } },
                    axisLabel: { color: '#5B616E' },
                  },
                  series: [
                    {
                      name: '标书数',
                      type: 'bar',
                      barWidth: 18,
                      itemStyle: { borderRadius: [4, 4, 0, 0] },
                      data: data.trends.map((item) => item.bids),
                    },
                    {
                      name: '中标率',
                      type: 'line',
                      smooth: true,
                      data: data.trends.map((item) => item.win_rate),
                    },
                  ],
                }}
              />
            </Card>
          </Col>
          <Col xs={24} xl={10}>
            <Card
              className="ai-card"
              title={
                <Space>
                  <span className="ai-chip">智能</span>
                  推荐标讯
                </Space>
              }
            >
              <List
                dataSource={data.recommended_tenders}
                locale={{ emptyText: '暂无匹配标讯，可到标讯大厅订阅来源' }}
                renderItem={(item) => (
                  <List.Item actions={[<Link key="detail" to={`/tenders/${item.id}`}>查看</Link>]}>
                    <List.Item.Meta
                      title={item.title}
                      description={`${item.region || item.purchaser || '-'} · 截止 ${formatDate(item.deadline)}`}
                    />
                    <Tag color="purple" className="data-mono">
                      匹配 {item.match_score}%
                    </Tag>
                  </List.Item>
                )}
              />
            </Card>
          </Col>
          <Col xs={24} xl={10}>
            <Card title="待我审批">
              <List
                dataSource={data.pending_approvals}
                locale={{ emptyText: '暂无待审批事项' }}
                renderItem={(item) => (
                  <List.Item actions={[<Link key="team" to="/team?tab=approvals">处理</Link>]}>
                    <List.Item.Meta
                      title={item.title}
                      description={`${item.bid_title || '-'} · 第 ${item.current_step} 级`}
                    />
                  </List.Item>
                )}
              />
            </Card>
          </Col>
          <Col xs={24} xl={14}>
            <Card title="通知">
              <List
                dataSource={data.notifications}
                locale={{ emptyText: '暂无通知' }}
                renderItem={(item) => (
                  <List.Item>
                    <List.Item.Meta
                      title={
                        <Space>
                          <Tag color={item.read_at ? 'default' : 'blue'}>
                            {item.read_at ? '已读' : '未读'}
                          </Tag>
                          {item.title}
                        </Space>
                      }
                      description={`${item.body} · ${formatDate(item.created_at)}`}
                    />
                  </List.Item>
                )}
              />
            </Card>
          </Col>
          <Col span={24}>
            <Table
              rowKey="id"
              pagination={false}
              dataSource={data.recent_projects}
              columns={[
                {
                  title: '最近项目',
                  dataIndex: 'name',
                  render: (value, row) => <Link to={`/projects/${row.id}`}>{value}</Link>,
                },
                {
                  title: '阶段',
                  dataIndex: 'status',
                  render: (value) => <Tag>{statusLabels[value] || value}</Tag>,
                },
                { title: '负责人', dataIndex: 'owner_name', render: (value) => value || '-' },
                {
                  title: '下一节点',
                  dataIndex: 'due_date',
                  className: 'data-mono',
                  render: formatDate,
                },
                {
                  title: '更新时间',
                  dataIndex: 'updated_at',
                  className: 'data-mono',
                  render: formatDate,
                },
              ]}
            />
          </Col>
        </Row>
      )}
    </PageFrame>
  )
}
