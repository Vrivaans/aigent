# Slice 15 — Secret rotation documentation & env adapter

**Phase:** 4-secrets  
**Depends on:** `14-separate-jwt-encryption-keys`

## Goal

Document rotation; optional read-from-env pattern for K8s/Docker secrets.

## Tasks

- [ ] `internal/secrets/` adapter interface: `GetSecret(key string) string`
- [ ] Default impl: env vars; stub `VaultProvider` interface (no full impl required)
- [ ] `docs/corporate-roadmap/SECRETS.md`: rotation runbook for JWT + DB key
- [ ] Provider/MCP handlers use adapter for master key lookup

## Definition of Done

- [ ] Runbook reviewed and accurate
- [ ] No regression in encrypt/decrypt tests
- [ ] Phase 4 complete → `16-scc-e2e-cache-test`
