import { useCallback } from 'react';
import {
  ReactFlow, Background, Controls, MiniMap,
  addEdge, applyEdgeChanges, applyNodeChanges,
  type Node, type Edge, type Connection, type OnNodesChange, type OnEdgesChange,
} from '@xyflow/react';
import { Button, Space } from 'antd';
import { nanoid } from 'nanoid';
import { nodeTypes, type WorkflowNodeKind } from './nodeTypes';

export type WorkflowState = { nodes: Node[]; edges: Edge[] };

export function emptyWorkflow(): WorkflowState {
  return { nodes: [], edges: [] };
}

type Props = {
  value: WorkflowState;
  onChange: (s: WorkflowState) => void;
};

export default function WorkflowCanvas({ value, onChange }: Props) {
  const onNodesChange: OnNodesChange = useCallback(
    (changes) => onChange({ ...value, nodes: applyNodeChanges(changes, value.nodes) }),
    [value, onChange],
  );
  const onEdgesChange: OnEdgesChange = useCallback(
    (changes) => onChange({ ...value, edges: applyEdgeChanges(changes, value.edges) }),
    [value, onChange],
  );
  const onConnect = useCallback(
    (c: Connection) => onChange({ ...value, edges: addEdge(c, value.edges) }),
    [value, onChange],
  );

  const addNode = (kind: WorkflowNodeKind) => {
    const id = nanoid(8);
    const base: Node = {
      id,
      type: kind,
      position: { x: 80 + Math.random() * 400, y: 80 + Math.random() * 200 },
      data: {
        onChange: (patch: Record<string, unknown>) => {
          onChange({
            ...value,
            nodes: value.nodes.map((n) =>
              n.id === id ? { ...n, data: { ...n.data, ...patch } } : n,
            ),
          });
        },
        ...defaultDataFor(kind),
      },
    };
    onChange({ ...value, nodes: [...value.nodes, base] });
  };

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '180px 1fr', gap: 16 }}>
      <Space direction="vertical" style={{ padding: 8 }}>
        <strong>Add node</strong>
        <Button block onClick={() => addNode('log')}>Log</Button>
        <Button block onClick={() => addNode('webhook')}>Webhook</Button>
        <Button block onClick={() => addNode('email')}>Email</Button>
      </Space>
      <div style={{ height: 420, border: '1px solid #eee' }}>
        <ReactFlow
          nodes={value.nodes}
          edges={value.edges}
          nodeTypes={nodeTypes}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          fitView
        >
          <Background />
          <Controls />
          <MiniMap pannable zoomable />
        </ReactFlow>
      </div>
    </div>
  );
}

function defaultDataFor(kind: WorkflowNodeKind): Record<string, unknown> {
  if (kind === 'webhook') return { url: '' };
  if (kind === 'email') return { to: '', subject: '' };
  return {};
}

// Convert UI state to the backend `config` payload: {nodes:[{id,kind,config}], edges:[{source,target}]}
export function toBackendConfig(s: WorkflowState): Record<string, unknown> {
  return {
    nodes: s.nodes.map((n) => {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { onChange: _oc, ...rest } = n.data as Record<string, unknown>;
      return { id: n.id, kind: n.type, config: rest };
    }),
    edges: s.edges.map((e) => ({ source: e.source, target: e.target })),
  };
}
