import { Card, Flex, Space, Tag, Typography } from 'antd'
import type { PropsWithChildren, ReactNode } from 'react'
import { StateBlocks } from './StateBlocks'

type PageFrameProps = PropsWithChildren<{
  title: string
  subtitle?: string
  module: string
  actions?: ReactNode
  tags?: string[]
  permission?: boolean
}>

export function PageFrame({
  title,
  subtitle,
  module,
  actions,
  tags = [],
  permission = true,
  children,
}: PageFrameProps) {
  return (
    <Space direction="vertical" size={16} className="page-stack">
      <Flex justify="space-between" gap={16} align="flex-start" wrap>
        <div>
          <Typography.Text type="secondary">{module}</Typography.Text>
          <Typography.Title level={2} className="page-title">
            {title}
          </Typography.Title>
          {subtitle ? <Typography.Paragraph>{subtitle}</Typography.Paragraph> : null}
          <Space wrap>
            {tags.map((tag) => (
              <Tag key={tag} color="blue">
                {tag}
              </Tag>
            ))}
          </Space>
        </div>
        {actions ? <Space wrap>{actions}</Space> : null}
      </Flex>
      <StateBlocks permission={permission} />
      <Card className="work-surface">{children}</Card>
    </Space>
  )
}
