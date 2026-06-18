# Slice 11 — Approval actor ID

**Phase:** 3-approvals  
**Depends on:** `10-audit-export-endpoint`

## Goal

Record **who** approved/rejected each sensitive action.

## Tasks

- [ ] Add `resolved_by_user_id`, `resolved_at` to `pending_actions` (migration)
- [ ] `HandleConfirm` sets actor from JWT `user_id`
- [ ] Audit event includes approver id (should already flow from slice 08 — verify)

## Definition of Done

- [ ] Approved action persists resolver user id
- [ ] History API exposes resolver when present
- [ ] Tests for confirm with mocked user context

## Out of scope

- Multi-level approval policies
