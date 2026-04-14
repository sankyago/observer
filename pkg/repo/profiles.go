package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeviceProfile struct {
	ID            uuid.UUID `json:"id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	Name          string    `json:"name"`
	DefaultFields []string  `json:"default_fields"`
	CreatedAt     time.Time `json:"created_at"`
}

type DeviceProfileInput struct {
	Name          string   `json:"name"`
	DefaultFields []string `json:"default_fields"`
}

func ListProfiles(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([]DeviceProfile, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, tenant_id, name, default_fields, created_at
		 FROM device_profiles WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []DeviceProfile{}
	for rows.Next() {
		var p DeviceProfile
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.DefaultFields, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func CreateProfile(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, in DeviceProfileInput) (DeviceProfile, error) {
	fields := in.DefaultFields
	if fields == nil { fields = []string{} }
	var p DeviceProfile
	err := pool.QueryRow(ctx,
		`INSERT INTO device_profiles (tenant_id, name, default_fields) VALUES ($1,$2,$3)
		 RETURNING id, tenant_id, name, default_fields, created_at`,
		tenantID, in.Name, fields,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.DefaultFields, &p.CreatedAt)
	return p, err
}

func DeleteProfile(ctx context.Context, pool *pgxpool.Pool, tenantID, id uuid.UUID) error {
	_, err := pool.Exec(ctx, `DELETE FROM device_profiles WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	return err
}
