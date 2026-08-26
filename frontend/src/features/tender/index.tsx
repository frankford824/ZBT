import {
  Alert,
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
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  createBidFromTender,
  createProjectFromTender,
  createTender,
  createTenderSource,
  favoriteTender,
  fetchExternalTools,
  fetchTender,
  fetchTenderSources,
  fetchTenders,
  getApiErrorMessage,
  invokeExternalTool,
  unfavoriteTender,
  verifyTenderSource,
  type TenderDTO,
  type TenderSourceDTO,
} from '../../shared/api/client'
import { PageFrame } from '../../shared/components/PageFrame'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { useCanAccess } from '../../shared/permissions/permissions'
import { PlatformPoolPanel } from './PlatformPoolPanel'

const hallTabs = ['全部', '智能推荐', '可投标', '收藏', '公共标讯池', '外部标讯', '监控设置'] as const
type HallTab = (typeof hallTabs)[number]

function isHallTab(value: string | null): value is HallTab {
  return Boolean(value && (hallTabs as readonly string[]).includes(value))
}

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

const externalTenderProviderKey = 'handaas-bidding'
const externalTenderSearchTool = 'bid_bigdata_bid_search'

type ExternalTenderSearchValues = {
  keyword?: string
  region?: string
  publish_start?: string
  publish_end?: string
}

type ExternalTenderCandidate = {
  key: string
  title: string
  purchaser: string
  region: string
  budget_text: string
  publish_date: string | null
  deadline: string | null
  summary: string
  requirements: string[]
  source_url: string
  raw: Record<string, unknown>
}

const externalResultArrayKeys = [
  'items',
  'data',
  'list',
  'records',
  'resultList',
  'results',
  'rows',
  'tenders',
  'bids',
  'notices',
]
const externalTitleKeys = [
  'title',
  'name',
  'project_name',
  'projectName',
  'projectTitle',
  'tender_name',
  'bid_title',
  'notice_title',
  'announcement_title',
  'biddingAnncTitle',
  '公告标题',
  '项目名称',
  '标讯名称',
]
const externalPurchaserKeys = [
  'purchaser',
  'buyer',
  'buyer_name',
  'owner',
  'tenderer',
  'agency',
  'biddingPurchaser',
  'biddingPurchaserName',
  '招标人',
  '采购人',
  '采购单位',
  '招标单位',
]
const externalRegionKeys = ['region', 'area', 'province', 'city', 'location', 'biddingRegion', '地区', '项目地区']
const externalBudgetKeys = [
  'budget',
  'budget_text',
  'budgetText',
  'amount',
  'project_amount',
  'biddingProjectAmount',
  '预算',
  '预算金额',
  '项目金额',
]
const externalPublishDateKeys = ['publish_date', 'publishDate', 'published_at', 'biddingPublishTime', '公告时间', '发布日期']
const externalDeadlineKeys = ['deadline', 'end_date', 'endDate', 'biddingEndTime', '截止时间', '投标截止']
const externalURLKeys = ['source_url', 'sourceUrl', 'url', 'link', 'detail_url', 'detailUrl', 'biddingUrl', '公告链接']
const externalSummaryKeys = ['summary', 'description', 'content', 'biddingContent', 'abstract', '正文', '摘要']
const externalRequirementKeys = ['requirements', 'qualification', 'biddingInfoType', 'biddingProjectType', '关键要求', '公告类型']

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function textValue(value: unknown): string {
  if (typeof value === 'string') return value.trim()
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return ''
}

function firstText(record: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const direct = textValue(record[key])
    if (direct) return direct
  }
  const wanted = new Set(keys.map((key) => key.toLowerCase()))
  for (const [key, value] of Object.entries(record)) {
    if (wanted.has(key.toLowerCase())) {
      const text = textValue(value)
      if (text) return text
    }
  }
  return ''
}

function parsePotentialJSON(text: string): unknown {
  const trimmed = text.trim()
  if (!trimmed || (!trimmed.startsWith('{') && !trimmed.startsWith('['))) {
    return null
  }
  try {
    return JSON.parse(trimmed)
  } catch {
    return null
  }
}

function collectExternalRecords(value: unknown, depth = 0): Record<string, unknown>[] {
  if (depth > 5 || value == null) return []
  if (typeof value === 'string') {
    return collectExternalRecords(parsePotentialJSON(value), depth + 1)
  }
  if (Array.isArray(value)) {
    return value.flatMap((item) => collectExternalRecords(item, depth + 1))
  }
  const record = asRecord(value)
  if (!record) return []

  const nested: Record<string, unknown>[] = []
  for (const key of externalResultArrayKeys) {
    const nestedValue = record[key]
    if (nestedValue !== undefined) {
      nested.push(...collectExternalRecords(nestedValue, depth + 1))
    }
  }
  const content = record.content
  if (Array.isArray(content)) {
    for (const item of content) {
      const contentRecord = asRecord(item)
      if (contentRecord) {
        nested.push(...collectExternalRecords(contentRecord.text, depth + 1))
      }
    }
  }

  const title = firstText(record, externalTitleKeys)
  const url = firstText(record, externalURLKeys)
  if (title || url) {
    return [record, ...nested]
  }
  return nested
}

