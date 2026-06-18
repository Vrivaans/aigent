# Slice 12 — Basic approval policies

**Phase:** 3-approvals  
**Depends on:** `11-approval-actor-id`

## Goal

Configurable rules: which tools require approval beyond `sensitive` flag.

## Tasks

- [ ] Table `approval_policies` (`tool_pattern`, `environment`, `requires_approval`, `min_role`)
- [ ] Default policy: keep existing `sensitive` registry behavior
- [ ] Admin UI section or extend Permissions tab to view policies
- [ ] `findSensitiveToolCalls` respects policies + registry

## Definition of Done

- [ ] Policy can force approval for tool matching `odoo_*` pattern
- [ ] Operator with `always_allow` permission still bypasses (existing behavior)
- [ ] `go test ./internal/ai/...` for sensitivity logic

## Out of scope

- Multi-step approval chains (manager → security)
