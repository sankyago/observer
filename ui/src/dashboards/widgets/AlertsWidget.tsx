import { useMemo } from 'react';
import { Card, List, Tag } from 'antd';
import { useSse } from '../../useSse';

type FiredEvt = {
  type: 'fired';
  data: { fired_at: string; device_id: string; kind: string; status: string; payload: Record<string, unknown> };
};

export default function AlertsWidget() {
  const events = useSse('/api/v1/stream');
  const fired = useMemo(
    () => events.filter((e): e is FiredEvt => e.type === 'fired').slice(-30).reverse(),
    [events],
  );
  return (
    <Card size="small" title="Recent alerts" style={{ height: '100%' }} styles={{ body: { padding: 8, height: 'calc(100% - 40px)', overflowY: 'auto' } }}>
      <List
        size="small"
        dataSource={fired}
        locale={{ emptyText: 'no alerts yet' }}
        renderItem={(e) => (
          <List.Item>
            <Tag color={e.data.status === 'ok' ? 'green' : 'red'}>{e.data.status}</Tag>
            <Tag color="purple">{e.data.kind}</Tag>
            <code style={{ fontSize: 11 }}>{new Date(e.data.fired_at).toLocaleTimeString()}</code>
            <code style={{ fontSize: 11, marginLeft: 6 }}>{e.data.device_id.slice(0, 8)}</code>
          </List.Item>
        )}
      />
    </Card>
  );
}
