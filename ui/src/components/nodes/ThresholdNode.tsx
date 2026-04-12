import { Handle, Position } from '@xyflow/react';
import { Card } from 'antd';
import type { NodeProps } from '@xyflow/react';

export default function ThresholdNode({ data }: NodeProps) {
  const d = data as { min?: number; max?: number };
  return (
    <div>
      <Handle type="target" position={Position.Left} />
      <Card size="small" title="Threshold" style={{ minWidth: 160 }}>
        <div style={{ fontSize: 12 }}>
          {d.min ?? '–'} .. {d.max ?? '–'}
        </div>
      </Card>
      <Handle type="source" position={Position.Right} />
    </div>
  );
}
