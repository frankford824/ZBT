import { ExclamationCircleOutlined, LoadingOutlined, ReloadOutlined, StopOutlined } from '@ant-design/icons'
import { Alert, Button, Spin, Typography } from 'antd'

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
  description = '数据没能加载出来，可能是网络波动或登录状态已过期。',
  onRetry,
}: {
  description?: string
  onRetry?: () => void
}) {
  return (
    <div className="state-tile state-tile-error">
      <ExclamationCircleOutlined className="state-error-icon" />
      <div className="state-error-body">
        <Typography.Text className="state-error-title">数据加载失败</Typography.Text>
        <Typography.Text type="secondary" className="state-copy">
          {description}
        </Typography.Text>
      </div>
      {onRetry ? (
        <Button size="small" icon={<ReloadOutlined />} onClick={onRetry}>
          重新加载
        </Button>
      ) : null}
    </div>
  )
}
