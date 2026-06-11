import {
  DiffOutlined,
  DownloadOutlined,
  FileZipOutlined,
  PlusOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import { zodResolver } from '@hookform/resolvers/zod'
import { EditorContent, useEditor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import {
  Button,
  Card,
  Col,
  Form,
  Input,
  Progress,
  Radio,
  Row,
  Segmented,
  Space,
  Steps,
  Table,
  Tabs,
  Tag,
  Timeline,
  Typography,
} from 'antd'
import { Controller, useForm } from 'react-hook-form'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { z } from 'zod'
import { PageFrame } from '../../shared/components/PageFrame'

const bidRows = [
  { id: 'bid-demo', name: '智慧交通平台技术标', project: '智慧交通综合治理', type: '技术标', status: '编制中', progress: 62 },
  { id: 'bid-2', name: '政务云迁移商务标', project: '政务云迁移服务', type: '商务标', status: '审核中', progress: 84 },
]

const bidSchema = z.object({
  projectName: z.string().min(1, '项目名称必填'),
  tenderOrg: z.string().optional(),
  deadline: z.string().optional(),
  budget: z.string().optional(),
  bidType: z.enum(['combined', 'separated', 'custom']),
})

type BidFormValues = z.infer<typeof bidSchema>

export function BidNewPage() {
  const navigate = useNavigate()
  const {
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<BidFormValues>({
    resolver: zodResolver(bidSchema),
    defaultValues: {
      projectName: '',
      tenderOrg: '',
      deadline: '',
      budget: '',
      bidType: 'separated',
    },
  })

  return (
    <PageFrame
      module="标书生成"
      title="新建标书"
      subtitle="创建综合标书、分离标书或自定义组合"
      tags={['page-generate-new', '/bids/new']}
    >
      <Row gutter={16}>
        <Col xs={24} lg={15}>
          <Form layout="vertical" onFinish={handleSubmit(() => navigate('/bids/bid-demo/wizard?step=1'))}>
            <Form.Item
              label="项目名称"
              validateStatus={errors.projectName ? 'error' : undefined}
              help={errors.projectName?.message}
            >
              <Controller
                name="projectName"
                control={control}
                render={({ field }) => <Input {...field} placeholder="某市智慧交通综合治理平台建设项目" />}
              />
            </Form.Item>
            <Form.Item label="招标单位">
              <Controller
                name="tenderOrg"
                control={control}
                render={({ field }) => <Input {...field} placeholder="某市交通运输局" />}
              />
            </Form.Item>
            <Row gutter={12}>
              <Col span={12}>
                <Form.Item label="投标截止日期">
                  <Controller
                    name="deadline"
                    control={control}
                    render={({ field }) => <Input {...field} placeholder="2026-06-18" />}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="项目预算">
                  <Controller
                    name="budget"
                    control={control}
                    render={({ field }) => <Input {...field} placeholder="12800000" />}
                  />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item label="标书类型">
              <Controller
                name="bidType"
                control={control}
                render={({ field }) => (
                  <Radio.Group {...field}>
                    <Radio.Button value="combined">综合标书</Radio.Button>
                    <Radio.Button value="separated">分离标书</Radio.Button>
                    <Radio.Button value="custom">自定义组合</Radio.Button>
                  </Radio.Group>
                )}
              />
            </Form.Item>
            <Space>
              <Button>
                <Link to="/bids">取消</Link>
              </Button>
              <Button type="primary" htmlType="submit" icon={<PlusOutlined />}>
                开始生成
              </Button>
            </Space>
          </Form>
        </Col>
        <Col xs={24} lg={9}>
          <Card title="生成队列">
            <Timeline
              items={[
                { color: 'blue', children: '创建 bid_document 草稿' },
                { color: 'blue', children: '按类型创建 bid_parts' },
                { color: 'gray', children: '上传招标文件并触发解析' },
                { color: 'gray', children: '进入 7 步向导' },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </PageFrame>
  )
}

export function BidListPage() {
  return (
    <PageFrame
      module="标书生成"
      title="我的标书"
      subtitle="状态筛选、批量导出、审批和编辑入口"
      tags={['page-generate-list', '/bids']}
      actions={[
        <Button key="new" type="primary" icon={<PlusOutlined />}>
          <Link to="/bids/new">新建标书</Link>
        </Button>,
      ]}
    >
      <Space direction="vertical" size={16} className="full-width">
        <Segmented options={['全部', '编制中', '审核中', '已完成']} />
        <Table
          rowKey="id"
          rowSelection={{}}
          dataSource={bidRows}
          columns={[
            {
              title: '标书名称',
              dataIndex: 'name',
              render: (value, row) => <Link to={`/bids/${row.id}/editor`}>{value}</Link>,
            },
            { title: '项目', dataIndex: 'project' },
            { title: '类型', dataIndex: 'type' },
            { title: '状态', dataIndex: 'status' },
            {
              title: '进度',
              dataIndex: 'progress',
              render: (value) => <Progress percent={value} size="small" />,
            },
            {
              title: '操作',
              render: (_, row) => (
                <Space>
                  <Link to={`/bids/${row.id}/wizard?step=5`}>生成</Link>
                  <Link to={`/bids/${row.id}/editor`}>编辑</Link>
                </Space>
              ),
            },
          ]}
        />
      </Space>
    </PageFrame>
  )
}

export function BidTemplatesPage() {
  return (
    <PageFrame
      module="标书生成"
      title="标书模板"
      subtitle="行业分类、预览和使用"
      tags={['page-generate-templates', '/bids/templates']}
    >
      <Row gutter={[16, 16]}>
        {['IT信息化综合标', '政府采购商务标', '工程建设技术标', '我的实施方案模板'].map((name, index) => (
          <Col xs={24} md={12} xl={6} key={name}>
            <Card title={name} actions={[<Link to="/bids/new">使用</Link>, <a>预览</a>]}>
              <Space direction="vertical">
                <Tag color="blue">{index === 3 ? '我的模板' : '行业模板'}</Tag>
                <Typography.Text type="secondary">使用 {120 - index * 17} 次</Typography.Text>
                <Typography.Text>评分 {4.8 - index * 0.2}</Typography.Text>
              </Space>
            </Card>
          </Col>
        ))}
      </Row>
    </PageFrame>
  )
}

export function BidWizardPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const step = Number(searchParams.get('step') || '1')
  const current = Math.min(Math.max(step, 1), 7) - 1
  const steps = ['项目信息', 'AI解析', '目录大纲', '知识库配置', '逐章生成', '标书编辑器', '导出提交']

  return (
    <PageFrame
      module="标书生成"
      title="标书生成 7 步向导"
      subtitle="分离标书支持技术标和商务标独立生成"
      tags={['page-generate', '/bids/:bidId/wizard?step=1..7']}
    >
      <Space direction="vertical" size={20} className="full-width">
        <Steps
          current={current}
          items={steps.map((title) => ({ title }))}
          onChange={(next) => setSearchParams({ step: String(next + 1) })}
        />
        <Card title={steps[current]}>
          {current === 0 ? <Button icon={<UploadOutlined />}>上传招标文件</Button> : null}
          {current === 1 ? (
            <Table
              pagination={false}
              rowKey="name"
              dataSource={[
                { name: '项目名称', value: '智慧交通综合治理平台建设' },
                { name: '废标条款', value: '签章、有效期、报价一致性' },
                { name: '评分重点', value: '实施方案、数据安全、运维能力' },
              ]}
              columns={[
                { title: '字段', dataIndex: 'name' },
                { title: '解析结果', dataIndex: 'value' },
              ]}
            />
          ) : null}
          {current === 4 ? (
            <Tabs
              items={[
                { key: 'tech', label: '技术标', children: <Progress percent={66} /> },
                { key: 'business', label: '商务标', children: <Progress percent={42} /> },
              ]}
            />
          ) : null}
          {current === 5 ? (
            <Button type="primary">
              <Link to="/bids/bid-demo/editor?part=tech">进入编辑器</Link>
            </Button>
          ) : null}
          {current === 6 ? (
            <Space>
              <Button icon={<DownloadOutlined />}>导出技术标.docx</Button>
              <Button icon={<DownloadOutlined />}>导出商务标.docx</Button>
              <Button icon={<FileZipOutlined />}>打包 ZIP</Button>
            </Space>
          ) : null}
        </Card>
      </Space>
    </PageFrame>
  )
}

export function BidEditorPage() {
  const { bidId } = useParams()
  const editor = useEditor({
    extensions: [StarterKit],
    content:
      '<h2>第四章 技术方案</h2><p>本章节基于企业知识库引用素材生成，事实性内容需要保留 source_refs。</p>',
  })

  return (
    <PageFrame
      module="标书生成"
      title="标书编辑器"
      subtitle={bidId}
      tags={['/bids/:bidId/editor', 'Tiptap']}
      actions={[
        <Button key="diff" icon={<DiffOutlined />}>
          查看差异
        </Button>,
        <Button key="export" type="primary" icon={<DownloadOutlined />}>
          导出 docx
        </Button>,
      ]}
    >
      <Row gutter={16}>
        <Col xs={24} xl={5}>
          <Card title="章节大纲">
            <Timeline
              items={[
                { color: 'green', children: '一、项目理解' },
                { color: 'green', children: '二、总体架构' },
                { color: 'blue', children: '三、实施方案' },
                { color: 'orange', children: '四、运维服务' },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} xl={13}>
          <Card title="内容编辑">
            <EditorContent editor={editor} className="editor-surface" />
          </Card>
        </Col>
        <Col xs={24} xl={6}>
          <Card title="AI 助手">
            <Space direction="vertical">
              <Tag color="purple">source_refs: 5</Tag>
              <Tag color="orange">needs_human_input: 2</Tag>
              <Button>优化文字</Button>
              <Button>查找类似案例</Button>
              <Button>检查当前章节</Button>
            </Space>
          </Card>
        </Col>
      </Row>
    </PageFrame>
  )
}
