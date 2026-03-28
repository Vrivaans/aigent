package handlers

import (
	"aigent/internal/ai"
	"aigent/internal/database"
	"aigent/internal/utils"
	"os"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ConfigHandler struct {
	Brain *ai.Brain
}

type HandsAIConfigRequest struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func (h *ConfigHandler) GetHandsAIConfig(c *fiber.Ctx) error {
	var config database.HandsAIConfig
	// Use Unscoped to verify even if we used soft deletes previously.
	if err := database.DB.Unscoped().First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(fiber.Map{
				"url":          "",
				"token":        "",
				"is_connected": false,
			})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Mask the encrypted token — never expose it to the frontend
	maskedToken := ""
	if config.Token != "" {
		maskedToken = "********"
	}

	return c.JSON(fiber.Map{
		"url":          config.URL,
		"token":        maskedToken,
		"is_connected": config.URL != "" && config.Token != "",
	})
}

func (h *ConfigHandler) UpdateHandsAIConfig(c *fiber.Ctx) error {
	var req HandsAIConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}

	var config database.HandsAIConfig
	// Use Unscoped to find even 'deleted' records and reuse (undelete) them
	result := database.DB.Unscoped().First(&config)

	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return c.Status(500).JSON(fiber.Map{"error": result.Error.Error()})
	}

	config.URL = req.URL

	// Only update the token if a new (non-masked) token was provided
	if req.Token != "" && req.Token != "********" {
		encryptedToken, err := utils.Encrypt(req.Token, masterKey)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt token"})
		}
		config.Token = encryptedToken
	}
	// If masked placeholder is sent, keep the existing encrypted token as-is

	var savedErr error
	if result.Error == gorm.ErrRecordNotFound {
		savedErr = database.DB.Create(&config).Error
	} else {
		// Reset DeletedAt if it was set (undeleting the record)
		config.DeletedAt = gorm.DeletedAt{}
		savedErr = database.DB.Unscoped().Save(&config).Error
	}
	if savedErr != nil {
		return c.Status(500).JSON(fiber.Map{"error": savedErr.Error()})
	}

	// Decrypt token to pass the plaintext to the live client
	if h.Brain != nil && h.Brain.HandsAI != nil {
		plainToken := ""
		if config.Token != "" {
			decrypted, err := utils.Decrypt(config.Token, masterKey)
			if err == nil {
				plainToken = decrypted
			}
		}
		h.Brain.HandsAI.UpdateConfig(config.URL, plainToken)
		// Trigger a fresh tool sync in the background
		go h.Brain.SyncTools(c.Context())
	}

	return c.JSON(fiber.Map{"status": "success", "url": config.URL})
}

func (h *ConfigHandler) DeleteHandsAIConfig(c *fiber.Ctx) error {
	// Perform a HARD DELETE (Unscoped) to permanently remove the row from the database.
	// This prevents unique key collisions on the 'username' field with 'ghost' soft-deleted rows.
	if err := database.DB.Unscoped().Where("1 = 1").Delete(&database.HandsAIConfig{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Disable the live client immediately so no stale tools are shown
	if h.Brain != nil && h.Brain.HandsAI != nil {
		h.Brain.HandsAI.UpdateConfig("", "")
		go h.Brain.SyncTools(c.Context())
	}

	return c.JSON(fiber.Map{"status": "deleted"})
}
