package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"

	"aigent/internal/database"
	"aigent/internal/utils"

	"github.com/gofiber/fiber/v2"
	"strings"
)

type ProviderRequest struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	DefaultModel string `json:"default_model"`
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
	}

	if err := database.DB.Create(&provider).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

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

	// Solo actualizar APIKey si se proporcionó una nueva
	if req.APIKey != "" && req.APIKey != "********" {
		masterKey := os.Getenv("DB_ENCRYPTION_KEY")
		encryptedKey, err := utils.Encrypt(req.APIKey, masterKey)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Encryption failed"})
		}
		provider.APIKey = encryptedKey
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
