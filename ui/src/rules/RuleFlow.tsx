import { useMemo, useState } from 'react';
import { ReactFlow, Background, Controls } from '@xyflow/react';
import { nodeTypes, type ConditionNodeData } from './nodeTypes';
import type { Device, Action, RuleInput } from '../api';

export type RuleFlowProps = {
  devices: Device[];
  actions: Action[];
  initial?: RuleInput;
  onChange: (r: RuleInput) => void;
};

export default function RuleFlow({ devices, actions, initial, onChange }: RuleFlowProps) {
  const [deviceId, setDeviceId] = useState(initial?.device_id ?? '');
  const [cond, setCond] = useState<Omit<ConditionNodeData, 'onChange'>>({
    field: initial?.field ?? 'temperature',
    op: initial?.op ?? '>',
    value: initial?.value ?? 80,
  });
  const [actionId, setActionId] = useState(initial?.action_id ?? '');

  const emit = (next: Partial<RuleInput>) => {
    onChange({
      device_id: next.device_id ?? deviceId,
      field: next.field ?? cond.field,
      op: (next.op ?? cond.op) as RuleInput['op'],
      value: next.value ?? cond.value,
      action_id: next.action_id ?? actionId,
      enabled: true,
    });
  };

  const nodes = useMemo(
    () => [
      {
        id: 'device', type: 'device', position: { x: 0, y: 80 },
        data: { devices, value: deviceId, onChange: (v: string) => { setDeviceId(v); emit({ device_id: v }); } },
      },
      {
        id: 'condition', type: 'condition', position: { x: 260, y: 40 },
        data: {
          ...cond,
          onChange: (patch: Partial<Omit<ConditionNodeData, 'onChange'>>) => {
            setCond({ ...cond, ...patch });
            emit(patch);
          },
        },
      },
      {
        id: 'action', type: 'action', position: { x: 560, y: 80 },
        data: { actions, value: actionId, onChange: (v: string) => { setActionId(v); emit({ action_id: v }); } },
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [devices, actions, deviceId, cond, actionId],
  );

  const edges = [
    { id: 'e1', source: 'device', target: 'condition' },
    { id: 'e2', source: 'condition', target: 'action' },
  ];

  return (
    <div style={{ height: 320, border: '1px solid #eee' }}>
      <ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes} fitView>
        <Background />
        <Controls />
      </ReactFlow>
    </div>
  );
}
