# Slice 06 — Minimal users UI (admin)

**Phase:** 1-rbac  
**Depends on:** `05-protect-admin-routes`

## Goal

Admin tab to list users and assign roles (minimal corporate UX).

## Tasks

- [ ] Backend: `GET/POST/PATCH /api/admin/users`, `PATCH /api/admin/users/:id/roles`
- [ ] Angular: tab or section under admin for Users (list, create, assign role)
- [ ] i18n keys EN/ES in `translation.service.ts`
- [ ] Only `admin` role can access

## Definition of Done

- [ ] Can create user and assign `operator` role
- [ ] `cd web && npm run build` passes
- [ ] `go test ./...` passes

## Phase complete

When this slice is done, set `phase` to `2-audit` and `current_slice` to `07-audit-events-table`.
