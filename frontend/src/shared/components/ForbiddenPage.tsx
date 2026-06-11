import { Button, Result } from 'antd'
import { Link } from 'react-router-dom'

export function ForbiddenPage() {
  return (
    <Result
      status="403"
      title="403"
      subTitle="当前账号没有访问该页面的权限"
      extra={
        <Button type="primary">
          <Link to="/dashboard">返回工作台</Link>
        </Button>
      }
    />
  )
}
