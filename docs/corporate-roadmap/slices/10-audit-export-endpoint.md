# Slice 10 — Audit export

**Phase:** 2-audit  
**Depends on:** `09-audit-ui-readonly`

## Goal

Export audit events for compliance.

## Tasks

- [ ] `GET /api/audit/events/export?format=csv` (same filters as list)
- [ ] Rate limit or max rows (e.g. 10k) with clear error
- [ ] UI button "Export CSV" on Audit tab

## Definition of Done

- [ ] CSV download works in browser
- [ ] Export action itself creates an audit event
- [ ] Phase 2 complete → advance to `11-approval-actor-id`
