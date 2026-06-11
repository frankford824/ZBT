import { Button, Card, Col, Descriptions, Form, Input, Row, Select, Space, Table, Tabs, Tag } from 'antd'
import { Link, useParams } from 'react-router-dom'
import { PageFrame } from '../../shared/components/PageFrame'

const tenderRows = [
  {
    id: 'tender-demo',
    title: '某市智慧交通综合治理平台建设项目',
    region: '浙江',
    budget: '1,280万',
    match: 91,
    deadline: '2026-06-18',
  },
  {
    id: 'tender-2',
    title: '园区能耗监测与运维服务采购',
    region: '江苏',
    budget: '640万',
    match: 86,
    deadline: '2026-06-22',
  },
]

export function TendersPage() {
  return (
    <PageFrame
      module="投标准备"
      title="标讯大厅"
      subtitle="标讯搜索、AI 推荐、收藏和数据源配置"
      tags={['page-tender', '/tenders']}
      actions={[
        <Button key="source">新增数据源</Button>,
        <Button key="verify" type="primary">
          URL 可达性检测
        </Button>,
      ]}
    >
      <Tabs
        items={['全部', 'AI推荐', '监控', '收藏', '监控设置'].map((label) => ({
          key: label,
          label,
          children:
            label === '监控设置' ? (
              <Row gutter={16}>
                <Col xs={24} lg={10}>
                  <Card title="数据源配置">
                    <Form layout="vertical">
                      <Form.Item label="平台名称">
                        <Input placeholder="中国招标投标公共服务平台" />
                      </Form.Item>
                      <Form.Item label="平台 URL">
                        <Input placeholder="https://www.example.com" />
                      </Form.Item>
                      <Form.Item label="平台类型">
                        <Select options={['政府采购', '建设工程', '产权交易', '其他'].map((value) => ({ value }))} />
                      </Form.Item>
                    </Form>
                  </Card>
                </Col>
                <Col xs={24} lg={14}>
                  <Card title="关键词监控">
                    <Space wrap>
                      {['智慧城市', '系统集成', '运维服务', '交通治理'].map((word) => (
                        <Tag key={word}>{word}</Tag>
                      ))}
                    </Space>
                  </Card>
                </Col>
              </Row>
            ) : (
              <Table
                rowKey="id"
                dataSource={tenderRows}
                columns={[
                  {
                    title: '标讯名称',
                    dataIndex: 'title',
                    render: (value, row) => <Link to={`/tenders/${row.id}`}>{value}</Link>,
                  },
                  { title: '地区', dataIndex: 'region' },
                  { title: '预算', dataIndex: 'budget' },
                  {
                    title: '匹配度',
                    dataIndex: 'match',
                    render: (value) => <Tag color="purple">{value}%</Tag>,
                  },
                  { title: '截止日期', dataIndex: 'deadline' },
                ]}
              />
            ),
        }))}
      />
    </PageFrame>
  )
}

export function TenderDetailPage() {
  const { tenderId } = useParams()
  return (
    <PageFrame
      module="标讯大厅"
      title="标讯详情"
      subtitle={tenderId}
      tags={['/tenders/:tenderId']}
      actions={[
        <Button key="project">创建项目</Button>,
        <Button key="bid" type="primary">
          <Link to="/bids/new">生成标书</Link>
        </Button>,
      ]}
    >
      <Descriptions bordered column={2}>
        <Descriptions.Item label="招标单位">某市交通运输局</Descriptions.Item>
        <Descriptions.Item label="预算金额">1,280万</Descriptions.Item>
        <Descriptions.Item label="投标截止">2026-06-18</Descriptions.Item>
        <Descriptions.Item label="匹配度">91%</Descriptions.Item>
        <Descriptions.Item label="关键要求" span={2}>
          要求具备信息系统建设、数据治理和平台运维相关业绩，技术方案需覆盖数据安全、实施计划和售后服务。
        </Descriptions.Item>
        <Descriptions.Item label="废标条款" span={2}>
          缺少法定代表人签章、投标有效期不足、报价大小写不一致。
        </Descriptions.Item>
      </Descriptions>
    </PageFrame>
  )
}
