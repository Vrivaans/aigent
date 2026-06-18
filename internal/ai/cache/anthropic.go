package cache

import "context"

// AnthropicAdapter marks Layer 2 with cache_control ephemeral (Claude via Anthropic/OpenRouter).
type AnthropicAdapter struct{}

func (AnthropicAdapter) Family() Family { return FamilyAnthropic }

func (AnthropicAdapter) Prepare(_ context.Context, in PrepareInput) (PrepareOutput, error) {
	if in.Layer2Content == "" {
		return emptyLayer2Output(in.Layer2Hash), nil
	}
	return PrepareOutput{
		Plan: baseLayer2Plan(in.Layer2Content, in.Layer2Hash, "anthropic_ephemeral", true, "ephemeral"),
		SessionUpdates: SessionUpdates{
			Layer2Hash: in.Layer2Hash,
			ClearCache: true,
		},
	}, nil
}
