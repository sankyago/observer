-- +goose Up
-- +goose StatementBegin
CREATE TABLE telemetry_raw (
    time        TIMESTAMPTZ NOT NULL,
    tenant_id   UUID NOT NULL,
    device_id   UUID NOT NULL,
    message_id  UUID NOT NULL,
    payload     JSONB NOT NULL
);
SELECT create_hypertable('telemetry_raw', 'time', chunk_time_interval => INTERVAL '1 day');
CREATE INDEX telemetry_raw_device_time_idx ON telemetry_raw (device_id, time DESC);
ALTER TABLE telemetry_raw SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'device_id',
    timescaledb.compress_orderby = 'time DESC'
);
SELECT add_compression_policy('telemetry_raw', INTERVAL '1 day');
SELECT add_retention_policy('telemetry_raw', INTERVAL '7 days');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT remove_retention_policy('telemetry_raw', if_exists => true);
SELECT remove_compression_policy('telemetry_raw', if_exists => true);
DROP TABLE IF EXISTS telemetry_raw;
-- +goose StatementEnd
