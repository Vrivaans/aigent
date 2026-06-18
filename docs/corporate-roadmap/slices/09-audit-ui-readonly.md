# Slice 09 — Audit UI (read-only)

**Phase:** 2-audit  
**Depends on:** `08-audit-middleware`

## Goal

Auditors can search and view audit trail.

## Tasks

- [ ] `GET /api/audit/events` with filters: date range, actor, action, resource_type
- [ ] Pagination (limit/offset)
- [ ] Angular tab **Audit** — table with filters (auditor + admin roles)
- [ ] i18n EN/ES

## Definition of Done

- [ ] Viewer role cannot access audit API
- [ ] `npm run build` passes
- [ ] `go test ./...` passes

## Out of scope

- Export CSV (next slice)
