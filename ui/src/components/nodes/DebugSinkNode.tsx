import { Handle, Position } from '@xyflow/react';
import { Card } from 'antd';
import type { NodeProps } from '@xyflow/react';

export default function DebugSinkNode(_: NodeProps) {
  return (
    <div>
      <Handle type="target" position={Position.Left} />
      <Card size="small" title="Debug Sink" style={{ minWidth: 140 }}>
        <div style={{ fontSize: 12, color: '#888' }}>logs events</div>
      </Card>
    </div>
  );
}