function normalizeExternalDate(value: string): string | null {
  if (!value) return null
  const normalized = value.replace(/[年月/.]/g, '-').replace(/日/g, '')
  const match = normalized.match(/(\d{4})-(\d{1,2})-(\d{1,2})/)
  if (!match) return null
  const [, year, month, day] = match
  return `${year}-${month.padStart(2, '0')}-${day.padStart(2, '0')}`
}

function normalizeExternalURL(value: string): string {
  if (!value) return ''
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.toString() : ''
  } catch {
    return ''
  }
}

function listFromExternalValue(value: string): string[] {
  if (!value) return []
  return value
    .split(/[\n,，;；]/)
    .map((item) => item.trim())
    .filter(Boolean)
    .slice(0, 6)
}

function externalCandidateKey(record: Record<string, unknown>, index: number) {
  return (
    firstText(record, ['id', 'biddingId', 'biddingProjectID', 'project_id', 'notice_id', '公告id', '项目编号']) ||
    `${firstText(record, externalTitleKeys)}-${firstText(record, externalDeadlineKeys)}-${index}`
  )
}

function normalizeExternalTenderCandidates(result: unknown): ExternalTenderCandidate[] {
  const records = collectExternalRecords(result)
  const seen = new Set<string>()
  const candidates: ExternalTenderCandidate[] = []
  records.forEach((record, index) => {
    const title = firstText(record, externalTitleKeys)
    if (!title) return
    const key = externalCandidateKey(record, index)
    if (seen.has(key)) return
    seen.add(key)
    const requirementText = firstText(record, externalRequirementKeys)
    const summary = firstText(record, externalSummaryKeys)
    candidates.push({
      key,
      title,
      purchaser: firstText(record, externalPurchaserKeys),
      region: firstText(record, externalRegionKeys),
      budget_text: firstText(record, externalBudgetKeys),
      publish_date: normalizeExternalDate(firstText(record, externalPublishDateKeys)),
      deadline: normalizeExternalDate(firstText(record, externalDeadlineKeys)),
      summary: summary.length > 500 ? `${summary.slice(0, 500)}...` : summary,
      requirements: listFromExternalValue(requirementText),
      source_url: normalizeExternalURL(firstText(record, externalURLKeys)),
      raw: record,
    })
  })
  return candidates.slice(0, 50)
}

function buildExternalTenderSearchArguments(values: ExternalTenderSearchValues) {
  return {
    matchKeyword: values.keyword?.trim() || undefined,
    biddingRegion: values.region?.trim() || undefined,
    biddingAnncPubStartTime: values.publish_start?.trim() || undefined,
    biddingAnncPubEndTime: values.publish_end?.trim() || undefined,
    searchMode: '全文匹配',
    pageIndex: 1,
    pageSize: 20,
  }
}

