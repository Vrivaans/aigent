# RBAC Permission Matrix

Route enforcement added in `cmd/server/main.go` (slice `05-protect-admin-routes`).

## Roles (seed)

| Role | Description |
|------|-------------|
| **admin** | Full access via wildcard `*:*` |
| **operator** | Configure integrations, chat, agents, workflows, tasks, rules |
| **auditor** | Read audit trail, chat, agents, permissions |
| **viewer** | Read-only chat and agent catalog visibility |

## Resource permissions by role

| Resource | Action | admin | operator | auditor | viewer |
|----------|--------|:-----:|:--------:|:-------:|:------:|
| `*` | `*` | ✓ | | | |
| `agents` | read | ✓ | ✓ | ✓ | ✓ |
| `agents` | write | ✓ | ✓ | | |
| `providers` | read | ✓ | ✓ | | |
| `providers` | write | ✓ | ✓ | | |
| `mcp` | read | ✓ | ✓ | | |
| `mcp` | write | ✓ | ✓ | | |
| `permissions` | read | ✓ | | ✓ | |
| `permissions` | write | ✓ | ✓ | | |
| `chat` | read | ✓ | ✓ | ✓ | ✓ |
| `chat` | write | ✓ | ✓ | | |
| `workflows` | read | ✓ | ✓ | | |
| `workflows` | write | ✓ | ✓ | | |
| `tasks` | read | ✓ | ✓ | | |
| `tasks` | write | ✓ | ✓ | | |
| `rules` | read | ✓ | ✓ | | |
| `rules` | write | ✓ | ✓ | | |
| `audit` | read | ✓ | | ✓ | |
| `audit` | export | ✓ | | ✓ | |

## API route mapping

| Routes | Required permission |
|--------|---------------------|
| `GET /api/sessions`, `GET /api/sessions/:id/chat`, `GET /api/sessions/:id/artifacts`, `GET /api/sessions/:id/files`, `GET /api/approvals` | `chat:read` |
| `POST/PATCH/DELETE /api/sessions/*`, chat stream, confirm, SCC goals/workspace/files | `chat:write` |
| `GET /api/active-tools` | `agents:read` |
| `GET /api/providers*`, `GET /api/models` | `providers:read` |
| `POST/PATCH/DELETE /api/providers*` | `providers:write` |
| `GET /api/config/handsai`, `GET /api/config/mcp-*` | `mcp:read` |
| `PATCH/DELETE /api/config/handsai`, MCP POST/PATCH/DELETE/test | `mcp:write` |
| `GET /api/admin/agents*` | `agents:read` |
| `POST/PUT/DELETE /api/admin/agents*` | `agents:write` |
| `GET /api/permissions` | `permissions:read` |
| `DELETE/POST /api/permissions/*` | `permissions:write` |
| `GET /api/tasks`, `GET /api/rules`, `GET /api/workflows*` | `tasks:read`, `rules:read`, `workflows:read` respectively |
| `POST/DELETE /api/tasks`, `POST/DELETE /api/rules`, workflow mutations | matching `:write` |
| `POST/GET /api/rag/*` | `providers:write` (knowledge ingestion) |
| `GET/POST/PATCH /api/admin/users*`, `GET /api/admin/roles` | **admin** role (`RequireRoleMiddleware`) |

## Wildcard matching

`UserHasPermission` treats `*:*`, `resource:*`, and `*:action` as supersets. The **admin** role seed uses `*:*` only.

## Bootstrap user

The first user created from `ADMIN_USERNAME` / `ADMIN_PASSWORD` receives the **admin** role automatically.
