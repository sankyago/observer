import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Layout,
  Table,
  Button,
  Switch,
  Modal,
  Input,
  Popconfirm,
  message,
  Space,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { listFlows, createFlow, updateFlow, deleteFlow } from '../api/client';
import type { Flow } from '../api/types';

const { Header, Content } = Layout;
const { Title } = Typography;

const STARTER_GRAPH = {
  nodes: [
    {
      id: 'src',
      type: 'mqtt_source' as const,
      position: { x: 50, y: 100 },
      data: { broker: 'tcp://localhost:1883', topic: 'sensors/#' },
    },
    {
      id: 'sink',
      type: 'debug_sink' as const,
      position: { x: 400, y: 100 },
      data: {},
    },
  ],
  edges: [{ id: 'e1', source: 'src', target: 'sink' }],
};

export default function FlowsListPage() {
  const navigate = useNavigate();
  const [flows, setFlows] = useState<Flow[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [newName, setNewName] = useState('');
  const [creating, setCreating] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const data = await listFlows();
      setFlows(data);
    } catch (e) {
      void message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const handleToggle = async (flow: Flow, enabled: boolean) => {
    try {
      const updated = await updateFlow(flow.id, { enabled });
      setFlows((prev) => prev.map((f) => (f.id === flow.id ? updated : f)));
    } catch (e) {
      void message.error((e as Error).message);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteFlow(id);
      setFlows((prev) => prev.filter((f) => f.id !== id));
    } catch (e) {
      void message.error((e as Error).message);
    }
  };

  const handleCreate = async () => {
    if (!newName.trim()) return;
    setCreating(true);
    try {
      const flow = await createFlow(newName.trim(), STARTER_GRAPH);
      setModalOpen(false);
      setNewName('');
      void navigate(`/flows/${flow.id}`);
    } catch (e) {
      void message.error((e as Error).message);
    } finally {
      setCreating(false);
    }
  };

  const columns: ColumnsType<Flow> = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record) => (
        <Button type="link" onClick={() => void navigate(`/flows/${record.id}`)}>
          {name}
        </Button>
      ),
    },
    {
      title: 'Enabled',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean, record) => (
        <Switch
          checked={enabled}
          onChange={(val) => void handleToggle(record, val)}
        />
      ),
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (ts: string) => new Date(ts).toLocaleString(),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => (
        <Space>
          <Button size="small" onClick={() => void navigate(`/flows/${record.id}`)}>
            Edit
          </Button>
          <Popconfirm
            title="Delete this flow?"
            onConfirm={() => void handleDelete(record.id)}
          >
            <Button size="small" danger>
              Delete
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
        <Title level={4} style={{ color: '#fff', margin: 0 }}>
          Observer — Flows
        </Title>
        <Button type="primary" onClick={() => setModalOpen(true)}>
          New flow
        </Button>
      </Header>
      <Content style={{ padding: 24 }}>
        <Table
          rowKey="id"
          dataSource={flows}
          columns={columns}
          loading={loading}
        />
      </Content>
      <Modal
        title="New flow"
        open={modalOpen}
        onOk={() => void handleCreate()}
        onCancel={() => setModalOpen(false)}
        confirmLoading={creating}
        okText="Create"
      >
        <Input
          placeholder="Flow name"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onPressEnter={() => void handleCreate()}
        />
      </Modal>
    </Layout>
  );
}
