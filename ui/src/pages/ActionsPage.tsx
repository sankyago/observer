import { useEffect, useState } from 'react';
import { Button, Form, Input, Modal, Popconfirm, Select, Space, Table, message } from 'antd';
import { api, type Action } from '../api';

export default function ActionsPage() {
  const [rows, setRows] = useState<Action[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<{ kind: Action['kind']; url?: string }>();

  const refresh = async () => {
    setLoading(true);
    try {
      setRows(await api.listActions());
    } catch (e) {
      message.error(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { refresh(); }, []);

  const onCreate = async () => {
    const vals = await form.validateFields();
    const config: Record<string, unknown> = {};
    if (vals.kind === 'webhook' && vals.url) config.url = vals.url;
    try {
      await api.createAction(vals.kind, config);
      setOpen(false);
      form.resetFields();
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setOpen(true)}>New action</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={rows}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 320 },
          { title: 'Kind', dataIndex: 'kind', width: 100 },
          {
            title: 'Config',
            render: (_, r: Action) => <code>{JSON.stringify(r.config)}</code>,
          },
          {
            title: '',
            width: 100,
            render: (_, r: Action) => (
              <Popconfirm title="Delete?" onConfirm={() => api.deleteAction(r.id).then(refresh)}>
                <Button danger size="small">Delete</Button>
              </Popconfirm>
            ),
          },
        ]}
      />
      <Modal title="New action" open={open} onOk={onCreate} onCancel={() => setOpen(false)}>
        <Form form={form} layout="vertical" initialValues={{ kind: 'log' }}>
          <Form.Item name="kind" label="Kind" rules={[{ required: true }]}>
            <Select options={[
              { value: 'log', label: 'log' },
              { value: 'webhook', label: 'webhook' },
              { value: 'email', label: 'email' },
            ]} />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, next) => prev.kind !== next.kind}>
            {({ getFieldValue }) =>
              getFieldValue('kind') === 'webhook' ? (
                <Form.Item name="url" label="Webhook URL" rules={[{ required: true }]}>
                  <Input placeholder="https://example.com/hook" />
                </Form.Item>
              ) : null
            }
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
