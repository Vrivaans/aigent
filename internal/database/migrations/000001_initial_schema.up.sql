CREATE TABLE IF NOT EXISTS llm_providers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    api_key TEXT NOT NULL DEFAULT '',
    default_model VARCHAR(255) NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS agents (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    llm_provider_id INTEGER REFERENCES llm_providers(id) ON DELETE SET NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_tools (
    id SERIAL PRIMARY KEY,
    agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tool_name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL DEFAULT 'Nueva conversacion',
    agent_id INTEGER NOT NULL DEFAULT 1 REFERENCES agents(id) ON DELETE SET DEFAULT,
    llm_provider_override_id INTEGER REFERENCES llm_providers(id) ON DELETE SET NULL,
    llm_model_override VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id SERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL DEFAULT 1 REFERENCES sessions(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    tool_call_id VARCHAR(100) NOT NULL DEFAULT '',
    raw_tool_calls TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pending_actions (
    id SERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    tool_name VARCHAR(255) NOT NULL DEFAULT '',
    arguments TEXT NOT NULL DEFAULT '',
    tool_call_id VARCHAR(100) NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS rules (
    id SERIAL PRIMARY KEY,
    category VARCHAR(100) NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    importance INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rule_agents (
    rule_id INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    PRIMARY KEY (rule_id, agent_id)
);

CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    cron_expression VARCHAR(100) NOT NULL,
    agent_id INTEGER NOT NULL DEFAULT 1 REFERENCES agents(id) ON DELETE SET DEFAULT,
    prompt TEXT NOT NULL DEFAULT '',
    one_shot BOOLEAN NOT NULL DEFAULT false,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_result TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS hands_ai_configs (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    url TEXT NOT NULL DEFAULT '',
    token TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS mcp_stdio_servers (
    id SERIAL PRIMARY KEY,
    alias VARCHAR(64) UNIQUE NOT NULL,
    command VARCHAR(512) NOT NULL,
    args_json JSONB NOT NULL DEFAULT '[]',
    env_cipher TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS mcp_stream_servers (
    id SERIAL PRIMARY KEY,
    alias VARCHAR(64) UNIQUE NOT NULL,
    base_url VARCHAR(2048) NOT NULL,
    headers_cipher TEXT NOT NULL DEFAULT '',
    disable_standalone_sse BOOLEAN NOT NULL DEFAULT false,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tool_permissions (
    id SERIAL PRIMARY KEY,
    agent_id INTEGER NOT NULL,
    tool_name VARCHAR(255) NOT NULL,
    action_type VARCHAR(20) NOT NULL DEFAULT 'always_allow',
    paused BOOLEAN NOT NULL DEFAULT false,
    paused_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tool_permissions_agent_tool ON tool_permissions(agent_id, tool_name);
CREATE INDEX IF NOT EXISTS idx_pending_actions_session_status ON pending_actions(session_id, status);
CREATE INDEX IF NOT EXISTS idx_chat_messages_session ON chat_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_agent ON sessions(agent_id);
CREATE INDEX IF NOT EXISTS idx_tasks_next_run ON tasks(next_run_at);

INSERT INTO agents (id, name, description, is_default, created_at, updated_at)
VALUES (1, 'General', 'Agente multiproposito con acceso completo a todas las herramientas configuradas.', true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

SELECT setval('agents_id_seq', (SELECT COALESCE(MAX(id), 1) FROM agents));
