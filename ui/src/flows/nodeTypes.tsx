import { Handle, Position } from '@xyflow/react';
import { Card, Tag } from 'antd';
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { useMemo } from 'react';

type NodeProps<D> = { data: D };

type TelemetrySample = { time: string; payload: Record<string, unknown> };

export type DeviceNodeData = {
  deviceName?: string;
  deviceId?: string;
  series?: TelemetrySample[];
};
export type ConditionNodeData = { field?: string; op?: string; value?: number };
export type ActionNodeData = { kind?: string; config?: Record<string, unknown> };

const COLORS = ['#1677ff', '#52c41a', '#fa541c', '#722ed1', '#eb2f96', '#faad14'];

export function DeviceNode({ data }: NodeProps<DeviceNodeData>) {
  const { fields, points, latest } = useMemo(() => {
    const series = data.series ?? [];
    type Pt = { t: number } & Record<string, number>;
    const pts: Pt[] = [];
    const fieldSet = new Set<string>();
    for (const s of series) {
      const pt: Pt = { t: new Date(s.time).getTime() } as Pt;
      for (const [k, v] of Object.entries(s.payload)) {
        if (typeof v === 'number' && Number.isFinite(v)) {
          pt[k] = v;
          fieldSet.add(k);
        }
      }
      pts.push(pt);
    }
    return {
      fields: [...fieldSet],
      points: pts,
      latest: series[series.length - 1],
    };
  }, [data.series]);

  return (
    <Card size="small" title="Device" style={{ minWidth: 260 }} styles={{ body: { padding: 8 } }}>
      <div style={{ fontWeight: 500, marginBottom: 4 }}>
        {data.deviceName || <em style={{ color: '#999' }}>select a device</em>}
      </div>
      {!data.deviceName ? null : points.length === 0 ? (
        <div style={{ fontSize: 11, color: '#aaa' }}>waiting for telemetry…</div>
      ) : (
        <>
          <div style={{ width: '100%', height: 110 }}>
            <ResponsiveContainer>
              <LineChart data={points} margin={{ top: 4, right: 6, bottom: 0, left: 0 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  dataKey="t"
                  type="number"
                  scale="time"
                  domain={['auto', 'auto']}
                  tickFormatter={(t: number) => new Date(t).toLocaleTimeString()}
                  tick={{ fontSize: 10 }}
                  hide
                />
                <YAxis tick={{ fontSize: 10 }} width={28} />
                <Tooltip
                  labelFormatter={(t) => new Date(t as number).toLocaleTimeString()}
                  contentStyle={{ fontSize: 11 }}
                />
                {fields.map((f, i) => (
                  <Line
                    key={f}
                    type="monotone"
                    dataKey={f}
                    stroke={COLORS[i % COLORS.length]}
                    dot={false}
                    strokeWidth={1.5}
                    isAnimationActive={false}
                  />
                ))}
              </LineChart>
            </ResponsiveContainer>
          </div>
          {latest && (
            <div style={{ marginTop: 4, borderTop: '1px solid #f0f0f0', paddingTop: 4 }}>
              {fields.map((f, i) => {
                const v = latest.payload[f];
                return (
                  <div key={f} style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11 }}>
                    <span style={{ color: COLORS[i % COLORS.length] }}>● {f}</span>
                    <code>{typeof v === 'number' ? v.toFixed(2) : String(v)}</code>
                  </div>
                );
              })}
            </div>
          )}
        </>
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
