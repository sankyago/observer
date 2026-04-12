# Flow UI — Design

**Date:** 2026-04-12
**Branch:** `feat/flow-ui` (stacked on `feat/flow-engine`)
**Status:** approved

## Context

The `feat/flow-engine` branch delivers REST+WS endpoints for persisted React-Flow graphs. This spec adds the client: a React app that lists flows, edits graphs visually, saves them via the API, and streams live events from the running flow. The whole app (UI + API) ships as a single Docker image built via multi-stage Dockerfile, with the UI embedded into the Go binary using `//go:embed`.

Single-tenant. No auth.

## Scope

In:
- React 18 + Vite + TypeScript SPA.
- Ant Design as primary UI kit; Tailwind used sparingly for layout utilities only (prefer Flex + AntD components).
- React Flow v12 canvas with a draggable node palette, per-node config panel, save button.
- Four node types visible in the palette matching the backend: `mqtt_source`, `threshold`, `rate_of_change`, `debug_sink`.
- Flows list page (create, rename, enable/disable, delete).
- Flow editor page with live event log (WS).
- Go server serves the built SPA from `/` (via `//go:embed`) and API from `/api/*`.
- Multi-stage Dockerfile producing a single distroless image.
- `docker-compose.yml` updates so `docker compose up` brings up app + Postgres + Mosquitto.

Out (deferred, future PRs):
- Auth, multi-tenancy.
- Import/export flow JSON, flow versioning.
- Custom theming, dark mode.
- Visual indication of node-level events (highlighting nodes as messages pass through).
- Error toasts beyond AntD `message.error`.
- Unit tests for UI components (visual-smoke via manual QA this round; revisit with Vitest once UI stabilizes — backend reliability bar does not transfer to untested UI scaffolding PRs).

## Architecture

```
┌─────────────────────────────────────────────┐
│  observer Docker image (distroless)         │
│                                             │
│  ┌───────────────────────────────────────┐  │
│  │  Go binary (cmd/observer)             │  │
│  │                                       │  │
│  │  chi router:                          │  │
│  │    /api/*           → FlowService     │  │
│  │    /api/flows/:id/events (WS)         │  │
│  │    /*               → embed.FS (SPA)  │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
         │                         │
         ▼                         ▼
     Postgres                  MQTT broker(s)
                               (user-provided)
```

- The Go server handles SPA routing: any non-API request falls through to `index.html` (classic SPA history-mode handling). Assets under `/assets/*` are served as-is with long cache headers.
- `internal/web/` package owns the `embed.FS` and the HTTP handler. `ui/dist` is copied into `internal/web/dist/` by the Docker build step; a local `go build` with no UI also works (handler returns 404 for `/` if the bundle is absent — developer runs `npm run dev` separately and hits `:5173` directly).

## Directory Layout

```
ui/                         ← new
├── package.json
├── tsconfig.json
├── vite.config.ts
├── index.html
├── src/
│   ├── main.tsx
│   ├── App.tsx             ← router
│   ├── api/
│   │   ├── client.ts       ← fetch wrapper
│   │   └── types.ts        ← mirrors Go Graph/Flow types
│   ├── pages/
│   │   ├── FlowsListPage.tsx
│   │   └── FlowEditorPage.tsx
│   ├── components/
│   │   ├── NodePalette.tsx
│   │   ├── NodeConfigPanel.tsx
│   │   ├── EventsLog.tsx
│   │   └── nodes/          ← custom React Flow node components
│   │       ├── MQTTSourceNode.tsx
│   │       ├── ThresholdNode.tsx
│   │       ├── RateOfChangeNode.tsx
│   │       └── DebugSinkNode.tsx
│   └── hooks/
│       └── useFlowEvents.ts ← WS subscription
└── public/
internal/web/               ← new
├── web.go                  ← embed.FS + Handler()
└── dist/                   ← populated by Docker build
Dockerfile                  ← new, multi-stage
docker-compose.yml          ← updated
```

## API Client

`ui/src/api/client.ts` wraps `fetch` with JSON helpers. Base URL:
- dev: `http://localhost:8080` (Vite proxies `/api` → `:8080`)
- prod: relative (`''`), same origin.

TypeScript types in `ui/src/api/types.ts` mirror Go structs:
```ts
export type Graph = { nodes: FlowNode[]; edges: FlowEdge[] };
export type FlowNode = { id: string; type: NodeType; position: {x,y}; data: Record<string, unknown> };
export type NodeType = 'mqtt_source' | 'threshold' | 'rate_of_change' | 'debug_sink';
export type FlowEdge = { id: string; source: string; target: string };
export type Flow = { id: string; name: string; graph: Graph; enabled: boolean };
export type FlowEvent = { kind: 'reading'|'alert'|'error'; node_id: string; reading?: {...}; detail?: string; ts: string };
```

