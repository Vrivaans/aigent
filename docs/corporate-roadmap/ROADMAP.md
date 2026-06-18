# Aigent Corporate Roadmap

Machine-driven iteration loop for enterprise features. The agent reads `STATE.json`, implements **one slice**, runs gates, updates state, and commits.

## Phases

| Phase | Goal | Slices |
|-------|------|--------|
| **1-rbac** | Users, roles, protected routes | `01`–`06` |
| **2-audit** | Append-only audit trail | `07`–`10` |
| **3-approvals** | Enterprise approval governance | `11`–`13` |
| **4-secrets** | Secret lifecycle (split keys, rotation) | `14`–`15` |
| **5-scc** | Smart Context Cache production-ready | `16`–`17` |
| **6-mcp-catalog** | Installable MCP templates | `18`–`19` |
| **7-multi-tenant** | Tenant isolation | `20`–`21` |

## How to run one iteration

In Cursor Agent:

```text
Follow .cursor/skills/corporate-iteration/SKILL.md — one iteration only.
```

## How to run a self-paced loop

```text
/loop 1h Follow .cursor/skills/corporate-iteration/SKILL.md — one iteration per wake.
```

Or enable `.cursor/hooks.json` (`stop` hook) to chain iterations in the same session.

## Gates (must pass before marking slice complete)

1. `go test ./...`
2. `go vet ./...`
3. `cd web && npm run build` — only if the slice touched `web/`

## Rules

- **One slice per iteration.** Do not skip ahead.
- **Do not start phase 7** until phases 1–2 are in `completed`.
- **Commit message format:** `feat(<phase-short>): <slice-id> <brief description>`
- **On blocker:** set `status` to `blocked`, append to `blockers`, stop.

## Current status

See [`STATE.json`](STATE.json).
