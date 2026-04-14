import { useEffect, useMemo, useState } from 'react';
import {
  Button, Drawer, Form, Input, Modal, Popconfirm, Segmented, Select, Space, Table, Tag, message,
} from 'antd';
import { SettingOutlined } from '@ant-design/icons';
import { api, type Device, type DeviceGroup, type DeviceProfile } from '../api';
import DeviceViewer from '../devices/DeviceViewer';
import ProfilesPanel from '../devices/ProfilesPanel';
import GroupsPanel from '../devices/GroupsPanel';

type EditState = { id?: string; name: string; type: string; profile_id?: string | null; group_id?: string | null };

export default function DevicesPage() {
  const [rows, setRows] = useState<Device[]>([]);
  const [profiles, setProfiles] = useState<DeviceProfile[]>([]);
  const [groups, setGroups] = useState<DeviceGroup[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<EditState | null>(null);
  const [selected, setSelected] = useState<Device | null>(null);
  const [groupFilter, setGroupFilter] = useState<string | 'all'>('all');
  const [adminOpen, setAdminOpen] = useState<false | 'profiles' | 'groups'>(false);
  const [form] = Form.useForm<EditState>();

  const refresh = async () => {
    setLoading(true);
    try {
      const [d, p, g] = await Promise.all([api.listDevices(), api.listProfiles(), api.listGroups()]);
      setRows(d); setProfiles(p); setGroups(g);
    } catch (e) { message.error(String(e)); }
    finally { setLoading(false); }
  };

  useEffect(() => { refresh(); }, []);

  const profileName = (id: string | null) => profiles.find((p) => p.id === id)?.name;
  const groupName = (id: string | null) => groups.find((g) => g.id === id)?.name;

  const filteredRows = useMemo(
    () => (groupFilter === 'all' ? rows : rows.filter((r) => r.group_id === groupFilter)),
    [rows, groupFilter],
  );

  const openNew = () => {
    setEditing({ name: '', type: '', profile_id: null, group_id: null });
    form.setFieldsValue({ name: '', type: '', profile_id: null, group_id: null });
    setOpen(true);
  };

  const openEdit = (d: Device) => {
    const e: EditState = { id: d.id, name: d.name, type: d.type, profile_id: d.profile_id, group_id: d.group_id };
    setEditing(e);
    form.setFieldsValue(e);
    setOpen(true);
  };

  const save = async () => {
    const vals = await form.validateFields();
    const input = {
      name: vals.name,
      type: vals.type || '',
      profile_id: vals.profile_id || null,
      group_id: vals.group_id || null,
    };
    try {
      if (editing?.id) await api.updateDevice(editing.id, input);
      else await api.createDevice(input);
      setOpen(false); form.resetFields(); refresh();
    } catch (e) { message.error(String(e)); }
  };

  const onDelete = async (id: string) => {
    try { await api.deleteDevice(id); refresh(); }
    catch (e) { message.error(String(e)); }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16, flexWrap: 'wrap' }}>
        <Button type="primary" onClick={openNew}>New device</Button>
        <Button onClick={refresh}>Refresh</Button>
        <Button icon={<SettingOutlined />} onClick={() => setAdminOpen('groups')}>Groups</Button>
        <Button icon={<SettingOutlined />} onClick={() => setAdminOpen('profiles')}>Profiles</Button>
        <Space>
          <span>Group:</span>
          <Select
            value={groupFilter}
            style={{ width: 200 }}
            options={[{ value: 'all', label: 'All groups' }, ...groups.map((g) => ({ value: g.id, label: g.name }))]}
            onChange={setGroupFilter}
          />
        </Space>
      </Space>

      <Table
        rowKey="id"
        loading={loading}
        dataSource={filteredRows}
        onRow={(r: Device) => ({
          onClick: () => setSelected(r),
          style: { cursor: 'pointer' },
        })}
        columns={[
          { title: 'Name', dataIndex: 'name' },
          { title: 'Type', dataIndex: 'type' },
          {
            title: 'Profile',
            render: (_, r: Device) =>
              r.profile_id ? <Tag color="geekblue">{profileName(r.profile_id) ?? r.profile_id.slice(0, 6)}</Tag> : '—',
          },
          {
            title: 'Group',
            render: (_, r: Device) =>
              r.group_id ? <Tag color="green">{groupName(r.group_id) ?? r.group_id.slice(0, 6)}</Tag> : '—',
          },
          {
            title: '',
            width: 180,
            render: (_, r: Device) => (
              <span onClick={(e) => e.stopPropagation()}>
                <Space>
                  <Button size="small" onClick={() => openEdit(r)}>Edit</Button>
                  <Popconfirm title="Delete?" onConfirm={() => onDelete(r.id)}>
                    <Button danger size="small">Delete</Button>
                  </Popconfirm>
                </Space>
              </span>
            ),
          },
        ]}
      />

      <Modal
        title={editing?.id ? 'Edit device' : 'New device'}
        open={open}
        onOk={save}
        onCancel={() => setOpen(false)}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="type" label="Type">
            <Input placeholder="sensor, boiler, bus, ..." />
          </Form.Item>
          <Form.Item name="profile_id" label="Profile">
            <Select
              allowClear placeholder="(none)"
              options={profiles.map((p) => ({ value: p.id, label: p.name }))}
            />
          </Form.Item>
          <Form.Item name="group_id" label="Group">
            <Select
              allowClear placeholder="(none)"
              options={groups.map((g) => ({ value: g.id, label: g.name }))}
            />
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

      <Drawer
        title="Device organization"
        placement="right"
        width={520}
        open={adminOpen !== false}
        onClose={() => { setAdminOpen(false); refresh(); }}
        destroyOnClose
      >
        <Segmented
          value={adminOpen || 'groups'}
          onChange={(v) => setAdminOpen(v as 'profiles' | 'groups')}
          options={[{ value: 'groups', label: 'Groups' }, { value: 'profiles', label: 'Profiles' }]}
          style={{ marginBottom: 16 }}
        />
        {adminOpen === 'groups' && <GroupsPanel />}
        {adminOpen === 'profiles' && <ProfilesPanel />}
      </Drawer>
    </div>
  );
}
