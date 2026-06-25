-- expires_at: optional expiry for timed items (e.g. the 30-day Perzen mounts,
-- item "X(30dias)"). NULL = permanent. Expired items are dropped on character load.
ALTER TABLE item ADD COLUMN expires_at TIMESTAMPTZ;
