# Slice 05 — Protect admin & config routes

**Phase:** 1-rbac  
**Depends on:** `04-rbac-middleware`

## Goal

Enforce RBAC on sensitive API routes in `cmd/server/main.go`.

## Tasks

- [ ] `/api/admin/*` → require `admin` role or `agents:*` permissions as defined
- [ ] Provider CRUD → `providers:write` (admin/operator)
- [ ] MCP config CRUD → `mcp:write`
- [ ] Permissions tab API → `permissions:write`
- [ ] Read-only routes (chat, sessions) → `chat:read` for viewer+

## Definition of Done

- [ ] Document permission matrix in `docs/corporate-roadmap/RBAC-MATRIX.md`
- [ ] Manual smoke: viewer cannot POST `/api/providers`
- [ ] `go test ./...` green

## Out of scope

- User management UI
