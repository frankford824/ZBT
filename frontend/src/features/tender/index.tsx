import {
  App as AntApp,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  List,
  Row,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  createBidFromTender,
  createProjectFromTender,
  createTenderSource,
  favoriteTender,
  fetchTender,
  fetchTenderSources,
  fetchTenders,
  getApiErrorMessage,
  unfavoriteTender,
  verifyTenderSource,
  type TenderDTO,
  type TenderSourceDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { useCanAccess } from '../../shared/permissions/permissions'

const tabParams: Record<string, Parameters<typeof fetchTenders>[0]> = {
  全部: {},
  智能推荐: { recommended: true },
  可投标: { status: 'open' },
  收藏: { favorite: true },
}

function dateText(value?: string | null) {
  return value ? value.slice(0, 10) : '-'
}

function tenderStatusLabel(value: string) {
  const labels: Record<string, string> = {
    open: '可投标',
    closed: '已截止',
    monitored: '已监控',
    favorite: '已收藏',
  }
  return labels[value] || '待确认'
}

export function TendersPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canWrite = useCanAccess('tender', 'full')
  const [activeTab, setActiveTab] = useState('全部')
  const [keyword, setKeyword] = useState('')
  const [sourceForm] = Form.useForm()
  const tenders = useQuery({
    queryKey: ['tenders', activeTab, keyword],
    queryFn: () => fetchTenders({ ...tabParams[activeTab], q: keyword || undefined }),
    enabled: activeTab !== '监控设置',
  })
  const sources = useQuery({
    queryKey: ['tender-sources'],
    queryFn: fetchTenderSources,
  })
  const favoriteMutation = useMutation({
    mutationFn: (tender: TenderDTO) => (tender.favorite ? unfavoriteTender(tender.id) : favoriteTender(tender.id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tenders'] })
      message.success('收藏状态已更新')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '收藏更新失败')),
  })
  const sourceMutation = useMutation({
    mutationFn: createTenderSource,
    onSuccess: () => {
      sourceForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['tender-sources'] })
      message.success('来源已添加')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '来源添加失败')),
  })
  const verifyMutation = useMutation({
    mutationFn: verifyTenderSource,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tender-sources'] })
      message.success('检测完成')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '检测失败')),
  })

  const tenderTable = () => (
    <Space orientation="vertical" size={16} className="full-width">
      <Input.Search
        allowClear
        placeholder="搜索标讯名称、招标单位关键词"
        style={{ maxWidth: 360 }}
        onSearch={(value) => setKeyword(value.trim())}
      />
      {tenderTableBody()}
    </Space>
  )

  const tenderTableBody = () => {
    if (tenders.isLoading) return <LoadingBlock />
    if (tenders.isError) return <ErrorBlock />
    if (!tenders.data?.length) return <EmptyBlock description={keyword ? `没有匹配「${keyword}」的标讯` : undefined} />
    return (
      <Table
        rowKey="id"
        dataSource={tenders.data}
        scroll={{ x: 860 }}
        columns={[
          {
            title: '标讯名称',
            dataIndex: 'title',
            width: 280,
            render: (value, row) => <Link to={`/tenders/${row.id}`}>{value}</Link>,
          },
          { title: '地区', dataIndex: 'region', width: 120 },
          { title: '预算', dataIndex: 'budget_text', width: 140 },
          {
            title: '匹配度',
            dataIndex: 'match_score',
            width: 100,
            render: (value) => <Tag color="purple">{value}%</Tag>,
          },
          { title: '截止日期', dataIndex: 'deadline', width: 120, render: dateText },
          {
            title: '操作',
            width: 100,
            render: (_, row) => (
              <Button
                type="link"
                loading={favoriteMutation.isPending && favoriteMutation.variables?.id === row.id}
                onClick={() => favoriteMutation.mutate(row)}
              >
                {row.favorite ? '取消收藏' : '收藏'}
              </Button>
            ),
          },
        ]}
      />
    )
  }

  const sourcePanel = () => (
    <Row gutter={[16, 16]}>
      {canWrite ? (
        <Col xs={24} lg={10}>
          <Card title="标讯来源">
            <Form form={sourceForm} layout="vertical" onFinish={sourceMutation.mutate}>
              <Form.Item name="name" label="平台名称" rules={[{ required: true, message: '平台名称必填' }]}>
                <Input placeholder="中国招标投标公共服务平台" />
              </Form.Item>
              <Form.Item name="url" label="平台网址" rules={[{ required: true, message: '平台网址必填' }]}>
                <Input placeholder="https://www.example.com" />
              </Form.Item>
              <Form.Item name="source_type" label="平台类型" initialValue="政府采购">
                <Select options={['政府采购', '建设工程', '产权交易', '公共资源', '其他'].map((value) => ({ value }))} />
              </Form.Item>
              <Button htmlType="submit" type="primary" loading={sourceMutation.isPending}>
                添加来源
              </Button>
            </Form>
          </Card>
        </Col>
      ) : null}
      <Col xs={24} lg={canWrite ? 14 : 24}>
        <Card title="已添加来源">
          {sources.isLoading && <LoadingBlock />}
          {sources.isError && <ErrorBlock />}
          {!sources.isLoading && !sources.isError && !sources.data?.length && <EmptyBlock />}
          <List
            dataSource={sources.data}
            renderItem={(source: TenderSourceDTO) => (
              <List.Item
                actions={
                  canWrite
                    ? [
                        <Button
                          key="verify"
                          type="link"
                          loading={verifyMutation.isPending && verifyMutation.variables === source.id}
                          onClick={() => verifyMutation.mutate(source.id)}
                        >
                          检测连接
                        </Button>,
                      ]
                    : undefined
                }
              >
                <List.Item.Meta
                  title={
                    <Space>
                      {source.name}
                      <Tag color={source.status === 'active' ? 'green' : 'red'}>
                        {source.status === 'active' ? '可用' : '需检查'}
                      </Tag>
                    </Space>
                  }
                  description={`${source.source_type} · ${source.url} · ${source.last_verify_message || '未检测'}`}
                />
              </List.Item>
            )}
          />
        </Card>
      </Col>
    </Row>
  )

  return (
    <PageFrame
      module="投标准备"
      title="标讯大厅"
      subtitle="标讯搜索、智能推荐、收藏和来源管理"
      tags={['标讯列表']}
    >
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={['全部', '智能推荐', '可投标', '收藏', '监控设置'].map((label) => ({
          key: label,
          label,
          children: label === '监控设置' ? sourcePanel() : tenderTable(),
        }))}
      />
    </PageFrame>
  )
}

