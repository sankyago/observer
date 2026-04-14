import { useEffect, useState } from 'react';
import {
  Button, Drawer, Form, Input, Modal, Popconfirm, Select, Space, Table, Tag, message,
} from 'antd';
import { api, type Action } from '../api';
import WorkflowCanvas, { emptyWorkflow, fromBackendConfig, toBackendConfig, type WorkflowState } from '../workflows/WorkflowCanvas';

export default function ActionsPage() {
  const [rows, setRows] = useState<Action[]>([]);
  const [loading, setLoading] = useState(false);

  const [simpleOpen, setSimpleOpen] = useState(false);
  const [simpleForm] = Form.useForm<{ kind: Action['kind']; url?: string }>();

  const [wfOpen, setWfOpen] = useState(false);
  const [wf, setWf] = useState<WorkflowState>(emptyWorkflow());
  const [editingId, setEditingId] = useState<string | null>(null);
  const [simpleEditingId, setSimpleEditingId] = useState<string | null>(null);

  const refresh = async () => {
    setLoading(true);
    try { setRows(await api.listActions()); }
    catch (e) { message.error(String(e)); }
    finally { setLoading(false); }
  };

  useEffect(() => { refresh(); }, []);

  const openNewWorkflow = () => {
    setEditingId(null);
    setWf(emptyWorkflow());
    setWfOpen(true);
  };

  const openEditWorkflow = (a: Action) => {
    setEditingId(a.id);
    setWf(fromBackendConfig(a.config));
    setWfOpen(true);
  };

  const openEditSimple = (a: Action) => {
    setSimpleEditingId(a.id);
    simpleForm.setFieldsValue({
      kind: a.kind,
      url: (a.config?.url as string | undefined) ?? '',
    });
    setSimpleOpen(true);
  };

  const saveSimple = async () => {
    const vals = await simpleForm.validateFields();
    const config: Record<string, unknown> = {};
    if (vals.kind === 'webhook' && vals.url) config.url = vals.url;
    try {
      if (simpleEditingId) {
        await api.updateAction(simpleEditingId, vals.kind, config);
      } else {
        await api.createAction(vals.kind, config);
      }
      setSimpleOpen(false); setSimpleEditingId(null); simpleForm.resetFields(); refresh();
    } catch (e) { message.error(String(e)); }
  };

  const saveWorkflow = async () => {
    if (wf.nodes.length === 0) { message.error('Drop at least one node'); return; }
    try {
      if (editingId) {
        await api.updateAction(editingId, 'workflow', toBackendConfig(wf));
      } else {
        await api.createAction('workflow', toBackendConfig(wf));
      }
      setWfOpen(false); setEditingId(null); setWf(emptyWorkflow()); refresh();
    } catch (e) { message.error(String(e)); }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={openNewWorkflow}>New workflow</Button>
        <Button onClick={() => { setSimpleEditingId(null); simpleForm.resetFields(); setSimpleOpen(true); }}>New simple action</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>

      <Table
        rowKey="id"
        loading={loading}
        dataSource={rows}
        onRow={(r: Action) => ({
          onClick: () => {
            if (r.kind === 'workflow') openEditWorkflow(r);
            else openEditSimple(r);
          },
          style: { cursor: 'pointer' },
        })}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 320 },
          {
            title: 'Kind',
            dataIndex: 'kind',
            width: 120,
            render: (k: string) => <Tag color={k === 'workflow' ? 'purple' : 'blue'}>{k}</Tag>,
          },
          {
            title: 'Config',
            render: (_: unknown, r: Action) => (
              <code style={{ fontSize: 12 }}>
                {r.kind === 'workflow'
                  ? `${(r.config.nodes as unknown[] | undefined)?.length ?? 0} nodes, ${(r.config.edges as unknown[] | undefined)?.length ?? 0} edges`
                  : JSON.stringify(r.config)}
              </code>
            ),
          },
          {
            title: '',
            width: 100,
            render: (_: unknown, r: Action) => (
              <span onClick={(e) => e.stopPropagation()}>
                <Popconfirm title="Delete?" onConfirm={() => api.deleteAction(r.id).then(refresh)}>
                  <Button danger size="small">Delete</Button>
                </Popconfirm>
              </span>
            ),
          },
        ]}
      />

      <Modal
        title={simpleEditingId ? `Edit ${simpleEditingId.slice(0, 8)}` : 'New simple action'}
        open={simpleOpen}
        onOk={saveSimple}
        onCancel={() => { setSimpleOpen(false); setSimpleEditingId(null); }}
      >
        <Form form={simpleForm} layout="vertical" initialValues={{ kind: 'log' }}>
          <Form.Item name="kind" label="Kind" rules={[{ required: true }]}>
            <Select options={[
              { value: 'log', label: 'log' },
              { value: 'webhook', label: 'webhook' },
              { value: 'email', label: 'email' },
            ]} />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(p, n) => p.kind !== n.kind}>
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

      <Drawer
        title={editingId ? `Edit workflow ${editingId.slice(0, 8)}` : 'New workflow'}
        placement="right"
        width={1100}
        open={wfOpen}
        onClose={() => { setWfOpen(false); setEditingId(null); }}
        destroyOnClose
        extra={
          <Space>
            <Button onClick={() => { setWfOpen(false); setEditingId(null); }}>Cancel</Button>
            <Button type="primary" onClick={saveWorkflow}>Save</Button>
          </Space>
        }
      >
        <WorkflowCanvas value={wf} onChange={setWf} />
      </Drawer>
    </div>
  );
}
