export type Device = {
  id: string;
  tenant_id: string;
  name: string;
  type: string;
  created_at: string;
};

export type FlowGraph = {
  nodes: Array<{
    id: string;
    type: 'device' | 'condition' | 'action';
    position: { x: number; y: number };
    data: Record<string, unknown>;
  }>;
  edges: Array<{ id: string; source: string; target: string }>;
};

export type Flow = {
  id: string;
  tenant_id: string;
  name: string;
  graph: FlowGraph;
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type FlowInput = {
  name: string;
  graph: FlowGraph;
  enabled: boolean;
};

const BASE = '/api/v1';

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${method} ${path}: ${res.status} ${text}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  listDevices: () => req<Device[]>('GET', '/devices'),
  createDevice: (name: string, type: string) => req<Device>('POST', '/devices', { name, type }),
  deleteDevice: (id: string) => req<void>('DELETE', `/devices/${id}`),

  listFlows: () => req<Flow[]>('GET', '/flows'),
  getFlow: (id: string) => req<Flow>('GET', `/flows/${id}`),
  createFlow: (in_: FlowInput) => req<Flow>('POST', '/flows', in_),
  updateFlow: (id: string, in_: FlowInput) => req<Flow>('PUT', `/flows/${id}`, in_),
  deleteFlow: (id: string) => req<void>('DELETE', `/flows/${id}`),

  recentTelemetry: (deviceId: string, limit = 100) =>
    req<Array<{ time: string; device_id: string; message_id: string; payload: Record<string, unknown> }>>(
      'GET',
      `/telemetry/recent?device_id=${deviceId}&limit=${limit}`,
    ),
};
