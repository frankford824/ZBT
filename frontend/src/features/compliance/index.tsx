import { CheckCircleOutlined, FileSearchOutlined, ToolOutlined } from '@ant-design/icons'
import { Button, Card, Col, Progress, Row, Steps, Table, Tabs, Tag, Upload } from 'antd'
import { Link, useParams } from 'react-router-dom'
import { PageFrame } from '../../shared/components/PageFrame'

const issueRows = [
  {
    id: 'issue-1',
    category: '废标条款',
    severity: 'fail',
    evidence: '投标有效期需不少于 90 天',
    suggestion: '将投标有效期调整为 90 天以上',
  },
  {
    id: 'issue-2',
    category: '语义一致性',
    severity: 'fail_candidate',
    evidence: '服务响应时间要求 2 小时内',
    suggestion: '补充 2 小时响应承诺和保障机制',
  },
  {
    id: 'issue-3',
    category: '格式规范',
    severity: 'warn',
    evidence: '目录页缺少页码',
    suggestion: '导出前刷新目录域',
  },
]

function severityTag(severity: string) {
  const color = severity === 'fail' ? 'red' : severity === 'fail_candidate' ? 'orange' : 'gold'
  return <Tag color={color}>{severity}</Tag>
}

export function CompliancePage() {
  return (
    <PageFrame
      module="投标管控"
      title="合规检查"
      subtitle="双文件入口、四层检查和编辑器闭环"
      tags={['page-compliance', '/compliance']}
      actions={[
        <Button key="rules">规则库</Button>,
        <Button key="start" type="primary">
          开始检查
        </Button>,
      ]}
    >
      <Row gutter={[16, 16]}>
        <Col xs={24} xl={10}>
          <Card title="三步流程">
            <Steps
              direction="vertical"
              current={1}
              items={[
                { title: '文件准备', description: '上传招标文件和投标文件' },
                { title: '检查配置', description: '选择 L1-L4 检查层级' },
                { title: '检查结果', description: '证据、建议和跳转修复' },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} xl={14}>
          <Card title="文件准备">
            <Row gutter={12}>
              <Col xs={24} md={12}>
                <Upload.Dragger beforeUpload={() => false}>
                  <FileSearchOutlined />
                  <p>投标文件</p>
                </Upload.Dragger>
              </Col>
              <Col xs={24} md={12}>
                <Upload.Dragger beforeUpload={() => false}>
                  <FileSearchOutlined />
                  <p>招标文件</p>
                </Upload.Dragger>
              </Col>
            </Row>
          </Card>
        </Col>
        <Col span={24}>
          <Card title="历史检查">
            <Table
              rowKey="id"
              dataSource={[
                { id: 'check-demo', name: '智慧交通技术标合规检查', score: 86, status: '待确认' },
                { id: 'check-2', name: '政务云商务标合规检查', score: 92, status: '通过' },
              ]}
              columns={[
                {
                  title: '检查名称',
                  dataIndex: 'name',
                  render: (value, row) => <Link to={`/compliance/${row.id}`}>{value}</Link>,
                },
                { title: '得分', dataIndex: 'score', render: (value) => <Progress percent={value} size="small" /> },
                { title: '状态', dataIndex: 'status' },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </PageFrame>
  )
}

export function ComplianceDetailPage() {
  const { checkId } = useParams()
  return (
    <PageFrame
      module="合规检查"
      title="检查结果"
      subtitle={checkId}
      tags={['/compliance/:checkId']}
      actions={[
        <Button key="report" icon={<CheckCircleOutlined />}>
          导出报告
        </Button>,
      ]}
    >
      <Tabs
        items={['资质要求', '格式规范', '评分标准', '废标条款', '语义一致性'].map((label) => ({
          key: label,
          label,
          children: (
            <Table
              rowKey="id"
              dataSource={issueRows}
              columns={[
                { title: '分类', dataIndex: 'category' },
                { title: '级别', dataIndex: 'severity', render: severityTag },
                { title: '证据', dataIndex: 'evidence' },
                { title: '修复建议', dataIndex: 'suggestion' },
                {
                  title: '操作',
                  render: () => (
                    <Button size="small" icon={<ToolOutlined />}>
                      跳转修复
                    </Button>
                  ),
                },
              ]}
            />
          ),
        }))}
      />
    </PageFrame>
  )
}
