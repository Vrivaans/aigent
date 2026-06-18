-- RBAC permission matrix (seeded by SeedRolesAndPermissions in Go):
--
-- | Role     | Permissions |
-- |----------|-------------|
-- | admin    | *:* (full access) |
-- | operator | agents, providers, mcp, permissions, chat, workflows, tasks, rules — read+write |
-- | auditor  | audit (read, export); chat, agents, permissions — read |
-- | viewer   | chat, agents — read |

CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(64) UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS role_permissions (
    id SERIAL PRIMARY KEY,
    role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    resource VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    UNIQUE (role_id, resource, action)
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);
