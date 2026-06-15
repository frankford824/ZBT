import { LoginOutlined, UserAddOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, Space, Typography } from 'antd'
import { useMutation } from '@tanstack/react-query'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useSessionStore } from '../../app/store/session'
import { login, registerTenant } from '../../shared/api/client'
import { safeReturnPath } from '../../shared/auth/session'

export function LoginPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const setSession = useSessionStore((state) => state.setSession)
  const sessionExpired = searchParams.get('session') === 'expired'
  const tenantId = searchParams.get('tenant') ?? undefined
  const mutation = useMutation({
    mutationFn: login,
    onSuccess: (payload) => {
      setSession(payload)
      navigate(safeReturnPath(searchParams.get('from')), { replace: true })
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
        <Form.Item label="账号" name="email" initialValue="admin@zbt.local">
          <Input />
        </Form.Item>
        <Form.Item label="密码" name="password" initialValue="demo-password">
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
  return (
    <Space direction="vertical" size={20} className="auth-stack">
      <div>
        <Typography.Title level={3} className="auth-form-title">
          企业初始化
        </Typography.Title>
        <Typography.Text type="secondary">设置默认部门与知识库分类，团队即可开工</Typography.Text>
      </div>
      <Form layout="vertical">
        <Form.Item label="默认部门" name="department">
          <Input placeholder="投标中心" />
        </Form.Item>
        <Form.Item label="知识库分类" name="knowledge_categories">
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
