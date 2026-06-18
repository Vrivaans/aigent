# Slice 04 — RBAC middleware

**Phase:** 1-rbac  
**Depends on:** `03-auth-jwt-claims`

## Goal

Reusable middleware to protect routes by permission.

## Tasks

- [ ] `auth.RequirePermissionMiddleware(resource, action)` Fiber middleware
- [ ] Return `403` with JSON error when denied
- [ ] Log denied attempts at warn level (prep for audit phase)

## Definition of Done

- [ ] Middleware unit tests
- [ ] Applied to at least one test route in isolation
- [ ] `go test ./internal/auth/...` passes

## Out of scope

- Applying to all routes (next slice)
