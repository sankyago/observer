export type Device = {
  id: string;
  tenant_id: string;
  name: string;
  type: string;
  created_at: string;
};

export type Action = {
  id: string;
  tenant_id: string;
  kind: 'log' | 'webhook' | 'email' | 'workflow';
  config: Record<string, unknown>;
  created_at: string;
};

export type Rule = {
  id: string;
  tenant_id: string;
  device_id: string;
  field: string;
  op: '>' | '<' | '>=' | '<=' | '=' | '!=';
  value: number;
  action_id: string;
  enabled: boolean;
  created_at: string;
};

export type RuleInput = Omit<Rule, 'id' | 'tenant_id' | 'created_at'>;

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

  listActions: () => req<Action[]>('GET', '/actions'),
  createAction: (kind: Action['kind'], config: Record<string, unknown>) =>
    req<Action>('POST', '/actions', { kind, config }),
  updateAction: (id: string, kind: Action['kind'], config: Record<string, unknown>) =>
    req<Action>('PUT', `/actions/${id}`, { kind, config }),
  deleteAction: (id: string) => req<void>('DELETE', `/actions/${id}`),

  listRules: () => req<Rule[]>('GET', '/rules'),
  createRule: (r: RuleInput) => req<Rule>('POST', '/rules', r),
  updateRule: (id: string, r: RuleInput) => req<Rule>('PUT', `/rules/${id}`, r),
  deleteRule: (id: string) => req<void>('DELETE', `/rules/${id}`),

  recentTelemetry: (deviceId: string, limit = 100) =>
    req<Array<{ time: string; device_id: string; message_id: string; payload: Record<string, unknown> }>>(
      'GET',
      `/telemetry/recent?device_id=${deviceId}&limit=${limit}`,
    ),
};
