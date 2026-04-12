package flow

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/flow/graph"
	"github.com/sankyago/observer/internal/flow/runtime"
	"github.com/sankyago/observer/internal/flow/store"
	"github.com/stretchr/testify/require"
)

// fakeRepo is an in-memory flowRepo for unit tests.
type fakeRepo struct {
	mu    sync.Mutex
	flows map[uuid.UUID]*store.Flow
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{flows: make(map[uuid.UUID]*store.Flow)}
}

func (r *fakeRepo) Create(ctx context.Context, f *store.Flow) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	cp := *f
	r.flows[f.ID] = &cp
	return nil
}

func (r *fakeRepo) Get(ctx context.Context, id uuid.UUID) (*store.Flow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.flows[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *f
	return &cp, nil
}

func (r *fakeRepo) List(ctx context.Context) ([]*store.Flow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*store.Flow
	for _, f := range r.flows {
		cp := *f
		out = append(out, &cp)
	}
	return out, nil
}

func (r *fakeRepo) ListEnabled(ctx context.Context) ([]*store.Flow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*store.Flow
	for _, f := range r.flows {
		if f.Enabled {
			cp := *f
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeRepo) Update(ctx context.Context, f *store.Flow) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.flows[f.ID]; !ok {
		return store.ErrNotFound
	}
	cp := *f
	r.flows[f.ID] = &cp
	return nil
}

func (r *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.flows[id]; !ok {
		return store.ErrNotFound
	}
	delete(r.flows, id)
	return nil
}

// debugGraph returns a minimal valid graph with a single debug_sink node.
func debugGraph() graph.Graph {
	return graph.Graph{
		Nodes: []graph.Node{{ID: "n1", Type: "debug_sink", Data: json.RawMessage(`{}`)}},
	}
}

// invalidGraph returns a graph that fails validation (unknown type).
func invalidGraph() graph.Graph {
	return graph.Graph{
		Nodes: []graph.Node{{ID: "n1", Type: "bad_type", Data: json.RawMessage(`{}`)}},
	}
}

func newTestService() (*Service, *fakeRepo, *runtime.Manager) {
	repo := newFakeRepo()
	mgr := runtime.NewManager()
	ctx := context.Background()
	svc := NewService(ctx, repo, mgr)
	return svc, repo, mgr
}

// ptr helpers
func strPtr(s string) *string   { return &s }
func boolPtr(b bool) *bool      { return &b }
func graphPtr(g graph.Graph) *graph.Graph { return &g }

// ---- Create ----------------------------------------------------------------

func TestService_Create_InvalidGraph_ReturnsError(t *testing.T) {
	svc, repo, mgr := newTestService()
	ctx := context.Background()

	_, err := svc.Create(ctx, "bad", invalidGraph(), false)
	require.Error(t, err)

	// Nothing stored, nothing started.
	list, _ := repo.List(ctx)
	require.Empty(t, list)
	require.False(t, mgr.Running(uuid.New()))
}

func TestService_Create_EnabledTrue_StartsManager(t *testing.T) {
	svc, repo, mgr := newTestService()
	ctx := context.Background()

	f, err := svc.Create(ctx, "on", debugGraph(), true)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, f.ID)

	// Stored in repo.
	got, err := repo.Get(ctx, f.ID)
	require.NoError(t, err)
	require.Equal(t, "on", got.Name)

	// Manager is running the flow.
	require.True(t, mgr.Running(f.ID))
	t.Cleanup(func() { mgr.Stop(f.ID) })
}

func TestService_Create_EnabledFalse_DoesNotStartManager(t *testing.T) {
	svc, repo, mgr := newTestService()
	ctx := context.Background()

	f, err := svc.Create(ctx, "off", debugGraph(), false)
	require.NoError(t, err)

	// Stored in repo.
	got, err := repo.Get(ctx, f.ID)
	require.NoError(t, err)
	require.Equal(t, "off", got.Name)

	// Manager is NOT running the flow.
	require.False(t, mgr.Running(f.ID))
}

// ---- Get / List ------------------------------------------------------------

func TestService_Get_Passthrough(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()

	f, err := svc.Create(ctx, "x", debugGraph(), false)
	require.NoError(t, err)

	got, err := svc.Get(ctx, f.ID)
	require.NoError(t, err)
	require.Equal(t, f.ID, got.ID)
}

func TestService_Get_Unknown_ReturnsErrNotFound(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.Get(ctx, uuid.New())
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestService_List_Passthrough(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.Create(ctx, "a", debugGraph(), false)
	require.NoError(t, err)
	_, err = svc.Create(ctx, "b", debugGraph(), false)
	require.NoError(t, err)

	list, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

// ---- Update ----------------------------------------------------------------

func TestService_Update_InvalidGraph_ReturnsError_WithoutTouchingRepo(t *testing.T) {
	svc, repo, _ := newTestService()
	ctx := context.Background()

	f, err := svc.Create(ctx, "orig", debugGraph(), false)
	require.NoError(t, err)

	_, err = svc.Update(ctx, f.ID, UpdateRequest{Graph: graphPtr(invalidGraph())})
	require.Error(t, err)

	// Repo still has original name.
	got, _ := repo.Get(ctx, f.ID)
	require.Equal(t, "orig", got.Name)
}

func TestService_Update_GraphChange_CallsManagerReplace(t *testing.T) {
	svc, _, mgr := newTestService()
	ctx := context.Background()

	// Create enabled so manager starts.
	f, err := svc.Create(ctx, "flow", debugGraph(), true)
	require.NoError(t, err)
	require.True(t, mgr.Running(f.ID))

	// Update graph — manager.Replace is called (re-starts under same ID).
	newGraph := graph.Graph{
		Nodes: []graph.Node{{ID: "n2", Type: "debug_sink", Data: json.RawMessage(`{}`)}},
	}
	updated, err := svc.Update(ctx, f.ID, UpdateRequest{Graph: graphPtr(newGraph)})
	require.NoError(t, err)
	require.True(t, mgr.Running(updated.ID))
	t.Cleanup(func() { mgr.Stop(updated.ID) })
}

func TestService_Update_EnableStoppedFlow_CallsManagerStart(t *testing.T) {
	svc, _, mgr := newTestService()
	ctx := context.Background()

	// Create disabled.
	f, err := svc.Create(ctx, "flow", debugGraph(), false)
	require.NoError(t, err)
	require.False(t, mgr.Running(f.ID))

	// Enable it via Update.
	_, err = svc.Update(ctx, f.ID, UpdateRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)
	require.True(t, mgr.Running(f.ID))
	t.Cleanup(func() { mgr.Stop(f.ID) })
}

func TestService_Update_DisableRunningFlow_CallsManagerStop(t *testing.T) {
	svc, _, mgr := newTestService()
	ctx := context.Background()

	// Create enabled.
	f, err := svc.Create(ctx, "flow", debugGraph(), true)
	require.NoError(t, err)
	require.True(t, mgr.Running(f.ID))

	// Disable it via Update.
	_, err = svc.Update(ctx, f.ID, UpdateRequest{Enabled: boolPtr(false)})
	require.NoError(t, err)
	require.False(t, mgr.Running(f.ID))
}

func TestService_Update_RenameOnly_DoesNotChangeManagerState(t *testing.T) {
	svc, _, mgr := newTestService()
	ctx := context.Background()

	f, err := svc.Create(ctx, "orig", debugGraph(), false)
	require.NoError(t, err)
	require.False(t, mgr.Running(f.ID))

	updated, err := svc.Update(ctx, f.ID, UpdateRequest{Name: strPtr("renamed")})
	require.NoError(t, err)
	require.Equal(t, "renamed", updated.Name)
	require.False(t, mgr.Running(f.ID))
}

func TestService_Update_UnknownID_ReturnsErrNotFound(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.Update(ctx, uuid.New(), UpdateRequest{Name: strPtr("x")})
	require.ErrorIs(t, err, store.ErrNotFound)
}

// ---- Delete ----------------------------------------------------------------

func TestService_Delete_StopsManagerThenDeletes(t *testing.T) {
	svc, repo, mgr := newTestService()
	ctx := context.Background()

	f, err := svc.Create(ctx, "flow", debugGraph(), true)
	require.NoError(t, err)
	require.True(t, mgr.Running(f.ID))

	err = svc.Delete(ctx, f.ID)
	require.NoError(t, err)
	require.False(t, mgr.Running(f.ID))

	_, err = repo.Get(ctx, f.ID)
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestService_Delete_UnknownID_ReturnsErrNotFound(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()

	// Delete a flow that was never created.
	err := svc.Delete(ctx, uuid.New())
	require.ErrorIs(t, err, store.ErrNotFound)
}

// ---- LoadEnabled -----------------------------------------------------------

func TestService_LoadEnabled_StartsEnabledFlows(t *testing.T) {
	svc, repo, mgr := newTestService()
	ctx := context.Background()

	// Pre-populate repo directly.
	f1 := &store.Flow{Name: "on", Enabled: true, Graph: debugGraph()}
	f2 := &store.Flow{Name: "off", Enabled: false, Graph: debugGraph()}
	require.NoError(t, repo.Create(ctx, f1))
	require.NoError(t, repo.Create(ctx, f2))

	require.NoError(t, svc.LoadEnabled(ctx))

	require.True(t, mgr.Running(f1.ID))
	require.False(t, mgr.Running(f2.ID))
	t.Cleanup(func() { mgr.Stop(f1.ID) })
}

// ---- Interface compliance --------------------------------------------------

func TestFakeRepo_SatisfiesFlowRepoInterface(t *testing.T) {
	// Compile-time check that fakeRepo satisfies flowRepo.
	var _ flowRepo = (*fakeRepo)(nil)
}

func TestStoreRepo_SatisfiesFlowRepoInterface(t *testing.T) {
	// Compile-time check that *store.Repo satisfies flowRepo.
	var _ flowRepo = (*store.Repo)(nil)
}

// Ensure errors.Is works for store.ErrNotFound across the package boundary.
func TestErrNotFound_IsStoreErrNotFound(t *testing.T) {
	require.True(t, errors.Is(store.ErrNotFound, store.ErrNotFound))
}
