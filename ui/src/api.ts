export type Device = {
  id: string;
  tenant_id: string;
  name: string;
  type: string;
  profile_id: string | null;
  group_id: string | null;
  created_at: string;
};

export type DeviceInput = {
  name: string;
  type: string;
  profile_id?: string | null;
  group_id?: string | null;
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

export type DeviceProfile = {
  id: string;
  tenant_id: string;
  name: string;
  default_fields: string[];
  created_at: string;
};

export type DeviceGroup = {
  id: string;
  tenant_id: string;
  name: string;
  created_at: string;
};

export type Widget = {
  id: string;
  type: 'chart' | 'map' | 'value' | 'alerts';
  config: Record<string, unknown>;
  x: number;
  y: number;
  w: number;
  h: number;
};

export type DashboardLayout = { widgets: Widget[] };

export type Dashboard = {
  id: string;
  tenant_id: string;
  name: string;
  layout: DashboardLayout;
  created_at: string;
  updated_at: string;
};

export type DashboardInput = { name: string; layout: DashboardLayout };

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
  createDevice: (in_: DeviceInput) => req<Device>('POST', '/devices', in_),
  updateDevice: (id: string, in_: DeviceInput) => req<Device>('PUT', `/devices/${id}`, in_),
  deleteDevice: (id: string) => req<void>('DELETE', `/devices/${id}`),

  listProfiles: () => req<DeviceProfile[]>('GET', '/profiles'),
  createProfile: (name: string, default_fields: string[]) => req<DeviceProfile>('POST', '/profiles', { name, default_fields }),
  deleteProfile: (id: string) => req<void>('DELETE', `/profiles/${id}`),

  listGroups: () => req<DeviceGroup[]>('GET', '/groups'),
  createGroup: (name: string) => req<DeviceGroup>('POST', '/groups', { name }),
  deleteGroup: (id: string) => req<void>('DELETE', `/groups/${id}`),

  listDashboards: () => req<Dashboard[]>('GET', '/dashboards'),
  getDashboard: (id: string) => req<Dashboard>('GET', `/dashboards/${id}`),
  createDashboard: (in_: DashboardInput) => req<Dashboard>('POST', '/dashboards', in_),
  updateDashboard: (id: string, in_: DashboardInput) => req<Dashboard>('PUT', `/dashboards/${id}`, in_),
  deleteDashboard: (id: string) => req<void>('DELETE', `/dashboards/${id}`),

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
