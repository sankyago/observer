import { useEffect, useState } from 'react';
import { Button, Input, Space, Switch, Typography, message, Spin } from 'antd';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { api, type Device, type FlowGraph } from '../api';
import FlowCanvas, { emptyGraph } from '../flows/FlowCanvas';

export default function FlowEditorPage() {
  const { id } = useParams<{ id?: string }>();
  const navigate = useNavigate();
  const editing = !!id;

  const [loading, setLoading] = useState(editing);
  const [name, setName] = useState('New flow');
  const [enabled, setEnabled] = useState(true);
  const [graph, setGraph] = useState<FlowGraph>(emptyGraph());
  const [devices, setDevices] = useState<Device[]>([]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const d = await api.listDevices();
        if (!cancelled) setDevices(d);
        if (id) {
          const f = await api.getFlow(id);
          if (!cancelled) {
            setName(f.name);
            setEnabled(f.enabled);
            setGraph(f.graph ?? emptyGraph());
          }
        }
      } catch (e) {
        message.error(String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [id]);

  const save = async () => {
    if (!name.trim()) { message.error('name required'); return; }
    try {
      if (id) {
        await api.updateFlow(id, { name, graph, enabled });
      } else {
        await api.createFlow({ name, graph, enabled });
      }
      message.success('saved');
      navigate('/flows');
    } catch (e) {
      message.error(String(e));
    }
  };

  if (loading) {
    return <div style={{ padding: 40, textAlign: 'center' }}><Spin /></div>;
  }

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/flows')}>Back</Button>
          <Typography.Title level={4} style={{ margin: 0 }}>
            {editing ? 'Edit flow' : 'New flow'}
          </Typography.Title>
        </Space>
        <Space>
          <Input
            placeholder="flow name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            style={{ width: 280 }}
          />
          <Space>Enabled <Switch checked={enabled} onChange={setEnabled} /></Space>
          <Button onClick={() => navigate('/flows')}>Cancel</Button>
          <Button type="primary" onClick={save}>Save</Button>
        </Space>
      </Space>
      <FlowCanvas value={graph} onChange={setGraph} devices={devices} />
    </div>
  );
}
