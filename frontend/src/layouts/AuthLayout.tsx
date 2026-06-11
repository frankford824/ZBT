import { Card, Layout, Typography } from 'antd'
import { Outlet } from 'react-router-dom'

export function AuthLayout() {
  return (
    <Layout className="auth-layout">
      <Card className="auth-card">
        <Typography.Title level={1}>智标通</Typography.Title>
        <Outlet />
      </Card>
    </Layout>
  )
}
