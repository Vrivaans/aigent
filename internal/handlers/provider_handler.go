package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"aigent/internal/database"
	"aigent/internal/utils"

	"github.com/gofiber/fiber/v2"
	"strings"
	"time"
)

type ProviderRequest struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	DefaultModel string `json:"default_model"`
	ProviderType string `json:"provider_type"`
}

func providerPresets() []fiber.Map {
	return []fiber.Map{
		{"type": "zen", "name": "OpenCode Zen", "base_url": "https://opencode.ai/zen/v1", "description": "Modelos gratuitos y premium via Zen API"},
		{"type": "go", "name": "OpenCode Go", "base_url": "https://opencode.ai/go/v1", "description": "Modelos avanzados via Go API"},
		{"type": "groq", "name": "Groq", "base_url": "https://api.groq.com/openai/v1", "description": "Inferencia ultra rapida"},
		{"type": "openrouter", "name": "OpenRouter", "base_url": "https://openrouter.ai/api/v1", "description": "Acceso unificado a cientos de modelos"},
		{"type": "openai", "name": "OpenAI", "base_url": "https://api.openai.com/v1", "description": "GPT-4 y otros modelos de OpenAI"},
		{"type": "custom", "name": "Custom", "base_url": "", "description": "URL personalizada (OpenAI-compatible)"},
	}
}

func HandleGetPrefilledProviders(c *fiber.Ctx) error {
	return c.JSON(providerPresets())
}

func HandleListProviders(c *fiber.Ctx) error {
	var providers []database.LLMProvider
	if err := database.DB.Find(&providers).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Omitir keys cifradas por seguridad en el listado
	for i := range providers {
		providers[i].APIKey = "********"
	}

	return c.JSON(providers)
}

func HandleCreateProvider(c *fiber.Ctx) error {
	var req ProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Name = strings.TrimSpace(req.Name)
	req.ProviderType = strings.TrimSpace(req.ProviderType)
	req.BaseURL = strings.TrimSpace(req.BaseURL)

	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name is required"})
	}

	if req.ProviderType == "" && req.Name != "" {
		lowerName := strings.ToLower(req.Name)
		switch {
		case strings.Contains(lowerName, "zen"):
			req.ProviderType = "zen"
		case strings.Contains(lowerName, "go"):
			req.ProviderType = "go"
		case strings.Contains(lowerName, "groq"):
			req.ProviderType = "groq"
		case strings.Contains(lowerName, "openrouter"):
			req.ProviderType = "openrouter"
		case strings.Contains(lowerName, "openai"):
			req.ProviderType = "openai"
		default:
			req.ProviderType = "custom"
		}
	}

	if req.BaseURL == "" {
		req.BaseURL = database.ProviderPresetBaseURL(req.ProviderType)
	}

	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}

	encryptedKey, err := utils.Encrypt(req.APIKey, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt API key"})
	}

	provider := database.LLMProvider{
		Name:         req.Name,
		BaseURL:      req.BaseURL,
		APIKey:       encryptedKey,
		DefaultModel: req.DefaultModel,
		ProviderType: req.ProviderType,
	}

	if err := database.DB.Create(&provider).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	go fetchAndStoreModels(provider.ID, req.BaseURL, req.APIKey)

	return c.Status(201).JSON(provider)
}

func HandleSetDefaultProvider(c *fiber.Ctx) error {
	id := c.Params("id")

	// Resetear otros defaults
	database.DB.Model(&database.LLMProvider{}).Where("1 = 1").Update("is_default", false)

	// Marcar este como default
	if err := database.DB.Model(&database.LLMProvider{}).Where("id = ?", id).Update("is_default", true).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

func HandleUpdateProvider(c *fiber.Ctx) error {
	id := c.Params("id")
	var req ProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	var provider database.LLMProvider
	if err := database.DB.First(&provider, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Provider not found"})
	}

	provider.Name = req.Name
	provider.BaseURL = req.BaseURL
	provider.DefaultModel = req.DefaultModel
	if req.ProviderType != "" {
		provider.ProviderType = req.ProviderType
	}

	// Solo actualizar APIKey si se proporcionó una nueva
	if req.APIKey != "" && req.APIKey != "********" {
		masterKey := os.Getenv("DB_ENCRYPTION_KEY")
		encryptedKey, err := utils.Encrypt(req.APIKey, masterKey)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Encryption failed"})
		}
		provider.APIKey = encryptedKey

		go fetchAndStoreModels(provider.ID, provider.BaseURL, req.APIKey)
	}

	if err := database.DB.Save(&provider).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(provider)
}

func HandleDeleteProvider(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Delete(&database.LLMProvider{}, id).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}

// HandleTestProvider realiza una llamada mínima al proveedor existente por ID.
func HandleTestProvider(c *fiber.Ctx) error {
	id := c.Params("id")

	var provider database.LLMProvider
	if err := database.DB.First(&provider, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Provider not found"})
	}

	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	apiKey, err := utils.Decrypt(provider.APIKey, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"ok": false, "error": "Failed to decrypt API key: " + err.Error()})
	}

	return performTestConnection(c, provider.Name, provider.BaseURL, apiKey, provider.DefaultModel)
}

