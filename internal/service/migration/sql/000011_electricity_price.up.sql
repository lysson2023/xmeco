-- 000011: create electricity_price table for time-of-use pricing.
-- Previously created lazily by SavePriceConfig; now managed via migration.
CREATE TABLE IF NOT EXISTS electricity_price (
    name       VARCHAR(20),
    start_hour INT,
    end_hour   INT,
    price      DECIMAL(6,3),
    PRIMARY KEY (name, start_hour)
);
