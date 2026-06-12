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
  Segmented,
  Select,
  Space,
  Table,
  Tag,
  Timeline,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  archiveProjectCase,
  createCostProject,
  createProject,
  createProjectMilestone,
  fetchProject,
  fetchProjectActivities,
  fetchProjectMilestones,
  fetchProjects,
  transitionProject,
  type ProjectDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'

const statuses: ProjectDTO['status'][] = ['opportunity', 'bidding', 'compliance_review', 'submitted', 'closed']
const statusLabels: Record<ProjectDTO['status'], string> = {
  opportunity: '商机评估',
  bidding: '标书制作',
  compliance_review: '合规审核',
  submitted: '投标中',
  closed: '已结果',
}
const resultLabels: Record<string, string> = {
  won: '中标',
  lost: '未中标',
  pending: '待确认',
}
const nextStatus: Partial<Record<ProjectDTO['status'], ProjectDTO['status']>> = {
  opportunity: 'bidding',
  bidding: 'compliance_review',
  compliance_review: 'submitted',
  submitted: 'closed',
}

function dateText(value?: string | null) {
  return value ? value.slice(0, 10) : '-'
}

export function ProjectsPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()
  const view = searchParams.get('view') || 'board'
  const projects = useQuery({
    queryKey: ['projects'],
    queryFn: () => fetchProjects(),
  })
  const createMutation = useMutation({
    mutationFn: createProject,
    onSuccess: () => {
      setOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      message.success('项目已创建')
    },
    onError: () => message.error('项目创建失败'),
  })

  const table = () => {
    if (projects.isLoading) return <LoadingBlock />
    if (projects.isError) return <ErrorBlock />
    if (!projects.data?.length) return <EmptyBlock />
    return (
      <Table
        rowKey="id"
        dataSource={projects.data}
        columns={[
          {
            title: '项目名称',
            dataIndex: 'name',
            render: (value, row) => <Link to={`/projects/${row.id}`}>{value}</Link>,
          },
          { title: '状态', dataIndex: 'status', render: (value: ProjectDTO['status']) => statusLabels[value] },
          { title: '负责人', dataIndex: 'owner_name', render: (value) => value || '-' },
          { title: '标书数', dataIndex: 'bid_count' },
          { title: '里程碑', dataIndex: 'milestone_count' },
          { title: '更新时间', dataIndex: 'updated_at', render: dateText },
        ]}
      />
    )
  }

  const board = () => {
    if (projects.isLoading) return <LoadingBlock />
    if (projects.isError) return <ErrorBlock />
    if (!projects.data?.length) return <EmptyBlock />
    return (
      <Row gutter={12} className="kanban-row">
        {statuses.map((status) => (
          <Col xs={24} md={12} xl={5} key={status}>
            <Card title={statusLabels[status]} size="small">
              <Space direction="vertical" className="full-width">
                {projects.data
                  .filter((project) => project.status === status)
                  .map((project) => (
                    <Card key={project.id} size="small" className="project-card">
                      <Link to={`/projects/${project.id}`}>{project.name}</Link>
                      <div>
                        <Tag color="blue">{project.owner_name || '未分配'}</Tag>
                        <Tag>{project.bid_count} 份标书</Tag>
                      </div>
                    </Card>
                  ))}
              </Space>
            </Card>
          </Col>
        ))}
      </Row>
    )
  }

  return (
    <PageFrame
      module="投标管控"
      title="项目管理"
      subtitle="看板、列表、里程碑、成员和推进记录"
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
        <Button key="create" type="primary" onClick={() => setOpen(true)}>
          新建项目
        </Button>,
      ]}
    >
      {view === 'list' ? table() : board()}
      <Modal open={open} title="新建项目" onCancel={() => setOpen(false)} onOk={form.submit} confirmLoading={createMutation.isPending}>
        <Form form={form} layout="vertical" initialValues={{ status: 'opportunity' }} onFinish={createMutation.mutate}>
          <Form.Item name="name" label="项目名称" rules={[{ required: true, message: '项目名称必填' }]}>
            <Input placeholder="输入项目名称" />
          </Form.Item>
          <Form.Item name="status" label="初始状态">
            <Select options={statuses.map((value) => ({ value, label: statusLabels[value] }))} />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  )
}

