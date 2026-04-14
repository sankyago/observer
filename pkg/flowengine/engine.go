package flowengine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/observer-io/observer/pkg/events"
)

// FlowIndex is one entry per device node found in any enabled flow.
type FlowIndex struct {
	FlowID       uuid.UUID
	TenantID     uuid.UUID
	Graph        Graph
	DeviceNodeID string
	// precomputed adjacency for cheap BFS
	Succ map[string][]string
}

// Cache is a thread-safe map from device_id to the FlowIndex entries that
// should trigger on telemetry from that device.
type Cache struct {
	mu    sync.RWMutex
	index map[uuid.UUID][]FlowIndex
}

func NewCache() *Cache {
	return &Cache{index: map[uuid.UUID][]FlowIndex{}}
}

// Replace atomically swaps the cache with entries built from `flows`.
// Each flow may have multiple device nodes, each producing its own entry.
func (c *Cache) Replace(flows []struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	Graph    []byte
}) error {
	next := map[uuid.UUID][]FlowIndex{}
	for _, f := range flows {
		var g Graph
		if err := json.Unmarshal(f.Graph, &g); err != nil {
			return fmt.Errorf("flow %s: %w", f.ID, err)
		}
		succ := buildAdjacency(g)
		for _, n := range g.Nodes {
			if n.Type != "device" {
				continue
			}
			var d DeviceNodeData
			if err := json.Unmarshal(n.Data, &d); err != nil || d.DeviceID == uuid.Nil {
				continue
			}
			next[d.DeviceID] = append(next[d.DeviceID], FlowIndex{
				FlowID: f.ID, TenantID: f.TenantID, Graph: g, DeviceNodeID: n.ID, Succ: succ,
			})
		}
	}
	c.mu.Lock()
	c.index = next
	c.mu.Unlock()
	return nil
}

func (c *Cache) GetByDevice(deviceID uuid.UUID) []FlowIndex {
	c.mu.RLock()
	defer c.mu.RUnlock()
	src := c.index[deviceID]
	if len(src) == 0 {
		return nil
	}
	out := make([]FlowIndex, len(src))
	copy(out, src)
	return out
}

func buildAdjacency(g Graph) map[string][]string {
	m := map[string][]string{}
	for _, e := range g.Edges {
		m[e.Source] = append(m[e.Source], e.Target)
	}
	return m
}

// ActionHit describes a fired action node inside a flow for a given message.
type ActionHit struct {
	FlowID   uuid.UUID
	TenantID uuid.UUID
	NodeID   string
	Kind     string
	Config   json.RawMessage
}

// Traverse walks the flow from its device node and collects action hits.
// At a condition node, if it fails, the branch is pruned. At an action node,
// the hit is recorded AND traversal continues into its successors.
func Traverse(fi FlowIndex, payload []byte) ([]ActionHit, error) {
	nodeByID := make(map[string]Node, len(fi.Graph.Nodes))
	for _, n := range fi.Graph.Nodes {
		nodeByID[n.ID] = n
	}
	var hits []ActionHit
	visited := map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if visited[id] {
			return nil
		}
		visited[id] = true
		n, ok := nodeByID[id]
		if !ok {
			return nil
		}
		switch n.Type {
		case "device":
			// pass-through
		case "condition":
			var c ConditionNodeData
			if err := json.Unmarshal(n.Data, &c); err != nil {
				return fmt.Errorf("condition %s: %w", id, err)
			}
			match, err := EvaluateCondition(c, payload)
			if err != nil {
				return err
			}
			if !match {
				return nil
			}
		case "action":
			var a ActionNodeData
			if err := json.Unmarshal(n.Data, &a); err != nil {
				return fmt.Errorf("action %s: %w", id, err)
			}
			hits = append(hits, ActionHit{
				FlowID: fi.FlowID, TenantID: fi.TenantID, NodeID: id,
				Kind: a.Kind, Config: a.Config,
			})
		}
		for _, next := range fi.Succ[id] {
			if err := walk(next); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(fi.DeviceNodeID); err != nil {
		return nil, err
	}
	return hits, nil
}

// LoadAll reads all enabled flows from DB.
func LoadAll(ctx context.Context, pool *pgxpool.Pool) ([]struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	Graph    []byte
}, error) {
	rows, err := pool.Query(ctx, `SELECT id, tenant_id, graph FROM flows WHERE enabled = TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID       uuid.UUID
		TenantID uuid.UUID
		Graph    []byte
	}
	for rows.Next() {
		var f struct {
			ID       uuid.UUID
			TenantID uuid.UUID
			Graph    []byte
		}
		if err := rows.Scan(&f.ID, &f.TenantID, &f.Graph); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// WatchBus keeps the cache fresh — subscribes to `flows_changed` events and
// does a periodic refresh every 30s as a safety net.
func WatchBus(ctx context.Context, bus *events.Bus, pool *pgxpool.Pool, cache *Cache, logger *slog.Logger) error {
	if err := reload(ctx, pool, cache); err != nil {
		logger.Error("flows initial load", "err", err)
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
			if err := reload(ctx, pool, cache); err != nil {
				logger.Error("flows periodic refresh", "err", err)
			}
		case ev, ok := <-sub:
			if !ok {
				sub = nil
				continue
			}
			if ev.Type != "flows_changed" {
				continue
			}
			if err := reload(ctx, pool, cache); err != nil {
				logger.Error("flows refresh", "err", err)
			}
		}
	}
}

func reload(ctx context.Context, pool *pgxpool.Pool, cache *Cache) error {
	fs, err := LoadAll(ctx, pool)
	if err != nil {
		return err
	}
	return cache.Replace(fs)
}
