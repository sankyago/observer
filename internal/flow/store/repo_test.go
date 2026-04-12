//go:build integration

package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/db"
	"github.com/sankyago/observer/internal/flow/graph"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setup(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "timescale/timescaledb:latest-pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, db.Migrate(ctx, pool, "../../../migrations"))
	return pool
}

func debugSinkGraph() graph.Graph {
	return graph.Graph{
		Nodes: []graph.Node{{ID: "a", Type: "debug_sink", Data: json.RawMessage(`{}`)}},
	}
}

func TestRepo_CRUD(t *testing.T) {
	pool := setup(t)
	repo := NewRepo(pool)
	ctx := context.Background()

	f := &Flow{
		Name:    "demo",
		Enabled: true,
		Graph:   debugSinkGraph(),
	}
	require.NoError(t, repo.Create(ctx, f))
	require.NotEqual(t, "00000000-0000-0000-0000-000000000000", f.ID.String())

	got, err := repo.Get(ctx, f.ID)
	require.NoError(t, err)
	require.Equal(t, "demo", got.Name)
	require.Len(t, got.Graph.Nodes, 1)

	f.Name = "renamed"
	require.NoError(t, repo.Update(ctx, f))
	got, err = repo.Get(ctx, f.ID)
	require.NoError(t, err)
	require.Equal(t, "renamed", got.Name)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	enabled, err := repo.ListEnabled(ctx)
	require.NoError(t, err)
	require.Len(t, enabled, 1)

	require.NoError(t, repo.Delete(ctx, f.ID))
	_, err = repo.Get(ctx, f.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRepo_GetUnknown_ReturnsErrNotFound(t *testing.T) {
	pool := setup(t)
	repo := NewRepo(pool)
	ctx := context.Background()

	_, err := repo.Get(ctx, uuid.New())
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRepo_UpdateUnknown_ReturnsErrNotFound(t *testing.T) {
	pool := setup(t)
	repo := NewRepo(pool)
	ctx := context.Background()

	f := &Flow{
		ID:    uuid.New(),
		Name:  "ghost",
		Graph: debugSinkGraph(),
	}
	err := repo.Update(ctx, f)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRepo_DeleteUnknown_ReturnsErrNotFound(t *testing.T) {
	pool := setup(t)
	repo := NewRepo(pool)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New())
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRepo_ListEnabled_ExcludesDisabled(t *testing.T) {
	pool := setup(t)
	repo := NewRepo(pool)
	ctx := context.Background()

	enabled := &Flow{Name: "on", Enabled: true, Graph: debugSinkGraph()}
	disabled := &Flow{Name: "off", Enabled: false, Graph: debugSinkGraph()}

	require.NoError(t, repo.Create(ctx, enabled))
	require.NoError(t, repo.Create(ctx, disabled))

	all, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)

	listed, err := repo.ListEnabled(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, enabled.ID, listed[0].ID)
}
