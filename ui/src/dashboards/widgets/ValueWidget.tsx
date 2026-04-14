import { useMemo } from 'react';
import { Card, Statistic } from 'antd';
import { useSse } from '../../useSse';

export default function ValueWidget({ deviceId, field, unit }: { deviceId: string; field: string; unit?: string }) {
  const events = useSse('/api/v1/stream');
  const latest = useMemo(() => {
    for (let i = events.length - 1; i >= 0; i--) {
      const e = events[i];
      if (e.type !== 'telemetry') continue;
      const d = e.data as { device_id?: string; payload?: Record<string, unknown>; time?: string };
      if (d.device_id === deviceId && d.payload && field in d.payload) {
        return { value: d.payload[field], time: d.time };
      }
    }
    return null;
  }, [events, deviceId, field]);

  const value = typeof latest?.value === 'number' ? latest.value : (latest?.value as string | undefined) ?? '—';

  return (
    <Card size="small" style={{ height: '100%' }} styles={{ body: { padding: 12 } }}>
      <Statistic
        title={`${field}${unit ? ' (' + unit + ')' : ''}`}
        value={typeof value === 'number' ? value.toFixed(2) : String(value)}
      />
    </Card>
  );
}
