package cache

import (
	"context"
	"strings"
	"time"
)

// Family groups providers that share the same SCC strategy.
type Family string

const (
	FamilyAnthropic    Family = "anthropic"
	FamilyPrefixStable Family = "prefix_stable"
	FamilyGemini       Family = "gemini"
	FamilyNone         Family = "none"
)

// SessionState holds persisted SCC metadata for a chat session.
type SessionState struct {
	ID              uint
	Layer2Hash      string
	ProviderCacheID string
	CacheExpiresAt  *time.Time
}

// PrepareInput describes Layer 2 and provider context for an adapter.
type PrepareInput struct {
	ProviderName  string
	ProviderType  string
	BaseURL       string
	Model         string
	APIKey        string
	Layer2Content string
	Layer2Hash    string
	Session       SessionState
}

// Layer2Plan tells the message builder how to apply Layer 2 for this request.
type Layer2Plan struct {
	IncludeInMessages bool
	MessageContent    string
	CacheControlType  string
	CachedContentName string
	Strategy          string
	Layer2Hash        string
}

// SessionUpdates persists SCC state when the adapter creates or invalidates cache.
type SessionUpdates struct {
	Layer2Hash      string
	ProviderCacheID string
	CacheExpiresAt  *time.Time
	ClearCache      bool
}

// PrepareOutput is the adapter result for one LLM iteration.
type PrepareOutput struct {
	Plan           Layer2Plan
	SessionUpdates SessionUpdates
}

// Adapter applies provider-specific SCC behavior for Layer 2.
type Adapter interface {
	Family() Family
	Prepare(ctx context.Context, in PrepareInput) (PrepareOutput, error)
}

// ResolveFamily maps provider name/type to an SCC adapter family.
func ResolveFamily(providerName, providerType string) Family {
	n := strings.ToLower(providerName)
	t := strings.ToLower(providerType)
	switch {
	case strings.Contains(n, "anthropic") || strings.Contains(n, "claude") || t == "anthropic":
		return FamilyAnthropic
	case strings.Contains(n, "gemini") || strings.Contains(t, "gemini") || strings.Contains(t, "google"):
		return FamilyGemini
	case strings.Contains(n, "deepseek") || t == "deepseek",
		strings.Contains(n, "openai") || t == "openai",
		strings.Contains(n, "groq") || t == "groq",
		strings.Contains(n, "zen") || t == "zen":
		return FamilyPrefixStable
	default:
		return FamilyPrefixStable
	}
}

// ForProvider returns the adapter for a provider family.
func ForProvider(providerName, providerType string) Adapter {
	switch ResolveFamily(providerName, providerType) {
	case FamilyAnthropic:
		return AnthropicAdapter{}
	case FamilyGemini:
		return GeminiAdapter{}
	default:
		return PrefixStableAdapter{}
	}
}

// Prepare runs the adapter for the given provider.
func Prepare(ctx context.Context, in PrepareInput) (PrepareOutput, error) {
	adapter := ForProvider(in.ProviderName, in.ProviderType)
	return adapter.Prepare(ctx, in)
}

func emptyLayer2Output(hash string) PrepareOutput {
	return PrepareOutput{
		Plan: Layer2Plan{Layer2Hash: hash, Strategy: string(FamilyNone)},
		SessionUpdates: SessionUpdates{
			Layer2Hash: hash,
			ClearCache: true,
		},
	}
}

func baseLayer2Plan(content, hash, strategy string, include bool, cacheControl string) Layer2Plan {
	return Layer2Plan{
		IncludeInMessages: include,
		MessageContent:    content,
		CacheControlType:  cacheControl,
		Strategy:          strategy,
		Layer2Hash:        hash,
	}
}

func cacheStillValid(session SessionState, layer2Hash string, now time.Time) bool {
	if session.ProviderCacheID == "" || session.Layer2Hash != layer2Hash {
		return false
	}
	if session.CacheExpiresAt != nil && !session.CacheExpiresAt.After(now) {
		return false
	}
	return true
}
