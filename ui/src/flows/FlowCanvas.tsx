import { useCallback, useMemo, useState } from 'react';
import {
  ReactFlow, Background, Controls, MiniMap,
  addEdge, applyEdgeChanges, applyNodeChanges,
  type Node, type Edge, type Connection, type OnNodesChange, type OnEdgesChange,
} from '@xyflow/react';
import { Button, Card, Form, Input, InputNumber, Select, Space, Typography } from 'antd';
import { nanoid } from 'nanoid';
import { nodeTypes } from './nodeTypes';
import type { Device, FlowGraph } from '../api';

type Props = {
  value: FlowGraph;
  onChange: (g: FlowGraph) => void;
  devices: Device[];
};

export default function FlowCanvas({ value, onChange, devices }: Props) {
  const [selected, setSelected] = useState<string | null>(null);

  const onNodesChange: OnNodesChange = useCallback(
    (changes) => onChange({ ...value, nodes: applyNodeChanges(changes, value.nodes) as FlowGraph['nodes'] }),
    [value, onChange],
  );
  const onEdgesChange: OnEdgesChange = useCallback(
    (changes) => onChange({ ...value, edges: applyEdgeChanges(changes, value.edges) as FlowGraph['edges'] }),
    [value, onChange],
  );
  const onConnect = useCallback(
    (c: Connection) => onChange({ ...value, edges: addEdge({ ...c, id: nanoid(8) }, value.edges) as FlowGraph['edges'] }),
    [value, onChange],
  );

  const addNode = (type: 'device' | 'condition' | 'action') => {
    const id = nanoid(8);
    const defaults: Record<string, Record<string, unknown>> = {
      device: {},
      condition: { field: 'temperature', op: '>', value: 80 },
      action: { kind: 'log', config: {} },
    };
    const node: FlowGraph['nodes'][number] = {
      id,
      type,
      position: { x: 80 + Math.random() * 400, y: 80 + Math.random() * 200 },
      data: defaults[type],
    };
    onChange({ ...value, nodes: [...value.nodes, node] });
    setSelected(id);
  };

  const updateNodeData = (id: string, patch: Record<string, unknown>) => {
    onChange({
      ...value,
      nodes: value.nodes.map((n) => (n.id === id ? { ...n, data: { ...n.data, ...patch } } : n)),
    });
  };

  const deleteSelected = () => {
    if (!selected) return;
    onChange({
      nodes: value.nodes.filter((n) => n.id !== selected),
      edges: value.edges.filter((e) => e.source !== selected && e.target !== selected),
    });
    setSelected(null);
  };

  // Decorate nodes for display (device node shows its device name)
  const displayNodes: Node[] = useMemo(
    () =>
      value.nodes.map((n) => {
        if (n.type === 'device') {
          const dev = devices.find((d) => d.id === (n.data.device_id as string | undefined));
          return { ...n, data: { ...n.data, deviceName: dev?.name, deviceId: n.data.device_id } } as Node;
        }
        return n as Node;
      }),
    [value.nodes, devices],
  );

  const selectedNode = selected ? value.nodes.find((n) => n.id === selected) : undefined;

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '160px 1fr 320px', gap: 16, height: 'calc(100vh - 220px)' }}>
      <Space direction="vertical" style={{ padding: 8 }}>
        <Typography.Text strong>Add node</Typography.Text>
        <Button block onClick={() => addNode('device')}>Device</Button>
        <Button block onClick={() => addNode('condition')}>Condition</Button>
        <Button block onClick={() => addNode('action')}>Action</Button>
        <Button block danger disabled={!selected} onClick={deleteSelected}>Delete selected</Button>
      </Space>

      <div style={{ border: '1px solid #eee' }}>
        <ReactFlow
          nodes={displayNodes}
          edges={value.edges as Edge[]}
          nodeTypes={nodeTypes}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          onNodeClick={(_, n) => setSelected(n.id)}
          onPaneClick={() => setSelected(null)}
          fitView
        >
          <Background />
          <Controls />
          <MiniMap pannable zoomable />
        </ReactFlow>
      </div>

      <Card size="small" title={selectedNode ? `Edit ${selectedNode.type}` : 'Properties'}>
        {!selectedNode && <Typography.Text type="secondary">Click a node to edit its fields.</Typography.Text>}
        {selectedNode?.type === 'device' && (
          <Form layout="vertical">
            <Form.Item label="Device">
              <Select
                placeholder="Select device"
                value={selectedNode.data.device_id as string | undefined}
                options={devices.map((d) => ({ value: d.id, label: d.name }))}
                onChange={(v) => updateNodeData(selectedNode.id, { device_id: v })}
              />
            </Form.Item>
          </Form>
        )}
        {selectedNode?.type === 'condition' && (
          <Form layout="vertical">
            <Form.Item label="Field">
              <Input
                value={selectedNode.data.field as string | undefined}
                onChange={(e) => updateNodeData(selectedNode.id, { field: e.target.value })}
              />
            </Form.Item>
            <Form.Item label="Operator">
              <Select
                value={selectedNode.data.op as string | undefined}
                options={['>', '<', '>=', '<=', '=', '!='].map((v) => ({ value: v, label: v }))}
                onChange={(v) => updateNodeData(selectedNode.id, { op: v })}
              />
            </Form.Item>
            <Form.Item label="Value">
              <InputNumber
                style={{ width: '100%' }}
                value={selectedNode.data.value as number | undefined}
                onChange={(v) => updateNodeData(selectedNode.id, { value: Number(v) || 0 })}
              />
            </Form.Item>
          </Form>
        )}
        {selectedNode?.type === 'action' && (
          <Form layout="vertical">
            <Form.Item label="Kind">
              <Select
                value={(selectedNode.data.kind as string | undefined) || 'log'}
                options={[
                  { value: 'log', label: 'log' },
                  { value: 'webhook', label: 'webhook' },
                  { value: 'email', label: 'email' },
                ]}
                onChange={(v) => updateNodeData(selectedNode.id, { kind: v, config: {} })}
              />
            </Form.Item>
            {selectedNode.data.kind === 'webhook' && (
              <Form.Item label="Webhook URL">
                <Input
                  placeholder="https://example.com/hook"
                  value={(selectedNode.data.config as { url?: string } | undefined)?.url || ''}
                  onChange={(e) =>
                    updateNodeData(selectedNode.id, {
                      config: { ...(selectedNode.data.config as Record<string, unknown>), url: e.target.value },
                    })
                  }
                />
              </Form.Item>
            )}
          </Form>
        )}
      </Card>
    </div>
  );
}

export function emptyGraph(): FlowGraph {
  return { nodes: [], edges: [] };
}
