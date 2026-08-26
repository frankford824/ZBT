import { isAxiosError } from 'axios'
import { Alert, Button, Input, Select, Space, Table, Tag, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import {
  fetchPlatformCollectorRuns,
  fetchPlatformTenders,
  getApiErrorMessage,
  type PlatformCollectorRunDTO,
  type PlatformTenderDTO,
} from '../../shared/api/client'
import { EmptyBlock, ErrorBlock, LoadingBlock } from '../../shared/components/StateBlocks'
import { formatDateOnly, formatDateTime } from '../../shared/format/date'

const pageSize = 20

const sourceLabels: Record<string, string> = {
  zbcg: '江苏交控 zbcg',
  iccec: '中交招采 iccec',
}

const runStatusMeta: Record<string, { color: string; label: string }> = {
  ok: { color: 'green', label: '正常' },
  partial: { color: 'gold', label: '部分失败' },
  failed: { color: 'red', label: '失败' },
  blocked: { color: 'volcano', label: '被拦截' },
}

function latestRunBySource(runs: PlatformCollectorRunDTO[]) {
  const latest = new Map<string, PlatformCollectorRunDTO>()
  for (const run of runs) {
    if (!latest.has(run.external_source)) latest.set(run.external_source, run)
  }
  return [...latest.values()]
}

function dimPreview(dims: Record<string, unknown>) {
  const extractedBy = typeof dims.extracted_by === 'string' ? dims.extracted_by : ''
  const confidence = typeof dims.confidence === 'string' ? dims.confidence : ''
  const nested = dims.dims && typeof dims.dims === 'object' ? (dims.dims as Record<string, unknown>) : {}
  const filled = Object.entries(nested)
    .filter(([, value]) => typeof value === 'string' && value.trim())
    .map(([key]) => key)
  return { extractedBy, confidence, filled }
}

function timelineEntries(timeline: Record<string, unknown>) {
  const labels: Record<string, string> = {
    submit_deadline: '递交截止',
    bid_opening: '开标',
    inquiry_deadline: '答疑截止',
    file_sale_end: '文件发售截止',
    objection_deadline: '异议截止',
  }
  return Object.entries(labels)
    .map(([key, label]) => {
      const value = timeline[key]
      return typeof value === 'string' && value.trim() ? { label, value } : null
    })
    .filter((item): item is { label: string; value: string } => Boolean(item))
}

export function PlatformPoolPanel() {
  const [keyword, setKeyword] = useState('')
  const [source, setSource] = useState<string>()
  const [page, setPage] = useState(1)

  const tenders = useQuery({
    queryKey: ['platform-tenders', keyword, source, page],
    queryFn: () =>
      fetchPlatformTenders({
        q: keyword || undefined,
        source: source || undefined,
        limit: pageSize,
        offset: (page - 1) * pageSize,
      }),
  })
  const runs = useQuery({
    queryKey: ['platform-collector-runs'],
    queryFn: () => fetchPlatformCollectorRuns({ limit: 20 }),
  })

  const forbidden =
    (isAxiosError(tenders.error) && tenders.error.response?.status === 403) ||
    (isAxiosError(runs.error) && runs.error.response?.status === 403)

  const sourceRuns = useMemo(() => latestRunBySource(runs.data ?? []), [runs.data])
  const unhealthy = sourceRuns.filter((run) => run.status !== 'ok')

  if (forbidden) {
    return (
      <Alert
        type="warning"
        showIcon
        message="平台公共标讯池当前未开放"
        description="需要后端开启 PLATFORM_TENDER_POOL_PUBLIC 后，采集器推入的公开标讯才会在这里显示。这和「外部标讯」的第三方检索不是同一条链路。"
      />
    )
  }

  return (
    <Space direction="vertical" size={16} className="full-width">
      {unhealthy.length ? (
        <Alert
          type="error"
          showIcon
          message="采集源异常"
          description={unhealthy
            .map((run) => {
              const name = sourceLabels[run.external_source] || run.external_source
              const status = runStatusMeta[run.status]?.label || run.status
              return `${name} ${status}${run.message ? `：${run.message}` : ''}`
            })
            .join('；')}
        />
      ) : null}
      {sourceRuns.length ? (
        <Space wrap>
          {sourceRuns.map((run) => {
            const meta = runStatusMeta[run.status] || { color: 'default', label: run.status }
            return (
              <Tag key={run.id} color={meta.color}>
                {sourceLabels[run.external_source] || run.external_source} · {meta.label} · 最近{' '}
                {formatDateTime(run.finished_at)} · 入库 {run.ingested_count}
              </Tag>
            )
          })}
        </Space>
      ) : null}
      <Space wrap>
        <Input.Search
          allowClear
          placeholder="搜索标题或采购人"
          style={{ width: 320 }}
          onSearch={(value) => {
            setKeyword(value.trim())
            setPage(1)
          }}
        />
        <Select
          allowClear
          placeholder="全部来源"
          style={{ width: 200 }}
          value={source}
          onChange={(value) => {
            setSource(value)
            setPage(1)
          }}
          options={[
            { value: 'zbcg', label: sourceLabels.zbcg },
            { value: 'iccec', label: sourceLabels.iccec },
          ]}
        />
        <Button
          onClick={() => {
            tenders.refetch()
            runs.refetch()
          }}
        >
          刷新
        </Button>
      </Space>
      {tenders.isLoading ? <LoadingBlock /> : null}
      {tenders.isError ? (
        <ErrorBlock
          description={getApiErrorMessage(tenders.error, '公共标讯池加载失败')}
          onRetry={() => tenders.refetch()}
        />
      ) : null}
      {!tenders.isLoading && !tenders.isError && !tenders.data?.items.length ? (
        <EmptyBlock description={keyword || source ? '没有匹配的采集标讯' : '采集器还没有推入公开标讯'} />
      ) : null}
      {tenders.data?.items.length ? (
        <Table<PlatformTenderDTO>
          rowKey="id"
          dataSource={tenders.data.items}
          scroll={{ x: 1080 }}
          pagination={{
            current: page,
            pageSize,
            total: tenders.data.total,
            showSizeChanger: false,
            showTotal: (total) => `共 ${total} 条`,
            onChange: setPage,
          }}
          expandable={{
            expandedRowRender: (row) => {
              const dims = dimPreview(row.requirement_dims)
              const nodes = timelineEntries(row.timeline)
              const hint =
                typeof row.attachments.download_hint === 'string' ? row.attachments.download_hint : ''
              const price = typeof row.attachments.file_price === 'string' ? row.attachments.file_price : ''
              return (
                <Space direction="vertical" size={8} className="full-width">
                  {nodes.length ? (
                    <Typography.Text type="secondary">
                      {nodes.map((item) => `${item.label} ${item.value}`).join('  ·  ')}
                    </Typography.Text>
                  ) : null}
                  {hint || price ? (
                    <Typography.Text type="secondary">
                      招标文件：{[price, hint].filter(Boolean).join(' · ')}
                    </Typography.Text>
                  ) : null}
                  {dims.extractedBy ? (
                    <Space wrap>
                      <Tag>{dims.extractedBy === 'native' ? '原生拆维' : '待 LLM 抽取'}</Tag>
                      {dims.confidence ? <Tag>{dims.confidence}</Tag> : null}
                      {dims.filled.map((name) => (
                        <Tag key={name}>{name}</Tag>
                      ))}
                    </Space>
                  ) : null}
                  {row.risk_flags.length ? (
                    <Space wrap>
                      {row.risk_flags.map((flag) => (
                        <Tag color="red" key={flag}>
                          {flag}
                        </Tag>
                      ))}
                    </Space>
                  ) : null}
                  <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                    {row.raw_content_preview || '暂无正文预览'}
                  </Typography.Paragraph>
                </Space>
              )
            },
          }}
          columns={[
            {
              title: '标讯名称',
              dataIndex: 'title',
              width: 280,
              render: (value: string, row) =>
                row.source_url ? (
                  <Typography.Link href={row.source_url} target="_blank" rel="noreferrer">
                    {value}
                  </Typography.Link>
                ) : (
                  value
                ),
            },
            {
              title: '来源',
              dataIndex: 'external_source',
              width: 130,
              render: (value: string) => sourceLabels[value] || value,
            },
            { title: '采购人', dataIndex: 'purchaser', width: 180, render: (value: string) => value || '-' },
            { title: '预算', dataIndex: 'budget_text', width: 120, render: (value: string) => value || '-' },
            { title: '截止', dataIndex: 'deadline', width: 120, render: (value: string | null) => formatDateOnly(value) },
            { title: '采集时间', dataIndex: 'collected_at', width: 170, render: (value: string) => formatDateTime(value) },
            {
              title: '复核',
              dataIndex: 'risk_flags',
              width: 90,
              render: (flags: string[]) =>
                flags?.length ? <Tag color="red">{flags.length} 项</Tag> : <Tag>无</Tag>,
            },
          ]}
        />
      ) : null}
    </Space>
  )
}
