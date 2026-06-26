-- XMECO Migration 012: Add missing foreign key constraints, indexes, and data integrity checks.

-- 1. FK: users.role_id → role(id) — 幂等：仅在约束不存在时创建
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_users_role') THEN
    ALTER TABLE users ADD CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES role(id);
  END IF;
END $$;

-- 2. FK: startup_execution.plan_id → startup_plan(id) — 幂等
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_execution_plan') THEN
    ALTER TABLE startup_execution ADD CONSTRAINT fk_execution_plan FOREIGN KEY (plan_id) REFERENCES startup_plan(id) ON DELETE CASCADE;
  END IF;
END $$;

-- 3. Indexes for common query patterns (幂等)
CREATE INDEX IF NOT EXISTS idx_alarm_log_created_at ON alarm_log (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_control_record_created_at ON control_record (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_device_telemetry_device_id ON device_telemetry (device_id);

-- 4. NOT NULL constraint on users.role_id (all users should have a role)
-- Skip if there are rows with NULL role_id — the migration will warn but not fail.
DO $$
BEGIN
  ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;
EXCEPTION WHEN others THEN
  RAISE WARNING 'Could not add NOT NULL to users.role_id — fix NULL rows manually';
END $$;
