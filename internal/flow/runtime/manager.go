package runtime

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/flow/graph"
)

type Manager struct {
	mu    sync.Mutex
	flows map[uuid.UUID]*CompiledFlow
}

func NewManager() *Manager {
	return &Manager{flows: make(map[uuid.UUID]*CompiledFlow)}
}

func (m *Manager) Start(ctx context.Context, id uuid.UUID, g graph.Graph) error {
	cf, err := Compile(g)
	if err != nil {
		return err
	}
	if err := cf.Start(ctx); err != nil {
		cf.Stop()
		return err
	}
	m.mu.Lock()
	if prev, ok := m.flows[id]; ok {
		prev.Stop()
	}
	m.flows[id] = cf
	m.mu.Unlock()
	return nil
}

func (m *Manager) Replace(ctx context.Context, id uuid.UUID, g graph.Graph) error {
	return m.Start(ctx, id, g)
}

func (m *Manager) Stop(id uuid.UUID) {
	m.mu.Lock()
	cf, ok := m.flows[id]
	delete(m.flows, id)
	m.mu.Unlock()
	if ok {
		cf.Stop()
	}
}

func (m *Manager) Running(id uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.flows[id]
	return ok
}

func (m *Manager) Bus(id uuid.UUID) *EventBus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cf, ok := m.flows[id]; ok {
		return cf.Bus()
	}
	return nil
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	flows := m.flows
	m.flows = make(map[uuid.UUID]*CompiledFlow)
	m.mu.Unlock()
	for _, cf := range flows {
		cf.Stop()
	}
}
