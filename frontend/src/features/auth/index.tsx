import { LoginOutlined } from '@ant-design/icons'
import { Button, Form, Input, Space, Typography } from 'antd'
import { Link, useNavigate } from 'react-router-dom'
import { useSessionStore } from '../../app/store/session'

export function LoginPage() {
  const navigate = useNavigate()
  const loginAsDemo = useSessionStore((state) => state.loginAsDemo)

  return (
    <Space direction="vertical" size={20} className="auth-stack">
      <Typography.Paragraph>企业投标团队工作入口</Typography.Paragraph>
      <Form
        layout="vertical"
        onFinish={() => {
          loginAsDemo()
          navigate('/dashboard')
        }}
      >
        <Form.Item label="账号" name="account" initialValue="admin@zbt.local">
          <Input />
        </Form.Item>
        <Form.Item label="密码" name="password" initialValue="demo-password">
          <Input.Password />
        </Form.Item>
        <Button type="primary" htmlType="submit" block icon={<LoginOutlined />}>
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
  return (
    <Space direction="vertical" size={20} className="auth-stack">
      <Typography.Title level={2}>注册企业</Typography.Title>
      <Form layout="vertical">
        <Form.Item label="企业名称">
          <Input placeholder="杭州智建科技有限公司" />
        </Form.Item>
        <Form.Item label="管理员邮箱">
          <Input placeholder="admin@example.com" />
        </Form.Item>
        <Button type="primary" block>
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
