-- +goose Up
-- +goose StatementBegin
CREATE TABLE telemetry_aggregates (
    bucket_start  TIMESTAMPTZ NOT NULL,
    tenant_id     UUID NOT NULL,
    device_id     UUID NOT NULL,
    field_name    TEXT NOT NULL,
    count         BIGINT NOT NULL,
    sum           DOUBLE PRECISION NOT NULL,
    min           DOUBLE PRECISION NOT NULL,
    max           DOUBLE PRECISION NOT NULL,
    last_value    DOUBLE PRECISION NOT NULL,
    last_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, device_id, field_name, bucket_start)
);
SELECT create_hypertable('telemetry_aggregates', 'bucket_start', chunk_time_interval => INTERVAL '7 days');
ALTER TABLE telemetry_aggregates SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'device_id, field_name',
    timescaledb.compress_orderby = 'bucket_start DESC'
);
SELECT add_compression_policy('telemetry_aggregates', INTERVAL '7 days');
SELECT add_retention_policy('telemetry_aggregates', INTERVAL '365 days');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT remove_retention_policy('telemetry_aggregates', if_exists => true);
SELECT remove_compression_policy('telemetry_aggregates', if_exists => true);
DROP TABLE IF EXISTS telemetry_aggregates;
-- +goose StatementEnd
