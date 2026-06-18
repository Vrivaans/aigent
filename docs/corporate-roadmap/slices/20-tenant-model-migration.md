# Slice 20 — Tenant model migration

**Phase:** 7-multi-tenant  
**Depends on:** `19-mcp-catalog-ui` **and** phases 1–2 complete

## Goal

Introduce `tenants` and scope core config.

## Tasks

- [ ] Migration: `tenants` table
- [ ] Add `tenant_id` to: `users`, `sessions`, `llm_providers`, `agents`, `hands_ai_configs` (nullable first)
- [ ] Backfill single default tenant `default` for existing rows
- [ ] JWT includes `tenant_id`

## Definition of Done

- [ ] Migration safe on existing DB
- [ ] Single-tenant behavior unchanged for default tenant
- [ ] Tests for tenant backfill

## Out of scope

- Query scoping (next slice)
