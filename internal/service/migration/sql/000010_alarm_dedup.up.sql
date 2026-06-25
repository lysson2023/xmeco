-- 000010: Add partial unique index on alarm_log for atomic deduplication.
-- Prevents duplicate un-acked alarms for the same device+alarm_type,
-- eliminating the TOCTOU race between SELECT and INSERT in alarm.Evaluate.
CREATE UNIQUE INDEX IF NOT EXISTS idx_alarm_log_dedup ON alarm_log (device_id, alarm_type) WHERE ack_at IS NULL;
