#!/usr/bin/env bash
# Chains corporate roadmap iterations on agent stop.
# Requires: jq (optional — falls back to grep if missing)
set -euo pipefail

STATE_FILE="docs/corporate-roadmap/STATE.json"
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

if [[ ! -f "$STATE_FILE" ]]; then
  exit 0
fi

read_phase() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.phase // "unknown"' "$STATE_FILE"
  else
    grep -o '"phase"[[:space:]]*:[[:space:]]*"[^"]*"' "$STATE_FILE" | head -1 | sed 's/.*"\([^"]*\)"$/\1/'
  fi
}

read_status() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.status // "unknown"' "$STATE_FILE"
  else
    grep -o '"status"[[:space:]]*:[[:space:]]*"[^"]*"' "$STATE_FILE" | head -1 | sed 's/.*"\([^"]*\)"$/\1/'
  fi
}

read_slice() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.current_slice // "none"' "$STATE_FILE"
  else
    grep -o '"current_slice"[[:space:]]*:[[:space:]]*"[^"]*"' "$STATE_FILE" | head -1 | sed 's/.*"\([^"]*\)"$/\1/'
  fi
}

PHASE="$(read_phase)"
STATUS="$(read_status)"
SLICE="$(read_slice)"

if [[ "$PHASE" == "done" ]]; then
  exit 0
fi

if [[ "$STATUS" == "blocked" ]]; then
  cat <<EOF
{
  "followup_message": "Corporate roadmap is BLOCKED. Read docs/corporate-roadmap/STATE.json blockers, try to resolve within slice ${SLICE}, or ask the user. Follow .cursor/skills/corporate-iteration/SKILL.md — one iteration."
}
EOF
  exit 0
fi

cat <<EOF
{
  "followup_message": "Continue corporate roadmap iteration. Phase: ${PHASE}, slice: ${SLICE}. Follow .cursor/skills/corporate-iteration/SKILL.md — implement ONE slice, run gates, update STATE.json, commit if green. Do not ask for confirmation."
}
EOF
