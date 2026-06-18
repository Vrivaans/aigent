# Slice 02 — Roles & permissions model

**Phase:** 1-rbac  
**Depends on:** `01-users-table`

## Goal

Add roles and a permission model that maps to HTTP routes / actions.

## Tasks

- [ ] Migration: `roles` (`id`, `name` unique, `description`)
- [ ] Migration: `role_permissions` (`role_id`, `resource`, `action`) — e.g. `agents`, `create`
- [ ] Migration: `user_roles` (`user_id`, `role_id`)
- [ ] Seed roles: `admin`, `operator`, `auditor`, `viewer`
- [ ] Seed default permissions per role (document matrix in migration comment)

## Definition of Done

- [ ] Models + associations in GORM
- [ ] Helper: `UserHasPermission(userID, resource, action) bool`
- [ ] Unit test for permission helper
- [ ] `go test ./...` green

## Out of scope

- JWT claims, middleware enforcement
