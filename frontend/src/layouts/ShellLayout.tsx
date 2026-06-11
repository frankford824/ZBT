import {
  BellOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Avatar, Badge, Button, Dropdown, Flex, Layout, Menu, Space, Typography } from 'antd'
import type { MenuProps } from 'antd'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useSessionStore } from '../app/store/session'
import { navGroups, type NavItem } from '../routes/routeManifest'
import { permissionAllows } from '../shared/permissions/permissions'

const { Header, Sider, Content } = Layout

function flattenNav(items: NavItem[]): NavItem[] {
  return items.flatMap((item) => [item, ...(item.children ? flattenNav(item.children) : [])])
}

const allNavItems = navGroups.flatMap((group) => flattenNav(group.items))

function useSelectedKeys() {
  const { pathname } = useLocation()
  const selected = [...allNavItems]
    .sort((a, b) => b.path.length - a.path.length)
    .find((item) => pathname === item.path || pathname.startsWith(`${item.path}/`))
  return selected ? [selected.key] : []
}

export function ShellLayout() {
  const navigate = useNavigate()
  const selectedKeys = useSelectedKeys()
  const collapsed = useSessionStore((state) => state.collapsed)
  const toggleCollapsed = useSessionStore((state) => state.toggleCollapsed)
  const logout = useSessionStore((state) => state.logout)
  const user = useSessionStore((state) => state.user)
  const tenant = useSessionStore((state) => state.tenant)
  const permissions = useSessionStore((state) => state.permissions)

  const menuItems: MenuProps['items'] = navGroups
    .map((group) => ({
      type: 'group' as const,
      label: collapsed ? undefined : group.title,
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
    if (target) navigate(target.path)
  }

  return (
    <Layout className="shell-layout">
      <Sider width={244} collapsedWidth={72} collapsed={collapsed} trigger={null}>
        <div className="brand">
          <div className="brand-mark">智</div>
          {!collapsed ? (
            <div>
              <Typography.Text className="brand-name">智标通</Typography.Text>
              <Typography.Text className="brand-subtitle">ZhiBiaoTong</Typography.Text>
            </div>
          ) : null}
        </div>
        <Menu
          mode="inline"
          theme="dark"
          selectedKeys={selectedKeys}
          defaultOpenKeys={['bid-root', 'knowledge-root']}
          items={menuItems}
          onClick={handleMenuClick}
        />
      </Sider>
      <Layout>
        <Header className="topbar">
          <Flex justify="space-between" align="center">
            <Space>
              <Button
                type="text"
                icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
                onClick={toggleCollapsed}
              />
              <div>
                <Typography.Text strong>{tenant.name}</Typography.Text>
                <Typography.Text type="secondary" className="tenant-id">
                  tenant-demo
                </Typography.Text>
              </div>
            </Space>
            <Space size={16}>
              <Badge count={7} size="small">
                <Button shape="circle" icon={<BellOutlined />} />
              </Badge>
              <Dropdown
                menu={{
                  items: [
                    {
                      key: 'profile',
                      icon: <UserOutlined />,
                      label: user.role,
                    },
                    {
                      key: 'logout',
                      icon: <LogoutOutlined />,
                      label: '退出登录',
                      onClick: logout,
                    },
                  ],
                }}
              >
                <Space className="user-menu">
                  <Avatar icon={<UserOutlined />} />
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
    </Layout>
  )
}
