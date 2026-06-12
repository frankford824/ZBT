import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ConfigProvider, App as AntApp, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import type { PropsWithChildren } from 'react'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 30_000,
    },
  },
})

// 设计令牌与 index.css 中的 CSS 变量保持一致：
// 墨 #1A1B33 / 靛 #4F46E5 / 智能紫 #7C3AED / 朱砂 #C8401A（仅品牌印章）/ 纸 #F4F5FA
const bodyFont =
  '"PingFang SC", "Microsoft YaHei", "Noto Sans SC", -apple-system, "Segoe UI", sans-serif'

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          borderRadius: 6,
          colorPrimary: '#4F46E5',
          colorInfo: '#4F46E5',
          colorSuccess: '#16A34A',
          colorWarning: '#D97706',
          colorError: '#DC2626',
          colorText: '#26273D',
          colorTextSecondary: '#5C5E78',
          colorTextTertiary: '#8A8CA3',
          colorBorder: '#DCDEE9',
          colorBorderSecondary: '#E8EAF2',
          colorBgLayout: '#F4F5FA',
          fontFamily: bodyFont,
          fontSize: 14,
        },
        components: {
          Layout: {
            bodyBg: '#F4F5FA',
            headerBg: '#FFFFFF',
            siderBg: '#1A1B33',
          },
          Menu: {
            darkItemBg: '#1A1B33',
            darkSubMenuItemBg: '#14152A',
            darkItemColor: 'rgba(235, 236, 245, 0.68)',
            darkItemHoverBg: 'rgba(79, 70, 229, 0.22)',
            darkItemSelectedBg: '#4F46E5',
            darkGroupTitleColor: 'rgba(235, 236, 245, 0.38)',
            itemBorderRadius: 6,
            itemMarginInline: 10,
          },
          Card: {
            headerBg: 'transparent',
            colorBorderSecondary: '#E8EAF2',
          },
          Table: {
            headerBg: '#F7F8FC',
            headerColor: '#5C5E78',
            headerSplitColor: 'transparent',
          },
          Button: {
            controlHeight: 34,
            fontWeight: 500,
          },
          Statistic: {
            contentFontSize: 26,
          },
          Tag: {
            borderRadiusSM: 4,
          },
        },
      }}
    >
      <AntApp>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </AntApp>
    </ConfigProvider>
  )
}
