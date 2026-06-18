# Slice 18 — MCP manifest format

**Phase:** 6-mcp-catalog  
**Depends on:** `17-scc-provider-adapters`

## Goal

Standard manifest for installable MCP templates.

## Tasks

- [ ] JSON schema `docs/corporate-roadmap/mcp-manifest.schema.json`
- [ ] Example manifests under `catalog/mcp/` (filesystem stdio, playwright stream)
- [ ] Backend: `POST /api/catalog/mcp/install` reads manifest → creates stdio or stream entry
- [ ] Validate manifest before install

## Definition of Done

- [ ] Can install filesystem MCP from catalog manifest via API
- [ ] Tests for manifest validation

## Out of scope

- UI gallery (next slice)
