-- 000016: Add name and company columns to maintenance_record.
ALTER TABLE maintenance_record ADD COLUMN IF NOT EXISTS name VARCHAR(200);
ALTER TABLE maintenance_record ADD COLUMN IF NOT EXISTS company VARCHAR(200);
