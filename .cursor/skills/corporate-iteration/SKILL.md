---
name: corporate-iteration
description: >-
  Run one iteration of the Aigent corporate roadmap. Read STATE.json,
  implement the current slice, run gates, update state, commit.
  Use when the user says corporate iteration, roadmap loop, /loop corporate,
  or wants RBAC/audit/multi-tenant work to proceed automatically.
---

# Corporate iteration (one slice)

Execute **exactly one slice** per invocation.

## Step 1 — Load state

Read:

- `docs/corporate-roadmap/STATE.json`
- `docs/corporate-roadmap/slices/<current_slice>.md`
- `docs/corporate-roadmap/ROADMAP.md` if phase boundaries are unclear

If `status` is `blocked`, read `blockers` first. Attempt to unblock only within the current slice scope; otherwise report and stop.

If `phase` is `done`, congratulate and stop.

## Step 2 — Implement

- Follow the slice **Tasks** and **Definition of Done** checklists.
- Respect **Out of scope** — do not expand.
- Reuse existing code style in `internal/`, `web/src/app/`, migrations under `internal/database/migrations/`.

## Step 3 — Gates

From `STATE.json` → `gates`:

```bash
go test ./...
go vet ./...
```

If any file under `web/` changed:

```bash
cd web && npm run build
```

All must pass before marking complete.

## Step 4 — Update STATE.json

On success:

```json
{
  "status": "pending",
  "last_iteration_at": "<ISO8601 UTC>",
  "last_commit": "<short hash if committed>",
  "completed": ["...", "<finished-slice-id>"],
  "current_slice": "<next-slice-id>",
  "phase": "<next-phase-if-slice-doc-says-phase-complete>"
}
```

Slice → next slice mapping:

| After slice | Next current_slice | Phase change |
|-------------|-------------------|--------------|
| 06-auth-ui-minimal | 07-audit-events-table | → 2-audit |
| 10-audit-export-endpoint | 11-approval-actor-id | → 3-approvals |
| 13-approval-audit-link | 14-separate-jwt-encryption-keys | → 4-secrets |
| 15-secret-rotation-doc-env | 16-scc-e2e-cache-test | → 5-scc |
| 17-scc-provider-adapters | 18-mcp-manifest-format | → 6-mcp-catalog |
| 19-mcp-catalog-ui | 20-tenant-model-migration | → 7-multi-tenant |
| 21-tenant-scoped-queries | null | → done |

On failure:

```json
{
  "status": "blocked",
  "blockers": ["<slice-id>: <reason>"]
}
```

## Step 5 — Commit

Only when user rules allow commits and gates passed:

```
feat(rbac): 01-users-table add users migration and model
```

Use prefix by phase: `rbac`, `audit`, `approvals`, `secrets`, `scc`, `mcp-catalog`, `multi-tenant`.

## Step 6 — Report

Reply with:

1. **Slice completed:** id + summary
2. **Gates:** pass/fail
3. **Next:** slice id + title
4. **Blockers:** if any

Do not start the next slice in the same turn unless the user explicitly asks for multiple iterations.

## Loop usage

**Manual loop:**

```text
/loop 1h Follow corporate-iteration skill — one slice per wake.
```

**Hook:** `.cursor/hooks/roadmap-continue.sh` chains on agent `stop` when phase ≠ done.
