import { LoginOutlined, UserAddOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, message, Space, Typography } from 'antd'
import { useMutation } from '@tanstack/react-query'
import { Link, Navigate, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { useSessionStore } from '../../app/store/session'
import { createKnowledgeCategory, getApiErrorMessage, login, registerTenant, updateTenant } from '../../shared/api/client'
import { safeReturnPath } from '../../shared/auth/session'

type LoginLocationState = {
  from?: string
}

type OnboardingValues = {
  tenant_name?: string
  knowledge_categories?: string
}

const showDemoLogin = import.meta.env.DEV || import.meta.env.VITE_SHOW_DEMO_LOGIN === 'true'

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const [searchParams] = useSearchParams()
  const setSession = useSessionStore((state) => state.setSession)
  const sessionExpired = searchParams.get('session') === 'expired'
  const tenantId = searchParams.get('tenant') ?? undefined
  const locationState = location.state as LoginLocationState | null
  const returnPath = safeReturnPath(searchParams.get('from') || locationState?.from)
  const mutation = useMutation({
    mutationFn: login,
    onSuccess: (payload) => {
      setSession(payload)
      navigate(returnPath, { replace: true })
    },
  })

  return (
    <Space direction="vertical" size={20} className="auth-stack">
      <div>
        <Typography.Title level={3} className="auth-form-title">
          登录工作台
        </Typography.Title>
        <Typography.Text type="secondary">使用企业账号进入投标工作台</Typography.Text>
      </div>
      {mutation.isError ? (
        <Alert type="error" showIcon message="登录失败" description="账号或密码不正确，请重新输入" />
      ) : null}
      {sessionExpired && !mutation.isError ? (
        <Alert type="warning" showIcon message="登录状态已过期" description="请重新登录后继续处理刚才的事项" />
      ) : null}
      <Form layout="vertical" onFinish={(values) => mutation.mutate(values)}>
        <Form.Item label="账号" name="email" initialValue={showDemoLogin ? 'admin@zbt.local' : undefined}>
          <Input />
        </Form.Item>
        <Form.Item label="密码" name="password" initialValue={showDemoLogin ? 'demo-password' : undefined}>
          <Input.Password />
        </Form.Item>
        <Form.Item hidden name="tenant_id" initialValue={tenantId}>
          <Input type="hidden" />
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
      <Space split={<Typography.Text type="secondary">·</Typography.Text>}>
        <Link to="/register">注册企业</Link>
        <Link to="/onboarding">企业初始化</Link>
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
      <div>
        <Typography.Title level={3} className="auth-form-title">
          注册企业
        </Typography.Title>
        <Typography.Text type="secondary">创建企业空间，并生成管理员账号</Typography.Text>
      </div>
      {mutation.isError ? (
        <Alert
          type="error"
          showIcon
          message="注册失败"
          description="请确认邮箱未被占用，且密码不少于 8 位"
        />
      ) : null}
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
  const navigate = useNavigate()
  const isAuthenticated = useSessionStore((state) => state.isAuthenticated)
  const tenant = useSessionStore((state) => state.tenant)
  const setTenant = useSessionStore((state) => state.setTenant)
  const mutation = useMutation({
    mutationFn: async (values: OnboardingValues) => {
      const tenantName = values.tenant_name?.trim()
      let updatedTenant: Awaited<ReturnType<typeof updateTenant>> | null = null
      if (tenantName && tenantName !== tenant.name) {
        updatedTenant = await updateTenant({ name: tenantName })
      }
      const categories = categoryNames(values.knowledge_categories)
      await Promise.all(categories.map((name) => createKnowledgeCategory({ name })))
      return { updatedTenant }
    },
    onSuccess: ({ updatedTenant }) => {
      if (updatedTenant) setTenant(updatedTenant)
      message.success('初始化已完成')
      navigate('/dashboard', { replace: true })
    },
    onError: (error) => message.error(getApiErrorMessage(error, '初始化失败')),
  })

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  return (
    <Space direction="vertical" size={20} className="auth-stack">
      <div>
        <Typography.Title level={3} className="auth-form-title">
          企业初始化
        </Typography.Title>
        <Typography.Text type="secondary">确认企业名称与资料分类，团队即可开工</Typography.Text>
      </div>
      <Form
        layout="vertical"
        initialValues={{
          tenant_name: tenant.name,
          knowledge_categories: '资质证书、业绩案例、技术方案',
        }}
        onFinish={(values) => mutation.mutate(values)}
      >
        <Form.Item label="企业名称" name="tenant_name" rules={[{ required: true, message: '企业名称必填' }]}>
          <Input placeholder="杭州智建科技有限公司" />
        </Form.Item>
        <Form.Item label="资料分类" name="knowledge_categories">
          <Input placeholder="资质证书、业绩案例、技术方案" />
        </Form.Item>
        <Button type="primary" htmlType="submit" block loading={mutation.isPending}>
          完成初始化
        </Button>
      </Form>
      <Link to="/dashboard">进入工作台</Link>
    </Space>
  )
}

function categoryNames(value?: string) {
  return Array.from(
    new Set(
      (value ?? '')
        .split(/[,\n，、]/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  )
}
