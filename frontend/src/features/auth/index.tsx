import { LoginOutlined, UserAddOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, Space, Typography } from 'antd'
import { useMutation } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { useSessionStore } from '../../app/store/session'
import { login, registerTenant } from '../../shared/api/client'

export function LoginPage() {
  const navigate = useNavigate()
  const setSession = useSessionStore((state) => state.setSession)
  const mutation = useMutation({
    mutationFn: login,
    onSuccess: (payload) => {
      setSession(payload)
      navigate('/dashboard')
    },
  })

  return (
    <Space direction="vertical" size={20} className="auth-stack">
      <Typography.Paragraph>企业投标团队工作入口</Typography.Paragraph>
      {mutation.isError ? <Alert type="error" showIcon message="账号或密码错误" /> : null}
      <Form
        layout="vertical"
        onFinish={(values) => mutation.mutate(values)}
      >
        <Form.Item label="账号" name="email" initialValue="admin@zbt.local">
          <Input />
        </Form.Item>
        <Form.Item label="密码" name="password" initialValue="demo-password">
          <Input.Password />
        </Form.Item>
        <Form.Item
          label="租户 ID"
          name="tenant_id"
          initialValue="00000000-0000-4000-8000-000000000001"
        >
          <Input />
        </Form.Item>
        <Button
          type="primary"
          htmlType="submit"
          block
          icon={<LoginOutlined />}
          loading={mutation.isPending}
        >
          登录
        </Button>
      </Form>
      <Space>
        <Link to="/register">注册企业</Link>
        <Link to="/onboarding">租户初始化</Link>
      </Space>
    </Space>
  )
}

export function RegisterPage() {
  const navigate = useNavigate()
  const setSession = useSessionStore((state) => state.setSession)
  const mutation = useMutation({
    mutationFn: registerTenant,
    onSuccess: (payload) => {
      setSession(payload)
      navigate('/onboarding')
    },
  })

  return (
    <Space direction="vertical" size={20} className="auth-stack">
      <Typography.Title level={2}>注册企业</Typography.Title>
      {mutation.isError ? <Alert type="error" showIcon message="注册失败，请确认邮箱未被占用且密码不少于 8 位" /> : null}
      <Form layout="vertical" onFinish={(values) => mutation.mutate(values)}>
        <Form.Item label="企业名称" name="tenant_name" rules={[{ required: true, message: '企业名称必填' }]}>
          <Input placeholder="杭州智建科技有限公司" />
        </Form.Item>
        <Form.Item label="管理员姓名" name="admin_name" rules={[{ required: true, message: '管理员姓名必填' }]}>
          <Input placeholder="陈思远" />
        </Form.Item>
        <Form.Item label="管理员邮箱" name="email" rules={[{ required: true, message: '管理员邮箱必填' }]}>
          <Input placeholder="admin@example.com" />
        </Form.Item>
        <Form.Item label="密码" name="password" rules={[{ required: true, min: 8, message: '密码至少 8 位' }]}>
          <Input.Password placeholder="至少 8 位" />
        </Form.Item>
        <Button type="primary" htmlType="submit" block icon={<UserAddOutlined />} loading={mutation.isPending}>
          创建账号
        </Button>
      </Form>
      <Link to="/login">返回登录</Link>
    </Space>
  )
}

export function OnboardingPage() {
  return (
    <Space direction="vertical" size={20} className="auth-stack">
      <Typography.Title level={2}>租户初始化</Typography.Title>
      <Form layout="vertical">
        <Form.Item label="默认部门">
          <Input placeholder="投标中心" />
        </Form.Item>
        <Form.Item label="知识库分类">
          <Input placeholder="资质证书、业绩案例、技术方案" />
        </Form.Item>
        <Button type="primary" block>
          完成初始化
        </Button>
      </Form>
      <Link to="/dashboard">进入工作台</Link>
    </Space>
  )
}
