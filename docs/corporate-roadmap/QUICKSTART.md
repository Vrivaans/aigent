# Corporate roadmap — Quick start

## 1. Enable the stop hook (optional)

Hooks are configured in `.cursor/hooks.json`. After pull, restart Cursor or reload hooks in **Settings → Hooks**.

Make the script executable (once):

```bash
chmod +x .cursor/hooks/roadmap-continue.sh
```

## 2. Run one iteration

In **Cursor Agent**:

```text
Follow .cursor/skills/corporate-iteration/SKILL.md — one iteration only.
```

## 3. Run a timed loop

```text
/loop 1h Follow corporate-iteration skill — one slice per wake.
```

Dynamic pacing (agent picks delay):

```text
/loop Follow corporate-iteration skill — one slice per wake.
```

## 4. Check progress

```bash
cat docs/corporate-roadmap/STATE.json
```

## 5. Stop the loop

- Say: **"stop corporate loop"** — agent should kill any background `/loop` shell.
- Or disable hook temporarily by renaming `.cursor/hooks.json`.

## Files

| Path | Purpose |
|------|---------|
| `STATE.json` | Current phase, slice, blockers, completed |
| `slices/*.md` | Task specs + Definition of Done |
| `.cursor/skills/corporate-iteration/SKILL.md` | Agent protocol |
| `.cursor/rules/corporate-roadmap.mdc` | Persistent guidance |
| `.cursor/hooks/roadmap-continue.sh` | Auto follow-up on agent stop |

## First slice

Start at **`01-users-table`** — see `slices/01-users-table.md`.
