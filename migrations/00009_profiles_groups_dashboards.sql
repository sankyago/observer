-- +goose Up
-- +goose StatementBegin
CREATE TABLE device_profiles (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    default_fields TEXT[] NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX device_profiles_tenant_idx ON device_profiles (tenant_id);

CREATE TABLE device_groups (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX device_groups_tenant_idx ON device_groups (tenant_id);

ALTER TABLE devices
    ADD COLUMN profile_id UUID NULL REFERENCES device_profiles(id) ON DELETE SET NULL,
    ADD COLUMN group_id   UUID NULL REFERENCES device_groups(id) ON DELETE SET NULL;

CREATE TABLE dashboards (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    layout     JSONB NOT NULL DEFAULT '{"widgets":[]}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX dashboards_tenant_idx ON dashboards (tenant_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE devices DROP COLUMN IF EXISTS profile_id;
ALTER TABLE devices DROP COLUMN IF EXISTS group_id;
DROP TABLE IF EXISTS dashboards;
DROP TABLE IF EXISTS device_groups;
DROP TABLE IF EXISTS device_profiles;
-- +goose StatementEnd
