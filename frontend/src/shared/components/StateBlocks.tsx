import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  LoadingOutlined,
} from '@ant-design/icons'
import { Alert, Col, Empty, Row, Spin, Typography } from 'antd'

type StateBlocksProps = {
  permission?: boolean
}

export function StateBlocks({ permission = true }: StateBlocksProps) {
  if (!permission) {
    return (
      <Alert
        type="error"
        showIcon
        icon={<CloseCircleOutlined />}
        message="403"
        description="当前账号没有访问该模块的权限"
      />
    )
  }

  return (
    <Row gutter={[12, 12]} className="state-strip">
      <Col xs={24} md={6}>
        <div className="state-tile">
          <Spin indicator={<LoadingOutlined spin />} />
          <Typography.Text>加载中</Typography.Text>
        </div>
      </Col>
      <Col xs={24} md={6}>
        <div className="state-tile">
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无数据" />
        </div>
      </Col>
      <Col xs={24} md={6}>
        <div className="state-tile">
          <ExclamationCircleOutlined className="state-icon warning" />
          <Typography.Text>请求失败</Typography.Text>
        </div>
      </Col>
      <Col xs={24} md={6}>
        <div className="state-tile">
          <CheckCircleOutlined className="state-icon success" />
          <Typography.Text>权限通过</Typography.Text>
        </div>
      </Col>
    </Row>
  )
}

export function LoadingBlock() {
  return (
    <div className="state-tile">
      <Spin indicator={<LoadingOutlined spin />} />
      <Typography.Text>加载中</Typography.Text>
    </div>
  )
}

export function EmptyBlock() {
  return (
    <div className="state-tile">
      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无数据" />
    </div>
  )
}

export function ErrorBlock() {
  return (
    <Alert
      type="error"
      showIcon
      icon={<ExclamationCircleOutlined />}
      message="请求失败"
      description="请稍后重试或联系管理员"
    />
  )
}
