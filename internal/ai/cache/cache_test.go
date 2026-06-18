package cache

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestResolveFamily(t *testing.T) {
	tests := []struct {
		name, typ string
		want      Family
	}{
		{"Claude Opus", "anthropic", FamilyAnthropic},
		{"My DeepSeek", "deepseek", FamilyPrefixStable},
		{"Gemini Pro", "gemini", FamilyGemini},
		{"OpenRouter", "openrouter", FamilyPrefixStable},
	}
	for _, tt := range tests {
		if got := ResolveFamily(tt.name, tt.typ); got != tt.want {
			t.Fatalf("ResolveFamily(%q,%q) = %q, want %q", tt.name, tt.typ, got, tt.want)
		}
	}
}

func TestAnthropicAdapterSetsEphemeral(t *testing.T) {
	out, err := AnthropicAdapter{}.Prepare(context.Background(), PrepareInput{
		Layer2Content: "goals and files",
		Layer2Hash:    "abc123",
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if out.Plan.CacheControlType != "ephemeral" {
		t.Fatalf("CacheControlType = %q", out.Plan.CacheControlType)
	}
	if out.Plan.Strategy != "anthropic_ephemeral" {
		t.Fatalf("strategy = %q", out.Plan.Strategy)
	}
}

func TestPrefixStableAdapterIncludesLayer2(t *testing.T) {
	out, err := PrefixStableAdapter{}.Prepare(context.Background(), PrepareInput{
		Layer2Content: "stable prefix",
		Layer2Hash:    "hash1",
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !out.Plan.IncludeInMessages || out.Plan.CacheControlType != "" {
		t.Fatalf("unexpected plan: %+v", out.Plan)
	}
	if out.Plan.Strategy != "prefix_stable" {
		t.Fatalf("strategy = %q", out.Plan.Strategy)
	}
}

func TestGeminiAdapterReusesValidCache(t *testing.T) {
	expires := time.Now().UTC().Add(time.Hour)
	out, err := GeminiAdapter{}.Prepare(context.Background(), PrepareInput{
		Layer2Content: strings.Repeat("x", geminiMinLayer2Bytes),
		Layer2Hash:    "same-hash",
		Session: SessionState{
			Layer2Hash:      "same-hash",
			ProviderCacheID: "cachedContents/abc",
			CacheExpiresAt:  &expires,
		},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if out.Plan.Strategy != "gemini_reuse" {
		t.Fatalf("strategy = %q", out.Plan.Strategy)
	}
	if out.Plan.CachedContentName != "cachedContents/abc" {
		t.Fatalf("cached content = %q", out.Plan.CachedContentName)
	}
	if out.Plan.IncludeInMessages {
		t.Fatal("expected layer2 omitted when cache reused")
	}
}

func TestGeminiAdapterCreatesCachedContent(t *testing.T) {
	restore := SetCachedContentCreatorForTest(CachedContentCreatorFunc(func(ctx context.Context, apiKey, baseURL, model, content, ttl string) (string, time.Time, error) {
		return "cachedContents/new", time.Now().UTC().Add(30 * time.Minute), nil
	}))
	defer restore()

	out, err := GeminiAdapter{}.Prepare(context.Background(), PrepareInput{
		ProviderName:  "Gemini",
		ProviderType:  "gemini",
		APIKey:        "key",
		Model:         "gemini-1.5-pro",
		Layer2Content: strings.Repeat("a", geminiMinLayer2Bytes),
		Layer2Hash:    "layer-hash",
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if out.Plan.Strategy != "gemini_cached_content" {
		t.Fatalf("strategy = %q", out.Plan.Strategy)
	}
	if out.SessionUpdates.ProviderCacheID != "cachedContents/new" {
		t.Fatalf("provider_cache_id = %q", out.SessionUpdates.ProviderCacheID)
	}
}

type CachedContentCreatorFunc func(ctx context.Context, apiKey, baseURL, model, content, ttl string) (string, time.Time, error)

func (f CachedContentCreatorFunc) Create(ctx context.Context, apiKey, baseURL, model, content, ttl string) (string, time.Time, error) {
	return f(ctx, apiKey, baseURL, model, content, ttl)
}
