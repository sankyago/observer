import { useState } from 'react';
import { Layout, Menu } from 'antd';
import { HomeOutlined, DatabaseOutlined, ApartmentOutlined } from '@ant-design/icons';
import DevicesPage from '../pages/DevicesPage';
import FlowsPage from '../pages/FlowsPage';
import HomePage from '../pages/HomePage';

const { Header, Sider, Content } = Layout;

type Tab = 'home' | 'devices' | 'flows';

export default function AppLayout() {
  const [tab, setTab] = useState<Tab>('home');

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ color: 'white', fontSize: 20 }}>Observer</Header>
      <Layout>
        <Sider width={220} theme="light">
          <Menu
            mode="inline"
            selectedKeys={[tab]}
            onClick={(e) => setTab(e.key as Tab)}
            items={[
              { key: 'home', icon: <HomeOutlined />, label: 'Home' },
              { key: 'devices', icon: <DatabaseOutlined />, label: 'Devices' },
              { key: 'flows', icon: <ApartmentOutlined />, label: 'Flows' },
            ]}
          />
        </Sider>
        <Content style={{ padding: 24, background: 'white' }}>
          {tab === 'home' && <HomePage />}
          {tab === 'devices' && <DevicesPage />}
          {tab === 'flows' && <FlowsPage />}
        </Content>
      </Layout>
    </Layout>
  );
}
