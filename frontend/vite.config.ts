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
              name: 'charts',
              test: /[\\/]node_modules[\\/](echarts|echarts-for-react|zrender)[\\/]/,
              priority: 30,
              maxSize: 360_000,
            },
            {
              name: 'antd',
              test: /[\\/]node_modules[\\/]antd[\\/]/,
              priority: 20,
              maxSize: 360_000,
            },
            {
              name: 'rc',
              test: /[\\/]node_modules[\\/](@rc-component|rc-)[\\/]/,
              priority: 15,
              maxSize: 300_000,
            },
            {
              name: 'editor',
              test: /[\\/]node_modules[\\/](@tiptap|prosemirror-)[\\/]/,
              priority: 10,
              maxSize: 360_000,
            },
          ],
        },
      },
    },
  },
})
