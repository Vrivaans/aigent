# Slice 07 — Audit events table

**Phase:** 2-audit  
**Depends on:** `06-auth-ui-minimal`

## Goal

Append-only audit log schema.

## Tasks

- [ ] Migration: `audit_events` with fields:
  - `id`, `occurred_at`, `actor_user_id` (nullable for system)
  - `action` (e.g. `provider.create`)
  - `resource_type`, `resource_id`
  - `session_id` (nullable), `ip`, `user_agent`
  - `payload_before`, `payload_after` (JSON text, optional)
  - `correlation_id`
- [ ] Model `AuditEvent` — no updates/deletes in application code
- [ ] Package `internal/audit/` with `Record(ctx, event)` function

## Definition of Done

- [ ] Migration up/down
- [ ] Unit test: Record creates row, no update helper exposed
- [ ] `go test ./internal/audit/...` passes

## Out of scope

- Middleware, UI
