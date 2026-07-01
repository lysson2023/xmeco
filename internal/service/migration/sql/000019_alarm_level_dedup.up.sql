-- 000019: Fix alarm dedup to include level, so different severity alarms
-- for the same device+metric are tracked independently.
DROP INDEX IF EXISTS idx_alarm_log_dedup;
CREATE UNIQUE INDEX IF NOT EXISTS idx_alarm_log_dedup ON alarm_log (device_id, alarm_type, level) WHERE ack_at IS NULL;
