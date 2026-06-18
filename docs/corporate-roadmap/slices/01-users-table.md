# Slice 01 — Users table

**Phase:** 1-rbac  
**Depends on:** —

## Goal

Replace single admin env login foundation with a `users` table while keeping backward-compatible login.

## Tasks

- [ ] Migration: `users` (`id`, `username` unique, `password_hash`, `is_active`, timestamps)
- [ ] GORM model `User` in `internal/database/models.go`
- [ ] Seed migration or startup seed: migrate existing `ADMIN_USERNAME` / `ADMIN_PASSWORD` into first user if table empty
- [ ] Password hashing with bcrypt (or argon2 if already used elsewhere)

## Definition of Done

- [ ] Migration up/down works
- [ ] `go test ./internal/database/...` passes (add test if none)
- [ ] Existing login still works via seeded admin user
- [ ] No breaking change to JWT response shape yet

## Out of scope

- Roles, RBAC middleware, UI user management
