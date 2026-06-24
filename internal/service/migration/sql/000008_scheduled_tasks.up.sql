-- XMECO Migration 008: Scheduled tasks for timed device control
CREATE TABLE IF NOT EXISTS scheduled_task (
    id            SERIAL PRIMARY KEY,
    name          VARCHAR(200) NOT NULL,
    building_id   INT REFERENCES building(id),
    device_id     INT NOT NULL REFERENCES device(id),
    action_type   VARCHAR(50) NOT NULL,       -- startup, shutdown, set_value, mode_change
    target_value  VARCHAR(100),               -- target value for set_value/mode_change
    schedule_type VARCHAR(20) NOT NULL DEFAULT 'once', -- once, daily, weekly
    schedule_time TIME NOT NULL,              -- HH:MM execution time
    days_of_week  VARCHAR(20),               -- 1-7 comma separated (1=Mon), for weekly
    enabled       BOOLEAN NOT NULL DEFAULT true,
    last_run_at   TIMESTAMPTZ,
    last_result   VARCHAR(50),               -- success, failed, skipped
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scheduled_task_building ON scheduled_task(building_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_task_device ON scheduled_task(device_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_task_enabled ON scheduled_task(enabled);
