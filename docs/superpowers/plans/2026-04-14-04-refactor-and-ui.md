# Observer — Sub-project 4: Bus Refactor + React UI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal, part A:** Remove Postgres-as-message-bus (`LISTEN/NOTIFY` for rule changes) and the unused `GET /fired-actions` read path; route rule-change signals through the in-process `events.Bus`.

**Goal, part B:** Ship the React UI so a user can create devices/rules/actions, draw a rule on a React Flow canvas, and watch telemetry + alerts stream live.

**Architecture:**
- `events.Bus` becomes the single cross-service signal mechanism. Three event types: `telemetry`, `fired`, `rules_changed`.
- `httpapi` publishes `rules_changed` after each successful rule write. `transport` subscribes, reloads its rule cache.
- UI is a separate Vite + React + TS SPA under `ui/`. Talks to the Go API over HTTPS/JSON + SSE.

**Tech Stack (UI):** Vite 5, React 19, TypeScript 5, Ant Design 5, React Flow 12 (`@xyflow/react`), `nanoid` for client-local IDs. Minimal vitest for hook coverage.

**Reference:** spec §14 (UI scope), §13 (tenant isolation — still PoC single-tenant).

---

## Conventions

- UI dev server: `http://localhost:5173`. Talks to API at `http://localhost:8080` (CORS already permissive).
- Tenant: implicit dev tenant everywhere (backend pins `DevTenantID`).
- Rule visual shape: `[Device] → [Condition] → [Action]` — three fixed node types on the canvas.
- Vitest used for the `useSse` hook and API client. No browser/DOM tests beyond `@testing-library/react` for one or two critical components.

---

## File Structure

```
D:/Work/Observer/
├── pkg/repo/fired.go                   # DELETE
├── pkg/httpapi/fired.go                # DELETE
├── pkg/httpapi/server.go               # remove GET /fired-actions route
├── pkg/httpapi/rules.go                # publish rules_changed on writes
├── pkg/repo/rules.go                   # drop pg_notify calls
├── pkg/rules/loader.go                 # replace WatchAndRefresh with WatchBus
├── internal/runservice/transport/      # pass bus into rules watcher
└── ui/
    ├── package.json
    ├── tsconfig.json / tsconfig.node.json
    ├── vite.config.ts
    ├── vitest.config.ts
    ├── index.html
    ├── .gitignore
    └── src/
        ├── main.tsx
        ├── App.tsx
        ├── api.ts
        ├── useSse.ts
        ├── useSse.test.ts
        ├── layout/AppLayout.tsx
        ├── pages/DevicesPage.tsx
        ├── pages/ActionsPage.tsx
        ├── pages/RulesPage.tsx
        ├── pages/LivePage.tsx
        └── rules/
            ├── RuleFlow.tsx            # React Flow canvas for rule editor
            └── nodeTypes.tsx           # DeviceNode, ConditionNode, ActionNode
```

---

## Part A — Backend Refactor

### Task 1: Remove `/fired-actions` read path

**Files:**
- Delete: `pkg/httpapi/fired.go`, `pkg/repo/fired.go`
- Modify: `pkg/httpapi/server.go` (drop the route)

- [ ] **Step 1:** Delete the two files.
- [ ] **Step 2:** In `pkg/httpapi/server.go`, remove the line `r.Get("/fired-actions", d.recentFired)`.
- [ ] **Step 3:** Build:

```bash
cd D:/Work/Observer && go build ./...
```

If compiler complains about `recentFired` or `FiredRow` unused, remove lingering references.

- [ ] **Step 4:** Commit:

```bash
git add -A && git commit -m "refactor(api): drop /fired-actions read endpoint; runner still writes fired_actions for audit"
```

### Task 2: Remove `pg_notify` from `pkg/repo/rules.go`

**Files:** `pkg/repo/rules.go`