export function TendersPage() {
  const { message } = AntApp.useApp()
  const queryClient = useQueryClient()
  const canWrite = useCanAccess('tender', 'full')
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab: HallTab = isHallTab(searchParams.get('tab')) ? (searchParams.get('tab') as HallTab) : '全部'
  const [keyword, setKeyword] = useState('')
  const [sourceForm] = Form.useForm()
  const [externalSearchForm] = Form.useForm<ExternalTenderSearchValues>()
  const [externalCandidates, setExternalCandidates] = useState<ExternalTenderCandidate[]>([])
  const tenders = useQuery({
    queryKey: ['tenders', activeTab, keyword],
    queryFn: () => fetchTenders({ ...tabParams[activeTab], q: keyword || undefined }),
    enabled: activeTab in tabParams,
  })
  const sources = useQuery({
    queryKey: ['tender-sources'],
    queryFn: fetchTenderSources,
  })
  const externalTools = useQuery({
    queryKey: ['external-tools', 'tender-search'],
    queryFn: fetchExternalTools,
    enabled: activeTab === '外部标讯',
  })
  const externalTenderConfig = useMemo(
    () => externalTools.data?.find((item) => item.provider_key === externalTenderProviderKey),
    [externalTools.data],
  )
  const externalTenderReady = Boolean(
    externalTenderConfig?.enabled && externalTenderConfig.allowed_tools.includes(externalTenderSearchTool),
  )
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
  const externalSearchMutation = useMutation({
    mutationFn: (values: ExternalTenderSearchValues) =>
      invokeExternalTool(externalTenderProviderKey, {
        tool_name: externalTenderSearchTool,
        arguments: buildExternalTenderSearchArguments(values),
        resource_type: 'tender_external_search',
      }),
    onSuccess: (result) => {
      const candidates = normalizeExternalTenderCandidates(result.result)
      setExternalCandidates(candidates)
      message.success(candidates.length ? `找到 ${candidates.length} 条外部标讯` : '未识别到可保存的标讯')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '外部标讯检索失败')),
  })
  const importExternalTenderMutation = useMutation({
    mutationFn: (candidate: ExternalTenderCandidate) =>
      createTender({
        title: candidate.title,
        purchaser: candidate.purchaser,
        region: candidate.region,
        budget_text: candidate.budget_text,
        publish_date: candidate.publish_date,
        deadline: candidate.deadline,
        status: 'open',
        match_score: 70,
        summary: candidate.summary,
        requirements: candidate.requirements,
        risk_flags: [],
        source_url: candidate.source_url,
        metadata: {
          source_type: 'external_mcp',
          external_mcp: {
            provider_key: externalTenderProviderKey,
            tool_name: externalTenderSearchTool,
            imported_at: new Date().toISOString(),
            candidate_key: candidate.key,
            source_snapshot: {
              title: candidate.title,
              purchaser: candidate.purchaser,
              region: candidate.region,
              budget_text: candidate.budget_text,
              publish_date: candidate.publish_date,
              deadline: candidate.deadline,
              source_url: candidate.source_url,
            },
          },
        },
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tenders'] })
      message.success('已保存为标讯')
    },
    onError: (error) => message.error(getApiErrorMessage(error, '保存标讯失败')),
  })

  const tenderTable = () => (
    <Space direction="vertical" size={16} className="full-width">
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

  const externalSearchPanel = () => (
    <Space direction="vertical" size={16} className="full-width">
      <Card title="公共数据源检索">
        <Space direction="vertical" size={16} className="full-width">
          {!externalTenderReady ? (
            <Alert
              type="warning"
              showIcon
              message="外部标讯数据源尚未启用"
              description={
                <Space wrap>
                  <span>请先由管理员在团队管理中启用中国招投标数据源。</span>
                  <Link to="/team?tab=external-tools">前往配置</Link>
                </Space>
              }
            />
          ) : null}
          <Form
            form={externalSearchForm}
            layout="inline"
            onFinish={(values) => externalSearchMutation.mutate(values)}
            disabled={!externalTenderReady}
          >
            <Form.Item name="keyword" rules={[{ required: true, message: '请输入关键词' }]}>
              <Input placeholder="项目、行业、采购人关键词" allowClear />
            </Form.Item>
            <Form.Item name="region">
              <Input placeholder="地区" allowClear />
            </Form.Item>
            <Form.Item name="publish_start">
              <Input placeholder="发布日期起 YYYY-MM-DD" allowClear />
            </Form.Item>
            <Form.Item name="publish_end">
              <Input placeholder="发布日期止 YYYY-MM-DD" allowClear />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" loading={externalSearchMutation.isPending}>
                检索标讯
              </Button>
            </Form.Item>
          </Form>
        </Space>
      </Card>
      {externalSearchMutation.isPending ? <LoadingBlock /> : null}
      {!externalSearchMutation.isPending && externalCandidates.length === 0 ? (
        <EmptyBlock description="输入关键词后可从已授权公共数据源检索标讯" />
      ) : null}
      {externalCandidates.length ? (
        <Table
          rowKey="key"
          dataSource={externalCandidates}
          scroll={{ x: 980 }}
          columns={[
            {
              title: '标讯名称',
              dataIndex: 'title',
              width: 300,
              render: (value) => value || '-',
            },
            { title: '招标单位', dataIndex: 'purchaser', width: 180, render: (value) => value || '-' },
            { title: '地区', dataIndex: 'region', width: 120, render: (value) => value || '-' },
            { title: '预算', dataIndex: 'budget_text', width: 140, render: (value) => value || '-' },
            { title: '发布日期', dataIndex: 'publish_date', width: 120, render: dateText },
            { title: '截止日期', dataIndex: 'deadline', width: 120, render: dateText },
            {
              title: '操作',
              width: 130,
              fixed: 'right',
              render: (_, row) => (
                <Button
                  type="link"
                  disabled={!canWrite}
                  loading={importExternalTenderMutation.isPending && importExternalTenderMutation.variables?.key === row.key}
                  onClick={() => importExternalTenderMutation.mutate(row)}
                >
                  保存为标讯
                </Button>
              ),
            },
          ]}
          expandable={{
            expandedRowRender: (row) => (
              <Space direction="vertical" size={8} className="full-width">
                <span>{row.summary || '暂无摘要'}</span>
                {row.requirements.length ? (
                  <Space wrap>
                    {row.requirements.map((item) => (
                      <Tag key={item}>{item}</Tag>
                    ))}
                  </Space>
                ) : null}
              </Space>
            ),
          }}
        />
      ) : null}
    </Space>
  )

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
      subtitle="租户标讯、采集公共池、第三方检索和来源管理"
      tags={['标讯列表']}
    >
      <Tabs
        activeKey={activeTab}
        onChange={(tab) => setSearchParams(tab === '全部' ? {} : { tab })}
        items={hallTabs.map((label) => ({
          key: label,
          label,
          children:
            label === '监控设置'
              ? sourcePanel()
              : label === '外部标讯'
                ? externalSearchPanel()
                : label === '公共标讯池'
                  ? <PlatformPoolPanel />
                  : tenderTable(),
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
