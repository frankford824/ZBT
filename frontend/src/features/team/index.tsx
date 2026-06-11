import { Button, Card, Col, Form, Input, Row, Select, Table, Tabs, Tag, Timeline } from 'antd'
import { PageFrame } from '../../shared/components/PageFrame'

export function TeamPage() {
  return (
    <PageFrame
      module="企业管理"
      title="团队协作"
      subtitle="成员、审批、日志、通知和模块权限"
      tags={['page-team', '/team?tab=members|approvals|logs|notifications']}
      actions={[<Button key="invite" type="primary">邀请成员</Button>]}
    >
      <Tabs
        items={[
          {
            key: 'members',
            label: '成员',
            children: (
              <Table
                rowKey="name"
                rowSelection={{}}
                dataSource={[
                  { name: '陈思远', role: '企业管理员', modules: '全部' },
                  { name: '林悦', role: '项目经理', modules: '投标准备、投标管控' },
                  { name: '赵宁', role: '投标专员', modules: '标书、合规、知识库只读' },
                ]}
                columns={[
                  { title: '姓名', dataIndex: 'name' },
                  { title: '角色', dataIndex: 'role' },
                  { title: '模块权限', dataIndex: 'modules' },
                ]}
              />
            ),
          },
          {
            key: 'approvals',
            label: '审批',
            children: (
              <Row gutter={16}>
                <Col xs={24} xl={10}>
                  <Card title="审批链配置">
                    <Form layout="vertical">
                      <Form.Item label="第一级">
                        <Select defaultValue="部门主管" options={['部门主管', '技术负责人', '总经理'].map((value) => ({ value }))} />
                      </Form.Item>
                      <Form.Item label="金额阈值">
                        <Input defaultValue="1000000" />
                      </Form.Item>
                    </Form>
                  </Card>
                </Col>
                <Col xs={24} xl={14}>
                  <Card title="审批实例">
                    <Timeline
                      items={[
                        { color: 'green', children: '投标专员提交审批' },
                        { color: 'blue', children: '项目经理审核中' },
                        { color: 'gray', children: '总经理待审批' },
                      ]}
                    />
                  </Card>
                </Col>
              </Row>
            ),
          },
          {
            key: 'logs',
            label: '日志',
            children: (
              <Timeline
                items={[
                  { children: '陈思远创建标书 智慧交通技术标' },
                  { children: '林悦将项目状态推进到 compliance_review' },
                  { children: '赵宁导出合规检查报告' },
                ]}
              />
            ),
          },
          {
            key: 'notifications',
            label: '通知',
            children: (
              <Card>
                <Tag color="orange">资质即将到期</Tag>
                <Tag color="blue">审批待处理</Tag>
                <Tag color="green">合规检查完成</Tag>
              </Card>
            ),
          },
        ]}
      />
    </PageFrame>
  )
}
