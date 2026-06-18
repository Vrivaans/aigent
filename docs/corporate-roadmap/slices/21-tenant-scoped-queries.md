# Slice 21 — Tenant-scoped queries

**Phase:** 7-multi-tenant  
**Depends on:** `20-tenant-model-migration`

## Goal

All data access filtered by tenant from JWT.

## Tasks

- [ ] Middleware sets `tenant_id` in context from JWT
- [ ] GORM scopes or helper `ForTenant(db, tenantID)` on all list/get handlers
- [ ] Prevent cross-tenant session/chat access (403)
- [ ] Admin can manage tenants (minimal CRUD) — admin role only

## Definition of Done

- [ ] Test: user tenant A cannot read session tenant B
- [ ] `go test ./...` green
- [ ] Set `phase` to `done`, `status` to `completed`, `current_slice` to null

## Roadmap complete

Update README corporate section when this slice merges.
