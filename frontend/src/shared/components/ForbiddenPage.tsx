import { Button, Result } from 'antd'
import { Link } from 'react-router-dom'

export function ForbiddenPage() {
  return (
    <Result
      status="403"
      title="无权访问"
      subTitle="当前账号未开通此功能，请联系企业管理员调整权限"
      extra={
        <Button type="primary">
          <Link to="/dashboard">返回工作台</Link>
        </Button>
      }
    />
  )
}
