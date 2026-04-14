import { useEffect, useMemo, useState } from 'react';
import { Empty, Table, Tag } from 'antd';
import { api } from '../api';
import { useSse } from '../useSse';

type Row = { time: string; message_id: string; payload: Record<string, unknown> };

const MAX_ROWS = 100;

export default function DeviceRaw({ deviceId }: { deviceId: string }) {
  const [rows, setRows] = useState<Row[]>([]);
  const events = useSse('/api/v1/stream');

  useEffect(() => {
    let cancelled = false;
    api.recentTelemetry(deviceId, MAX_ROWS).then((r) => {
      if (cancelled) return;
      setRows(r.map((x) => ({ time: x.time, message_id: x.message_id, payload: x.payload })));
    });
    return () => { cancelled = true; };
  }, [deviceId]);

  useEffect(() => {
    if (events.length === 0) return;
    const last = events[events.length - 1];
    if (last.type !== 'telemetry') return;
    const d = last.data as { device_id?: string; time?: string; message_id?: string; payload?: Record<string, unknown> };
    if (d.device_id !== deviceId || !d.time || !d.payload || !d.message_id) return;
    setRows((prev) => {
      const next = [{ time: d.time!, message_id: d.message_id!, payload: d.payload! }, ...prev];
      return next.length > MAX_ROWS ? next.slice(0, MAX_ROWS) : next;
    });
  }, [events, deviceId]);

  const fields = useMemo(() => {
    const s = new Set<string>();
    for (const r of rows) for (const k of Object.keys(r.payload)) s.add(k);
    return [...s];
  }, [rows]);

  if (rows.length === 0) return <Empty description="No telemetry yet." />;

  return (
    <Table
      size="small"
      rowKey="message_id"
      dataSource={rows}
      pagination={{ pageSize: 25, showSizeChanger: false }}
      columns={[
        {
          title: 'Time',
          dataIndex: 'time',
          width: 180,
          render: (t: string) => <Tag color="blue">{new Date(t).toLocaleString()}</Tag>,
        },
        ...fields.map((f) => ({
          title: f,
          dataIndex: ['payload', f],
          render: (v: unknown) =>
            v === undefined ? '' : typeof v === 'number' ? <code>{v.toFixed(2)}</code> : <code>{String(v)}</code>,
        })),
      ]}
    />
  );
}
