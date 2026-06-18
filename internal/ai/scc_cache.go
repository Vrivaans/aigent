package ai

import (
	"context"
	"log"

	"aigent/internal/ai/cache"
	"aigent/internal/database"
)


func applySessionCacheUpdates(session *database.Session, updates cache.SessionUpdates) {
	if session == nil || session.ID == 0 {
		return
	}

	fields := map[string]interface{}{}
	if updates.Layer2Hash != "" {
		fields["layer2_hash"] = updates.Layer2Hash
		session.Layer2Hash = updates.Layer2Hash
	}
	if updates.ClearCache {
		fields["provider_cache_id"] = ""
		fields["cache_expires_at"] = nil
		session.ProviderCacheID = ""
		session.CacheExpiresAt = nil
	}
	if updates.ProviderCacheID != "" {
		fields["provider_cache_id"] = updates.ProviderCacheID
		session.ProviderCacheID = updates.ProviderCacheID
	}
	if updates.CacheExpiresAt != nil {
		fields["cache_expires_at"] = updates.CacheExpiresAt
		session.CacheExpiresAt = updates.CacheExpiresAt
	}
	if len(fields) == 0 {
		return
	}
	if err := database.DB.Model(&database.Session{}).Where("id = ?", session.ID).Updates(fields).Error; err != nil {
		log.Printf("⚠️ SCC: failed to persist session cache fields: %v", err)
	}
}

func (b *Brain) prepareSCC(
	ctx context.Context,
	session *database.Session,
	sessionFiles []database.SessionFile,
	provider database.LLMProvider,
	model string,
	apiKey string,
) (cache.Layer2Plan, string) {
	layer2Content := buildLayer2ContentFromSession(*session, sessionFiles)
	layer2Hash := layer2HashFromContent(layer2Content)
	if layer2Content == "" {
		return cache.Layer2Plan{Layer2Hash: layer2Hash, Strategy: string(cache.FamilyNone)}, layer2Content
	}

	out, err := cache.Prepare(ctx, cache.PrepareInput{
		ProviderName:  provider.Name,
		ProviderType:  provider.ProviderType,
		BaseURL:       provider.BaseURL,
		Model:         model,
		APIKey:        apiKey,
		Layer2Content: layer2Content,
		Layer2Hash:    layer2Hash,
		Session: cache.SessionState{
			ID:              session.ID,
			Layer2Hash:      session.Layer2Hash,
			ProviderCacheID: session.ProviderCacheID,
			CacheExpiresAt:  session.CacheExpiresAt,
		},
	})
	if err != nil {
		log.Printf("⚠️ SCC: adapter prepare failed: %v", err)
		return cache.Layer2Plan{
			IncludeInMessages: true,
			MessageContent:    layer2Content,
			Strategy:          "fallback_prefix",
			Layer2Hash:        layer2Hash,
		}, layer2Content
	}

	applySessionCacheUpdates(session, out.SessionUpdates)
	log.Printf("🔒 SmartContextCache: strategy=%s hash=%s cached_content=%t",
		out.Plan.Strategy, shortHash(out.Plan.Layer2Hash), out.Plan.CachedContentName != "")
	return out.Plan, layer2Content
}

func shortHash(full string) string {
	if len(full) >= 8 {
		return full[:8]
	}
	return full
}
