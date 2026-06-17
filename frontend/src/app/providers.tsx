import { QueryClient, QueryClientProvider, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { ConfigProvider, App as AntApp, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { useEffect, useRef, type PropsWithChildren } from 'react'
import { useSessionStore } from './store/session'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      // 4xx responses are deterministic; retry only transient network/5xx failures.
      retry: (failureCount, error) => {
        const status = isAxiosError(error) ? error.response?.status : undefined
        if (status && status >= 400 && status < 500) return false
        return failureCount < 1
      },
      staleTime: 30_000,
    },
  },
})

// Keep Ant Design tokens aligned with the CSS variables in index.css.
const bodyFont =
  '"Noto Sans SC", "PingFang SC", "Microsoft YaHei", -apple-system, "Segoe UI", sans-serif'

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          borderRadius: 8,
          colorPrimary: '#2C5FA8',
          colorInfo: '#2C5FA8',
          colorLink: '#2C5FA8',
          colorSuccess: '#2F9E63',
          colorWarning: '#D97706',
          colorError: '#D43030',
          colorText: '#20242B',
          colorTextSecondary: '#5B616E',
          colorTextTertiary: '#8E94A0',
          colorBorder: '#DDDBD2',
          colorBorderSecondary: '#ECEAE4',
          colorBgLayout: '#F6F6F3',
          colorBgContainer: '#FFFFFF',
          fontFamily: bodyFont,
          fontSize: 14,
        },
        components: {
          Layout: {
            bodyBg: '#F6F6F3',
            headerBg: 'transparent',
            siderBg: '#FFFFFF',
          },
          Menu: {
            itemBg: 'transparent',
            subMenuItemBg: 'transparent',
            itemColor: '#5B616E',
            itemHoverBg: '#F1F0EB',
            itemHoverColor: '#20242B',
            itemSelectedBg: '#EDF2F9',
            itemSelectedColor: '#2C5FA8',
            itemBorderRadius: 8,
            itemMarginInline: 0,
            itemHeight: 38,
            iconMarginInlineEnd: 10,
          },
          Card: {
            headerBg: 'transparent',
            colorBorderSecondary: '#ECEAE4',
            borderRadiusLG: 10,
          },
          Table: {
            headerBg: 'transparent',
            headerColor: '#8E94A0',
            headerSplitColor: 'transparent',
            rowHoverBg: '#F8F8F5',
            borderColor: '#F0EFEA',
            cellPaddingBlock: 13,
          },
          Button: {
            controlHeight: 34,
            fontWeight: 500,
            defaultShadow: 'none',
            primaryShadow: 'none',
          },
          Input: {
            controlHeight: 36,
          },
          Select: {
            controlHeight: 36,
          },
          Statistic: {
            contentFontSize: 26,
          },
          Tag: {
            borderRadiusSM: 5,
          },
          Tabs: {
            inkBarColor: '#B08530',
            itemSelectedColor: '#20242B',
            itemHoverColor: '#20242B',
            titleFontSize: 14,
          },
          Modal: {
            borderRadiusLG: 12,
          },
        },
      }}
    >
      <AntApp>
        <QueryClientProvider client={queryClient}>
          <SessionScopedQueryCache>{children}</SessionScopedQueryCache>
        </QueryClientProvider>
      </AntApp>
    </ConfigProvider>
  )
}

function SessionScopedQueryCache({ children }: PropsWithChildren) {
  const client = useQueryClient()
  const isAuthenticated = useSessionStore((state) => state.isAuthenticated)
  const userId = useSessionStore((state) => state.user.id)
  const tenantId = useSessionStore((state) => state.tenant.id)
  const scope = isAuthenticated ? `${tenantId}:${userId}` : 'anonymous'
  const lastScope = useRef(scope)

  useEffect(() => {
    if (lastScope.current === scope) return
    client.clear()
    lastScope.current = scope
  }, [client, scope])

  return children
}
