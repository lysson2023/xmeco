-- 000020: Covering index for per-device latest-metric lookups.
-- Speeds up LATERAL subqueries in ScreenData meters query from
-- "scan rows until metric match" to direct index seek.
CREATE INDEX IF NOT EXISTS idx_telemetry_device_metric_ts
    ON device_telemetry (device_id, metric, ts DESC);