- [ ] **Step 1:** Strip the three `pool.Exec(ctx, "SELECT pg_notify('rules_changed', ...)")` lines from `CreateRule`, `UpdateRule`, `DeleteRule`. Leave the rest of the SQL untouched.
- [ ] **Step 2:** Run `go test ./pkg/repo/...` — should still pass (the test doesn't depend on NOTIFY).
- [ ] **Step 3:** Commit:

```bash
git add pkg/repo/rules.go && git commit -m "refactor(repo): stop emitting pg_notify; bus publishes rules_changed instead"
```

### Task 3: Replace `WatchAndRefresh` with `WatchBus`

**Files:** `pkg/rules/loader.go`

- [ ] **Step 1:** Replace the body of `pkg/rules/loader.go` with:

```go
package rules

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/events"
	"github.com/observer-io/observer/pkg/models"
)

// LoadAll reads all enabled rules from the database.
func LoadAll(ctx context.Context, pool *pgxpool.Pool) ([]models.Rule, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, device_id, field, op, value, action_id, enabled, debug_until, created_at
		 FROM rules WHERE enabled = TRUE`)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var out []models.Rule
	for rows.Next() {
		var r models.Rule
		var op string
		if err := rows.Scan(&r.ID, &r.TenantID, &r.DeviceID, &r.Field, &op, &r.Value, &r.ActionID, &r.Enabled, &r.DebugUntil, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		r.Op = models.RuleOp(op)
		out = append(out, r)
	}
	return out, rows.Err()
}

// WatchBus subscribes to the events bus and reloads the rule cache whenever a
// "rules_changed" event arrives. Falls back to a 30-second periodic refresh
// so the cache eventually catches up if an event is missed.
func WatchBus(ctx context.Context, bus *events.Bus, pool *pgxpool.Pool, cache *Cache, logger *slog.Logger) error {
	if err := refresh(ctx, pool, cache); err != nil {
		logger.Error("rules initial load", "err", err)
	}

	var sub <-chan events.Event
	if bus != nil {
		sub = bus.Subscribe(ctx)
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := refresh(ctx, pool, cache); err != nil {
				logger.Error("rules periodic refresh", "err", err)
			}
		case ev, ok := <-sub:
			if !ok {
				sub = nil // bus closed; fall back to ticker only
				continue
			}
			if ev.Type != "rules_changed" {
				continue
			}
			if err := refresh(ctx, pool, cache); err != nil {
				logger.Error("rules refresh", "err", err)
			}
		}
	}
}

func refresh(ctx context.Context, pool *pgxpool.Pool, cache *Cache) error {
	rs, err := LoadAll(ctx, pool)
	if err != nil {
		return err
	}
	cache.Replace(rs)
	return nil
}
```

- [ ] **Step 2:** Update `internal/runservice/transport/transport.go`:

Replace the `go func() { rules.WatchAndRefresh(...) }` call with:

```go
go func() {
    if err := rules.WatchBus(ctx, bus, pool, cache, logger); err != nil {
        logger.Error("rules watcher exited", "err", err)
    }
}()
```

- [ ] **Step 3:** In `pkg/httpapi/rules.go`, publish `rules_changed` after each successful write. In `createRule`, `updateRule`, `deleteRule` handlers, after the success write, add before `writeJSON` / `w.WriteHeader`:

```go
if d.Bus != nil {
    d.Bus.Publish(events.Event{Type: "rules_changed", Data: []byte(`{}`)})
}
```

(Add `"github.com/observer-io/observer/pkg/events"` import.)

- [ ] **Step 4:** Build + test:

```bash
cd D:/Work/Observer && go build ./... && go test -short ./...
```

- [ ] **Step 5:** Commit:

```bash
git add pkg/rules/loader.go internal/runservice/transport/ pkg/httpapi/rules.go
git commit -m "refactor(rules): WatchBus subscribes to events.Bus in place of LISTEN/NOTIFY"
```

---

## Part B — React UI

### Task 4: Scaffold `ui/` — Vite + React + TS + AntD + React Flow

**Files:** `ui/` directory, many files

- [ ] **Step 1:** Initialize the Vite project:

```bash
cd D:/Work/Observer
npm create vite@latest ui -- --template react-ts
```

Accept prompts. Then:

```bash
cd D:/Work/Observer/ui
npm install
npm install antd @ant-design/icons @xyflow/react nanoid
npm install -D @testing-library/react @testing-library/jest-dom jsdom vitest
```

- [ ] **Step 2:** Overwrite `ui/vite.config.ts`:

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
```

