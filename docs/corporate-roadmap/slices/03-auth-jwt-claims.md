# Slice 03 — JWT claims & user context

**Phase:** 1-rbac  
**Depends on:** `02-roles-permissions-model`

## Goal

Embed `user_id` and `roles` in JWT; expose user in request context.

## Tasks

- [ ] Extend `auth.Claims` with `UserID uint` and `Roles []string`
- [ ] Update `HandleLogin` to authenticate against `users` table (fallback env admin only if no users — remove in later slice)
- [ ] Store user in Fiber locals: `c.Locals("user_id")`, `c.Locals("roles")`
- [ ] Add `auth.GetUserID(c)`, `auth.RequirePermission(c, resource, action)`

## Definition of Done

- [ ] Login returns token with user_id
- [ ] Tests for token generation/parsing with new claims
- [ ] Existing frontend login flow still works

## Out of scope

- Route-level enforcement (next slice)
