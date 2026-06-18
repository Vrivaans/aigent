# Slice 13 — Approval ↔ audit link

**Phase:** 3-approvals  
**Depends on:** `12-approval-policies-basic`

## Goal

Full traceability from chat approval to audit row.

## Tasks

- [ ] Audit UI: link from approval events to session/chat message id if available
- [ ] Approvals tab shows `resolved_by` username
- [ ] Export includes approval resolution fields

## Definition of Done

- [ ] End-to-end: approve in chat → audit row → visible in Audit tab
- [ ] Phase 3 complete → advance to `14-separate-jwt-encryption-keys`
