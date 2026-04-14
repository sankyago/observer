import { useEffect, useMemo, useState } from 'react';
import { Empty, Space, Typography } from 'antd';
import {
  CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts';
import { api } from '../api';
import { useSse } from '../useSse';

type Point = { t: number } & Record<string, number>;

type TelemetryEvt = {
  type: 'telemetry';
  data: { time: string; device_id: string; message_id: string; payload: Record<string, unknown> };
};

const MAX_POINTS = 200;

function numericFields(payload: Record<string, unknown>): Record<string, number> {
  const out: Record<string, number> = {};
  for (const [k, v] of Object.entries(payload)) {
    if (typeof v === 'number' && Number.isFinite(v)) out[k] = v;
  }
  return out;
}

const COLORS = ['#1677ff', '#52c41a', '#fa541c', '#722ed1', '#eb2f96', '#faad14'];

export default function DeviceChart({ deviceId }: { deviceId: string }) {
  const [points, setPoints] = useState<Point[]>([]);
  const events = useSse('/api/v1/stream');

  // Initial load
  useEffect(() => {
    let cancelled = false;
    api.recentTelemetry(deviceId, MAX_POINTS).then((rows) => {
      if (cancelled) return;
      // rows are DESC; flip to ASC for chart left→right
      const pts: Point[] = rows
        .slice()
        .reverse()
        .map((r) => ({ t: new Date(r.time).getTime(), ...numericFields(r.payload) }));
      setPoints(pts);
    });
    return () => { cancelled = true; };
  }, [deviceId]);

  // Live append from SSE
  useEffect(() => {
    if (events.length === 0) return;
    const last = events[events.length - 1];
    if (last.type !== 'telemetry') return;
    const evt = last as TelemetryEvt;
    if (evt.data.device_id !== deviceId) return;
    setPoints((prev) => {
      const next = [...prev, { t: new Date(evt.data.time).getTime(), ...numericFields(evt.data.payload) }];
      return next.length > MAX_POINTS ? next.slice(next.length - MAX_POINTS) : next;
    });
  }, [events, deviceId]);

  const fields = useMemo(() => {
    const s = new Set<string>();
    for (const p of points) for (const k of Object.keys(p)) if (k !== 't') s.add(k);
    return [...s];
  }, [points]);

  if (points.length === 0) {
    return (
      <Space direction="vertical" style={{ width: '100%' }}>
        <Empty description="No telemetry yet. Publish a message and it'll appear here live." />
      </Space>
    );
  }

  return (
    <div style={{ width: '100%', height: 360 }}>
      <Typography.Text type="secondary">
        {points.length} points · fields: {fields.join(', ') || '(none numeric)'}
      </Typography.Text>
      <ResponsiveContainer>
        <LineChart data={points} margin={{ top: 16, right: 24, left: 0, bottom: 8 }}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis
            dataKey="t"
            tickFormatter={(t: number) => new Date(t).toLocaleTimeString()}
            domain={['auto', 'auto']}
            type="number"
            scale="time"
          />
          <YAxis />
          <Tooltip labelFormatter={(t) => new Date(t as number).toLocaleString()} />
          <Legend />
          {fields.map((f, i) => (
            <Line
              key={f}
              type="monotone"
              dataKey={f}
              stroke={COLORS[i % COLORS.length]}
              dot={false}
              isAnimationActive={false}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
