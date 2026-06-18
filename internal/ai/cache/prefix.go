package cache

import "context"

// PrefixStableAdapter relies on deterministic Layer 1+2 message ordering for implicit prefix caching
// (OpenAI, DeepSeek, Groq, and other OpenAI-compatible providers). No extra API fields are required;
// providers cache identical byte prefixes automatically when Layer 2 hash is stable.
type PrefixStableAdapter struct{}

func (PrefixStableAdapter) Family() Family { return FamilyPrefixStable }

func (PrefixStableAdapter) Prepare(_ context.Context, in PrepareInput) (PrepareOutput, error) {
	if in.Layer2Content == "" {
		return emptyLayer2Output(in.Layer2Hash), nil
	}
	return PrepareOutput{
		Plan: baseLayer2Plan(in.Layer2Content, in.Layer2Hash, "prefix_stable", true, ""),
		SessionUpdates: SessionUpdates{
			Layer2Hash: in.Layer2Hash,
			ClearCache: true,
		},
	}, nil
}
