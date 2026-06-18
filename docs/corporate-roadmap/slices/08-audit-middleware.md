# Slice 08 — Audit middleware & emitters

**Phase:** 2-audit  
**Depends on:** `07-audit-events-table`

## Goal

Automatically record config changes and approval actions.

## Tasks

- [ ] Emit audit on: provider create/update/delete, MCP create/update/delete
- [ ] Emit audit on: approval approve/reject (`PendingAction`)
- [ ] Emit audit on: permission create/pause/revoke
- [ ] Emit audit on: login success/failure (no password in payload)
- [ ] Correlation ID: propagate from `X-Request-ID` or generate UUID per request

## Definition of Done

- [ ] Integration test or handler test proving event written
- [ ] `go test ./...` green

## Out of scope

- UI, export
