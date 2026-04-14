import { useEffect, useState } from 'react';
import { Button, Form, Input, List, Popconfirm, Space, Tag, message } from 'antd';
import { api, type DeviceProfile } from '../api';

export default function ProfilesPanel() {
  const [rows, setRows] = useState<DeviceProfile[]>([]);
  const [form] = Form.useForm<{ name: string; fields: string }>();

  const refresh = async () => {
    try { setRows(await api.listProfiles()); }
    catch (e) { message.error(String(e)); }
  };

  useEffect(() => { refresh(); }, []);

  const add = async () => {
    const vals = await form.validateFields();
    const fields = vals.fields ? vals.fields.split(',').map((s) => s.trim()).filter(Boolean) : [];
    try {
      await api.createProfile(vals.name, fields);
      form.resetFields();
      refresh();
    } catch (e) { message.error(String(e)); }
  };

  return (
    <>
      <Form form={form} layout="vertical" onFinish={add} initialValues={{ name: '', fields: '' }}>
        <Form.Item name="name" label="Profile name" rules={[{ required: true }]}>
          <Input placeholder="e.g. Boiler, Chiller, Bus" />
        </Form.Item>
        <Form.Item name="fields" label="Default fields (comma-separated)">
          <Input placeholder="temperature, humidity, battery" />
        </Form.Item>
        <Button type="primary" htmlType="submit">Add profile</Button>
      </Form>
      <div style={{ marginTop: 16 }}>
        <List
          bordered
          size="small"
          dataSource={rows}
          locale={{ emptyText: 'no profiles yet' }}
          renderItem={(p) => (
            <List.Item
              actions={[
                <Popconfirm key="del" title="Delete?" onConfirm={() => api.deleteProfile(p.id).then(refresh)}>
                  <Button danger size="small">Delete</Button>
                </Popconfirm>,
              ]}
            >
              <Space direction="vertical" style={{ width: '100%' }}>
                <strong>{p.name}</strong>
                <div>{p.default_fields.map((f) => <Tag key={f}>{f}</Tag>)}</div>
              </Space>
            </List.Item>
          )}
        />
      </div>
    </>
  );
}
