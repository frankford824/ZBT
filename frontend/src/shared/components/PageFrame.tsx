import { Flex, Space, Typography } from 'antd'
import { Children, type PropsWithChildren, type ReactNode } from 'react'
import { ForbiddenBlock } from './StateBlocks'

type PageFrameProps = PropsWithChildren<{
  title: string
  subtitle?: string
  module: string
  actions?: ReactNode
  /** 页面标识，保留供验收脚本与测试定位，不再渲染到界面 */
  tags?: string[]
  permission?: boolean
  /** 工作区类页面不包裹白卡，由页面自行组织面板 */
  bare?: boolean
}>

export function PageFrame({
  title,
  subtitle,
  module,
  actions,
  tags = [],
  permission = true,
  bare = false,
  children,
}: PageFrameProps) {
  const actionItems = Children.toArray(actions).filter(Boolean)

  return (
    <Space orientation="vertical" size={16} className="page-stack" data-page-tags={tags.join(' ')}>
      <Flex justify="space-between" gap={16} align="flex-end" wrap>
        <div>
          <Typography.Text className="page-eyebrow">{module}</Typography.Text>
          <Typography.Title level={2} className="page-title">
            {title}
          </Typography.Title>
          {subtitle ? (
            <Typography.Paragraph className="page-subtitle">{subtitle}</Typography.Paragraph>
          ) : null}
        </div>
        {actionItems.length ? <Space wrap>{actionItems}</Space> : null}
      </Flex>
      {!permission ? (
        <ForbiddenBlock />
      ) : bare ? (
        children
      ) : (
        <div className="work-surface">{children}</div>
      )}
    </Space>
  )
}
