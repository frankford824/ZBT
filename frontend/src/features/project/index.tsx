import { Button, Card, Col, Descriptions, Modal, Row, Segmented, Space, Table, Tag, Timeline } from 'antd'
import { useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { PageFrame } from '../../shared/components/PageFrame'

const statuses = ['opportunity', 'bidding', 'compliance_review', 'submitted', 'closed']
const statusLabels: Record<string, string> = {
  opportunity: '商机评估',
  bidding: '标书制作',
  compliance_review: '合规审核',
  submitted: '投标中',
  closed: '已结果',
}

const projectRows = [
  { id: 'project-demo', name: '智慧交通综合治理', status: 'bidding', owner: '林悦', amount: '1,280万' },
  { id: 'project-2', name: '政务云迁移服务', status: 'compliance_review', owner: '赵宁', amount: '860万' },
  { id: 'project-3', name: '园区能耗监测', status: 'submitted', owner: '周源', amount: '640万' },
]

export function ProjectsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const view = searchParams.get('view') || 'board'

  return (
    <PageFrame
      module="投标管控"
      title="项目管理"
      subtitle="看板、列表、里程碑、成员和状态机"
      tags={['page-project', '/projects?view=board|list']}
      actions={[
        <Segmented
          key="view"
          value={view}
          options={[
            { label: '看板', value: 'board' },
            { label: '列表', value: 'list' },
          ]}
          onChange={(value) => setSearchParams({ view: String(value) })}
        />,
      ]}
    >
      {view === 'list' ? (
        <Table
          rowKey="id"
          dataSource={projectRows}
          columns={[
            {
              title: '项目名称',
              dataIndex: 'name',
              render: (value, row) => <Link to={`/projects/${row.id}`}>{value}</Link>,
            },
            { title: '状态', dataIndex: 'status', render: (value) => statusLabels[value] },
            { title: '负责人', dataIndex: 'owner' },
            { title: '金额', dataIndex: 'amount' },
          ]}
        />
      ) : (
        <Row gutter={12} className="kanban-row">
          {statuses.map((status) => (
            <Col xs={24} md={12} xl={5} key={status}>
              <Card title={statusLabels[status]} size="small">
                <Space direction="vertical" className="full-width">
                  {projectRows
                    .filter((project) => project.status === status)
                    .map((project) => (
                      <Card key={project.id} size="small" className="project-card">
                        <Link to={`/projects/${project.id}`}>{project.name}</Link>
                        <div>
                          <Tag color="blue">{project.owner}</Tag>
                          <Tag>{project.amount}</Tag>
                        </div>
                      </Card>
                    ))}
                </Space>
              </Card>
            </Col>
          ))}
        </Row>
      )}
    </PageFrame>
  )
}

export function ProjectDetailPage() {
  const { projectId } = useParams()
  const [open, setOpen] = useState(false)
  return (
    <PageFrame
      module="项目管理"
      title="项目详情"
      subtitle={projectId}
      tags={['/projects/:projectId']}
      actions={[
        <Button key="milestone" onClick={() => setOpen(true)}>
          编辑里程碑
        </Button>,
        <Button key="cost" type="primary">
          <Link to="/costs/cost-demo">创建成本项目</Link>
        </Button>,
      ]}
    >
      <Descriptions bordered column={2}>
        <Descriptions.Item label="项目名称">智慧交通综合治理</Descriptions.Item>
        <Descriptions.Item label="状态">标书制作</Descriptions.Item>
        <Descriptions.Item label="负责人">林悦</Descriptions.Item>
        <Descriptions.Item label="中标结果">pending</Descriptions.Item>
      </Descriptions>
      <Row gutter={16} className="section-row">
        <Col xs={24} xl={12}>
          <Card title="里程碑">
            <Timeline
              items={[
                { color: 'green', children: '项目创建 2026-06-01' },
                { color: 'blue', children: '标书制作 2026-06-10' },
                { color: 'gray', children: '提交投标 2026-06-18' },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title="关联标书">
            <Table
              rowKey="name"
              pagination={false}
              dataSource={[{ name: '智慧交通技术标', status: '编制中' }]}
              columns={[
                { title: '标书', dataIndex: 'name' },
                { title: '状态', dataIndex: 'status' },
              ]}
            />
          </Card>
        </Col>
      </Row>
      <Modal open={open} title="编辑里程碑" onCancel={() => setOpen(false)} onOk={() => setOpen(false)}>
        <p>状态、日期、备注字段将在 Loop-9 接入后端。</p>
      </Modal>
    </PageFrame>
  )
}
