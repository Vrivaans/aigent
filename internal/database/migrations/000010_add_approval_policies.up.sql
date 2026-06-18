CREATE TABLE IF NOT EXISTS approval_policies (
    id SERIAL PRIMARY KEY,
    tool_pattern VARCHAR(255) NOT NULL,
    environment VARCHAR(64) NOT NULL DEFAULT '*',
    requires_approval BOOLEAN NOT NULL DEFAULT true,
    min_role VARCHAR(64) NOT NULL DEFAULT 'operator',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_approval_policies_tool_pattern ON approval_policies(tool_pattern);
