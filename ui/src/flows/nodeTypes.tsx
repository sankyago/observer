import { Handle, Position } from '@xyflow/react';
import { Card, Tag } from 'antd';

type NodeProps<D> = { data: D };

export type DeviceNodeData = {
  deviceName?: string;
  deviceId?: string;
  latestPayload?: Record<string, unknown>;
  latestTime?: string;
};
export type ConditionNodeData = { field?: string; op?: string; value?: number };
export type ActionNodeData = { kind?: string; config?: Record<string, unknown> };

export function DeviceNode({ data }: NodeProps<DeviceNodeData>) {
  const entries = data.latestPayload
    ? Object.entries(data.latestPayload).filter(([, v]) => v !== null && v !== undefined)
    : [];
  return (
    <Card size="small" title="Device" style={{ minWidth: 220 }}>
      <div style={{ fontWeight: 500 }}>
        {data.deviceName || <em style={{ color: '#999' }}>select a device</em>}
      </div>
      {data.deviceName && (
        entries.length > 0 ? (
          <div style={{ marginTop: 6, borderTop: '1px solid #f0f0f0', paddingTop: 6 }}>
            {entries.map(([k, v]) => (
              <div key={k} style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
                <span style={{ color: '#666' }}>{k}</span>
                <code>{typeof v === 'number' ? v.toFixed(2) : String(v)}</code>
              </div>
            ))}
            {data.latestTime && (
              <div style={{ fontSize: 10, color: '#aaa', marginTop: 4 }}>
                {new Date(data.latestTime).toLocaleTimeString()}
              </div>
            )}
          </div>
        ) : (
          <div style={{ marginTop: 6, fontSize: 11, color: '#aaa' }}>waiting for telemetry…</div>
        )
      )}
      <Handle type="source" position={Position.Right} />
    </Card>
  );
}

export function ConditionNode({ data }: NodeProps<ConditionNodeData>) {
  const filled = data.field && data.op;
  return (
    <Card size="small" title="Condition" style={{ minWidth: 180 }}>
      <Handle type="target" position={Position.Left} />
      <div>
        {filled ? (
          <code>{`${data.field} ${data.op} ${data.value ?? 0}`}</code>
        ) : (
          <em style={{ color: '#999' }}>configure threshold</em>
        )}
      </div>
      <Handle type="source" position={Position.Right} />
    </Card>
  );
}

export function ActionNode({ data }: NodeProps<ActionNodeData>) {
  return (
    <Card size="small" title="Action" style={{ minWidth: 180 }}>
      <Handle type="target" position={Position.Left} />
      <Tag color="purple">{data.kind || 'log'}</Tag>
      {data.kind === 'webhook' && (data.config?.url as string | undefined) && (
        <div style={{ fontSize: 11, marginTop: 4, color: '#666', wordBreak: 'break-all' }}>
          {data.config?.url as string}
        </div>
      )}
      <Handle type="source" position={Position.Right} />
    </Card>
  );
}

export const nodeTypes = { device: DeviceNode, condition: ConditionNode, action: ActionNode };
