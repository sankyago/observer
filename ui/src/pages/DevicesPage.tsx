import { useEffect, useState } from 'react';
import { Button, Drawer, Form, Input, Modal, Popconfirm, Space, Table, message } from 'antd';
import { api, type Device } from '../api';
import DeviceViewer from '../devices/DeviceViewer';

export default function DevicesPage() {
  const [rows, setRows] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<Device | null>(null);
  const [form] = Form.useForm<{ name: string; type: string }>();

  const refresh = async () => {
    setLoading(true);
    try {
      setRows(await api.listDevices());
    } catch (e) {
      message.error(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { refresh(); }, []);

  const onCreate = async () => {
    const vals = await form.validateFields();
    try {
      await api.createDevice(vals.name, vals.type || '');
      setOpen(false);
      form.resetFields();
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  const onDelete = async (id: string) => {
    try {
      await api.deleteDevice(id);
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setOpen(true)}>New device</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={rows}
        onRow={(r) => ({ onClick: () => setSelected(r), style: { cursor: 'pointer' } })}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 320 },
          { title: 'Name', dataIndex: 'name' },
          { title: 'Type', dataIndex: 'type' },
          {
            title: '',
            width: 100,
            render: (_, r: Device) => (
              <span onClick={(e) => e.stopPropagation()}>
                <Popconfirm title="Delete?" onConfirm={() => onDelete(r.id)}>
                  <Button danger size="small">Delete</Button>
                </Popconfirm>
              </span>
            ),
          },
        ]}
      />
      <Modal title="New device" open={open} onOk={onCreate} onCancel={() => setOpen(false)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="type" label="Type">
            <Input placeholder="sensor, boiler, ..." />
          </Form.Item>
        </Form>
      </Modal>
      <Drawer
        title={selected ? `${selected.name} — telemetry` : ''}
        placement="right"
        width={900}
        open={!!selected}
        onClose={() => setSelected(null)}
        destroyOnClose
        styles={{ body: { height: 'calc(100vh - 56px)', padding: 16 } }}
      >
        {selected && <DeviceViewer deviceId={selected.id} />}
      </Drawer>
    </div>
  );
}
