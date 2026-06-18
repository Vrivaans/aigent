package ai

import "log"

// TokenUsage mirrors OpenAI-compatible usage blocks (OpenRouter, Anthropic, etc.).
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// CachedTokens returns provider-reported cache hits when present.
func (u *TokenUsage) CachedTokens() int {
	if u == nil {
		return 0
	}
	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
		return u.PromptTokensDetails.CachedTokens
	}
	return u.CacheReadInputTokens
}

// LogTokenUsage logs prompt/completion totals and cached_tokens when the API reports them.
func LogTokenUsage(model string, usage *TokenUsage) {
	if usage == nil {
		return
	}
	cached := usage.CachedTokens()
	if cached > 0 {
		log.Printf("📊 LLM usage model=%s prompt=%d completion=%d total=%d cached_tokens=%d",
			model, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, cached)
		return
	}
	log.Printf("📊 LLM usage model=%s prompt=%d completion=%d total=%d",
		model, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
}
