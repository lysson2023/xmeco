-- XMECO Seed Data V003
-- Dashboard 看板配置初始数据（从 autoMigrate 移出）

INSERT INTO dashboard_config (key, value) VALUES
('service_projects', '156'),
('service_area',     '12.8万㎡'),
('service_cities',   '8'),
('power_saved',      '1,245'),
('carbon_saved',     '986'),
('days_start',       '2021-01-01')
ON CONFLICT (key) DO NOTHING;
