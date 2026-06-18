DROP INDEX IF EXISTS idx_pending_actions_resolved_by_user_id;

ALTER TABLE pending_actions
    DROP COLUMN IF EXISTS resolved_at,
    DROP COLUMN IF EXISTS resolved_by_user_id;