- [ ] **Step 3:** Add `ui/vitest.config.ts`:

```ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
  },
});
```

- [ ] **Step 4:** Overwrite `ui/src/main.tsx`:

```tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider } from 'antd';
import 'antd/dist/reset.css';
import '@xyflow/react/dist/style.css';
import App from './App';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider>
      <App />
    </ConfigProvider>
  </React.StrictMode>
);
```

- [ ] **Step 5:** Overwrite `ui/src/App.tsx` with a stub that imports the AppLayout we'll write in Task 7:

```tsx
import AppLayout from './layout/AppLayout';

export default function App() {
  return <AppLayout />;
}
```

- [ ] **Step 6:** Update root `.gitignore` to exclude `ui/node_modules` (add a line `ui/node_modules/` if not already covered).

- [ ] **Step 7:** Verify the scaffold starts:

```bash
cd D:/Work/Observer/ui && npm run build
```

Build may fail until Task 7 lands the `AppLayout` — that's fine at this stage; just ensure `npm install` and the three configs are valid. You can also run `npm run dev` briefly (the page will error that `./layout/AppLayout` is missing — expected).

- [ ] **Step 8:** Commit:

```bash
cd D:/Work/Observer
git add ui/ .gitignore
git commit -m "feat(ui): scaffold Vite + React + TS + AntD + React Flow"
```

### Task 5: API client + SSE hook

**Files:** `ui/src/api.ts`, `ui/src/useSse.ts`, `ui/src/useSse.test.ts`

- [ ] **Step 1:** Write `ui/src/api.ts`:

```ts
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
  kind: 'log' | 'webhook' | 'email';
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
```

- [ ] **Step 2:** Write `ui/src/useSse.ts`:

```ts
import { useEffect, useState } from 'react';

export type SseEvent = { type: string; data: unknown };

export function useSse(url: string, max = 200): SseEvent[] {
  const [events, setEvents] = useState<SseEvent[]>([]);

  useEffect(() => {
    const es = new EventSource(url);

    const onTelemetry = (ev: MessageEvent) => push('telemetry', ev.data);
    const onFired = (ev: MessageEvent) => push('fired', ev.data);
    const onRules = (ev: MessageEvent) => push('rules_changed', ev.data);

    function push(type: string, raw: string) {
      try {
        const data = JSON.parse(raw);
        setEvents((prev) => {
          const next = [...prev, { type, data }];
          return next.length > max ? next.slice(next.length - max) : next;
        });
      } catch {
        // ignore malformed frames
      }
    }

    es.addEventListener('telemetry', onTelemetry);
    es.addEventListener('fired', onFired);
    es.addEventListener('rules_changed', onRules);

    return () => {
      es.removeEventListener('telemetry', onTelemetry);
      es.removeEventListener('fired', onFired);
      es.removeEventListener('rules_changed', onRules);
      es.close();
    };
  }, [url, max]);

  return events;
}
```

- [ ] **Step 3:** Write `ui/src/useSse.test.ts` — a minimal smoke test that verifies the hook parses events:

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSse } from './useSse';

type Listener = (ev: MessageEvent) => void;

class FakeEventSource {
  static last: FakeEventSource | null = null;
  listeners = new Map<string, Listener>();
  url: string;
  constructor(url: string) {
    this.url = url;
    FakeEventSource.last = this;
  }
  addEventListener(type: string, fn: Listener) { this.listeners.set(type, fn); }
  removeEventListener(type: string, fn: Listener) {
    if (this.listeners.get(type) === fn) this.listeners.delete(type);
  }
  close() {}
  fire(type: string, data: string) {
    this.listeners.get(type)?.({ data } as MessageEvent);
  }
}

