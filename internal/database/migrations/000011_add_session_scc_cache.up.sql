ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS layer2_hash VARCHAR(64),
    ADD COLUMN IF NOT EXISTS provider_cache_id VARCHAR(512),
    ADD COLUMN IF NOT EXISTS cache_expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sessions_layer2_hash ON sessions(layer2_hash);