// HandleTestProviderConfig realiza una prueba de conexión con datos crudos (sin guardar en DB).
func HandleTestProviderConfig(c *fiber.Ctx) error {
	var req ProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	if req.BaseURL == "" || req.APIKey == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Base URL and API Key are required"})
	}

	apiKey := req.APIKey
	// Si la key es el placeholder, cargamos la real de la DB si tenemos ID
	if apiKey == "********" && req.ID > 0 {
		var provider database.LLMProvider
		if err := database.DB.First(&provider, req.ID).Error; err == nil {
			masterKey := os.Getenv("DB_ENCRYPTION_KEY")
			decrypted, err := utils.Decrypt(provider.APIKey, masterKey)
			if err == nil {
				apiKey = decrypted
			}
		}
	}

	return performTestConnection(c, req.Name, req.BaseURL, apiKey, req.DefaultModel)
}

func performTestConnection(c *fiber.Ctx, name, baseURL, apiKey, model string) error {
	// Normalización
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
		"max_tokens": 5,
	}
	bodyBytes, _ := json.Marshal(body)

	testURL := baseURL + "/chat/completions"
	req2, _ := http.NewRequestWithContext(c.Context(), "POST", testURL, bytes.NewReader(bodyBytes))
	req2.Header.Set("Authorization", "Bearer "+apiKey)
	req2.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req2)
	if err != nil {
		return c.JSON(fiber.Map{"ok": false, "error": "Fallo al conectar: " + err.Error()})
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return c.JSON(fiber.Map{"ok": true, "message": "✅ Conexión exitosa con " + name})
	}
	
	return c.JSON(fiber.Map{
		"ok":      false,
		"status":  resp.StatusCode,
		"message": "El proveedor respondió con error HTTP " + resp.Status + ". Verificá la URL y la API key.",
	})
}

type modelsResponse struct {
	Data []modelData `json:"data"`
}

type modelData struct {
	ID string `json:"id"`
}

func fetchAndStoreModels(providerID uint, baseURL, apiKey string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)

	url := baseURL + "/v1/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("⚠️ Failed to create models request for provider %d: %v", providerID, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️ Failed to fetch models from provider %d: %v", providerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠️ Provider %d returned %d for /v1/models", providerID, resp.StatusCode)
		return
	}

	var result modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("⚠️ Failed to decode models response for provider %d: %v", providerID, err)
		return
	}

	now := time.Now()
	zenFreeModels := map[string]bool{
		"big-pickle": true, "minimax-m2.5-free": true, "gpt-5-nano": true,
		"ling-2.6-flash-free": true, "hy3": true, "nemotron": true,
	}

	for _, m := range result.Data {
		if m.ID == "" {
			continue
		}
		isFree := zenFreeModels[strings.ToLower(m.ID)]

		var existing database.Model
		err := database.DB.Where("provider_id = ? AND model_id = ?", providerID, m.ID).First(&existing).Error
		if err == nil {
			existing.Name = m.ID
			existing.IsFree = isFree
			existing.LastSeen = now
			database.DB.Save(&existing)
		} else {
			newModel := database.Model{
				ProviderID: providerID,
				ModelID:    m.ID,
				Name:       m.ID,
				IsFree:     isFree,
				LastSeen:   now,
			}
			database.DB.Create(&newModel)
		}
	}

	log.Printf("✅ Fetched %d models for provider %d", len(result.Data), providerID)
}

func HandleGetProviderModels(c *fiber.Ctx) error {
	id := c.Params("id")

	var models []database.Model
	if err := database.DB.Where("provider_id = ?", id).Order("is_free desc, model_id asc").Find(&models).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(models)
}

func HandleRefreshProviderModels(c *fiber.Ctx) error {
	id := c.Params("id")

	var provider database.LLMProvider
	if err := database.DB.First(&provider, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Provider not found"})
	}

	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	apiKey, err := utils.Decrypt(provider.APIKey, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"ok": false, "error": "Failed to decrypt API key: " + err.Error()})
	}

	fetchAndStoreModels(provider.ID, provider.BaseURL, apiKey)

	var models []database.Model
	database.DB.Where("provider_id = ?", id).Order("is_free desc, model_id asc").Find(&models)

	return c.JSON(fiber.Map{"ok": true, "message": "Models refreshed", "models": models})
}

func HandleGetAllModels(c *fiber.Ctx) error {
	var models []database.Model
	database.DB.Preload("Provider").Order("is_free desc, model_id asc").Find(&models)

	var providerMap = make(map[uint]database.LLMProvider)
	var providers []database.LLMProvider
	database.DB.Where("is_active = ?", true).Find(&providers)
	for _, p := range providers {
		providerMap[p.ID] = p
	}

	type groupedModels struct {
		Provider database.LLMProvider `json:"provider"`
		Models   []database.Model     `json:"models"`
	}

	groups := make(map[uint]*groupedModels)
	for _, m := range models {
		if _, ok := providerMap[m.ProviderID]; !ok {
			continue
		}
		if _, exists := groups[m.ProviderID]; !exists {
			groups[m.ProviderID] = &groupedModels{
				Provider: m.Provider,
				Models:   []database.Model{},
			}
		}
		groups[m.ProviderID].Models = append(groups[m.ProviderID].Models, m)
	}

	var result []groupedModels
	for _, g := range groups {
		result = append(result, *g)
	}

	return c.JSON(result)
}
