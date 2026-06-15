import { CheckOutlined, CloseOutlined, DeleteOutlined, EditOutlined, PlusOutlined, TeamOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
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
  deleteMember,
  fetchAICallLogs,
  fetchApprovalChains,
  fetchApprovals,
  fetchMembers,
  fetchNotifications,
  fetchRoles,
  getApiErrorMessage,
  inviteMember,
  markNotificationsRead,
  rejectApproval,
  updateMember,
  updateApprovalChain,
  type ApprovalChainDTO,
  type ApprovalStepDTO,
  type MemberDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { formatDateTime } from '../../shared/format/date'
import { useCanAccess } from '../../shared/permissions/permissions'

const teamTabs = ['members', 'approvals', 'logs', 'notifications'] as const
type TeamTab = (typeof teamTabs)[number]

function normalizeTeamTab(value: string | null): TeamTab {
  return teamTabs.includes(value as TeamTab) ? (value as TeamTab) : 'members'
}

function statusTag(status: string) {
  const color =
    status === 'approved' || status === 'done'
      ? 'green'
      : status === 'rejected' || status === 'failed' || status === 'disabled'
        ? 'red'
        : status === 'pending' || status === 'running' || status === 'queued' || status === 'invited'
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
    invited: '已邀请',
    disabled: '已禁用',
  }
  return <Tag color={color}>{label[status] || status}</Tag>
}

const taskTypeLabels: Record<string, string> = {
  tender_parse: '招标文件解读',
  knowledge_process: '知识库处理',
  knowledge_embedding: '知识库整理',
  knowledge_rerank: '知识库匹配',
  outline_generate: '目录大纲生成',
  chapter_generate: '章节生成',
  chapter_ai_action: '章节处理',
  document_export: '文档导出',
  compliance_check: '合规检查',
  cost_advice: '成本建议',
}

const resourceLabels: Record<string, string> = {
  bid: '标书',
  bid_export: '导出文件',
  bid_chapter: '标书章节',
  chapter: '章节',
  knowledge: '知识库',
  knowledge_document: '知识文档',
  document: '文档',
  tender: '标讯',
  cost: '成本',
  cost_project: '成本测算',
}

function formatTime(value?: string | null) {
  return formatDateTime(value)
}

function formatBizRef(value: Record<string, unknown>) {
  const resourceType = typeof value.resource_type === 'string' ? value.resource_type : '-'
  return resourceLabels[resourceType] || '相关内容'
}

function formatUsage(input: number, output: number) {
  const total = Number(input || 0) + Number(output || 0)
  if (!total) return '-'
  return `${total.toLocaleString()} 字`
}

function formatLatency(value: number) {
  if (!value) return '-'
  return value >= 1000 ? `${(value / 1000).toFixed(1)} 秒` : `${value} 毫秒`
}

