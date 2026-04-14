-- +goose Up
CREATE TABLE actions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('log','webhook','email')),
    config      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX actions_tenant_idx ON actions (tenant_id);

CREATE TABLE rules (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    device_id    UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    field        TEXT NOT NULL,
    op           TEXT NOT NULL CHECK (op IN ('>','<','>=','<=','=','!=')),
    value        DOUBLE PRECISION NOT NULL,
    action_id    UUID NOT NULL REFERENCES actions(id) ON DELETE RESTRICT,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    debug_until  TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX rules_device_enabled_idx ON rules (device_id) WHERE enabled;

-- +goose Down
DROP TABLE IF EXISTS rules;
DROP TABLE IF EXISTS actions;
