import { useEffect, useState } from 'react';
import { Button, Modal, Popconfirm, Space, Table, message } from 'antd';
import { api, type Action, type Device, type Rule, type RuleInput } from '../api';
import RuleFlow from '../rules/RuleFlow';

export default function RulesPage() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [actions, setActions] = useState<Action[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<RuleInput>({
    device_id: '', field: 'temperature', op: '>', value: 80, action_id: '', enabled: true,
  });

  const refresh = async () => {
    const [d, a, r] = await Promise.all([api.listDevices(), api.listActions(), api.listRules()]);
    setDevices(d); setActions(a); setRules(r);
  };

  useEffect(() => { refresh(); }, []);

  const onSave = async () => {
    if (!draft.device_id || !draft.action_id || !draft.field) {
      message.error('device, field, and action are required');
      return;
    }
    try {
      await api.createRule(draft);
      setOpen(false);
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setOpen(true)}>New rule</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>
      <Table
        rowKey="id"
        dataSource={rules}
        columns={[
          { title: 'Device', render: (_, r: Rule) => devices.find((d) => d.id === r.device_id)?.name ?? r.device_id },
          { title: 'Condition', render: (_, r: Rule) => `${r.field} ${r.op} ${r.value}` },
          { title: 'Action', render: (_, r: Rule) => actions.find((a) => a.id === r.action_id)?.kind ?? r.action_id },
          { title: 'Enabled', dataIndex: 'enabled', render: (v: boolean) => (v ? 'yes' : 'no') },
          {
            title: '',
            render: (_, r: Rule) => (
              <Popconfirm title="Delete?" onConfirm={() => api.deleteRule(r.id).then(refresh)}>
                <Button danger size="small">Delete</Button>
              </Popconfirm>
            ),
          },
        ]}
      />
      <Modal title="New rule" open={open} onOk={onSave} onCancel={() => setOpen(false)} width={900}>
        <RuleFlow devices={devices} actions={actions} initial={draft} onChange={setDraft} />
      </Modal>
    </div>
  );
}
