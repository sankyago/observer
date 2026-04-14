-- +goose Up
-- +goose StatementBegin
CREATE TABLE fired_actions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fired_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id      UUID NOT NULL,
    device_id      UUID NOT NULL,
    rule_id        UUID NOT NULL,
    action_id      UUID NOT NULL,
    message_id     UUID NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('ok','error')),
    error          TEXT,
    payload        JSONB NOT NULL
);
CREATE INDEX fired_actions_tenant_time_idx ON fired_actions (tenant_id, fired_at DESC);
CREATE INDEX fired_actions_device_time_idx ON fired_actions (device_id, fired_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS fired_actions;
-- +goose StatementEnd
