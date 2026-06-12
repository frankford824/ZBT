import { ExclamationCircleOutlined, LoadingOutlined, StopOutlined } from '@ant-design/icons'
import { Alert, Spin, Typography } from 'antd'

const emptyIllustration = '/illustrations/zbt-empty-office.png'

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
    <div className="state-tile state-tile-empty">
      <img className="state-illustration" src={emptyIllustration} alt="" aria-hidden="true" />
      <Typography.Text className="state-copy" type="secondary">
        {description}
      </Typography.Text>
    </div>
  )
}

export function ErrorBlock({
  description = '请刷新页面重试；若多次失败，请联系管理员处理',
}: {
  description?: string
}) {
  return (
    <Alert
      type="error"
      showIcon
      icon={<ExclamationCircleOutlined />}
      message="数据加载失败"
      description={description}
    />
  )
}
