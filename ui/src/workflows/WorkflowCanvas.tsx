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
    const node: Node = {
      id,
      type: kind,
      position: { x: 80 + Math.random() * 400, y: 80 + Math.random() * 200 },
      data: defaultDataFor(kind),
    };
    onChange({ ...value, nodes: [...value.nodes, node] });
  };

  const nodesWithHandlers = value.nodes.map((n) => ({
    ...n,
    data: {
      ...n.data,
      onChange: (patch: Record<string, unknown>) => {
        onChange({
          ...value,
          nodes: value.nodes.map((m) => (m.id === n.id ? { ...m, data: { ...m.data, ...patch } } : m)),
        });
      },
    },
  }));

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
          nodes={nodesWithHandlers}
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
    nodes: s.nodes.map((n) => ({ id: n.id, kind: n.type, config: { ...n.data } })),
    edges: s.edges.map((e) => ({ source: e.source, target: e.target })),
  };
}

// fromBackendConfig reconstructs WorkflowState from the saved `config` JSONB.
// The persisted shape is { nodes: [{id, kind, config}], edges: [{source, target}] }.
export function fromBackendConfig(cfg: Record<string, unknown>): WorkflowState {
  const rawNodes = (cfg.nodes as Array<{ id: string; kind: string; config?: Record<string, unknown> }> | undefined) ?? [];
  const rawEdges = (cfg.edges as Array<{ source: string; target: string }> | undefined) ?? [];
  const nodes: Node[] = rawNodes.map((n, i) => ({
    id: n.id,
    type: n.kind,
    position: { x: 80 + (i % 3) * 260, y: 80 + Math.floor(i / 3) * 160 },
    data: { ...(n.config ?? {}) },
  }));
  const edges: Edge[] = rawEdges.map((e, i) => ({ id: `e${i}`, source: e.source, target: e.target }));
  return { nodes, edges };
}
