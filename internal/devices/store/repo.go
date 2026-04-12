package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("device not found")

type Device struct {
	ID        uuid.UUID
	Name      string
	Token     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repo struct{ pool *pgxpool.Pool }

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, d *Device) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO devices (id, name, token, created_at, updated_at) VALUES ($1,$2,$3,$4,$4)`,
		d.ID, d.Name, d.Token, now)
	if err != nil {
		return err
	}
	d.CreatedAt, d.UpdatedAt = now, now
	return nil
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (*Device, error) {
	return scan(r.pool.QueryRow(ctx,
		`SELECT id, name, token, created_at, updated_at FROM devices WHERE id=$1`, id))
}

func (r *Repo) GetByToken(ctx context.Context, token string) (*Device, error) {
	return scan(r.pool.QueryRow(ctx,
		`SELECT id, name, token, created_at, updated_at FROM devices WHERE token=$1`, token))
}

func (r *Repo) List(ctx context.Context) ([]*Device, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, token, created_at, updated_at FROM devices ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Device
	for rows.Next() {
		d, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repo) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE devices SET name=$2, updated_at=$3 WHERE id=$1`, id, name, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateToken(ctx context.Context, id uuid.UUID, token string) error {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE devices SET token=$2, updated_at=$3 WHERE id=$1`, id, token, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM devices WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (*Device, error) {
	var d Device
	if err := s.Scan(&d.ID, &d.Name, &d.Token, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}
