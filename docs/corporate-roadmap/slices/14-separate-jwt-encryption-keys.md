# Slice 14 — Separate JWT and encryption keys

**Phase:** 4-secrets  
**Depends on:** `13-approval-audit-link`

## Goal

Stop using `DB_ENCRYPTION_KEY` for both AES and JWT signing.

## Tasks

- [ ] New env: `JWT_SECRET` (required)
- [ ] Keep `DB_ENCRYPTION_KEY` for AES-256-GCM only
- [ ] Migration guide in `docs/corporate-roadmap/SECRETS.md`
- [ ] Update `.env.example`
- [ ] Startup validation for both keys

## Definition of Done

- [ ] JWT uses `JWT_SECRET`
- [ ] Existing encrypted DB fields still decrypt with `DB_ENCRYPTION_KEY`
- [ ] Tests updated
- [ ] README mentions both vars

## Out of scope

- External vault
