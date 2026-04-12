package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/flow/graph"
)

var ErrNotFound = errors.New("flow not found")

type Flow struct {
	ID        uuid.UUID
	Name      string
	Graph     graph.Graph
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, f *Flow) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	raw, err := json.Marshal(f.Graph)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = r.pool.Exec(ctx,
		`INSERT INTO flows (id, name, graph, enabled, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$5)`,
		f.ID, f.Name, raw, f.Enabled, now)
	if err != nil {
		return fmt.Errorf("insert flow: %w", err)
	}
	f.CreatedAt, f.UpdatedAt = now, now
	return nil
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (*Flow, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, graph, enabled, created_at, updated_at FROM flows WHERE id=$1`, id)
	return scan(row)
}

func (r *Repo) List(ctx context.Context) ([]*Flow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, graph, enabled, created_at, updated_at FROM flows ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Flow
	for rows.Next() {
		f, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *Repo) ListEnabled(ctx context.Context) ([]*Flow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, graph, enabled, created_at, updated_at FROM flows WHERE enabled=true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Flow
	for rows.Next() {
		f, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *Repo) Update(ctx context.Context, f *Flow) error {
	raw, err := json.Marshal(f.Graph)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE flows SET name=$2, graph=$3, enabled=$4, updated_at=$5 WHERE id=$1`,
		f.ID, f.Name, raw, f.Enabled, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	f.UpdatedAt = now
	return nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM flows WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scan(s scanner) (*Flow, error) {
	var f Flow
	var raw []byte
	if err := s.Scan(&f.ID, &f.Name, &raw, &f.Enabled, &f.CreatedAt, &f.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &f.Graph); err != nil {
		return nil, fmt.Errorf("unmarshal graph: %w", err)
	}
	return &f, nil
}
