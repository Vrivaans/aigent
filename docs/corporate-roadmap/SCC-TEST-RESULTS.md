# SCC test results (slices 16–17)

**Date:** 2026-06-18  
**Scope:** Layer 2 determinism, usage `cached_tokens` parsing, multi-provider SCC adapters

## Automated tests

Run:

```bash
go test ./internal/ai/ -run 'Layer2|TokenUsage|CreateChatCompletionParsesUsage|BuildRuntime' -v
go test ./internal/ai/cache/ -v
```

| Test | Result | Notes |
|------|--------|-------|
| `TestLayer2HashDeterministicForSameInputs` | PASS | Same goals + session files + workspace → identical SHA-256 |
| `TestLayer2HashStableRegardlessOfFileOrder` | PASS | Files sorted alphabetically before hash |
| `TestLayer2HashChangesWhenGoalsChange` | PASS | Goal edit invalidates Layer 2 hash |
| `TestLayer2HashChangesWhenSessionFileChanges` | PASS | File content edit invalidates hash |
| `TestLayer2HashChangesWhenWorkspaceChanges` | PASS | Workspace snippet change invalidates hash |
| `TestBuildRuntimeMessagesWithCacheIncludesLayer2` | PASS | Layer 2 injected as second system message via adapter plan |
| `TestTokenUsageCachedTokensOpenAIFormat` | PASS | `prompt_tokens_details.cached_tokens` |
| `TestTokenUsageCachedTokensAnthropicFormat` | PASS | `cache_read_input_tokens` fallback |
| `TestCreateChatCompletionParsesUsageFromMock` | PASS | Mock HTTP server; no live Claude/OpenRouter |
| `TestResolveFamilyAnthropic` / `PrefixStable` / `Gemini` | PASS | Provider → SCC family mapping |
| `TestAnthropicAdapterSetsCacheControl` | PASS | `cache_control: ephemeral` on Layer 2 |
| `TestPrefixStableAdapterIncludesLayer2` | PASS | Stable prefix; no extra provider fields |
| `TestGeminiAdapterReusesCachedContent` | PASS | Session `provider_cache_id` reuse when hash matches |
| `TestGeminiAdapterCreatesCachedContent` | PASS | Mock creator; persists id + expiry on session |

Full package gate: `go test ./internal/ai/... ./internal/ai/cache/...` — run after slice 17.

## Layer 2 hash behavior

- Hash input: concatenation of session goals, sorted session files, and pre-scanned workspace text (`buildLayer2Content`).
- Function: `Layer2SHA256(session, files, workspaceContent)` in `internal/ai/history_builder.go`.
- Persisted on session as `layer2_hash` when adapters run (`prepareSCC` in `internal/ai/scc_cache.go`).
- Runtime logs first 8 hex chars: `SmartContextCache: Layer 2 SHA-256 Hash = …`.

## Provider adapters (slice 17)

Package: `internal/ai/cache/`

| Family | Providers | Strategy |
|--------|-----------|----------|
| `anthropic` | Claude / Anthropic | Layer 2 system message with `cache_control: ephemeral` |
| `prefix_stable` | OpenAI, DeepSeek, Groq, Zen | Layer 2 inline; prefix stability only (no provider cache API) |
| `gemini` | Google Gemini | Reuse/create `CachedContent` when Layer 2 ≥ ~120KB; `cached_content` on request |

Session fields (`000011_add_session_scc_cache`): `layer2_hash`, `provider_cache_id`, `cache_expires_at`.

Gemini edge cases:
- Layer 2 below minimum size → `gemini_deferred_small` (inline message, no CachedContent).
- Create failure → `gemini_fallback_prefix` (inline, same as prefix_stable).

## Token usage / cache metrics

- `CreateChatCompletion` parses `usage` from provider JSON.
- `LogTokenUsage` emits `cached_tokens` when `prompt_tokens_details.cached_tokens` or `cache_read_input_tokens` is present.
- Streaming completions do not yet parse final-chunk usage (future work).

## Manual live-provider test (optional)

1. Configure an Anthropic model on a provider with SCC Layer 2 filled (>1024 tokens).
2. Send two consecutive messages without changing goals/files/workspace.
3. Inspect server logs for stable Layer 2 hash prefix, `strategy=anthropic`, and non-zero `cached_tokens` on the second call.

For Gemini CachedContent: use a session with large Layer 2 (>120KB) and inspect logs for `strategy=gemini_cached` and `cached_content=true`.

## Out of scope (slice 18+)

- Automated cost/latency benchmarks across providers
- MCP catalog manifest format (phase 6)