export function TenderDetailPage() {
  const { tenderId } = useParams()
  const navigate = useNavigate()
  const { message } = AntApp.useApp()
  const canWrite = useCanAccess('tender', 'full')
  const tender = useQuery({
    queryKey: ['tender', tenderId],
    queryFn: () => fetchTender(tenderId || ''),
    enabled: Boolean(tenderId),
  })
  const projectMutation = useMutation({
    mutationFn: () => createProjectFromTender(tenderId || ''),
    onSuccess: (project) => {
      message.success('项目已创建')
      navigate(`/projects/${project.id}`)
    },
    onError: (error) => message.error(getApiErrorMessage(error, '创建项目失败')),
  })
  const bidMutation = useMutation({
    mutationFn: () => createBidFromTender(tenderId || ''),
    onSuccess: ({ bid }) => {
      message.success('标书已创建')
      navigate(`/bids/${bid.id}/wizard?step=1`)
    },
    onError: (error) => message.error(getApiErrorMessage(error, '生成标书失败')),
  })

  if (tender.isLoading) return <LoadingBlock />
  if (tender.isError) return <ErrorBlock />
  if (!tender.data) return <EmptyBlock />

  return (
    <PageFrame
      module="标讯大厅"
      title={tender.data.title}
      subtitle={tender.data.source_name || tender.data.region}
      tags={['招标详情']}
      actions={
        canWrite
          ? [
              <Button key="project" loading={projectMutation.isPending} onClick={() => projectMutation.mutate()}>
                创建项目
              </Button>,
              <Button key="bid" type="primary" loading={bidMutation.isPending} onClick={() => bidMutation.mutate()}>
                生成标书
              </Button>,
            ]
          : undefined
      }
    >
      <Descriptions bordered column={2}>
        <Descriptions.Item label="招标单位">{tender.data.purchaser || '-'}</Descriptions.Item>
        <Descriptions.Item label="预算金额">{tender.data.budget_text || '-'}</Descriptions.Item>
        <Descriptions.Item label="投标截止">{dateText(tender.data.deadline)}</Descriptions.Item>
        <Descriptions.Item label="匹配度">{tender.data.match_score}%</Descriptions.Item>
        <Descriptions.Item label="地区">{tender.data.region || '-'}</Descriptions.Item>
        <Descriptions.Item label="状态">{tenderStatusLabel(tender.data.status)}</Descriptions.Item>
        <Descriptions.Item label="关键要求" span={2}>
          <Space wrap>
            {tender.data.requirements.map((item) => (
              <Tag key={item}>{item}</Tag>
            ))}
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label="废标条款" span={2}>
          <Space wrap>
            {tender.data.risk_flags.map((item) => (
              <Tag key={item} color="red">
                {item}
              </Tag>
            ))}
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label="摘要" span={2}>
          {tender.data.summary || '-'}
        </Descriptions.Item>
      </Descriptions>
    </PageFrame>
  )
}
