-- 014_sensor_config.up.sql
-- 温湿度传感器配置表
CREATE TABLE IF NOT EXISTS sensor_config (
    id SERIAL PRIMARY KEY,
    device_id INT NOT NULL REFERENCES device(id) ON DELETE CASCADE,
    channel_no INT DEFAULT 1,           -- 传感器通道号
    sensor_no VARCHAR(50) DEFAULT '',   -- 传感器编号
    interval_minutes INT DEFAULT 5,     -- 时间间隔(分钟)
    sensor_type VARCHAR(50) DEFAULT '温湿度', -- 传感器类型
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 每个设备只允许一条传感器配置
CREATE UNIQUE INDEX IF NOT EXISTS idx_sensor_config_device_id ON sensor_config (device_id);
