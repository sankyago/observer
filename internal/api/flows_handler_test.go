package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/flow"
	"github.com/sankyago/observer/internal/flow/graph"
	"github.com/sankyago/observer/internal/flow/runtime"
	"github.com/sankyago/observer/internal/flow/store"
	"github.com/sankyago/observer/internal/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo is an in-memory implementation of the flowRepo interface.
type fakeRepo struct {
	mu    sync.Mutex
	flows map[uuid.UUID]*store.Flow
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{flows: make(map[uuid.UUID]*store.Flow)}
}

func (r *fakeRepo) Create(ctx context.Context, f *store.Flow) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
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
	out := make([]*store.Flow, 0, len(r.flows))
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

// debugOnlyGraph returns a valid Graph with a single debug_sink node (no MQTT).
// Such a graph compiles and starts without any external dependencies.
func debugOnlyGraph() graph.Graph {
	return graph.Graph{
		Nodes: []graph.Node{
			{ID: "sink1", Type: "debug_sink", Data: json.RawMessage(`{}`)},
		},
		Edges: nil,
	}
}

// newTestService builds a *flow.Service backed by a fakeRepo and a real Manager.
func newTestService(t *testing.T) (*flow.Service, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	mgr := runtime.NewManager(ingest.NewRouter())
	svc := flow.NewService(context.Background(), repo, mgr)
	return svc, repo
}

// do executes a request against a test server backed by NewRouter(svc).
func do(t *testing.T, svc *flow.Service, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	NewRouter(svc, nil).ServeHTTP(rr, req)
	return rr
}

// TestHealth verifies GET /api/health returns 200 with {"status":"ok"}.
func TestHealth(t *testing.T) {
	rr := do(t, nil, http.MethodGet, "/api/health", "")
	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
}

// TestCreateFlow_HappyPath verifies POST /api/flows with enabled=false returns 201.
func TestCreateFlow_HappyPath(t *testing.T) {
	svc, repo := newTestService(t)
	g := debugOnlyGraph()
	body, err := json.Marshal(map[string]any{
		"name":    "my-flow",
		"graph":   g,
		"enabled": false,
	})
	require.NoError(t, err)

	rr := do(t, svc, http.MethodPost, "/api/flows", string(body))
	require.Equal(t, http.StatusCreated, rr.Code)

	var resp flowDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "my-flow", resp.Name)
	assert.False(t, resp.Enabled)
	assert.NotEqual(t, uuid.Nil, resp.ID)

	// Verify persisted in fake repo.
	_, err = repo.Get(context.Background(), resp.ID)
	require.NoError(t, err)
}

// TestCreateFlow_InvalidJSON verifies POST /api/flows with bad JSON returns 400.
func TestCreateFlow_InvalidJSON(t *testing.T) {
	svc, _ := newTestService(t)
	rr := do(t, svc, http.MethodPost, "/api/flows", "{not-json")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestCreateFlow_EmptyName verifies POST /api/flows with empty name returns 400.
func TestCreateFlow_EmptyName(t *testing.T) {
	svc, _ := newTestService(t)
	body, _ := json.Marshal(map[string]any{
		"name":  "",
		"graph": debugOnlyGraph(),
	})
	rr := do(t, svc, http.MethodPost, "/api/flows", string(body))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestCreateFlow_InvalidGraph verifies POST /api/flows with an unknown node type returns 400.
func TestCreateFlow_InvalidGraph(t *testing.T) {
	svc, _ := newTestService(t)
	invalidGraph := graph.Graph{
		Nodes: []graph.Node{
			{ID: "n1", Type: "unknown_type", Data: json.RawMessage(`{}`)},
		},
	}
	body, _ := json.Marshal(map[string]any{
		"name":  "bad",
		"graph": invalidGraph,
	})
	rr := do(t, svc, http.MethodPost, "/api/flows", string(body))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestListFlows verifies GET /api/flows returns 200 with array.
func TestListFlows(t *testing.T) {
	svc, _ := newTestService(t)

	// Seed two flows.
	for _, name := range []string{"alpha", "beta"} {
		body, _ := json.Marshal(map[string]any{"name": name, "graph": debugOnlyGraph()})
		rr := do(t, svc, http.MethodPost, "/api/flows", string(body))
		require.Equal(t, http.StatusCreated, rr.Code)
	}

	rr := do(t, svc, http.MethodGet, "/api/flows", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var resp []flowDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp, 2)
}

// TestGetFlow_HappyPath verifies GET /api/flows/:id returns 200.
func TestGetFlow_HappyPath(t *testing.T) {
	svc, _ := newTestService(t)

	body, _ := json.Marshal(map[string]any{"name": "get-me", "graph": debugOnlyGraph()})
	rr := do(t, svc, http.MethodPost, "/api/flows", string(body))
	require.Equal(t, http.StatusCreated, rr.Code)
	var created flowDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))

	rr2 := do(t, svc, http.MethodGet, "/api/flows/"+created.ID.String(), "")
	require.Equal(t, http.StatusOK, rr2.Code)
	var got flowDTO
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&got))
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "get-me", got.Name)
}

