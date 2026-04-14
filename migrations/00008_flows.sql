-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS rules;
DROP TABLE IF EXISTS actions;

CREATE TABLE flows (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    graph       JSONB NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX flows_tenant_idx ON flows (tenant_id) WHERE enabled;

TRUNCATE fired_actions;
ALTER TABLE fired_actions RENAME TO flow_executions;
ALTER TABLE flow_executions DROP COLUMN rule_id;
ALTER TABLE flow_executions DROP COLUMN action_id;
ALTER TABLE flow_executions ADD COLUMN flow_id UUID NOT NULL;
ALTER TABLE flow_executions ADD COLUMN node_id TEXT NOT NULL;
CREATE INDEX flow_executions_flow_time_idx ON flow_executions (flow_id, fired_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE flow_executions DROP COLUMN flow_id;
ALTER TABLE flow_executions DROP COLUMN node_id;
ALTER TABLE flow_executions ADD COLUMN rule_id UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE flow_executions ADD COLUMN action_id UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE flow_executions RENAME TO fired_actions;

DROP TABLE IF EXISTS flows;

CREATE TABLE actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('log','webhook','email')),
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX actions_tenant_idx ON actions (tenant_id);

CREATE TABLE rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    field TEXT NOT NULL,
    op TEXT NOT NULL CHECK (op IN ('>','<','>=','<=','=','!=')),
    value DOUBLE PRECISION NOT NULL,
    action_id UUID NOT NULL REFERENCES actions(id) ON DELETE RESTRICT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    debug_until TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX rules_device_enabled_idx ON rules (device_id) WHERE enabled;
-- +goose StatementEnd
