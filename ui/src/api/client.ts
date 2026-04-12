import type { Flow, Graph } from './types';

const BASE = '';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  });
  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      const body = await res.json() as { error?: string };
      if (body.error) msg = body.error;
    } catch {
      // ignore parse errors
    }
    throw new Error(msg);
  }
  return res.json() as Promise<T>;
}

export function listFlows(): Promise<Flow[]> {
  return request<Flow[]>('/api/flows');
}

export function getFlow(id: string): Promise<Flow> {
  return request<Flow>(`/api/flows/${id}`);
}

export function createFlow(name: string, graph: Graph): Promise<Flow> {
  return request<Flow>('/api/flows', {
    method: 'POST',
    body: JSON.stringify({ name, graph }),
  });
}

export function updateFlow(
  id: string,
  patch: Partial<{ name: string; graph: Graph; enabled: boolean }>,
): Promise<Flow> {
  return request<Flow>(`/api/flows/${id}`, {
    method: 'PUT',
    body: JSON.stringify(patch),
  });
}

export function deleteFlow(id: string): Promise<void> {
  return request<void>(`/api/flows/${id}`, { method: 'DELETE' });
}
