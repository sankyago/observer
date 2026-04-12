import { List, Tag, Typography } from 'antd';
import type { FlowEvent } from '../api/types';

const { Text } = Typography;

interface Props {
  events: FlowEvent[];
  connected: boolean;
}

const KIND_COLOR: Record<FlowEvent['kind'], string> = {
  reading: 'blue',
  alert: 'red',
  error: 'orange',
};

export default function EventsLog({ events, connected }: Props) {
  return (
    <div>
      <div style={{ marginBottom: 4 }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          Events{' '}
          {connected ? (
            <Tag color="green" style={{ fontSize: 11 }}>live</Tag>
          ) : (
            <Tag color="default" style={{ fontSize: 11 }}>disconnected</Tag>
          )}
        </Text>
      </div>
      <div style={{ maxHeight: 300, overflowY: 'auto', border: '1px solid #f0f0f0', borderRadius: 4 }}>
        <List
          size="small"
          dataSource={events}
          locale={{ emptyText: 'No events yet.' }}
          renderItem={(ev) => (
            <List.Item style={{ padding: '4px 8px', fontSize: 12 }}>
              <Tag color={KIND_COLOR[ev.kind]} style={{ marginRight: 4 }}>
                {ev.kind}
              </Tag>
              <Text type="secondary" style={{ marginRight: 4, fontSize: 11 }}>
                {new Date(ev.ts).toLocaleTimeString()}
              </Text>
              <Text style={{ fontSize: 12 }}>
                {ev.detail ?? (ev.reading ? `${ev.reading.metric}=${ev.reading.value}` : '')}
              </Text>
            </List.Item>
          )}
        />
      </div>
    </div>
  );
}