export function TeamPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = normalizeTeamTab(searchParams.get('tab'))
  const canWrite = useCanAccess('team', 'full')
  const [inviteOpen, setInviteOpen] = useState(false)
  const [chainOpen, setChainOpen] = useState(false)
  const [memberOpen, setMemberOpen] = useState(false)
  const [editingMember, setEditingMember] = useState<MemberDTO | null>(null)
  const [inviteForm] = Form.useForm()
  const [chainForm] = Form.useForm()
  const [memberForm] = Form.useForm()

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
    onError: (error) => message.error(getApiErrorMessage(error, '邀请成员失败')),
  })

  const updateMemberMutation = useMutation({
    mutationFn: (values: { name?: string; status?: 'active' | 'invited' | 'disabled'; role_codes?: string[] }) => {
      if (!editingMember) throw new Error('missing member')
      return updateMember(editingMember.id, values)
    },
    onSuccess: () => {
      setMemberOpen(false)
      setEditingMember(null)
      memberForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['team', 'members'] })
      message.success('成员已更新')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '成员更新失败')),
  })

  const deleteMemberMutation = useMutation({
    mutationFn: deleteMember,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'members'] })
      message.success('成员已禁用')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '成员禁用失败')),
  })

  const createChainMutation = useMutation({
    mutationFn: createApprovalChain,
    onSuccess: () => {
      setChainOpen(false)
      chainForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['team', 'approval-chains'] })
      message.success('审批流程已创建')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '审批流程创建失败')),
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
      message.success('审批流程已更新')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '审批流程更新失败')),
  })

  const deleteChainMutation = useMutation({
    mutationFn: deleteApprovalChain,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'approval-chains'] })
      message.success('审批流程已删除')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '审批流程删除失败')),
  })

  const approvalMutation = useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'approve' | 'reject' }) =>
      action === 'approve' ? approveApproval(id, '审批通过') : rejectApproval(id, '审批驳回'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'approvals'] })
      queryClient.invalidateQueries({ queryKey: ['team', 'notifications'] })
      message.success('审批状态已更新')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '审批操作失败')),
  })

  const readMutation = useMutation({
    mutationFn: () => markNotificationsRead(),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['team', 'notifications'] })
      message.success(`已读 ${result.updated} 条通知`)
    },
    onError: (error) => message.error(getApiErrorMessage(error, '通知状态更新失败')),
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

  const openMemberEditor = (member: MemberDTO) => {
    setEditingMember(member)
    memberForm.setFieldsValue({
      name: member.user.name,
      status: member.status,
      role_codes: member.roles.map((role) => role.code),
    })
    setMemberOpen(true)
  }

  return (
    <PageFrame
      module="企业管理"
      title="团队协作"
      subtitle="成员、审批流程、待办审批、使用记录和通知"
      tags={['page-team', '/team?tab=members|approvals|logs|notifications']}
      actions={[
        canWrite ? <Button key="chain" icon={<PlusOutlined />} onClick={() => setChainOpen(true)}>
          审批流程
        </Button> : null,
        canWrite ? <Button key="invite" type="primary" icon={<TeamOutlined />} onClick={() => setInviteOpen(true)}>
          邀请成员
        </Button> : null,
      ]}
    >
      <Tabs
        activeKey={activeTab}
        onChange={(tab) => setSearchParams({ tab })}
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
                scroll={{ x: 760 }}
                columns={[
                  { title: '姓名', dataIndex: ['user', 'name'], width: 120 },
                  { title: '邮箱', dataIndex: ['user', 'email'], width: 220 },
                  {
                    title: '角色',
                    width: 220,
                    render: (_, record) => (
                      <Space wrap size={[4, 4]}>
                        {record.roles.map((role) => <Tag key={role.id}>{role.name}</Tag>)}
                      </Space>
                    ),
                  },
                  { title: '状态', dataIndex: 'status', width: 100, render: statusTag },
                  {
                    title: '操作',
                    width: 150,
                    render: (_, record) => canWrite ? (
                      <Space size={8} wrap={false}>
                        <Button size="small" icon={<EditOutlined />} onClick={() => openMemberEditor(record)}>
                          编辑
                        </Button>
                        <Popconfirm title="禁用该成员" onConfirm={() => deleteMemberMutation.mutate(record.id)}>
                          <Button size="small" danger icon={<DeleteOutlined />} loading={deleteMemberMutation.isPending}>
                            禁用
                          </Button>
                        </Popconfirm>
                      </Space>
                    ) : '-',
                  },
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
                    <Typography.Title level={4}>待办审批</Typography.Title>
                    {approvalsQuery.isLoading && <LoadingBlock />}
                    {approvalsQuery.isError && <ErrorBlock />}
                    {!approvalsQuery.isLoading && !approvalsQuery.isError && !approvalsQuery.data?.length && <EmptyBlock />}
                    {!approvalsQuery.isLoading && !approvalsQuery.isError && Boolean(approvalsQuery.data?.length) && (
                      <Table
                        rowKey="id"
                        dataSource={approvalsQuery.data}
                        scroll={{ x: 840 }}
                        columns={[
                          { title: '审批标题', dataIndex: 'title', width: 220 },
                          { title: '标书', dataIndex: 'bid_title', width: 220 },
                          { title: '提交人', dataIndex: 'submitted_by_name', width: 110, render: (value) => value || '-' },
                          { title: '当前环节', dataIndex: 'current_step', width: 110, render: (value: number) => `第 ${value} 级` },
                          { title: '状态', dataIndex: 'status', width: 100, render: statusTag },
                          {
                            title: '操作',
                            width: 150,
                            render: (_, row) =>
                              row.status === 'pending' && canWrite ? (
                                <Space size={8} wrap={false}>
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
                    <Typography.Title level={4}>审批流程</Typography.Title>
                    {chainsQuery.isLoading && <LoadingBlock />}
                    {chainsQuery.isError && <ErrorBlock />}
                    {!chainsQuery.isLoading && !chainsQuery.isError && !chainsQuery.data?.length && <EmptyBlock />}
                    {!chainsQuery.isLoading && !chainsQuery.isError && Boolean(chainsQuery.data?.length) && (
                      <Table
                        rowKey="id"
                        dataSource={chainsQuery.data}
                        pagination={false}
                        scroll={{ x: 640 }}
                        columns={[
                          { title: '名称', dataIndex: 'name', width: 160 },
                          {
                            title: '流程',
                            width: 300,
                            render: (_, row) =>
                              (
                                <Space wrap size={[4, 4]}>
                                  {row.steps.map((step) => (
                                    <Tag key={step.order} color={step.required ? 'blue' : 'default'}>
                                      {step.order}.{step.name}
                                    </Tag>
                                  ))}
                                </Space>
                              ),
                          },
                          {
                            title: '启用',
                            dataIndex: 'enabled',
                            width: 80,
                            render: (enabled, row) => (
                              <Switch size="small" checked={enabled} disabled={!canWrite} onChange={(checked) => updateChainMutation.mutate({ ...row, enabled: checked })} />
                            ),
                          },
                          {
                            title: '操作',
                            width: 80,
                            render: (_, row) => canWrite ? (
                              <Popconfirm title="删除审批流程" onConfirm={() => deleteChainMutation.mutate(row.id)}>
                                <Button size="small" danger icon={<DeleteOutlined />} />
                              </Popconfirm>
                            ) : '-',
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
            label: '使用记录',
            children: aiLogsQuery.isLoading ? (
              <LoadingBlock />
            ) : aiLogsQuery.isError ? (
              <ErrorBlock />
            ) : aiLogsQuery.data?.length ? (
              <Table
                rowKey="id"
                dataSource={aiLogsQuery.data}
                scroll={{ x: 920 }}
                columns={[
                  { title: '事项', dataIndex: 'task_type', width: 150, render: (value) => taskTypeLabels[value] || '智能处理' },
                  { title: '处理方式', width: 130, render: () => '平台智能处理' },
                  { title: '调用人', dataIndex: 'user_name', width: 120, render: (value) => value || '系统' },
                  {
                    title: '用量',
                    width: 110,
                    render: (_, row) => formatUsage(row.input_tokens, row.output_tokens),
                  },
                  { title: '处理时长', dataIndex: 'latency_ms', width: 110, render: formatLatency },
                  { title: '状态', dataIndex: 'status', width: 90, render: statusTag },
                  {
                    title: '关联内容',
                    width: 120,
                    render: (_, row) => formatBizRef(row.biz_ref),
                  },
                  { title: '时间', dataIndex: 'created_at', width: 190, render: formatTime },
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
                  scroll={{ x: 820 }}
                  columns={[
                    { title: '标题', dataIndex: 'title', width: 220, render: (value, row) => <Tag color={row.read_at ? 'default' : 'blue'}>{value}</Tag> },
                    { title: '内容', dataIndex: 'body', width: 360, ellipsis: true },
                    { title: '时间', dataIndex: 'created_at', width: 160, render: formatTime },
                    { title: '状态', dataIndex: 'read_at', width: 80, render: (value) => (value ? <Tag>已读</Tag> : <Tag color="blue">未读</Tag>) },
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
          <Form.Item label="初始密码" name="initial_password" rules={[{ required: true, min: 8, message: '密码至少 8 位' }]}>
            <Input.Password placeholder="至少 8 位" />
          </Form.Item>
          <Form.Item label="角色" name="role_code">
            <Select options={roleOptions} loading={rolesQuery.isLoading} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        open={memberOpen}
        title="成员设置"
        onCancel={() => {
          setMemberOpen(false)
          setEditingMember(null)
        }}
        onOk={memberForm.submit}
        confirmLoading={updateMemberMutation.isPending}
      >
        <Form form={memberForm} layout="vertical" onFinish={updateMemberMutation.mutate}>
          <Form.Item label="姓名" name="name" rules={[{ required: true, message: '姓名必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="状态" name="status" rules={[{ required: true, message: '状态必填' }]}>
            <Select
              options={[
                { value: 'active', label: '正常' },
                { value: 'invited', label: '已邀请' },
                { value: 'disabled', label: '已禁用' },
              ]}
            />
          </Form.Item>
          <Form.Item label="角色" name="role_codes" rules={[{ required: true, message: '至少选择一个角色' }]}>
            <Select mode="multiple" options={roleOptions} loading={rolesQuery.isLoading} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal open={chainOpen} title="审批流程配置" onCancel={() => setChainOpen(false)} onOk={chainForm.submit} confirmLoading={createChainMutation.isPending}>
        <Form
          form={chainForm}
          layout="vertical"
          initialValues={{ name: '标书两级审批流程', first_role: 'department_admin', second_role: 'project_manager', executive_enabled: false }}
          onFinish={createChain}
        >
          <Form.Item label="流程名称" name="name" rules={[{ required: true, message: '名称必填' }]}>
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
