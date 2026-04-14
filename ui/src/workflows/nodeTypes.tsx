import { Handle, Position } from '@xyflow/react';
import { Card, Input } from 'antd';

export type BaseNodeData = { onChange: (patch: Record<string, unknown>) => void };
export type LogNodeData = BaseNodeData;
export type WebhookNodeData = BaseNodeData & { url: string };
export type EmailNodeData = BaseNodeData & { to: string; subject: string };

function NodeFrame({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Card size="small" title={title} style={{ minWidth: 220 }}>
      <Handle type="target" position={Position.Left} />
      {children}
      <Handle type="source" position={Position.Right} />
    </Card>
  );
}

export function LogNode(_: { data: LogNodeData }) {
  return <NodeFrame title="Log">writes an ALERT log line</NodeFrame>;
}

export function WebhookNode({ data }: { data: WebhookNodeData }) {
  return (
    <NodeFrame title="Webhook">
      <Input
        placeholder="https://example.com/hook"
        value={data.url}
        onChange={(e) => data.onChange({ url: e.target.value })}
      />
    </NodeFrame>
  );
}

export function EmailNode({ data }: { data: EmailNodeData }) {
  return (
    <NodeFrame title="Email">
      <Input
        placeholder="to: foo@bar.com"
        value={data.to}
        onChange={(e) => data.onChange({ to: e.target.value })}
        style={{ marginBottom: 6 }}
      />
      <Input
        placeholder="subject"
        value={data.subject}
        onChange={(e) => data.onChange({ subject: e.target.value })}
      />
    </NodeFrame>
  );
}

export const nodeTypes = { log: LogNode, webhook: WebhookNode, email: EmailNode };
export type WorkflowNodeKind = keyof typeof nodeTypes;
