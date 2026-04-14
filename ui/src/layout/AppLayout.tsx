import { useState } from 'react';
import { Layout, Menu } from 'antd';
import DevicesPage from '../pages/DevicesPage';
import ActionsPage from '../pages/ActionsPage';
import RulesPage from '../pages/RulesPage';
import LivePage from '../pages/LivePage';

const { Header, Sider, Content } = Layout;

type Tab = 'devices' | 'actions' | 'rules' | 'live';

export default function AppLayout() {
  const [tab, setTab] = useState<Tab>('live');

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ color: 'white', fontSize: 20 }}>Observer</Header>
      <Layout>
        <Sider width={200} theme="light">
          <Menu
            mode="inline"
            selectedKeys={[tab]}
            onClick={(e) => setTab(e.key as Tab)}
            items={[
              { key: 'live', label: 'Live' },
              { key: 'devices', label: 'Devices' },
              { key: 'actions', label: 'Actions' },
              { key: 'rules', label: 'Rules' },
            ]}
          />
        </Sider>
        <Content style={{ padding: 24, background: 'white' }}>
          {tab === 'live' && <LivePage />}
          {tab === 'devices' && <DevicesPage />}
          {tab === 'actions' && <ActionsPage />}
          {tab === 'rules' && <RulesPage />}
        </Content>
      </Layout>
    </Layout>
  );
}
