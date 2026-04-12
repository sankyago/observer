export type NodeType = 'mqtt_source' | 'threshold' | 'rate_of_change' | 'debug_sink';

export interface FlowNode {
  id: string;
  type: NodeType;
  position: { x: number; y: number };
  data: Record<string, unknown>;
}

export interface FlowEdge {
  id: string;
  source: string;
  target: string;
}

export interface Graph {
  nodes: FlowNode[];
  edges: FlowEdge[];
}

export interface Flow {
  id: string;
  name: string;
  graph: Graph;
  enabled: boolean;
  created_at: string;
}

export interface FlowEvent {
  kind: 'reading' | 'alert' | 'error';
  node_id: string;
  reading?: {
    device_id: string;
    metric: string;
    value: number;
    ts: string;
  };
  detail?: string;
  ts: string;
}

export interface MQTTSourceData {
  broker: string;
  topic: string;
  username?: string;
  password?: string;
}

export interface ThresholdData {
  min: number;
  max: number;
}

export interface RateOfChangeData {
  max_per_second: number;
  window_size: number;
}
