import { Handle, Position } from '@xyflow/react';
import { Card } from 'antd';
import type { NodeProps } from '@xyflow/react';

export default function MQTTSourceNode({ data }: NodeProps) {
  const d = data as { broker?: string; topic?: string };
  return (
    <div>
      <Card size="small" title="MQTT Source" style={{ minWidth: 160 }}>
        <div style={{ fontSize: 12 }}>{d.topic ?? 'sensors/#'}</div>
        <div style={{ fontSize: 11, color: '#888' }}>{d.broker ?? ''}</div>
      </Card>
      <Handle type="source" position={Position.Right} />
    </div>
  );
}
