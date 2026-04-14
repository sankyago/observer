import { useMemo } from 'react';
import { List, Tag, Typography } from 'antd';
import { useSse } from '../useSse';

type TelemetryEvt = {
  type: 'telemetry';
  data: { time: string; device_id: string; message_id: string; payload: Record<string, unknown> };
};

type FiredEvt = {
  type: 'fired';
  data: {
    fired_at: string; device_id: string; flow_id: string; node_id: string; kind: string;
    message_id: string; status: string; error: string; payload: Record<string, unknown>;
  };
};

export default function HomePage() {
  const events = useSse('/api/v1/stream');
  const telemetry = useMemo(
    () => events.filter((e): e is TelemetryEvt => e.type === 'telemetry').slice(-50).reverse(),
    [events],
  );
  const fired = useMemo(
    () => events.filter((e): e is FiredEvt => e.type === 'fired').slice(-50).reverse(),
    [events],
  );

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
      <div>
        <Typography.Title level={4}>Live telemetry</Typography.Title>
        <List
          bordered
          size="small"
          dataSource={telemetry}
          locale={{ emptyText: 'no messages yet — publish to mqtt://localhost:1883' }}
          renderItem={(e) => (
            <List.Item>
              <Tag color="blue">{new Date(e.data.time).toLocaleTimeString()}</Tag>
              <code style={{ marginLeft: 8 }}>{e.data.device_id.slice(0, 8)}</code>
              <code style={{ marginLeft: 8 }}>{JSON.stringify(e.data.payload)}</code>
            </List.Item>
          )}
        />
      </div>
      <div>
        <Typography.Title level={4}>Alerts</Typography.Title>
        <List
          bordered
          size="small"
          dataSource={fired}
          locale={{ emptyText: 'no alerts yet' }}
          renderItem={(e) => (
            <List.Item>
              <Tag color={e.data.status === 'ok' ? 'green' : 'red'}>{e.data.status}</Tag>
              <Tag>{new Date(e.data.fired_at).toLocaleTimeString()}</Tag>
              <Tag color="purple">{e.data.kind}</Tag>
              <code style={{ marginLeft: 8 }}>{e.data.device_id.slice(0, 8)}</code>
              <code style={{ marginLeft: 8 }}>{JSON.stringify(e.data.payload)}</code>
              {e.data.error && <span style={{ color: 'red', marginLeft: 8 }}>{e.data.error}</span>}
            </List.Item>
          )}
        />
      </div>
    </div>
  );
}
