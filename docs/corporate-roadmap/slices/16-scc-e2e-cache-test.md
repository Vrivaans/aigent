# Slice 16 — SCC E2E cache validation

**Phase:** 5-scc  
**Depends on:** `15-secret-rotation-doc-env`

## Goal

Prove Layer 2 determinism and measure cache behavior.

## Tasks

- [ ] Test: same session goals + files → identical Layer 2 SHA-256 hash (`history_builder`)
- [ ] Test: changing goals invalidates hash
- [ ] Log `cached_tokens` from provider response when available (parse usage block)
- [ ] Document results in `docs/corporate-roadmap/SCC-TEST-RESULTS.md`

## Definition of Done

- [ ] Automated tests pass without live API (mock client) OR documented manual test with real Claude
- [ ] README SCC section updated with test status

## Out of scope

- Gemini CachedContent adapter (next slice)
