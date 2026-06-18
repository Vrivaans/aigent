package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const geminiMinLayer2Bytes = 120_000 // ~30k tokens heuristic for CachedContent minimum

// CachedContentCreator creates Google Gemini CachedContent resources.
type CachedContentCreator interface {
	Create(ctx context.Context, apiKey, baseURL, model, content, ttl string) (resourceName string, expiresAt time.Time, err error)
}

type googleCachedContentCreator struct {
	client *http.Client
}

func (g googleCachedContentCreator) Create(ctx context.Context, apiKey, baseURL, model, content, ttl string) (string, time.Time, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", time.Time{}, fmt.Errorf("gemini cached content requires api key")
	}
	root := geminiAPIRoot(baseURL)
	url := fmt.Sprintf("%s/v1beta/cachedContents?key=%s", root, apiKey)

	body := map[string]any{
		"model": normalizeGeminiModel(model),
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": content},
				},
			},
		},
		"ttl": ttl,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", time.Time{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := g.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("gemini cachedContents HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed struct {
		Name      string `json:"name"`
		ExpireTime string `json:"expireTime"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", time.Time{}, err
	}
	if parsed.Name == "" {
		return "", time.Time{}, fmt.Errorf("gemini cachedContents response missing name")
	}

	expiresAt := time.Now().UTC().Add(30 * time.Minute)
	if parsed.ExpireTime != "" {
		if t, err := time.Parse(time.RFC3339, parsed.ExpireTime); err == nil {
			expiresAt = t.UTC()
		}
	}
	return parsed.Name, expiresAt, nil
}

var cachedContentCreator CachedContentCreator = googleCachedContentCreator{}

// SetCachedContentCreatorForTest overrides the Gemini CachedContent client. Returns restore func.
func SetCachedContentCreatorForTest(c CachedContentCreator) func() {
	prev := cachedContentCreator
	cachedContentCreator = c
	return func() { cachedContentCreator = prev }
}

func geminiAPIRoot(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "https://generativelanguage.googleapis.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.Contains(baseURL, "generativelanguage.googleapis.com") {
		return baseURL
	}
	return "https://generativelanguage.googleapis.com"
}

func normalizeGeminiModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "models/gemini-1.5-pro"
	}
	if strings.HasPrefix(model, "models/") {
		return model
	}
	return "models/" + model
}

// GeminiAdapter creates or reuses Gemini CachedContent when Layer 2 is large enough.
type GeminiAdapter struct{}

func (GeminiAdapter) Family() Family { return FamilyGemini }

func (GeminiAdapter) Prepare(ctx context.Context, in PrepareInput) (PrepareOutput, error) {
	if in.Layer2Content == "" {
		return emptyLayer2Output(in.Layer2Hash), nil
	}

	now := time.Now().UTC()
	if cacheStillValid(in.Session, in.Layer2Hash, now) {
		return PrepareOutput{
			Plan: Layer2Plan{
				IncludeInMessages: false,
				CachedContentName: in.Session.ProviderCacheID,
				Strategy:          "gemini_reuse",
				Layer2Hash:        in.Layer2Hash,
			},
			SessionUpdates: SessionUpdates{Layer2Hash: in.Layer2Hash},
		}, nil
	}

	if len(in.Layer2Content) < geminiMinLayer2Bytes {
		return PrepareOutput{
			Plan: baseLayer2Plan(in.Layer2Content, in.Layer2Hash, "gemini_deferred_small", true, ""),
			SessionUpdates: SessionUpdates{
				Layer2Hash: in.Layer2Hash,
				ClearCache: true,
			},
		}, nil
	}

	name, expiresAt, err := cachedContentCreator.Create(ctx, in.APIKey, in.BaseURL, in.Model, in.Layer2Content, "1800s")
	if err != nil {
		return PrepareOutput{
			Plan: baseLayer2Plan(in.Layer2Content, in.Layer2Hash, "gemini_fallback_prefix", true, ""),
			SessionUpdates: SessionUpdates{
				Layer2Hash: in.Layer2Hash,
				ClearCache: true,
			},
		}, nil
	}

	return PrepareOutput{
		Plan: Layer2Plan{
			IncludeInMessages: false,
			CachedContentName: name,
			Strategy:          "gemini_cached_content",
			Layer2Hash:        in.Layer2Hash,
		},
		SessionUpdates: SessionUpdates{
			Layer2Hash:      in.Layer2Hash,
			ProviderCacheID: name,
			CacheExpiresAt:  &expiresAt,
		},
	}, nil
}
