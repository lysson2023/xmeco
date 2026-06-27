-- Add device_id to control_record to enable project/building filtering.
-- Existing rows with empty/null device_name get NULL device_id.
ALTER TABLE control_record ADD COLUMN IF NOT EXISTS device_id INT;
CREATE INDEX IF NOT EXISTS idx_control_record_device_id ON control_record (device_id);
