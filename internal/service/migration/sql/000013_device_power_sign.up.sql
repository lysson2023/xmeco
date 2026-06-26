-- 013_add_device_power_sign.sql
-- 电表功率方向：1=加（默认），-1=减
-- 用于大屏总功率计算：总功率 = Σ(电表功率 × power_sign)
ALTER TABLE device ADD COLUMN IF NOT EXISTS power_sign SMALLINT DEFAULT 1;
