import { Handle, Position } from '@xyflow/react';
import { Card, Select, InputNumber } from 'antd';
import type { Device, Action } from '../api';

export type DeviceNodeData = {
  devices: Device[];
  value: string;
  onChange: (id: string) => void;
};
export type ConditionNodeData = {
  field: string;
  op: '>' | '<' | '>=' | '<=' | '=' | '!=';
  value: number;
  onChange: (patch: Partial<Omit<ConditionNodeData, 'onChange'>>) => void;
};
export type ActionNodeData = {
  actions: Action[];
  value: string;
  onChange: (id: string) => void;
};

export function DeviceNode({ data }: { data: DeviceNodeData }) {
  const d = data;
  return (
    <Card size="small" title="Device" style={{ minWidth: 200 }}>
      <Select
        style={{ width: '100%' }}
        placeholder="Select device"
        value={d.value || undefined}
        options={d.devices.map((x) => ({ value: x.id, label: x.name }))}
        onChange={d.onChange}
      />
      <Handle type="source" position={Position.Right} />
    </Card>
  );
}

export function ConditionNode({ data }: { data: ConditionNodeData }) {
  const d = data;
  return (
    <Card size="small" title="Condition" style={{ minWidth: 240 }}>
      <Handle type="target" position={Position.Left} />
      <input
        value={d.field}
        placeholder="field (e.g. temperature)"
        onChange={(e) => d.onChange({ field: e.target.value })}
        style={{ width: '100%', marginBottom: 6, padding: 4 }}
      />
      <Select
        style={{ width: '100%', marginBottom: 6 }}
        value={d.op}
        options={['>', '<', '>=', '<=', '=', '!='].map((v) => ({ value: v, label: v }))}
        onChange={(v) => d.onChange({ op: v as ConditionNodeData['op'] })}
      />
      <InputNumber
        style={{ width: '100%' }}
        value={d.value}
        onChange={(v) => d.onChange({ value: Number(v) || 0 })}
      />
      <Handle type="source" position={Position.Right} />
    </Card>
  );
}

export function ActionNode({ data }: { data: ActionNodeData }) {
  const d = data;
  return (
    <Card size="small" title="Action" style={{ minWidth: 200 }}>
      <Handle type="target" position={Position.Left} />
      <Select
        style={{ width: '100%' }}
        placeholder="Select action"
        value={d.value || undefined}
        options={d.actions.map((x) => ({ value: x.id, label: `${x.kind} — ${x.id.slice(0, 6)}` }))}
        onChange={d.onChange}
      />
    </Card>
  );
}

export const nodeTypes = { device: DeviceNode, condition: ConditionNode, action: ActionNode };