// TestGetFlow_Unknown verifies GET /api/flows/:id with unknown id returns 404.
func TestGetFlow_Unknown(t *testing.T) {
	svc, _ := newTestService(t)
	rr := do(t, svc, http.MethodGet, "/api/flows/"+uuid.New().String(), "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestGetFlow_InvalidUUID verifies GET /api/flows/:id with bad uuid returns 400.
func TestGetFlow_InvalidUUID(t *testing.T) {
	svc, _ := newTestService(t)
	rr := do(t, svc, http.MethodGet, "/api/flows/not-a-uuid", "")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestUpdateFlow_HappyPath verifies PUT /api/flows/:id updates name/graph/enabled.
func TestUpdateFlow_HappyPath(t *testing.T) {
	svc, _ := newTestService(t)

	body, _ := json.Marshal(map[string]any{"name": "original", "graph": debugOnlyGraph()})
	rr := do(t, svc, http.MethodPost, "/api/flows", string(body))
	require.Equal(t, http.StatusCreated, rr.Code)
	var created flowDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))

	newName := "updated"
	updateBody, _ := json.Marshal(map[string]any{"name": &newName})
	rr2 := do(t, svc, http.MethodPut, "/api/flows/"+created.ID.String(), string(updateBody))
	require.Equal(t, http.StatusOK, rr2.Code)
	var updated flowDTO
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&updated))
	assert.Equal(t, "updated", updated.Name)
}

// TestUpdateFlow_UnknownID verifies PUT /api/flows/:id with unknown id returns 404.
func TestUpdateFlow_UnknownID(t *testing.T) {
	svc, _ := newTestService(t)
	body, _ := json.Marshal(map[string]any{"name": ptrStr("x")})
	rr := do(t, svc, http.MethodPut, "/api/flows/"+uuid.New().String(), string(body))
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestUpdateFlow_InvalidGraph verifies PUT /api/flows/:id with invalid graph returns 400.
func TestUpdateFlow_InvalidGraph(t *testing.T) {
	svc, _ := newTestService(t)

	body, _ := json.Marshal(map[string]any{"name": "orig", "graph": debugOnlyGraph()})
	rr := do(t, svc, http.MethodPost, "/api/flows", string(body))
	require.Equal(t, http.StatusCreated, rr.Code)
	var created flowDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))

	badGraph := graph.Graph{
		Nodes: []graph.Node{{ID: "x", Type: "bogus_type", Data: json.RawMessage(`{}`)}},
	}
	updateBody, _ := json.Marshal(map[string]any{"graph": badGraph})
	rr2 := do(t, svc, http.MethodPut, "/api/flows/"+created.ID.String(), string(updateBody))
	assert.Equal(t, http.StatusBadRequest, rr2.Code)
}

// TestDeleteFlow_HappyPath verifies DELETE /api/flows/:id returns 204.
func TestDeleteFlow_HappyPath(t *testing.T) {
	svc, repo := newTestService(t)

	body, _ := json.Marshal(map[string]any{"name": "delete-me", "graph": debugOnlyGraph()})
	rr := do(t, svc, http.MethodPost, "/api/flows", string(body))
	require.Equal(t, http.StatusCreated, rr.Code)
	var created flowDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))

	rr2 := do(t, svc, http.MethodDelete, "/api/flows/"+created.ID.String(), "")
	assert.Equal(t, http.StatusNoContent, rr2.Code)

	// Verify removed from repo.
	_, err := repo.Get(context.Background(), created.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// TestDeleteFlow_Unknown verifies DELETE /api/flows/:id with unknown id returns 404.
func TestDeleteFlow_Unknown(t *testing.T) {
	svc, _ := newTestService(t)
	rr := do(t, svc, http.MethodDelete, "/api/flows/"+uuid.New().String(), "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ptrStr is a test helper.
func ptrStr(s string) *string { return &s }

// Ensure the bytes import is used (the plan stub imported it; we keep it via buffer).
var _ = bytes.NewBuffer

// errRepo wraps fakeRepo and forces Create/Update to return a sentinel error.
type errRepo struct {
	*fakeRepo
	createErr error
	updateErr error
}

func (r *errRepo) Create(ctx context.Context, f *store.Flow) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.fakeRepo.Create(ctx, f)
}

func (r *errRepo) Update(ctx context.Context, f *store.Flow) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.fakeRepo.Update(ctx, f)
}

var errDB = errors.New("db unavailable")

// TestCreateFlow_RepoError_Returns500 verifies that an infrastructure error from
// the repo maps to 500, not 400.
func TestCreateFlow_RepoError_Returns500(t *testing.T) {
	repo := &errRepo{fakeRepo: newFakeRepo(), createErr: errDB}
	mgr := runtime.NewManager(ingest.NewRouter())
	svc := flow.NewService(context.Background(), repo, mgr)

	body, _ := json.Marshal(map[string]any{"name": "x", "graph": debugOnlyGraph()})
	rr := do(t, svc, http.MethodPost, "/api/flows", string(body))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// TestUpdateFlow_RepoError_Returns500 verifies that an infrastructure error from
// the repo on Update maps to 500, not 400.
func TestUpdateFlow_RepoError_Returns500(t *testing.T) {
	inner := newFakeRepo()
	repo := &errRepo{fakeRepo: inner}
	mgr := runtime.NewManager(ingest.NewRouter())
	svc := flow.NewService(context.Background(), repo, mgr)

	// Create a flow first using the underlying repo directly.
	ctx := context.Background()
	f := &store.Flow{Name: "orig", Graph: debugOnlyGraph()}
	require.NoError(t, inner.Create(ctx, f))

	// Now force Update to fail.
	repo.updateErr = errDB

	body, _ := json.Marshal(map[string]any{"name": ptrStr("new-name")})
	rr := do(t, svc, http.MethodPut, "/api/flows/"+f.ID.String(), string(body))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
