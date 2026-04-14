import { useEffect, useState } from 'react';
import { Button, Popconfirm, Space, Table, Tag, message } from 'antd';
import { useNavigate } from 'react-router-dom';
import { api, type Flow } from '../api';

export default function FlowsPage() {
  const [flows, setFlows] = useState<Flow[]>([]);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const refresh = async () => {
    setLoading(true);
    try { setFlows(await api.listFlows()); }
    catch (e) { message.error(String(e)); }
    finally { setLoading(false); }
  };

  useEffect(() => { refresh(); }, []);

  const onDelete = async (id: string) => {
    try { await api.deleteFlow(id); refresh(); }
    catch (e) { message.error(String(e)); }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => navigate('/flows/new')}>New flow</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={flows}
        onRow={(r: Flow) => ({
          onClick: () => navigate(`/flows/${r.id}`),
          style: { cursor: 'pointer' },
        })}
        columns={[
          { title: 'Name', dataIndex: 'name' },
          {
            title: 'Enabled',
            dataIndex: 'enabled',
            width: 100,
            render: (v: boolean) => <Tag color={v ? 'green' : 'default'}>{v ? 'yes' : 'no'}</Tag>,
          },
          {
            title: 'Nodes',
            width: 100,
            render: (_, r: Flow) => r.graph?.nodes?.length ?? 0,
          },
          {
            title: '',
            width: 100,
            render: (_, r: Flow) => (
              <span onClick={(e) => e.stopPropagation()}>
                <Popconfirm title="Delete?" onConfirm={() => onDelete(r.id)}>
                  <Button danger size="small">Delete</Button>
                </Popconfirm>
              </span>
            ),
          },
        ]}
      />
    </div>
  );
}
