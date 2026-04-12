CREATE TABLE IF NOT EXISTS flows (
    id         UUID PRIMARY KEY,
    name       TEXT NOT NULL,
    graph      JSONB NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS flows_enabled_idx ON flows (enabled);