describe('useSse', () => {
  beforeEach(() => {
    vi.stubGlobal('EventSource', FakeEventSource as unknown as typeof EventSource);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('parses telemetry frames', () => {
    const { result } = renderHook(() => useSse('/api/v1/stream'));
    act(() => {
      FakeEventSource.last!.fire('telemetry', JSON.stringify({ x: 1 }));
    });
    expect(result.current).toEqual([{ type: 'telemetry', data: { x: 1 } }]);
  });
});
```

- [ ] **Step 4:** Run the test:

```bash
cd D:/Work/Observer/ui && npx vitest run
```

Expect 1 passing test.

- [ ] **Step 5:** Commit:

```bash
cd D:/Work/Observer
git add ui/src/api.ts ui/src/useSse.ts ui/src/useSse.test.ts
git commit -m "feat(ui): API client and SSE hook with minimal test"
```

### Task 6: Layout + navigation

**Files:** `ui/src/layout/AppLayout.tsx`

- [ ] **Step 1:** Write:

```tsx
import { useState } from 'react';
import { Layout, Menu } from 'antd';
import DevicesPage from '../pages/DevicesPage';
import ActionsPage from '../pages/ActionsPage';
import RulesPage from '../pages/RulesPage';
import LivePage from '../pages/LivePage';

const { Header, Sider, Content } = Layout;

type Tab = 'devices' | 'actions' | 'rules' | 'live';

export default function AppLayout() {
  const [tab, setTab] = useState<Tab>('live');

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ color: 'white', fontSize: 20 }}>Observer</Header>
      <Layout>
        <Sider width={200} theme="light">
          <Menu
            mode="inline"
            selectedKeys={[tab]}
            onClick={(e) => setTab(e.key as Tab)}
            items={[
              { key: 'live', label: 'Live' },
              { key: 'devices', label: 'Devices' },
              { key: 'actions', label: 'Actions' },
              { key: 'rules', label: 'Rules' },
            ]}
          />
        </Sider>
        <Content style={{ padding: 24, background: 'white' }}>
          {tab === 'live' && <LivePage />}
          {tab === 'devices' && <DevicesPage />}
          {tab === 'actions' && <ActionsPage />}
          {tab === 'rules' && <RulesPage />}
        </Content>
      </Layout>
    </Layout>
  );
}
```

- [ ] **Step 2:** Commit (pages stubs coming in next tasks; temporarily create four empty page files so build passes):

For now create `ui/src/pages/DevicesPage.tsx`, `ActionsPage.tsx`, `RulesPage.tsx`, `LivePage.tsx`, each:

```tsx
export default function Page() { return <div>TODO</div>; }
```

Then build:

```bash
cd D:/Work/Observer/ui && npm run build
```

Commit:

```bash
cd D:/Work/Observer
git add ui/src/layout/ ui/src/pages/ ui/src/App.tsx
git commit -m "feat(ui): app layout with nav and page stubs"
```

### Task 7: Devices page

**File:** `ui/src/pages/DevicesPage.tsx`

```tsx
import { useEffect, useState } from 'react';
import { Button, Form, Input, Modal, Popconfirm, Space, Table, message } from 'antd';
import { api, type Device } from '../api';

