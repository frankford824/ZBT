import { ExclamationCircleOutlined, LoadingOutlined, StopOutlined } from '@ant-design/icons'
import { Alert, Empty, Spin, Typography } from 'antd'

export function ForbiddenBlock() {
  return (
    <Alert
      type="error"
      showIcon
      icon={<StopOutlined />}
      message="无权访问"
      description="当前账号未被授予此模块的权限，请联系企业管理员在「团队协作 · 成员管理」中调整角色"
    />
  )
}

export function LoadingBlock() {
  return (
    <div className="state-tile">
      <Spin indicator={<LoadingOutlined spin />} />
      <Typography.Text>正在加载…</Typography.Text>
    </div>
  )
}

export function EmptyBlock({ description = '暂无数据' }: { description?: string }) {
  return (
    <div className="state-tile">
      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={description} />
    </div>
  )
}

export function ErrorBlock() {
  return (
    <Alert
      type="error"
      showIcon
      icon={<ExclamationCircleOutlined />}
      message="数据加载失败"
      description="请刷新页面重试；若多次失败，请联系管理员处理"
    />
  )
}