export function ProjectDetailPage() {
  const { projectId = '' } = useParams()
  const navigate = useNavigate()
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()
  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => fetchProject(projectId),
    enabled: Boolean(projectId),
  })
  const milestones = useQuery({
    queryKey: ['project-milestones', projectId],
    queryFn: () => fetchProjectMilestones(projectId),
    enabled: Boolean(projectId),
  })
  const activities = useQuery({
    queryKey: ['project-activities', projectId],
    queryFn: () => fetchProjectActivities(projectId),
    enabled: Boolean(projectId),
  })
  const transitionMutation = useMutation({
    mutationFn: (payload: { status: ProjectDTO['status']; result?: ProjectDTO['result'] }) =>
      transitionProject(projectId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project', projectId] })
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      queryClient.invalidateQueries({ queryKey: ['project-activities', projectId] })
      message.success('项目状态已更新')
    },
    onError: () => message.error('状态更新失败'),
  })
  const milestoneMutation = useMutation({
    mutationFn: (payload: { title: string; status?: 'pending' | 'done'; due_date?: string; sort_order?: number; note?: string }) =>
      createProjectMilestone(projectId, payload),
    onSuccess: () => {
      setOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['project-milestones', projectId] })
      queryClient.invalidateQueries({ queryKey: ['project-activities', projectId] })
      message.success('里程碑已创建')
    },
    onError: () => message.error('里程碑创建失败'),
  })
  const costMutation = useMutation({
    mutationFn: () => createCostProject(projectId),
    onSuccess: (cost) => {
      message.success('成本项目已创建')
      navigate(`/costs/${cost.id}`)
    },
    onError: () => message.error('仅中标项目可创建成本项目'),
  })
  const archiveCaseMutation = useMutation({
    mutationFn: () => archiveProjectCase(projectId),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['project-activities', projectId] })
      message.success(`${result.case.title}已回流知识库`)
      navigate('/knowledge/docs')
    },
    onError: () => message.error('仅中标项目可回流知识库'),
  })

  if (project.isLoading) return <LoadingBlock />
  if (project.isError) return <ErrorBlock />
  if (!project.data) return <EmptyBlock />

  const next = nextStatus[project.data.status]
  const canCreateCost = project.data.status === 'closed' && project.data.result === 'won'

  return (
    <PageFrame
      module="项目管理"
      title={project.data.name}
      subtitle={statusLabels[project.data.status]}
      tags={['/projects/:projectId']}
      actions={[
        next && (
          <Button key="next" loading={transitionMutation.isPending} onClick={() => transitionMutation.mutate({ status: next })}>
            推进到{statusLabels[next]}
          </Button>
        ),
        <Button
          key="won"
          loading={transitionMutation.isPending}
          onClick={() => transitionMutation.mutate({ status: 'closed', result: 'won' })}
        >
          标记中标
        </Button>,
        <Button key="milestone" onClick={() => setOpen(true)}>
          新增里程碑
        </Button>,
        <Button key="archive-case" disabled={!canCreateCost} loading={archiveCaseMutation.isPending} onClick={() => archiveCaseMutation.mutate()}>
          回流知识库
        </Button>,
        <Button key="cost" type="primary" disabled={!canCreateCost} loading={costMutation.isPending} onClick={() => costMutation.mutate()}>
          创建成本项目
        </Button>,
      ].filter(Boolean)}
    >
      <Descriptions bordered column={2}>
        <Descriptions.Item label="项目名称">{project.data.name}</Descriptions.Item>
        <Descriptions.Item label="状态">{statusLabels[project.data.status]}</Descriptions.Item>
        <Descriptions.Item label="负责人">{project.data.owner_name || '-'}</Descriptions.Item>
        <Descriptions.Item label="中标结果">{project.data.result ? resultLabels[project.data.result] : '-'}</Descriptions.Item>
        <Descriptions.Item label="关联标书">{project.data.bid_count}</Descriptions.Item>
        <Descriptions.Item label="里程碑">{project.data.milestone_count}</Descriptions.Item>
      </Descriptions>
      <Row gutter={16} className="section-row">
        <Col xs={24} xl={12}>
          <Card title="里程碑">
            {milestones.isLoading && <LoadingBlock />}
            {milestones.isError && <ErrorBlock />}
            {!milestones.isLoading && !milestones.isError && !milestones.data?.length && <EmptyBlock />}
            <Timeline
              items={milestones.data?.map((milestone) => ({
                color: milestone.status === 'done' ? 'green' : 'blue',
                children: `${milestone.title} · ${dateText(milestone.due_date)} · ${milestone.status === 'done' ? '已完成' : '待完成'}`,
              }))}
            />
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title="项目活动">
            {activities.isLoading && <LoadingBlock />}
            {activities.isError && <ErrorBlock />}
            {!activities.isLoading && !activities.isError && !activities.data?.length && <EmptyBlock />}
            <Timeline
              items={activities.data?.map((activity) => ({
                color: 'gray',
                children: `${dateText(activity.created_at)} · ${activity.actor_name || '系统'} · ${activity.action}`,
              }))}
            />
          </Card>
        </Col>
      </Row>
      <Modal open={open} title="新增里程碑" onCancel={() => setOpen(false)} onOk={form.submit} confirmLoading={milestoneMutation.isPending}>
        <Form form={form} layout="vertical" initialValues={{ status: 'pending', sort_order: 10 }} onFinish={milestoneMutation.mutate}>
          <Form.Item name="title" label="里程碑名称" rules={[{ required: true, message: '里程碑名称必填' }]}>
            <Input placeholder="例如：完成技术标初稿" />
          </Form.Item>
          <Form.Item name="due_date" label="计划日期">
            <Input placeholder="YYYY-MM-DD" />
          </Form.Item>
          <Form.Item name="status" label="状态">
            <Select
              options={[
                { value: 'pending', label: '待完成' },
                { value: 'done', label: '已完成' },
              ]}
            />
          </Form.Item>
          <Form.Item name="sort_order" label="排序">
            <InputNumber min={0} step={10} className="full-width" />
          </Form.Item>
          <Form.Item name="note" label="备注">
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  )
}
