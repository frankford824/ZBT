import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: 'react-vendor',
              test: /[\\/]node_modules[\\/](react|react-dom|scheduler|use-sync-external-store)[\\/]/,
              priority: 60,
            },
            {
              name: 'router-query',
              test: /[\\/]node_modules[\\/](react-router|react-router-dom|@tanstack[\\/]react-query|@tanstack[\\/]query-core|zustand)[\\/]/,
              priority: 55,
            },
            {
              name: 'charts-renderer',
              test: /[\\/]node_modules[\\/]zrender[\\/]/,
              priority: 45,
            },
            {
              name: 'charts-core',
              test: /[\\/]node_modules[\\/](echarts|echarts-for-react|size-sensor|fast-deep-equal)[\\/]/,
              priority: 40,
            },
            {
              name: 'antd-icons',
              test: /[\\/]node_modules[\\/]@ant-design[\\/](icons|icons-svg)[\\/]/,
              priority: 38,
            },
            {
              name: 'table-foundation',
              test: /[\\/]node_modules[\\/](@rc-component[\\/](table|pagination|resize-observer|virtual-list|overflow|util)|rc-table|rc-pagination|rc-resize-observer|rc-virtual-list)[\\/]/,
              priority: 34,
            },
            {
              name: 'antd-table',
              test: /[\\/]node_modules[\\/]antd[\\/]es[\\/](table|pagination)[\\/]/,
              priority: 33,
            },
            {
              name: 'antd-form',
              test: /[\\/]node_modules[\\/](antd[\\/]es[\\/](form|input|input-number|select|checkbox|switch|upload)|@rc-component[\\/](color-picker|mini-decimal)|rc-field-form|rc-input|rc-input-number|rc-select|rc-upload)[\\/]/,
              priority: 29,
            },
            {
              name: 'antd-overlay',
              test: /[\\/]node_modules[\\/](antd[\\/]es[\\/](app|modal|drawer|dropdown|tooltip|popover|popconfirm|message|notification)|@rc-component[\\/](portal|trigger)|rc-dialog|rc-drawer|rc-dropdown|rc-motion|rc-tooltip)[\\/]/,
              priority: 28,
            },
            {
              name: 'antd-layout',
              test: /[\\/]node_modules[\\/]antd[\\/]es[\\/](avatar|badge|card|col|descriptions|flex|grid|layout|list|menu|row|statistic|tabs)[\\/]/,
              priority: 27,
            },
            {
              name: 'antd-core',
              test: /[\\/]node_modules[\\/](antd[\\/](es|locale)|@ant-design|classnames|copy-to-clipboard|dayjs|rc-util|throttle-debounce)[\\/]/,
              priority: 26,
            },
            {
              name: 'rc',
              test: /[\\/]node_modules[\\/](@rc-component|rc-)[\\/]/,
              priority: 15,
            },
            {
              name: 'editor',
              test: /[\\/]node_modules[\\/](@tiptap|prosemirror-)[\\/]/,
              priority: 10,
            },
          ],
        },
      },
    },
  },
})
