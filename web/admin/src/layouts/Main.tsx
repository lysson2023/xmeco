import { useState } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu, Button, Dropdown, type MenuProps } from 'antd';
import {
  ProjectOutlined,
  HomeOutlined,
  ApiOutlined,
  SettingOutlined,
  CodeOutlined,
  AlertOutlined,
  PlayCircleOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  UserOutlined,
  TeamOutlined,
  SafetyCertificateOutlined,
  FileTextOutlined,
  RobotOutlined,
  DashboardOutlined,
  ToolOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons';
import ErrorBoundary from '../components/ErrorBoundary';

const { Header, Sider, Content } = Layout;

const menuItems: MenuProps['items'] = [
  { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
  { type: 'divider' },
  { key: '/users', icon: <UserOutlined />, label: '用户管理' },
  { key: '/agents', icon: <TeamOutlined />, label: '代理商管理' },
  { key: '/permissions', icon: <SafetyCertificateOutlined />, label: '权限管理' },
  { type: 'divider' },
  { key: '/projects', icon: <ProjectOutlined />, label: '项目管理' },
  { key: '/buildings', icon: <HomeOutlined />, label: '楼宇管理' },
  { key: '/devices', icon: <ApiOutlined />, label: '设备管理' },
  { key: '/properties', icon: <SettingOutlined />, label: '属性配置' },
  { key: '/registers', icon: <CodeOutlined />, label: '寄存器' },
  { key: '/alarms', icon: <AlertOutlined />, label: '告警管理' },
  { key: '/startup-plans', icon: <PlayCircleOutlined />, label: '启停配置' },
  { key: '/task-center', icon: <UnorderedListOutlined />, label: '任务中心' },
  { key: '/maintenance-records', icon: <ToolOutlined />, label: '维保记录' },
  { type: 'divider' },
  { key: '/multi-agent', icon: <RobotOutlined />, label: '多智能体' },
  { key: '/logs', icon: <FileTextOutlined />, label: '日志管理' },
];

export default function MainLayout() {
  const [collapsed, setCollapsed] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  const logout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    navigate('/login');
  };

  const getUser = () => {
    try {
      return JSON.parse(localStorage.getItem('user') || '{}');
    } catch {
      return {};
    }
  };
  const user = getUser();

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider trigger={null} collapsible collapsed={collapsed} theme="dark">
        <div
          style={{
            height: 64,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#00daf3',
            fontSize: collapsed ? 14 : 18,
            fontWeight: 700,
          }}
        >
          {collapsed ? 'XM' : 'XMECO'}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            background: '#fff',
            padding: '0 24px',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Button
              type="text"
              icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              onClick={() => setCollapsed(!collapsed)}
            />
            <span style={{ fontSize: 16, fontWeight: 600, color: '#006875', letterSpacing: 1 }}>
              深圳市高海拔科技有限公司
            </span>
            <span
              style={{
                flex: 1,
                textAlign: 'center',
                fontSize: 14,
                fontWeight: 500,
                color: '#333',
              }}
            >
              熊猫智控 XMECO 多智能体能效节能系统
            </span>
          </div>
          <Dropdown
            menu={{
              items: [
                {
                  key: 'logout',
                  icon: <LogoutOutlined />,
                  label: '退出登录',
                  onClick: logout,
                },
              ],
            }}
          >
            <span style={{ cursor: 'pointer' }}>{user.username || 'admin'}</span>
          </Dropdown>
        </Header>
        <Content
          style={{
            margin: 24,
            background: '#fff',
            borderRadius: 8,
            padding: 24,
            minHeight: 280,
          }}
        >
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </Content>
      </Layout>
    </Layout>
  );
}
