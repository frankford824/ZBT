import { Button, Card, Col, Form, Input, Row, Select, Table, Tabs, Tag, Timeline } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { PageFrame } from '../../shared/components/PageFrame'
import { fetchMembers, fetchNotifications, fetchRoles, inviteMember } from '../../shared/api/client'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'

export function TeamPage() {
  const queryClient = useQueryClient()
  const membersQuery = useQuery({ queryKey: ['team', 'members'], queryFn: fetchMembers })
  const rolesQuery = useQuery({ queryKey: ['team', 'roles'], queryFn: fetchRoles })
  const notificationsQuery = useQuery({
    queryKey: ['team', 'notifications'],
    queryFn: fetchNotifications,
  })
  const inviteMutation = useMutation({
    mutationFn: inviteMember,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', 'members'] })
    },
  })
  const roleOptions =
    rolesQuery.data?.map((role) => ({ value: role.code, label: role.name })) ?? []

  return (
    <PageFrame
      module="企业管理"
      title="团队协作"
      subtitle="成员、审批、日志、通知和模块权限"
      tags={['page-team', '/team?tab=members|approvals|logs|notifications']}
      actions={[<Button key="invite" type="primary">邀请成员</Button>]}
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
                    render: (_, record) =>
                      record.roles.map((role) => <Tag key={role.id}>{role.name}</Tag>),
                  },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    render: (status) => <Tag color={status === 'active' ? 'green' : 'orange'}>{status}</Tag>,
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
              <Row gutter={16}>
                <Col xs={24} xl={10}>
                  <Card title="邀请成员">
                    <Form
                      layout="vertical"
                      onFinish={(values) => inviteMutation.mutate(values)}
                    >
                      <Form.Item label="姓名" name="name" rules={[{ required: true }]}>
                        <Input placeholder="新成员姓名" />
                      </Form.Item>
                      <Form.Item label="邮箱" name="email" rules={[{ required: true }]}>
                        <Input placeholder="member@example.com" />
                      </Form.Item>
                      <Form.Item label="角色" name="role_code" initialValue="viewer">
                        <Select options={roleOptions} loading={rolesQuery.isLoading} />
                      </Form.Item>
                      <Button htmlType="submit" type="primary" loading={inviteMutation.isPending}>
                        发送邀请
                      </Button>
                    </Form>
                  </Card>
                </Col>
                <Col xs={24} xl={14}>
                  <Card title="审批实例">
                    <Timeline
                      items={[
                        { color: 'green', children: '投标专员提交审批' },
                        { color: 'blue', children: '项目经理审核中' },
                        { color: 'gray', children: '总经理待审批' },
                      ]}
                    />
                  </Card>
                </Col>
              </Row>
            ),
          },
          {
            key: 'logs',
            label: '日志',
            children: (
              <Timeline
                items={[
                  { children: '陈思远创建标书 智慧交通技术标' },
                  { children: '林悦将项目状态推进到 compliance_review' },
                  { children: '赵宁导出合规检查报告' },
                ]}
              />
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
              <Card>
                {notificationsQuery.data?.map((item) => (
                  <Tag key={item.id} color={item.read_at ? 'default' : 'blue'}>
                    {item.title}
                  </Tag>
                ))}
              </Card>
            ),
          },
        ]}
      />
    </PageFrame>
  )
}
