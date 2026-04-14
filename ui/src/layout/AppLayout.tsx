import { Layout, Menu } from 'antd';
import { HomeOutlined, DatabaseOutlined, ApartmentOutlined } from '@ant-design/icons';
import { Routes, Route, useLocation, useNavigate, Navigate } from 'react-router-dom';
import DevicesPage from '../pages/DevicesPage';
import FlowsPage from '../pages/FlowsPage';
import FlowEditorPage from '../pages/FlowEditorPage';
import HomePage from '../pages/HomePage';

const { Header, Sider, Content } = Layout;

function selectedKey(pathname: string): string {
  if (pathname.startsWith('/devices')) return 'devices';
  if (pathname.startsWith('/flows')) return 'flows';
  return 'home';
}

export default function AppLayout() {
  const location = useLocation();
  const navigate = useNavigate();

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ color: 'white', fontSize: 20 }}>Observer</Header>
      <Layout>
        <Sider width={220} theme="light">
          <Menu
            mode="inline"
            selectedKeys={[selectedKey(location.pathname)]}
            onClick={(e) => {
              if (e.key === 'home') navigate('/');
              if (e.key === 'devices') navigate('/devices');
              if (e.key === 'flows') navigate('/flows');
            }}
            items={[
              { key: 'home', icon: <HomeOutlined />, label: 'Home' },
              { key: 'devices', icon: <DatabaseOutlined />, label: 'Devices' },
              { key: 'flows', icon: <ApartmentOutlined />, label: 'Flows' },
            ]}
          />
        </Sider>
        <Content style={{ padding: 24, background: 'white' }}>
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/devices" element={<DevicesPage />} />
            <Route path="/flows" element={<FlowsPage />} />
            <Route path="/flows/new" element={<FlowEditorPage />} />
            <Route path="/flows/:id" element={<FlowEditorPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  );
}
