-- 000015: Maintenance records table for equipment servicing tracking.
CREATE TABLE IF NOT EXISTS maintenance_record (
    id SERIAL PRIMARY KEY,
    device_id INT NOT NULL REFERENCES device(id) ON DELETE CASCADE,
    device_name VARCHAR(200),
    building_id INT,
    project_id INT,
    record_type VARCHAR(50) NOT NULL DEFAULT '维修',
    description TEXT,
    operator VARCHAR(100),
    record_date DATE NOT NULL DEFAULT CURRENT_DATE,
    next_date DATE,
    cost DECIMAL(10,2) DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT '已完成',
    remark TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_mr_device_id ON maintenance_record (device_id);
CREATE INDEX IF NOT EXISTS idx_mr_record_date ON maintenance_record (record_date DESC);
CREATE INDEX IF NOT EXISTS idx_mr_building_id ON maintenance_record (building_id);
