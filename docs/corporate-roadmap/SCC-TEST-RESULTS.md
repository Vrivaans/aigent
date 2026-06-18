# SCC test results (slice 16)

**Date:** 2026-06-18  
**Scope:** Layer 2 determinism + usage `cached_tokens` parsing (mock client, no live API)

## Automated tests

Run:

```bash
go test ./internal/ai/ -run 'Layer2|TokenUsage|CreateChatCompletionParsesUsage' -v
```

| Test | Result | Notes |
|------|--------|-------|
| `TestLayer2HashDeterministicForSameInputs` | PASS | Same goals + session files + workspace → identical SHA-256 |
| `TestLayer2HashStableRegardlessOfFileOrder` | PASS | Files sorted alphabetically before hash |
| `TestLayer2HashChangesWhenGoalsChange` | PASS | Goal edit invalidates Layer 2 hash |
| `TestLayer2HashChangesWhenSessionFileChanges` | PASS | File content edit invalidates hash |
| `TestLayer2HashChangesWhenWorkspaceChanges` | PASS | Workspace snippet change invalidates hash |
| `TestBuildRuntimeMessagesWithCacheIncludesLayer2` | PASS | Layer 2 injected as second system message |
| `TestTokenUsageCachedTokensOpenAIFormat` | PASS | `prompt_tokens_details.cached_tokens` |
| `TestTokenUsageCachedTokensAnthropicFormat` | PASS | `cache_read_input_tokens` fallback |
| `TestCreateChatCompletionParsesUsageFromMock` | PASS | Mock HTTP server; no live Claude/OpenRouter |

Full package gate: `go test ./internal/ai/...` — PASS (2026-06-18).

## Layer 2 hash behavior

- Hash input: concatenation of session goals, sorted session files, and pre-scanned workspace text (`buildLayer2Content`).
- Function: `Layer2SHA256(session, files, workspaceContent)` in `internal/ai/history_builder.go`.
- Runtime logs first 8 hex chars: `SmartContextCache: Layer 2 SHA-256 Hash = …`.

## Token usage / cache metrics

- `CreateChatCompletion` parses `usage` from provider JSON.
- `LogTokenUsage` emits `cached_tokens` when `prompt_tokens_details.cached_tokens` or `cache_read_input_tokens` is present.
- Streaming completions do not yet parse final-chunk usage (future work).

## Manual live-provider test (optional)

Not required for slice 16 DoD. To validate real cache hits with Claude via OpenRouter:

1. Configure an Anthropic model on a provider with SCC Layer 2 filled (>1024 tokens).
2. Send two consecutive messages without changing goals/files/workspace.
3. Inspect server logs for stable Layer 2 hash prefix and non-zero `cached_tokens` on the second call.

## Out of scope (slice 17+)

- Gemini `CachedContent` adapter
- Automated cost/latency benchmarks across providers
