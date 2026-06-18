# Slice 17 — SCC multi-provider adapters

**Phase:** 5-scc  
**Depends on:** `16-scc-e2e-cache-test`

## Goal

Extend beyond Anthropic `cache_control` ephemeral.

## Tasks

- [ ] Adapter interface in `internal/ai/cache/` per provider family
- [ ] Implement or stub: OpenAI/DeepSeek prefix stability (document behavior)
- [ ] Gemini: `CachedContent` create/reuse when Layer 2 hash unchanged (store id on session)
- [ ] Session fields: `layer2_hash`, `provider_cache_id`, `cache_expires_at`

## Definition of Done

- [ ] At least Anthropic + one more provider path implemented or explicitly deferred with issue note in STATE
- [ ] Phase 5 complete → `18-mcp-manifest-format`
