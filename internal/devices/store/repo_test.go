//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/db"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setup(t *testing.T) *pgxpool.Pool {
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

func TestRepo_CRUD(t *testing.T) {
	repo := NewRepo(setup(t))
	ctx := context.Background()

	d := &Device{Name: "pump-1", Token: "tok-abc"}
	require.NoError(t, repo.Create(ctx, d))
	require.NotEqual(t, uuid.Nil, d.ID)

	got, err := repo.Get(ctx, d.ID)
	require.NoError(t, err)
	require.Equal(t, "pump-1", got.Name)

	byTok, err := repo.GetByToken(ctx, "tok-abc")
	require.NoError(t, err)
	require.Equal(t, d.ID, byTok.ID)

	require.NoError(t, repo.UpdateName(ctx, d.ID, "pump-2"))
	got, _ = repo.Get(ctx, d.ID)
	require.Equal(t, "pump-2", got.Name)

	require.NoError(t, repo.UpdateToken(ctx, d.ID, "tok-xyz"))
	_, err = repo.GetByToken(ctx, "tok-abc")
	require.ErrorIs(t, err, ErrNotFound)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, repo.Delete(ctx, d.ID))
	_, err = repo.Get(ctx, d.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRepo_ErrNotFound(t *testing.T) {
	repo := NewRepo(setup(t))
	ctx := context.Background()
	bogus := uuid.New()
	_, err := repo.Get(ctx, bogus)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = repo.GetByToken(ctx, "nope")
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, repo.UpdateName(ctx, bogus, "x"), ErrNotFound)
	require.ErrorIs(t, repo.UpdateToken(ctx, bogus, "x"), ErrNotFound)
	require.ErrorIs(t, repo.Delete(ctx, bogus), ErrNotFound)
}
