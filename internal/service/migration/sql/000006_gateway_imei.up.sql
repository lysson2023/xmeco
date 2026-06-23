-- XMECO Migration 006: Extend gateway_imei to support USR-IOT and future gateway IDs
ALTER TABLE device ALTER COLUMN gateway_imei TYPE VARCHAR(64);
ALTER TABLE gateway_config ALTER COLUMN gateway_imei TYPE VARCHAR(64);
