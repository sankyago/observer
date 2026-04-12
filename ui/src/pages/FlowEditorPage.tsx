import { useCallback, useEffect, useRef, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  ReactFlowProvider,
} from '@xyflow/react';
import type { Connection, Node, Edge, ReactFlowInstance } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Layout, Button, Switch, Input, message, Typography, Divider } from 'antd';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { getFlow, updateFlow } from '../api/client';
import type { Flow } from '../api/types';
import { nodeTypes } from '../components/nodes';
import NodePalette from '../components/NodePalette';
import NodeConfigPanel from '../components/NodeConfigPanel';
import EventsLog from '../components/EventsLog';
import { useFlowEvents } from '../hooks/useFlowEvents';

const { Header, Content } = Layout;
const { Text } = Typography;

let nodeCounter = 100;

function FlowEditorInner() {
  const { id } = useParams<{ id: string }>();
  const [flow, setFlow] = useState<Flow | null>(null);
  const [name, setName] = useState('');
  const [enabled, setEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  const reactFlowWrapper = useRef<HTMLDivElement>(null);
  const rfInstance = useRef<ReactFlowInstance | null>(null);

  const { events, connected } = useFlowEvents(id ?? '', enabled);

  useEffect(() => {
    if (!id) return;
    getFlow(id)
      .then((f) => {
        setFlow(f);
        setName(f.name);
        setEnabled(f.enabled);
        setNodes(f.graph.nodes as Node[]);
        setEdges(f.graph.edges as Edge[]);
      })
      .catch((e: Error) => void message.error(e.message));
  }, [id, setNodes, setEdges]);

  const onConnect = useCallback(
    (connection: Connection) => setEdges((eds) => addEdge(connection, eds)),
    [setEdges],
  );

  const onDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  }, []);

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const type = e.dataTransfer.getData('application/reactflow');
      if (!type || !rfInstance.current) return;

      const position = rfInstance.current.screenToFlowPosition({
        x: e.clientX,
        y: e.clientY,
      });

      const newNode: Node = {
        id: `node_${++nodeCounter}`,
        type,
        position,
        data: {},
      };
      setNodes((nds) => [...nds, newNode]);
    },
    [setNodes],
  );

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedNode(node);
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedNode(null);
  }, []);

  const handleNodeDataChange = useCallback(
    (nodeId: string, data: Record<string, unknown>) => {
      setNodes((nds) =>
        nds.map((n) => (n.id === nodeId ? { ...n, data } : n)),
      );
      setSelectedNode((prev) =>
        prev?.id === nodeId ? { ...prev, data } : prev,
      );
    },
    [setNodes],
  );

  const handleSave = async () => {
    if (!id) return;
    setSaving(true);
    try {
      await updateFlow(id, {
        name,
        enabled,
        graph: {
          nodes: nodes.map((n) => ({
            id: n.id,
            type: n.type as Flow['graph']['nodes'][0]['type'],
            position: n.position,
            data: n.data as Record<string, unknown>,
          })),
          edges: edges.map((e) => ({
            id: e.id,
            source: e.source,
            target: e.target,
          })),
        },
      });
      void message.success('Saved');
    } catch (e) {
      void message.error((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  if (!flow) {
    return <div style={{ padding: 24 }}>Loading...</div>;
  }

  return (
    <Layout style={{ height: '100vh' }}>
      <Header
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          padding: '0 16px',
        }}
      >
        <Link to="/">
          <Button icon={<ArrowLeftOutlined />} type="text" style={{ color: '#fff' }} />
        </Link>
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          style={{ width: 200 }}
        />
        <Switch
          checked={enabled}
          onChange={(val) => setEnabled(val)}
          checkedChildren="On"
          unCheckedChildren="Off"
        />
        <Button type="primary" onClick={() => void handleSave()} loading={saving}>
          Save
        </Button>
      </Header>
      <Content style={{ display: 'flex', height: 'calc(100vh - 64px)' }}>
        {/* Left palette */}
        <div style={{ width: 220, padding: 12, borderRight: '1px solid #f0f0f0', overflowY: 'auto' }}>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>Node Types</Text>
          <NodePalette />
        </div>

        {/* Center canvas */}
        <div ref={reactFlowWrapper} style={{ flex: 1 }}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onDrop={onDrop}
            onDragOver={onDragOver}
            onNodeClick={onNodeClick}
            onPaneClick={onPaneClick}
            onInit={(instance) => { rfInstance.current = instance; }}
            fitView
          >
            <Background />
            <Controls />
            <MiniMap />
          </ReactFlow>
        </div>

        {/* Right panel */}
        <div style={{ width: 320, padding: 12, borderLeft: '1px solid #f0f0f0', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <Text strong style={{ display: 'block', marginBottom: 8 }}>Config</Text>
            <NodeConfigPanel node={selectedNode} onChange={handleNodeDataChange} />
          </div>
          <Divider style={{ margin: '4px 0' }} />
          <EventsLog events={events} connected={connected} />
        </div>
      </Content>
    </Layout>
  );
}

export default function FlowEditorPage() {
  return (
    <ReactFlowProvider>
      <FlowEditorInner />
    </ReactFlowProvider>
  );
}
