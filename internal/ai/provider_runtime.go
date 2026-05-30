package ai

import (
	"context"
	"fmt"
	"log"

	"aigent/internal/database"
	"aigent/internal/utils"
)

func modelForActiveProvider(session *database.Session, provider database.LLMProvider) string {
	model := provider.DefaultModel
	if session.LLMModelOverride != "" {
		model = session.LLMModelOverride
	}
	if model == "" {
		model = DefaultModel
	}
	return model
}

func modelForFallbackProvider(provider database.LLMProvider) string {
	if provider.DefaultModel != "" {
		return provider.DefaultModel
	}
	return DefaultModel
}

func (b *Brain) createChatCompletionWithFallback(
	ctx context.Context,
	req ChatCompletionRequest,
	session *database.Session,
	providerCandidates []database.LLMProvider,
	activeProviderIdx *int,
	masterKey string,
) (*ChatCompletionResponse, *ProviderSwitchInfo, error) {
	activeProvider := providerCandidates[*activeProviderIdx]
	activeModel := modelForActiveProvider(session, activeProvider)
	req.Model = activeModel

	apiKey, decErr := utils.Decrypt(activeProvider.APIKey, masterKey)
	if decErr != nil {
		return nil, nil, fmt.Errorf("error al descifrar la API Key del proveedor '%s': %w", activeProvider.Name, decErr)
	}
	log.Printf("🔑 Using provider '%s' keyLen=%d baseURL=%s model=%s", activeProvider.Name, len(apiKey), activeProvider.BaseURL, activeModel)

	llmClient := NewClient(apiKey, activeProvider.BaseURL)
	resp, err := llmClient.CreateChatCompletion(ctx, req)
	if err == nil {
		return resp, nil, nil
	}

	log.Printf("❌ LLM API Error (%s): %v", activeProvider.Name, err)
	if !isRecoverableProviderError(err) {
		return nil, nil, fmt.Errorf("llm inference failed: %w", err)
	}

	fromProvider := activeProvider.Name
	fromModel := activeModel
	var lastErr error = err

	for nextIdx := *activeProviderIdx + 1; nextIdx < len(providerCandidates); nextIdx++ {
		nextProvider := providerCandidates[nextIdx]
		nextModel := modelForFallbackProvider(nextProvider)

		nextKey, decErr2 := utils.Decrypt(nextProvider.APIKey, masterKey)
		if decErr2 != nil {
			lastErr = fmt.Errorf("error al descifrar la API Key del proveedor '%s': %w", nextProvider.Name, decErr2)
			log.Printf("❌ %v", lastErr)
			continue
		}

		nextClient := NewClient(nextKey, nextProvider.BaseURL)
		req.Model = nextModel
		resp, err = nextClient.CreateChatCompletion(ctx, req)
		if err != nil {
			lastErr = err
			log.Printf("❌ LLM API Error en fallback (%s): %v", nextProvider.Name, err)
			if !isRecoverableProviderError(err) {
				return nil, nil, fmt.Errorf("llm inference failed tras fallback (%s): %w", nextProvider.Name, err)
			}
			continue
		}

		*activeProviderIdx = nextIdx
		session.LLMProviderOverrideID = &nextProvider.ID
		session.LLMModelOverride = ""
		_ = database.DB.Model(&database.Session{}).Where("id = ?", session.ID).Updates(map[string]interface{}{
			"llm_provider_override_id": nextProvider.ID,
			"llm_model_override":       "",
		}).Error

		switchNotice := &ProviderSwitchInfo{
			Reason:       "provider_fallback",
			FromProvider: fromProvider,
			FromModel:    fromModel,
			ToProvider:   nextProvider.Name,
			ToModel:      nextModel,
		}
		log.Printf("🔁 Fallback automático aplicado: %s/%s -> %s/%s", fromProvider, fromModel, nextProvider.Name, nextModel)
		return resp, switchNotice, nil
	}

	return nil, nil, fmt.Errorf("llm inference failed tras fallback: %w", lastErr)
}
