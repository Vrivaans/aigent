# Slice 19 — MCP catalog UI

**Phase:** 6-mcp-catalog  
**Depends on:** `18-mcp-manifest-format`

## Goal

Self-service MCP installation from UI.

## Tasks

- [ ] Angular tab or section: **MCP Catalog** — list bundled templates
- [ ] One-click Install → calls install API → refreshes tools
- [ ] Show install status / last sync error
- [ ] i18n EN/ES
- [ ] RBAC: `mcp:write` required

## Definition of Done

- [ ] Install Playwright or filesystem template from UI end-to-end
- [ ] `npm run build` passes
- [ ] Phase 6 complete → `20-tenant-model-migration`
