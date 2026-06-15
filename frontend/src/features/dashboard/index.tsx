import { useQuery } from '@tanstack/react-query'
import ReactECharts from 'echarts-for-react'
import { Button, Card, Col, List, Row, Skeleton, Space, Statistic, Table, Tag } from 'antd'
import { Link } from 'react-router-dom'
import { fetchPlatformSummary } from '../../shared/api/client'
import { IconfontGlyph, type IconfontGlyphName } from '../../shared/components/IconfontGlyph'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock } from '../../shared/components/StateBlocks'
import { useCanAccess } from '../../shared/permissions/permissions'

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
  const canCreateBid = useCanAccess('bid', 'full')
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['dashboard-summary'],
    queryFn: fetchPlatformSummary,
  })

  const stats = data?.stats

  const statItems: Array<{ title: string; value: number; suffix?: string; icon: IconfontGlyphName }> = stats
    ? [
        { title: '进行中项目', value: stats.active_projects, icon: 'opportunity' },
        { title: '本月标书', value: stats.monthly_bids, icon: 'upload' },
        { title: '合规通过率', value: stats.compliance_pass_rate, suffix: '%', icon: 'target' },
        { title: '中标率', value: stats.win_rate, suffix: '%', icon: 'data' },
        { title: '待办', value: stats.pending_tasks, icon: 'time' },
        { title: '知识库文档', value: stats.knowledge_docs, icon: 'mail' },
      ]
    : []

  return (
    <PageFrame
      module="概览"
      title="工作台"
      subtitle="标讯、项目、审批和知识资产集中视图"
      tags={['page-dashboard', '/dashboard']}
      bare
      actions={[
        canCreateBid ? <Button key="bid" type="primary">
          <Link to="/bids/new">新建标书</Link>
        </Button> : null,
        <Button key="project">
          <Link to="/projects">项目管理</Link>
        </Button>,
      ]}
    >
      {isLoading ? (
        <Skeleton active />
      ) : isError || !data || !stats ? (
        <ErrorBlock onRetry={() => void refetch()} />
      ) : (
        <Row gutter={[16, 16]}>
          {statItems.map((item) => (
            <Col key={item.title} xs={12} md={8} xl={4}>
              <Card size="small" className="stat-card">
                <div className="stat-card-shell">
                  <Statistic title={item.title} value={item.value} suffix={item.suffix} />
                  <span className="stat-card-glyph">
                    <IconfontGlyph name={item.icon} />
                  </span>
                </div>
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
                locale={{ emptyText: <EmptyBlock description="暂无匹配标讯，可到标讯大厅订阅来源" /> }}
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
                locale={{ emptyText: <EmptyBlock description="暂无待审批事项" /> }}
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
                locale={{ emptyText: <EmptyBlock description="暂无通知" /> }}
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
            <Card title="最近项目" className="compact-projects">
              <List
                className="recent-project-list"
                dataSource={data.recent_projects}
                locale={{ emptyText: <EmptyBlock description="暂无最近项目" /> }}
                renderItem={(item) => (
                  <List.Item>
                    <List.Item.Meta
                      title={<Link to={`/projects/${item.id}`}>{item.name}</Link>}
                      description={
                        <Space size={[6, 6]} wrap className="recent-project-meta">
                          <Tag>{statusLabels[item.status] || item.status}</Tag>
                          <span>负责人 {item.owner_name || '-'}</span>
                          <span>下一节点 {formatDate(item.due_date)}</span>
                        </Space>
                      }
                    />
                  </List.Item>
                )}
              />
            </Card>
            <div className="desktop-projects">
              <Table
                rowKey="id"
                pagination={false}
                scroll={{ x: 760 }}
                dataSource={data.recent_projects}
                columns={[
                  {
                    title: '最近项目',
                    dataIndex: 'name',
                    width: 240,
                    render: (value, row) => <Link to={`/projects/${row.id}`}>{value}</Link>,
                  },
                  {
                    title: '阶段',
                    dataIndex: 'status',
                    width: 110,
                    render: (value) => <Tag>{statusLabels[value] || value}</Tag>,
                  },
                  { title: '负责人', dataIndex: 'owner_name', width: 96, render: (value) => value || '-' },
                  {
                    title: '下一节点',
                    dataIndex: 'due_date',
                    className: 'data-mono',
                    width: 120,
                    render: formatDate,
                  },
                  {
                    title: '更新时间',
                    dataIndex: 'updated_at',
                    className: 'data-mono',
                    width: 180,
                    render: formatDate,
                  },
                ]}
              />
            </div>
          </Col>
        </Row>
      )}
    </PageFrame>
  )
}