export default function DevicesPage() {
  const [rows, setRows] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<{ name: string; type: string }>();

  const refresh = async () => {
    setLoading(true);
    try {
      setRows(await api.listDevices());
    } catch (e) {
      message.error(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  const onCreate = async () => {
    const vals = await form.validateFields();
    try {
      await api.createDevice(vals.name, vals.type || '');
      setOpen(false);
      form.resetFields();
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  const onDelete = async (id: string) => {
    try {
      await api.deleteDevice(id);
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setOpen(true)}>New device</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={rows}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 320 },
          { title: 'Name', dataIndex: 'name' },
          { title: 'Type', dataIndex: 'type' },
          {
            title: '',
            width: 100,
            render: (_, r: Device) => (
              <Popconfirm title="Delete?" onConfirm={() => onDelete(r.id)}>
                <Button danger size="small">Delete</Button>
              </Popconfirm>
            ),
          },
        ]}
      />
      <Modal title="New device" open={open} onOk={onCreate} onCancel={() => setOpen(false)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="type" label="Type">
            <Input placeholder="sensor, boiler, ..." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
```

Commit:

```bash
cd D:/Work/Observer
git add ui/src/pages/DevicesPage.tsx
git commit -m "feat(ui): devices page with create/delete"
```

### Task 8: Actions page

**File:** `ui/src/pages/ActionsPage.tsx`

```tsx
import { useEffect, useState } from 'react';
import { Button, Form, Input, Modal, Popconfirm, Select, Space, Table, message } from 'antd';
import { api, type Action } from '../api';

export default function ActionsPage() {
  const [rows, setRows] = useState<Action[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<{ kind: Action['kind']; url?: string }>();

  const refresh = async () => {
    setLoading(true);
    try {
      setRows(await api.listActions());
    } catch (e) {
      message.error(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  const onCreate = async () => {
    const vals = await form.validateFields();
    const config: Record<string, unknown> = {};
    if (vals.kind === 'webhook' && vals.url) config.url = vals.url;
    try {
      await api.createAction(vals.kind, config);
      setOpen(false);
      form.resetFields();
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setOpen(true)}>New action</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={rows}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 320 },
          { title: 'Kind', dataIndex: 'kind', width: 100 },
          {
            title: 'Config',
            render: (_, r: Action) => <code>{JSON.stringify(r.config)}</code>,
          },
          {
            title: '',
            width: 100,
            render: (_, r: Action) => (
              <Popconfirm title="Delete?" onConfirm={() => api.deleteAction(r.id).then(refresh)}>
                <Button danger size="small">Delete</Button>
              </Popconfirm>
            ),
          },
        ]}
      />
      <Modal title="New action" open={open} onOk={onCreate} onCancel={() => setOpen(false)}>
        <Form form={form} layout="vertical" initialValues={{ kind: 'log' }}>
          <Form.Item name="kind" label="Kind" rules={[{ required: true }]}>
            <Select options={[
              { value: 'log', label: 'log' },
              { value: 'webhook', label: 'webhook' },
              { value: 'email', label: 'email' },
            ]} />
          </Form.Item>
          <Form.Item
            noStyle
            shouldUpdate={(prev, next) => prev.kind !== next.kind}
          >
            {({ getFieldValue }) =>
              getFieldValue('kind') === 'webhook' ? (
                <Form.Item name="url" label="Webhook URL" rules={[{ required: true }]}>
                  <Input placeholder="https://example.com/hook" />
                </Form.Item>
              ) : null
            }
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
```

Commit:

```bash
cd D:/Work/Observer
git add ui/src/pages/ActionsPage.tsx
git commit -m "feat(ui): actions page (log/webhook)"
```

### Task 9: Rules page with React Flow builder

**Files:** `ui/src/pages/RulesPage.tsx`, `ui/src/rules/RuleFlow.tsx`, `ui/src/rules/nodeTypes.tsx`

- [ ] **Step 1:** Write `ui/src/rules/nodeTypes.tsx`:

```tsx
import { Handle, Position, type NodeProps } from '@xyflow/react';
import { Card, Select, InputNumber } from 'antd';
import type { Device, Action } from '../api';

export type DeviceNodeData = {
  devices: Device[];
  value: string;
  onChange: (id: string) => void;
};
export type ConditionNodeData = {
  field: string;
  op: '>' | '<' | '>=' | '<=' | '=' | '!=';
  value: number;
  onChange: (patch: Partial<Omit<ConditionNodeData, 'onChange'>>) => void;
};
export type ActionNodeData = {
  actions: Action[];
  value: string;
  onChange: (id: string) => void;
};

export function DeviceNode({ data }: NodeProps<{ data: DeviceNodeData }>) {
  const d = data as unknown as DeviceNodeData;
  return (
    <Card size="small" title="Device" style={{ minWidth: 200 }}>
      <Select
        style={{ width: '100%' }}
        placeholder="Select device"
        value={d.value || undefined}
        options={d.devices.map((x) => ({ value: x.id, label: x.name }))}
        onChange={d.onChange}
      />
      <Handle type="source" position={Position.Right} />
    </Card>
  );
}

export function ConditionNode({ data }: NodeProps<{ data: ConditionNodeData }>) {
  const d = data as unknown as ConditionNodeData;
  return (
    <Card size="small" title="Condition" style={{ minWidth: 240 }}>
      <Handle type="target" position={Position.Left} />
      <input
        value={d.field}
        placeholder="field (e.g. temperature)"
        onChange={(e) => d.onChange({ field: e.target.value })}
        style={{ width: '100%', marginBottom: 6, padding: 4 }}
      />
      <Select
        style={{ width: '100%', marginBottom: 6 }}
        value={d.op}
        options={['>', '<', '>=', '<=', '=', '!='].map((v) => ({ value: v, label: v }))}
        onChange={(v) => d.onChange({ op: v as ConditionNodeData['op'] })}
      />
      <InputNumber
        style={{ width: '100%' }}
        value={d.value}
        onChange={(v) => d.onChange({ value: Number(v) || 0 })}
      />
      <Handle type="source" position={Position.Right} />
    </Card>
  );
}

export function ActionNode({ data }: NodeProps<{ data: ActionNodeData }>) {
  const d = data as unknown as ActionNodeData;
  return (
    <Card size="small" title="Action" style={{ minWidth: 200 }}>
      <Handle type="target" position={Position.Left} />
      <Select
        style={{ width: '100%' }}
        placeholder="Select action"
        value={d.value || undefined}
        options={d.actions.map((x) => ({ value: x.id, label: `${x.kind} — ${x.id.slice(0, 6)}` }))}
        onChange={d.onChange}
      />
    </Card>
  );
}

export const nodeTypes = { device: DeviceNode, condition: ConditionNode, action: ActionNode };
```

- [ ] **Step 2:** Write `ui/src/rules/RuleFlow.tsx`:

```tsx
import { useMemo, useState } from 'react';
import { ReactFlow, Background, Controls } from '@xyflow/react';
import { nodeTypes, type ConditionNodeData } from './nodeTypes';
import type { Device, Action, RuleInput } from '../api';

export type RuleFlowProps = {
  devices: Device[];
  actions: Action[];
  initial?: RuleInput;
  onChange: (r: RuleInput) => void;
};

export default function RuleFlow({ devices, actions, initial, onChange }: RuleFlowProps) {
  const [deviceId, setDeviceId] = useState(initial?.device_id ?? '');
  const [cond, setCond] = useState<Omit<ConditionNodeData, 'onChange'>>({
    field: initial?.field ?? 'temperature',
    op: initial?.op ?? '>',
    value: initial?.value ?? 80,
  });
  const [actionId, setActionId] = useState(initial?.action_id ?? '');

  // Emit changes upward on every edit
  const emit = (next: Partial<RuleInput>) => {
    const merged: RuleInput = {
      device_id: next.device_id ?? deviceId,
      field: next.field ?? cond.field,
      op: (next.op ?? cond.op) as RuleInput['op'],
      value: next.value ?? cond.value,
      action_id: next.action_id ?? actionId,
      enabled: true,
    };
    onChange(merged);
  };

  const nodes = useMemo(
    () => [
      {
        id: 'device', type: 'device', position: { x: 0, y: 80 },
        data: { devices, value: deviceId, onChange: (v: string) => { setDeviceId(v); emit({ device_id: v }); } },
      },
      {
        id: 'condition', type: 'condition', position: { x: 260, y: 40 },
        data: {
          ...cond,
          onChange: (patch: Partial<Omit<ConditionNodeData, 'onChange'>>) => {
            setCond({ ...cond, ...patch });
            emit(patch);
          },
        },
      },
      {
        id: 'action', type: 'action', position: { x: 560, y: 80 },
        data: { actions, value: actionId, onChange: (v: string) => { setActionId(v); emit({ action_id: v }); } },
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [devices, actions, deviceId, cond, actionId],
  );

  const edges = [
    { id: 'e1', source: 'device', target: 'condition' },
    { id: 'e2', source: 'condition', target: 'action' },
  ];

  return (
    <div style={{ height: 320, border: '1px solid #eee' }}>
      <ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes} fitView>
        <Background />
        <Controls />
      </ReactFlow>
    </div>
  );
}
```

- [ ] **Step 3:** Write `ui/src/pages/RulesPage.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { Button, Modal, Popconfirm, Space, Table, message } from 'antd';
import { api, type Action, type Device, type Rule, type RuleInput } from '../api';
import RuleFlow from '../rules/RuleFlow';

export default function RulesPage() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [actions, setActions] = useState<Action[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<RuleInput>({
    device_id: '', field: 'temperature', op: '>', value: 80, action_id: '', enabled: true,
  });

  const refresh = async () => {
    const [d, a, r] = await Promise.all([api.listDevices(), api.listActions(), api.listRules()]);
    setDevices(d); setActions(a); setRules(r);
  };

  useEffect(() => { refresh(); }, []);

  const onSave = async () => {
    if (!draft.device_id || !draft.action_id || !draft.field) {
      message.error('device, field, and action are required');
      return;
    }
    try {
      await api.createRule(draft);
      setOpen(false);
      refresh();
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setOpen(true)}>New rule</Button>
        <Button onClick={refresh}>Refresh</Button>
      </Space>
      <Table
        rowKey="id"
        dataSource={rules}
        columns={[
          {
            title: 'Device',
            render: (_, r: Rule) => devices.find((d) => d.id === r.device_id)?.name ?? r.device_id,
          },
          { title: 'Condition', render: (_, r: Rule) => `${r.field} ${r.op} ${r.value}` },
          {
            title: 'Action',
            render: (_, r: Rule) => actions.find((a) => a.id === r.action_id)?.kind ?? r.action_id,
          },
          {
            title: 'Enabled',
            dataIndex: 'enabled',
            render: (v: boolean) => (v ? 'yes' : 'no'),
          },
          {
            title: '',
            render: (_, r: Rule) => (
              <Popconfirm title="Delete?" onConfirm={() => api.deleteRule(r.id).then(refresh)}>
                <Button danger size="small">Delete</Button>
              </Popconfirm>
            ),
          },
        ]}
      />
      <Modal title="New rule" open={open} onOk={onSave} onCancel={() => setOpen(false)} width={900}>
        <RuleFlow devices={devices} actions={actions} initial={draft} onChange={setDraft} />
      </Modal>
    </div>
  );
}
```

- [ ] **Step 4:** Build:

```bash
cd D:/Work/Observer/ui && npm run build
```

- [ ] **Step 5:** Commit:

```bash
cd D:/Work/Observer
git add ui/src/pages/RulesPage.tsx ui/src/rules/
git commit -m "feat(ui): rules page with React Flow rule builder"
```

### Task 10: Live page — telemetry + alerts

**File:** `ui/src/pages/LivePage.tsx`

```tsx
import { useMemo } from 'react';
import { List, Tag, Typography } from 'antd';
import { useSse } from '../useSse';

type TelemetryEvt = { type: 'telemetry'; data: { time: string; device_id: string; message_id: string; payload: Record<string, unknown> } };
type FiredEvt = { type: 'fired'; data: { fired_at: string; device_id: string; rule_id: string; action_id: string; message_id: string; status: string; error: string; payload: Record<string, unknown> } };

export default function LivePage() {
  const events = useSse('/api/v1/stream');
  const telemetry = useMemo(
    () => events.filter((e): e is TelemetryEvt => e.type === 'telemetry').slice(-50).reverse(),
    [events],
  );
  const fired = useMemo(
    () => events.filter((e): e is FiredEvt => e.type === 'fired').slice(-50).reverse(),
    [events],
  );

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
      <div>
        <Typography.Title level={4}>Live telemetry</Typography.Title>
        <List
          bordered
          size="small"
          dataSource={telemetry}
          locale={{ emptyText: 'no messages yet — publish to mqtt://localhost:1883' }}
          renderItem={(e) => (
            <List.Item>
              <Tag color="blue">{new Date(e.data.time).toLocaleTimeString()}</Tag>
              <code style={{ marginLeft: 8 }}>{e.data.device_id.slice(0, 8)}</code>
              <code style={{ marginLeft: 8 }}>{JSON.stringify(e.data.payload)}</code>
            </List.Item>
          )}
        />
      </div>
      <div>
        <Typography.Title level={4}>Alerts (fired actions)</Typography.Title>
        <List
          bordered
          size="small"
          dataSource={fired}
          locale={{ emptyText: 'no alerts yet' }}
          renderItem={(e) => (
            <List.Item>
              <Tag color={e.data.status === 'ok' ? 'green' : 'red'}>{e.data.status}</Tag>
              <Tag>{new Date(e.data.fired_at).toLocaleTimeString()}</Tag>
              <code style={{ marginLeft: 8 }}>{e.data.device_id.slice(0, 8)}</code>
              <code style={{ marginLeft: 8 }}>{JSON.stringify(e.data.payload)}</code>
              {e.data.error && <span style={{ color: 'red', marginLeft: 8 }}>{e.data.error}</span>}
            </List.Item>
          )}
        />
      </div>
    </div>
  );
}
```

Commit:

```bash
cd D:/Work/Observer
git add ui/src/pages/LivePage.tsx
git commit -m "feat(ui): live page with SSE-driven telemetry and alerts"
```

### Task 11: Browser smoke test

Verification only — no code changes.

- [ ] **Step 1:** Start stack + monolith (backend):

```bash
cd D:/Work/Observer
docker compose -f deploy/docker-compose.yml up -d
OBSERVER_DB_DSN="postgres://observer:observer@localhost:5432/observer?sslmode=disable" \
OBSERVER_MQTT_URL="tcp://localhost:1883" \
go run ./cmd/all > /tmp/observer.log 2>&1 &
```

- [ ] **Step 2:** Start UI dev server:

```bash
cd D:/Work/Observer/ui && npm run dev
```

Open `http://localhost:5173` in a browser.

- [ ] **Step 3:** Manual test path:
  1. **Devices**: create `boiler-01` (type `sensor`). Confirm it appears.
  2. **Actions**: create `log` action. Confirm it appears.
  3. **Rules**: click **New rule** → React Flow canvas opens. Pick boiler-01 in the Device node, field=`temperature`, op=`>`, value=`80`, pick the log action. Click OK. Rule appears in the table.
  4. **Live**: switch to the Live tab. Publish two messages via `mosquitto_pub`:
     ```bash
     DEVICE=<id from devices page>
     docker run --rm --network host eclipse-mosquitto:2 mosquitto_pub -h localhost -p 1883 -t "tenants/dev/devices/$DEVICE/telemetry" -m '{"temperature": 72}'
     docker run --rm --network host eclipse-mosquitto:2 mosquitto_pub -h localhost -p 1883 -t "tenants/dev/devices/$DEVICE/telemetry" -m '{"temperature": 90}'
     ```
     Expected: two rows in **Live telemetry**, one row in **Alerts** (for the `90` message), within ~1 second.

Report success with screenshots or a short description in the task output.

---

## Self-Review

**Part A — Refactor coverage:**
- ✅ `/fired-actions` endpoint removed (Task 1)
- ✅ `pg_notify` calls removed from repo (Task 2)
- ✅ `LISTEN/NOTIFY` removed from rules loader, replaced with `WatchBus` (Task 3)
- ✅ API publishes `rules_changed` on writes (Task 3 Step 3)

**Part B — UI coverage (against spec §14):**
- Tenant dashboard: device list ✅ (Task 7), alerts feed ✅ (Task 10)
- Rule editor with React Flow ✅ (Task 9)
- Auth/login/password reset: out of scope (PoC uses dev tenant)

**Placeholder scan:** none. All code blocks are complete and compilable.

**Type consistency:** `api.ts` `Rule`/`RuleInput`/`Action`/`Device` shapes match the Go `repo` structs exactly (field names, JSON tags). `RuleFlow.onChange` emits a `RuleInput` — matches what `api.createRule` accepts. SSE event types (`telemetry` / `fired` / `rules_changed`) match the server.
