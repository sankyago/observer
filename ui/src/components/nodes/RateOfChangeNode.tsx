import { Handle, Position } from '@xyflow/react';
import { Card } from 'antd';
import type { NodeProps } from '@xyflow/react';

export default function RateOfChangeNode({ data }: NodeProps) {
  const d = data as { max_per_second?: number; window_size?: number };
  return (
    <div>
      <Handle type="target" position={Position.Left} />
      <Card size="small" title="Rate of Change" style={{ minWidth: 160 }}>
        <div style={{ fontSize: 12 }}>
          max {d.max_per_second ?? '–'}/s · window {d.window_size ?? '–'}
        </div>
      </Card>
      <Handle type="source" position={Position.Right} />
    </div>
  );
}