A tiny runtime validator (hand-written, not zod for this PR) checks shape on the flows-list response so the UI fails loudly on drift.

## Pages

### FlowsListPage (`/`)
- AntD `Table` with columns: name, enabled toggle, created_at, actions (edit, delete).
- "New flow" button opens a `Modal` with a name field; POST `/api/flows` with an empty graph (two starter nodes: a `mqtt_source` and a `debug_sink` connected).
- Row click → navigate to `/flows/:id`.

### FlowEditorPage (`/flows/:id`)
Layout (using flex + AntD `Layout`):
```
┌─────────────────────────────────────────────────────┐
│ Header: flow name (editable) | enabled toggle | Save│
├──────────┬────────────────────────────┬─────────────┤
│          │                            │             │
│ Palette  │  React Flow canvas         │  Config     │
│          │                            │  Panel      │
│ (AntD    │                            │  +          │
│ Card     │                            │  Events     │
│ list)    │                            │  Log        │
│          │                            │             │
└──────────┴────────────────────────────┴─────────────┘
```

- Palette: list of 4 node types with icons; drag-drop onto canvas (React Flow's `onDrop` pattern).
- Canvas: custom node components per type (show key config fields as inline text).
- Config panel: right side. When a node is selected, show an AntD `Form` with the config schema for that type. Changes update the local graph state.
- Events log: bottom of right panel. If flow is enabled and running, opens WS on mount and shows last 100 events (AntD `List` with color-coded `Tag` for kind).
- Save button: PUT `/api/flows/:id` with current `{name, graph, enabled}`. On 400 show `message.error` with backend error detail.

### Node Config Schemas

| Type             | Fields                                             |
|------------------|----------------------------------------------------|
| `mqtt_source`    | broker (string, required), topic (string, required), username?, password? |
| `threshold`      | min (number), max (number) — client enforces min < max |
| `rate_of_change` | max_per_second (number > 0), window_size (int ≥ 2) |
| `debug_sink`     | (no fields)                                        |

## WebSocket Handling

`useFlowEvents(flowId)` hook:
- Connects to `ws://<host>/api/flows/:id/events` (or `wss://` if https).
- Reconnects on close with 1s backoff up to 5s; stops if the hook unmounts.
- Maintains a ring of last 100 events in React state.
- Returns `{ events, connected }`.

## Go Integration

`internal/web/web.go`:
```go
//go:embed all:dist
var distFS embed.FS

func Handler() http.Handler {
    // Serve /assets/* from embed FS with long cache.
    // Any other path: try file in dist, else fall back to index.html (SPA).
}
```

In `internal/api/router.go`, register after API routes:
```go
r.Handle("/*", web.Handler())
```

If `dist/` is empty (dev build without UI), `Handler()` returns 404 for everything — developer uses Vite dev server directly.

## Docker

Multi-stage `Dockerfile`:
1. `node:20-alpine` — install deps, build UI → `/ui/dist`.
2. `golang:1.25-alpine` — copy Go source, copy UI build into `internal/web/dist`, `go build` static binary.
3. `gcr.io/distroless/static` — copy binary + `migrations/`, expose 8080, `ENTRYPOINT ["/observer"]`.

`docker-compose.yml` gains an `app` service built from the Dockerfile, depending on `postgres` and `mosquitto` with the appropriate `DATABASE_URL` and port mapping.

## Dev Workflow

- `cd ui && npm install && npm run dev` → Vite dev server on `:5173`, proxies `/api` and `/api/*/events` to `:8080`.
- Separate terminal: `go run ./cmd/observer` → API on `:8080`.
- OR `docker compose up --build` for a full prod-like run on `:8080`.

## Testing

- Backend changes (web handler) get unit + integration tests (power-plant bar).
  - Unit: `Handler()` with a fake embed FS serves files, falls back to index.html for unknown paths, serves 404 when dist is empty, cache headers on `/assets/*`.
  - Integration: hit `/` via `httptest.NewServer` and confirm 200 + HTML content.
- Frontend: no automated tests this PR (documented in "Out"). Add Vitest + Playwright in a follow-up once the UI stabilizes.

## Risks / Open Questions

- **Bundle size:** Ant Design is hefty. Use `babel-plugin-import` or AntD v5's default ESM tree-shaking. Target: initial JS payload under 500 KB gzipped.
- **React Flow licensing:** v12 is MIT — fine.
- **Hot-reload inside Docker:** not a goal; dev workflow uses Vite + `go run` outside the container.
- **CORS in dev:** avoided via Vite proxy; no CORS middleware added server-side.
- **Browser history mode:** server falls back to index.html for non-API 404s — supports React Router `BrowserRouter`.
