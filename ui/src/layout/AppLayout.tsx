import { useState } from 'react';
import { Button, Layout, Menu, Tooltip } from 'antd';
import {
  ThunderboltOutlined, DatabaseOutlined, ApartmentOutlined,
  EnvironmentOutlined, AppstoreOutlined, MessageOutlined, CloseOutlined,
} from '@ant-design/icons';
import { Routes, Route, useLocation, useNavigate, Navigate } from 'react-router-dom';
import DevicesPage from '../pages/DevicesPage';
import FlowsPage from '../pages/FlowsPage';
import FlowEditorPage from '../pages/FlowEditorPage';
import HomePage from '../pages/HomePage';
import MapPage from '../pages/MapPage';
import { DashboardsListPage, DashboardEditorPage } from '../pages/DashboardsPage';
import ChatPanel from '../chat/ChatPanel';

const { Header, Sider, Content } = Layout;

function selectedKey(pathname: string): string {
  if (pathname.startsWith('/devices')) return 'devices';
  if (pathname.startsWith('/map')) return 'map';
  if (pathname.startsWith('/dashboards')) return 'dashboards';
  if (pathname.startsWith('/flows')) return 'flows';
  return 'live';
}

export default function AppLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const [chatOpen, setChatOpen] = useState(false);

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header
        style={{
          color: 'white', fontSize: 20,
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          paddingInline: 24,
        }}
      >
        <span>Observer</span>
        <Tooltip title={chatOpen ? 'Close AI chat' : 'Open AI chat'}>
          <Button
            type="text"
            icon={chatOpen ? <CloseOutlined style={{ color: 'white' }} /> : <MessageOutlined style={{ color: 'white' }} />}
            onClick={() => setChatOpen((v) => !v)}
          />
        </Tooltip>
      </Header>
      <Layout>
        <Sider width={220} theme="light">
          <Menu
            mode="inline"
            selectedKeys={[selectedKey(location.pathname)]}
            onClick={(e) => {
              if (e.key === 'live') navigate('/');
              if (e.key === 'devices') navigate('/devices');
              if (e.key === 'map') navigate('/map');
              if (e.key === 'dashboards') navigate('/dashboards');
              if (e.key === 'flows') navigate('/flows');
            }}
            items={[
              { key: 'live', icon: <ThunderboltOutlined />, label: 'Live' },
              { key: 'devices', icon: <DatabaseOutlined />, label: 'Devices' },
              { key: 'map', icon: <EnvironmentOutlined />, label: 'Map' },
              { key: 'dashboards', icon: <AppstoreOutlined />, label: 'Dashboards' },
              { key: 'flows', icon: <ApartmentOutlined />, label: 'Flows' },
            ]}
          />
        </Sider>
        <Content style={{ padding: 24, background: 'white', minWidth: 0 }}>
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/devices" element={<DevicesPage />} />
            <Route path="/map" element={<MapPage />} />
            <Route path="/dashboards" element={<DashboardsListPage />} />
            <Route path="/dashboards/:id" element={<DashboardEditorPage />} />
            <Route path="/flows" element={<FlowsPage />} />
            <Route path="/flows/new" element={<FlowEditorPage />} />
            <Route path="/flows/:id" element={<FlowEditorPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Content>
        {chatOpen && (
          <Sider
            width={360}
            theme="light"
            style={{
              borderLeft: '1px solid #f0f0f0',
              background: 'white',
              overflow: 'hidden',
            }}
          >
            <ChatPanel />
          </Sider>
        )}
      </Layout>
    </Layout>
  );
}
