ALTER TABLE pending_actions
    ADD COLUMN IF NOT EXISTS resolved_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_pending_actions_resolved_by_user_id ON pending_actions(resolved_by_user_id);
