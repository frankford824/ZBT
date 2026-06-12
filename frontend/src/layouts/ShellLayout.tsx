import {
  BellOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Avatar, Badge, Button, Drawer, Dropdown, Flex, Grid, Layout, Menu, Space, Typography } from 'antd'
import type { MenuProps } from 'antd'
import { useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useSessionStore } from '../app/store/session'
import { navGroups, type NavItem } from '../routes/routeManifest'
import { logoutSession } from '../shared/api/client'
import { permissionAllows } from '../shared/permissions/permissions'

const { Header, Sider, Content } = Layout

function flattenNav(items: NavItem[]): NavItem[] {
  return items.flatMap((item) => [item, ...(item.children ? flattenNav(item.children) : [])])
}

const allNavItems = navGroups.flatMap((group) => flattenNav(group.items))

const roleLabels: Record<string, string> = {
  company_admin: '企业管理员',
  department_admin: '部门管理员',
  project_manager: '项目经理',
  bid_specialist: '标书专员',
  reviewer: '审核员',
  viewer: '只读成员',
}

function useSelectedNav() {
  const { pathname } = useLocation()
  return [...allNavItems]
    .sort((a, b) => b.path.length - a.path.length)
    .find((item) => pathname === item.path || pathname.startsWith(`${item.path}/`))
}

export function ShellLayout() {
  const navigate = useNavigate()
  const screens = Grid.useBreakpoint()
  const isMobile = !screens.md
  const selectedNav = useSelectedNav()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const collapsed = useSessionStore((state) => state.collapsed)
  const toggleCollapsed = useSessionStore((state) => state.toggleCollapsed)
  const logout = useSessionStore((state) => state.logout)
  const user = useSessionStore((state) => state.user)
  const tenant = useSessionStore((state) => state.tenant)
  const permissions = useSessionStore((state) => state.permissions)

  const menuCollapsed = isMobile ? false : collapsed
  const menuItems: MenuProps['items'] = navGroups
    .map((group) => ({
      type: 'group' as const,
      label: menuCollapsed ? undefined : group.title,
      children: group.items
        .filter((item) => permissionAllows(permissions[item.module], 'read'))
        .map((item) => ({
          key: item.key,
          icon: item.icon,
          label: item.label,
          children: item.children
            ?.filter((child) => permissionAllows(permissions[child.module], 'read'))
            .map((child) => ({
              key: child.key,
              icon: child.icon,
              label: child.label,
            })),
        })),
    }))
    .filter((group) => group.children.length > 0)

  const handleMenuClick: MenuProps['onClick'] = ({ key }) => {
    const target = allNavItems.find((item) => item.key === key)
    if (target) {
      navigate(target.path)
      setMobileNavOpen(false)
    }
  }

  const handleNavButton = () => {
    if (isMobile) {
      setMobileNavOpen(true)
      return
    }
    toggleCollapsed()
  }

  const handleLogout = async () => {
    try {
      await logoutSession()
    } finally {
      logout()
      navigate('/login')
    }
  }

  return (
    <Layout className="shell-layout">
      {!isMobile ? (
      <Sider width={240} collapsedWidth={72} collapsed={collapsed} trigger={null}>
        <Flex vertical style={{ height: '100%' }}>
          <div className="brand">
            <div className="seal-mark">标</div>
            {!collapsed ? (
              <div>
                <Typography.Text className="brand-name">智标通</Typography.Text>
                <Typography.Text className="brand-subtitle">ZhiBiaoTong</Typography.Text>
              </div>
            ) : null}
          </div>
          <div style={{ flex: 1, overflowY: 'auto' }}>
            <Menu
              mode="inline"
              selectedKeys={selectedNav ? [selectedNav.key] : []}
              defaultOpenKeys={['bid-root', 'knowledge-root']}
              items={menuItems}
              onClick={handleMenuClick}
            />
          </div>
          {!collapsed ? <div className="sider-foot">企业智能投标工作台</div> : null}
        </Flex>
      </Sider>
      ) : null}
      <Layout>
        <Header className="topbar">
          <Flex justify="space-between" align="center" style={{ height: '100%' }}>
            <Space size={12}>
              <Button
                type="text"
                aria-label={isMobile ? '打开导航' : collapsed ? '展开导航' : '收起导航'}
                icon={isMobile || collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
                onClick={handleNavButton}
              />
              <div className="topbar-context">
                <Typography.Text strong>{tenant.name}</Typography.Text>
                <Typography.Text className="tenant-id">企业投标工作台</Typography.Text>
              </div>
            </Space>
            <Space size={16}>
              <Badge dot>
                <Button shape="circle" aria-label="通知" icon={<BellOutlined />} />
              </Badge>
              <Dropdown
                menu={{
                  items: [
                    {
                      key: 'profile',
                      icon: <UserOutlined />,
                      label: roleLabels[user.role] || '团队成员',
                    },
                    {
                      key: 'logout',
                      icon: <LogoutOutlined />,
                      label: '退出登录',
                      onClick: handleLogout,
                    },
                  ],
                }}
              >
                <Space className="user-menu">
                  <Avatar style={{ background: '#2C5FA8' }}>
                    {(user.name || '用').slice(0, 1)}
                  </Avatar>
                  <Typography.Text>{user.name}</Typography.Text>
                </Space>
              </Dropdown>
            </Space>
          </Flex>
        </Header>
        <Content className="content-shell">
          <Outlet />
        </Content>
      </Layout>
      <Drawer
        className="mobile-nav-drawer"
        placement="left"
        width={280}
        open={mobileNavOpen}
        onClose={() => setMobileNavOpen(false)}
        closable={false}
        destroyOnHidden
      >
        <div className="brand">
          <div className="seal-mark">标</div>
          <div>
            <Typography.Text className="brand-name">智标通</Typography.Text>
            <Typography.Text className="brand-subtitle">ZhiBiaoTong</Typography.Text>
          </div>
        </div>
        <Menu
          mode="inline"
          selectedKeys={selectedNav ? [selectedNav.key] : []}
          defaultOpenKeys={['bid-root', 'knowledge-root']}
          items={menuItems}
          onClick={handleMenuClick}
        />
      </Drawer>
    </Layout>
  )
}
