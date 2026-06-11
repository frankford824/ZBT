import { CheckOutlined, CloseOutlined, DeleteOutlined, PlusOutlined, TeamOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  App as AntApp,
  Button,
  Col,
  Form,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
} from 'antd'
import { useState } from 'react'
import {
  approveApproval,
  createApprovalChain,
  deleteApprovalChain,
  fetchAICallLogs,
  fetchApprovalChains,
  fetchApprovals,
  fetchMembers,
  fetchNotifications,
  fetchRoles,
  inviteMember,
  markNotificationsRead,
  rejectApproval,
  updateApprovalChain,
  type ApprovalChainDTO,
  type ApprovalStepDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'

function statusTag(status: string) {
  const color =
    status === 'approved' || status === 'done'
      ? 'green'
      : status === 'rejected' || status === 'failed'
        ? 'red'
        : status === 'pending' || status === 'running' || status === 'queued'
          ? 'blue'
          : 'default'
  const label: Record<string, string> = {
    pending: '审批中',
    approved: '已通过',
    rejected: '已驳回',
    cancelled: '已取消',
    done: '完成',
    failed: '失败',
    running: '运行中',
    queued: '排队中',
    active: '正常',
  }
  return <Tag color={color}>{label[status] || status}</Tag>
}

const taskTypeLabels: Record<string, string> = {
  knowledge_process: '知识库处理',
  knowledge_embedding: '知识库向量化',
  chapter_generate: '章节生成',
  document_export: '文档导出',
}

function formatTime(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function formatBizRef(value: Record<string, unknown>) {
  const resourceType = typeof value.resource_type === 'string' ? value.resource_type : '-'
  const resourceId = typeof value.resource_id === 'string' ? value.resource_id : '-'
  return `${resourceType} · ${resourceId}`
}

export function TeamPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const [inviteOpen, setInviteOpen] = useState(false)
  const [chainOpen, setChainOpen] = useState(false)
  const [inviteForm] = Form.useForm()
  const [chainForm] = Form.useForm()

  const membersQuery = useQuery({ queryKey: ['team', 'members'], queryFn: fetchMembers })
  const rolesQuery = useQuery({ queryKey: ['team', 'roles'], queryFn: fetchRoles })
  const notificationsQuery = useQuery({ queryKey: ['team', 'notifications'], queryFn: fetchNotifications })
  const approvalsQuery = useQuery({ queryKey: ['team', 'approvals'], queryFn: () => fetchApprovals() })
  const chainsQuery = useQuery({ queryKey: ['team', 'approval-chains'], queryFn: fetchApprovalChains })
  const aiLogsQuery = useQuery({ queryKey: ['team', 'ai-call-logs'], queryFn: () => fetchAICallLogs(50) })

  const roleOptions = rolesQuery.data?.map((role) => ({ value: role.code, label: role.name })) ?? []

  const inviteMutation = useMutation({
    mutationFn: inviteMember,
    onSuccess: () => {
      setInviteOpen(false)
      inviteForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['team', 'members'] })
      message.success('成员已邀请')
    },
    onError: () => message.error('邀请成员失败'),
  })

  const createChainMutation = useMutation({
    mutationFn: createApprovalChain,
    onSuccess: () => {
      setChainOpen(false)
      chainForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['team', 'approval-chains'] })
      message.success('审批链已创建')
    },
    onError: () => message.error('审批链创建失败'),
  })

  const updateChainMutation = useMutation({
    mutationFn: (chain: ApprovalChainDTO) =>
      updateApprovalChain(chain.id, {
        name: chain.name,
        description: chain.description,
        resource_type: chain.resource_type,
        steps: chain.steps,
        enabled: chain.enabled,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'approval-chains'] })
      message.success('审批链已更新')
    },
    onError: () => message.error('审批链更新失败'),
  })

  const deleteChainMutation = useMutation({
    mutationFn: deleteApprovalChain,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'approval-chains'] })
      message.success('审批链已删除')
    },
    onError: () => message.error('审批链删除失败'),
  })

  const approvalMutation = useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'approve' | 'reject' }) =>
      action === 'approve' ? approveApproval(id, '审批通过') : rejectApproval(id, '审批驳回'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'approvals'] })
      queryClient.invalidateQueries({ queryKey: ['team', 'notifications'] })
      message.success('审批状态已更新')
    },
    onError: () => message.error('审批操作失败'),
  })

  const readMutation = useMutation({
    mutationFn: () => markNotificationsRead(),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['team', 'notifications'] })
      message.success(`已读 ${result.updated} 条通知`)
    },
    onError: () => message.error('通知状态更新失败'),
  })

  const createChain = (values: {
    name: string
    description?: string
    first_role: string
    second_role: string
    executive_enabled?: boolean
  }) => {
    const steps: ApprovalStepDTO[] = [
      { order: 1, name: '部门主管审批', role_code: values.first_role, required: true, condition: '' },
      { order: 2, name: '项目经理审批', role_code: values.second_role, required: true, condition: '' },
      {
        order: 3,
        name: '总经理审批',
        role_code: 'company_admin',
        required: Boolean(values.executive_enabled),
        condition: '金额 > 100 万时启用',
      },
    ]
    createChainMutation.mutate({
      name: values.name,
      description: values.description,
      resource_type: 'bid',
      steps,
      enabled: true,
    })
  }

  return (
    <PageFrame
      module="企业管理"
      title="团队协作"
      subtitle="成员、审批链、审批实例、日志和通知"
      tags={['page-team', '/team?tab=members|approvals|logs|notifications']}
      actions={[
        <Button key="chain" icon={<PlusOutlined />} onClick={() => setChainOpen(true)}>
          审批链
        </Button>,
        <Button key="invite" type="primary" icon={<TeamOutlined />} onClick={() => setInviteOpen(true)}>
          邀请成员
        </Button>,
      ]}
    >
      <Tabs
        items={[
          {
            key: 'members',
            label: '成员',
            children: membersQuery.isLoading ? (
              <LoadingBlock />
            ) : membersQuery.isError ? (
              <ErrorBlock />
            ) : membersQuery.data?.length ? (
              <Table
                rowKey="id"
                dataSource={membersQuery.data}
                columns={[
                  { title: '姓名', dataIndex: ['user', 'name'] },
                  { title: '邮箱', dataIndex: ['user', 'email'] },
                  {
                    title: '角色',
                    render: (_, record) => record.roles.map((role) => <Tag key={role.id}>{role.name}</Tag>),
                  },
                  { title: '状态', dataIndex: 'status', render: statusTag },
                ]}
              />
            ) : (
              <EmptyBlock />
            ),
          },
          {
            key: 'approvals',
            label: '审批',
            children: (
              <Space direction="vertical" size={16} className="full-width">
                <Row gutter={[16, 16]}>
                  <Col xs={24} xl={14}>
                    <Typography.Title level={4}>审批实例</Typography.Title>
                    {approvalsQuery.isLoading && <LoadingBlock />}
                    {approvalsQuery.isError && <ErrorBlock />}
                    {!approvalsQuery.isLoading && !approvalsQuery.isError && !approvalsQuery.data?.length && <EmptyBlock />}
                    {!approvalsQuery.isLoading && !approvalsQuery.isError && Boolean(approvalsQuery.data?.length) && (
                      <Table
                        rowKey="id"
                        dataSource={approvalsQuery.data}
                        columns={[
                          { title: '审批标题', dataIndex: 'title' },
                          { title: '标书', dataIndex: 'bid_title' },
                          { title: '提交人', dataIndex: 'submitted_by_name', render: (value) => value || '-' },
                          { title: '当前级次', dataIndex: 'current_step' },
                          { title: '状态', dataIndex: 'status', render: statusTag },
                          {
                            title: '操作',
                            render: (_, row) =>
                              row.status === 'pending' ? (
                                <Space>
                                  <Button
                                    size="small"
                                    icon={<CheckOutlined />}
                                    loading={approvalMutation.isPending}
                                    onClick={() => approvalMutation.mutate({ id: row.id, action: 'approve' })}
                                  >
                                    通过
                                  </Button>
                                  <Button
                                    size="small"
                                    danger
                                    icon={<CloseOutlined />}
                                    loading={approvalMutation.isPending}
                                    onClick={() => approvalMutation.mutate({ id: row.id, action: 'reject' })}
                                  >
                                    驳回
                                  </Button>
                                </Space>
                              ) : (
                                '-'
                              ),
                          },
                        ]}
                      />
                    )}
                  </Col>
                  <Col xs={24} xl={10}>
                    <Typography.Title level={4}>审批链</Typography.Title>
                    {chainsQuery.isLoading && <LoadingBlock />}
                    {chainsQuery.isError && <ErrorBlock />}
                    {!chainsQuery.isLoading && !chainsQuery.isError && !chainsQuery.data?.length && <EmptyBlock />}
                    {!chainsQuery.isLoading && !chainsQuery.isError && Boolean(chainsQuery.data?.length) && (
                      <Table
                        rowKey="id"
                        dataSource={chainsQuery.data}
                        pagination={false}
                        columns={[
                          { title: '名称', dataIndex: 'name' },
                          {
                            title: '级次',
                            render: (_, row) =>
                              row.steps.map((step) => (
                                <Tag key={step.order} color={step.required ? 'blue' : 'default'}>
                                  {step.order}.{step.name}
                                </Tag>
                              )),
                          },
                          {
                            title: '启用',
                            dataIndex: 'enabled',
                            render: (enabled, row) => (
                              <Switch size="small" checked={enabled} onChange={(checked) => updateChainMutation.mutate({ ...row, enabled: checked })} />
                            ),
                          },
                          {
                            title: '操作',
                            render: (_, row) => (
                              <Popconfirm title="删除审批链" onConfirm={() => deleteChainMutation.mutate(row.id)}>
                                <Button size="small" danger icon={<DeleteOutlined />} />
                              </Popconfirm>
                            ),
                          },
                        ]}
                      />
                    )}
                  </Col>
                </Row>
              </Space>
            ),
          },
          {
            key: 'logs',
            label: '日志',
            children: aiLogsQuery.isLoading ? (
              <LoadingBlock />
            ) : aiLogsQuery.isError ? (
              <ErrorBlock />
            ) : aiLogsQuery.data?.length ? (
              <Table
                rowKey="id"
                dataSource={aiLogsQuery.data}
                columns={[
                  { title: '任务', dataIndex: 'task_type', render: (value) => taskTypeLabels[value] || value },
                  { title: '模型路由', render: (_, row) => `${row.provider} / ${row.model}` },
                  { title: '调用人', dataIndex: 'user_name', render: (value) => value || '系统' },
                  {
                    title: 'Token',
                    render: (_, row) => `${row.input_tokens} / ${row.output_tokens}`,
                  },
                  { title: '耗时', dataIndex: 'latency_ms', render: (value) => `${value} ms` },
                  { title: '状态', dataIndex: 'status', render: statusTag },
                  {
                    title: '资源',
                    render: (_, row) => formatBizRef(row.biz_ref),
                  },
                  { title: '时间', dataIndex: 'created_at', render: formatTime },
                ]}
              />
            ) : (
              <EmptyBlock />
            ),
          },
          {
            key: 'notifications',
            label: '通知',
            children: notificationsQuery.isLoading ? (
              <LoadingBlock />
            ) : notificationsQuery.isError ? (
              <ErrorBlock />
            ) : (
              <Space direction="vertical" size={16} className="full-width">
                <Button loading={readMutation.isPending} onClick={() => readMutation.mutate()}>
                  全部标记已读
                </Button>
                <Table
                  rowKey="id"
                  dataSource={notificationsQuery.data ?? []}
                  locale={{ emptyText: <EmptyBlock /> }}
                  columns={[
                    { title: '标题', dataIndex: 'title', render: (value, row) => <Tag color={row.read_at ? 'default' : 'blue'}>{value}</Tag> },
                    { title: '内容', dataIndex: 'body' },
                    { title: '时间', dataIndex: 'created_at', render: formatTime },
                    { title: '状态', dataIndex: 'read_at', render: (value) => (value ? <Tag>已读</Tag> : <Tag color="blue">未读</Tag>) },
                  ]}
                />
              </Space>
            ),
          },
        ]}
      />
      <Modal open={inviteOpen} title="邀请成员" onCancel={() => setInviteOpen(false)} onOk={inviteForm.submit} confirmLoading={inviteMutation.isPending}>
        <Form form={inviteForm} layout="vertical" initialValues={{ role_code: 'viewer' }} onFinish={inviteMutation.mutate}>
          <Form.Item label="姓名" name="name" rules={[{ required: true, message: '姓名必填' }]}>
            <Input placeholder="新成员姓名" />
          </Form.Item>
          <Form.Item label="邮箱" name="email" rules={[{ required: true, message: '邮箱必填' }]}>
            <Input placeholder="member@example.com" />
          </Form.Item>
          <Form.Item label="角色" name="role_code">
            <Select options={roleOptions} loading={rolesQuery.isLoading} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal open={chainOpen} title="审批流程配置" onCancel={() => setChainOpen(false)} onOk={chainForm.submit} confirmLoading={createChainMutation.isPending}>
        <Form
          form={chainForm}
          layout="vertical"
          initialValues={{ name: '标书两级审批链', first_role: 'department_admin', second_role: 'project_manager', executive_enabled: false }}
          onFinish={createChain}
        >
          <Form.Item label="审批链名称" name="name" rules={[{ required: true, message: '名称必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="说明" name="description">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item label="第一级审批角色" name="first_role">
            <Select options={roleOptions} loading={rolesQuery.isLoading} />
          </Form.Item>
          <Form.Item label="第二级审批角色" name="second_role">
            <Select options={roleOptions} loading={rolesQuery.isLoading} />
          </Form.Item>
          <Form.Item label="启用总经理审批" name="executive_enabled" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  )
}
