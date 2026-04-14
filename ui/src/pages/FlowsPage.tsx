import { useEffect, useState } from 'react';
import { Button, Drawer, Form, Input, Modal, Popconfirm, Space, Switch, Table, Tag, message } from 'antd';
import { api, type Device, type Flow, type FlowGraph } from '../api';
import FlowCanvas, { emptyGraph } from '../flows/FlowCanvas';

type DraftMeta = { id?: string; name: string; enabled: boolean };

export default function FlowsPage() {
  const [flows, setFlows] = useState<Flow[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);

  const [open, setOpen] = useState(false);
  const [graph, setGraph] = useState<FlowGraph>(emptyGraph());
  const [meta, setMeta] = useState<DraftMeta>({ name: '', enabled: true });

  const [namingOpen, setNamingOpen] = useState(false);
  const [nameForm] = Form.useForm<{ name: string }>();

  const refresh = async () => {
    setLoading(true);
    try {
      const [f, d] = await Promise.all([api.listFlows(), api.listDevices()]);
      setFlows(f);
      setDevices(d);
    } catch (e) {
      message.error(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { refresh(); }, []);

  const openCreate = () => {
    nameForm.setFieldsValue({ name: 'New flow' });
    setNamingOpen(true);
  };

  const confirmCreate = async () => {
    const vals = await nameForm.validateFields();
    setMeta({ name: vals.name, enabled: true });
    setGraph(emptyGraph());
    setNamingOpen(false);
    setOpen(true);
  };

  const openEdit = async (id: string) => {
    try {
      const f = await api.getFlow(id);
      setMeta({ id: f.id, name: f.name, enabled: f.enabled });
      setGraph(f.graph);
      setOpen(true);
    } catch (e) {
      message.error(String(e));
    }
  };

  const save = async () => {
    if (!meta.name.trim()) { message.error('name required'); return; }
    try {
      if (meta.id) {
        await api.updateFlow(meta.id, { name: meta.name, graph, enabled: meta.enabled });
      } else {
        await api.createFlow({ name: meta.name, graph, enabled: meta.enabled });
      }
      setOpen(false);
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  const onDelete = async (id: string) => {
    try {
      await api.deleteFlow(id);
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={openCreate}>New flow</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>

      <Table
        rowKey="id"
        loading={loading}
        dataSource={flows}
        onRow={(r: Flow) => ({ onClick: () => openEdit(r.id), style: { cursor: 'pointer' } })}
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

      <Modal title="New flow" open={namingOpen} onOk={confirmCreate} onCancel={() => setNamingOpen(false)}>
        <Form form={nameForm} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={
          <Space>
            <Input
              placeholder="flow name"
              value={meta.name}
              onChange={(e) => setMeta({ ...meta, name: e.target.value })}
              style={{ width: 280 }}
            />
            <Space>
              Enabled
              <Switch checked={meta.enabled} onChange={(v) => setMeta({ ...meta, enabled: v })} />
            </Space>
          </Space>
        }
        placement="right"
        width={1280}
        open={open}
        onClose={() => setOpen(false)}
        destroyOnClose
        extra={
          <Space>
            <Button onClick={() => setOpen(false)}>Cancel</Button>
            <Button type="primary" onClick={save}>Save</Button>
          </Space>
        }
      >
        <FlowCanvas value={graph} onChange={setGraph} devices={devices} />
      </Drawer>
    </div>
  );
}
