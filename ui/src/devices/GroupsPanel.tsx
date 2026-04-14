import { useEffect, useState } from 'react';
import { Button, Form, Input, List, Popconfirm, message } from 'antd';
import { api, type DeviceGroup } from '../api';

export default function GroupsPanel() {
  const [rows, setRows] = useState<DeviceGroup[]>([]);
  const [form] = Form.useForm<{ name: string }>();

  const refresh = async () => {
    try { setRows(await api.listGroups()); }
    catch (e) { message.error(String(e)); }
  };

  useEffect(() => { refresh(); }, []);

  const add = async () => {
    const vals = await form.validateFields();
    try {
      await api.createGroup(vals.name);
      form.resetFields();
      refresh();
    } catch (e) { message.error(String(e)); }
  };

  return (
    <>
      <Form form={form} layout="vertical" onFinish={add}>
        <Form.Item name="name" label="Group name" rules={[{ required: true }]}>
          <Input placeholder="e.g. Building A, Fleet North, Site 42" />
        </Form.Item>
        <Button type="primary" htmlType="submit">Add group</Button>
      </Form>
      <div style={{ marginTop: 16 }}>
        <List
          bordered
          size="small"
          dataSource={rows}
          locale={{ emptyText: 'no groups yet' }}
          renderItem={(g) => (
            <List.Item
              actions={[
                <Popconfirm key="del" title="Delete?" onConfirm={() => api.deleteGroup(g.id).then(refresh)}>
                  <Button danger size="small">Delete</Button>
                </Popconfirm>,
              ]}
            >
              {g.name}
            </List.Item>
          )}
        />
      </div>
    </>
  );
}
