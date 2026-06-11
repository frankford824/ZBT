import {
  AuditOutlined,
  CalculatorOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  FileDoneOutlined,
  FileSearchOutlined,
  FileTextOutlined,
  FolderOpenOutlined,
  FormOutlined,
  ProjectOutlined,
  SafetyCertificateOutlined,
  SearchOutlined,
  TagsOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import type { ReactNode } from 'react'

export type ModuleKey =
  | 'dashboard'
  | 'tender'
  | 'bid'
  | 'compliance'
  | 'project'
  | 'cost'
  | 'knowledge'
  | 'team'

export type NavItem = {
  key: string
  label: string
  path: string
  module: ModuleKey
  icon: ReactNode
  children?: NavItem[]
}

export type NavGroup = {
  title: string
  items: NavItem[]
}

export const navGroups: NavGroup[] = [
  {
    title: '概览',
    items: [
      {
        key: 'dashboard',
        label: '工作台',
        path: '/dashboard',
        module: 'dashboard',
        icon: <DashboardOutlined />,
      },
    ],
  },
  {
    title: '投标准备',
    items: [
      {
        key: 'tender',
        label: '标讯大厅',
        path: '/tenders',
        module: 'tender',
        icon: <SearchOutlined />,
      },
      {
        key: 'bid-root',
        label: '标书生成',
        path: '/bids',
        module: 'bid',
        icon: <FileTextOutlined />,
        children: [
          {
            key: 'bid-new',
            label: '新建标书',
            path: '/bids/new',
            module: 'bid',
            icon: <FormOutlined />,
          },
          {
            key: 'bid-list',
            label: '我的标书',
            path: '/bids',
            module: 'bid',
            icon: <FileDoneOutlined />,
          },
          {
            key: 'bid-templates',
            label: '标书模板',
            path: '/bids/templates',
            module: 'bid',
            icon: <FolderOpenOutlined />,
          },
        ],
      },
    ],
  },
  {
    title: '投标管控',
    items: [
      {
        key: 'compliance',
        label: '合规检查',
        path: '/compliance',
        module: 'compliance',
        icon: <SafetyCertificateOutlined />,
      },
      {
        key: 'project',
        label: '项目管理',
        path: '/projects',
        module: 'project',
        icon: <ProjectOutlined />,
      },
    ],
  },
  {
    title: '企业管理',
    items: [
      {
        key: 'cost',
        label: '成本管理',
        path: '/costs',
        module: 'cost',
        icon: <CalculatorOutlined />,
      },
      {
        key: 'knowledge-root',
        label: '知识库',
        path: '/knowledge',
        module: 'knowledge',
        icon: <DatabaseOutlined />,
        children: [
          {
            key: 'knowledge-docs',
            label: '文档库',
            path: '/knowledge/docs',
            module: 'knowledge',
            icon: <FileSearchOutlined />,
          },
          {
            key: 'knowledge-templates',
            label: '文档模板',
            path: '/knowledge/templates',
            module: 'knowledge',
            icon: <AuditOutlined />,
          },
          {
            key: 'knowledge-tags',
            label: '标签管理',
            path: '/knowledge/tags',
            module: 'knowledge',
            icon: <TagsOutlined />,
          },
        ],
      },
      {
        key: 'team',
        label: '团队协作',
        path: '/team',
        module: 'team',
        icon: <TeamOutlined />,
      },
    ],
  },
]

export const moduleNames: Record<ModuleKey, string> = {
  dashboard: '工作台',
  tender: '标讯大厅',
  bid: '标书生成',
  compliance: '合规检查',
  project: '项目管理',
  cost: '成本管理',
  knowledge: '知识库',
  team: '团队协作',
}
