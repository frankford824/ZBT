import { useQuery } from '@tanstack/react-query'
import ReactECharts from 'echarts-for-react'
import { Button, Card, Col, List, Row, Skeleton, Statistic, Table, Tag } from 'antd'
import { Link } from 'react-router-dom'
import { fetchPlatformStats } from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'

export function DashboardPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['platform-stats'],
    queryFn: fetchPlatformStats,
  })

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
      {isLoading || !data ? (
        <Skeleton active />
      ) : (
        <Row gutter={[16, 16]}>
          <Col xs={24} md={8} xl={4}>
            <Statistic title="进行中项目" value={data.activeProjects} />
          </Col>
          <Col xs={24} md={8} xl={4}>
            <Statistic title="本月标书" value={data.monthlyBids} />
          </Col>
          <Col xs={24} md={8} xl={4}>
            <Statistic title="合规通过率" value={data.compliancePassRate} suffix="%" />
          </Col>
          <Col xs={24} md={8} xl={4}>
            <Statistic title="中标率" value={data.winRate} suffix="%" />
          </Col>
          <Col xs={24} md={8} xl={4}>
            <Statistic title="待办" value={data.pendingTasks} />
          </Col>
          <Col xs={24} md={8} xl={4}>
            <Statistic title="知识库文档" value={data.knowledgeDocs} />
          </Col>
          <Col xs={24} xl={14}>
            <Card title="趋势">
              <ReactECharts
                style={{ height: 260 }}
                option={{
                  tooltip: {},
                  legend: { data: ['标书数', '中标率'] },
                  xAxis: { type: 'category', data: ['1月', '2月', '3月', '4月', '5月', '6月'] },
                  yAxis: { type: 'value' },
                  series: [
                    { name: '标书数', type: 'bar', data: [18, 24, 21, 29, 34, 31] },
                    { name: '中标率', type: 'line', data: [22, 28, 31, 29, 38, 41] },
                  ],
                }}
              />
            </Card>
          </Col>
          <Col xs={24} xl={10}>
            <Card title="AI 推荐">
              <List
                dataSource={[
                  ['市政数字孪生平台建设', '匹配度 91%', '3天后截止'],
                  ['园区智慧运维系统采购', '匹配度 86%', '7天后截止'],
                  ['交通信号控制优化服务', '匹配度 82%', '12天后截止'],
                ]}
                renderItem={(item) => (
                  <List.Item actions={[<Link key="detail" to="/tenders/tender-demo">查看</Link>]}>
                    <List.Item.Meta title={item[0]} description={item[2]} />
                    <Tag color="purple">{item[1]}</Tag>
                  </List.Item>
                )}
              />
            </Card>
          </Col>
          <Col span={24}>
            <Table
              rowKey="name"
              pagination={false}
              dataSource={[
                { name: '智慧水务平台投标', stage: '标书制作', owner: '林悦', due: '2026-06-18' },
                { name: '政务云迁移服务', stage: '合规审核', owner: '赵宁', due: '2026-06-20' },
              ]}
              columns={[
                { title: '最近项目', dataIndex: 'name' },
                { title: '阶段', dataIndex: 'stage' },
                { title: '负责人', dataIndex: 'owner' },
                { title: '截止日期', dataIndex: 'due' },
              ]}
            />
          </Col>
        </Row>
      )}
    </PageFrame>
  )
}
