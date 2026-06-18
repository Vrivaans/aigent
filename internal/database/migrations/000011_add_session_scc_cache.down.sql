DROP INDEX IF EXISTS idx_sessions_layer2_hash;

ALTER TABLE sessions
    DROP COLUMN IF EXISTS cache_expires_at,
    DROP COLUMN IF EXISTS provider_cache_id,
    DROP COLUMN IF EXISTS layer2_hash;
