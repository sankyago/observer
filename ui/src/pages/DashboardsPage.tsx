import { useEffect, useState } from 'react';
import { Button, Form, Input, Modal, Popconfirm, Select, Space, Table, message } from 'antd';
import { useNavigate, useParams } from 'react-router-dom';
import { nanoid } from 'nanoid';
import { api, type Dashboard, type DashboardLayout, type Device, type Widget } from '../api';
import DashboardGrid from '../dashboards/DashboardGrid';

export function DashboardsListPage() {
  const [rows, setRows] = useState<Dashboard[]>([]);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<{ name: string }>();
  const navigate = useNavigate();

  const refresh = async () => {
    try { setRows(await api.listDashboards()); }
    catch (e) { message.error(String(e)); }
  };

  useEffect(() => { refresh(); }, []);

  const create = async () => {
    const vals = await form.validateFields();
    try {
      const d = await api.createDashboard({ name: vals.name, layout: { widgets: [] } });
      setOpen(false); form.resetFields();
      navigate(`/dashboards/${d.id}`);
    } catch (e) { message.error(String(e)); }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setOpen(true)}>New dashboard</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>
      <Table
        rowKey="id"
        dataSource={rows}
        onRow={(r: Dashboard) => ({ onClick: () => navigate(`/dashboards/${r.id}`), style: { cursor: 'pointer' } })}
        columns={[
          { title: 'Name', dataIndex: 'name' },
          {
            title: 'Widgets',
            render: (_, r: Dashboard) => r.layout?.widgets?.length ?? 0,
          },
          {
            title: '',
            width: 100,
            render: (_, r: Dashboard) => (
              <span onClick={(e) => e.stopPropagation()}>
                <Popconfirm title="Delete?" onConfirm={() => api.deleteDashboard(r.id).then(refresh)}>
                  <Button danger size="small">Delete</Button>
                </Popconfirm>
              </span>
            ),
          },
        ]}
      />
      <Modal title="New dashboard" open={open} onOk={create} onCancel={() => setOpen(false)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}

export function DashboardEditorPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [layout, setLayout] = useState<DashboardLayout>({ widgets: [] });
  const [devices, setDevices] = useState<Device[]>([]);
  const [addOpen, setAddOpen] = useState(false);
  const [addForm] = Form.useForm<{ type: Widget['type']; device_id?: string; field?: string; unit?: string }>();

  useEffect(() => {
    if (!id) return;
    api.getDashboard(id).then((d) => { setDashboard(d); setLayout(d.layout ?? { widgets: [] }); });
    api.listDevices().then(setDevices);
  }, [id]);

  const addWidget = async () => {
    const vals = await addForm.validateFields();
    const config: Record<string, unknown> = {};
    if (vals.device_id) config.device_id = vals.device_id;
    if (vals.field) config.field = vals.field;
    if (vals.unit) config.unit = vals.unit;

    const defaultSize: Record<Widget['type'], { w: number; h: number }> = {
      chart: { w: 6, h: 5 },
      map: { w: 6, h: 5 },
      value: { w: 3, h: 2 },
      alerts: { w: 6, h: 5 },
    };
    const nextY = layout.widgets.reduce((max, w) => Math.max(max, w.y + w.h), 0);
    const widget: Widget = {
      id: nanoid(8),
      type: vals.type,
      config,
      x: 0, y: nextY,
      ...defaultSize[vals.type],
    };
    setLayout({ widgets: [...layout.widgets, widget] });
    setAddOpen(false); addForm.resetFields();
  };

  const removeWidget = (wid: string) => {
    setLayout({ widgets: layout.widgets.filter((w) => w.id !== wid) });
  };

  const save = async () => {
    if (!id || !dashboard) return;
    try {
      await api.updateDashboard(id, { name: dashboard.name, layout });
      message.success('saved');
    } catch (e) { message.error(String(e)); }
  };

  const currentType = Form.useWatch('type', addForm);

  return (
    <div>
      <Space style={{ marginBottom: 12, width: '100%', justifyContent: 'space-between' }}>
        <Space>
          <Button onClick={() => navigate('/dashboards')}>Back</Button>
          <strong>{dashboard?.name ?? '...'}</strong>
        </Space>
        <Space>
          <Button onClick={() => setAddOpen(true)}>Add widget</Button>
          <Button type="primary" onClick={save}>Save</Button>
        </Space>
      </Space>

      <div style={{ background: '#f5f5f5', padding: 8, borderRadius: 4, minHeight: 300 }}>
        <DashboardGrid value={layout} onChange={setLayout} />
        {layout.widgets.length === 0 && (
          <div style={{ textAlign: 'center', color: '#888', padding: 40 }}>
            No widgets yet. Click <strong>Add widget</strong>.
          </div>
        )}
        {layout.widgets.length > 0 && (
          <div style={{ marginTop: 12, fontSize: 12, color: '#888' }}>
            Drag widget headers to move, drag bottom-right corner to resize.
            <ul style={{ margin: '8px 0 0 0', paddingLeft: 20 }}>
              {layout.widgets.map((w) => (
                <li key={w.id}>
                  {w.type}{w.config?.device_id ? ` — ${(w.config.device_id as string).slice(0, 8)}` : ''}
                  <Button size="small" danger style={{ marginLeft: 8 }} onClick={() => removeWidget(w.id)}>remove</Button>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>

      <Modal title="Add widget" open={addOpen} onOk={addWidget} onCancel={() => setAddOpen(false)}>
        <Form form={addForm} layout="vertical" initialValues={{ type: 'chart' }}>
          <Form.Item name="type" label="Type" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'chart', label: 'Line chart (device)' },
                { value: 'map', label: 'Map (device)' },
                { value: 'value', label: 'Single value (device + field)' },
                { value: 'alerts', label: 'Alerts feed' },
              ]}
            />
          </Form.Item>
          {(currentType === 'chart' || currentType === 'map' || currentType === 'value') && (
            <Form.Item name="device_id" label="Device" rules={[{ required: true }]}>
              <Select options={devices.map((d) => ({ value: d.id, label: d.name }))} />
            </Form.Item>
          )}
          {currentType === 'value' && (
            <>
              <Form.Item name="field" label="Field" rules={[{ required: true }]}>
                <Input placeholder="e.g. temperature" />
              </Form.Item>
              <Form.Item name="unit" label="Unit">
                <Input placeholder="e.g. °C" />
              </Form.Item>
            </>
          )}
        </Form>
      </Modal>
    </div>
  );
}
