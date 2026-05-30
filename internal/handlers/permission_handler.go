package handlers

import (
	"log"
	"time"

	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
)

func HandleListPermissions(c *fiber.Ctx) error {
	var perms []database.ToolPermission
	if err := database.DB.Order("created_at desc").Find(&perms).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(perms)
}

func HandleDeletePermission(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Delete(&database.ToolPermission{}, id).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}

func HandleTogglePausePermission(c *fiber.Ctx) error {
	id := c.Params("id")

	var perm database.ToolPermission
	if err := database.DB.First(&perm, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Permission not found"})
	}

	now := time.Now()
	if !perm.Paused {
		perm.Paused = true
		perm.PausedAt = &now
	} else {
		perm.Paused = false
		perm.PausedAt = nil
	}

	if err := database.DB.Save(&perm).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(perm)
}

func AutoSaveToolPermission(agentID uint, toolName string) {
	log.Printf("🛡️ AutoSaveToolPermission: attempting to allow tool '%s' for agent #%d", toolName, agentID)
	var existing database.ToolPermission
	result := database.DB.Where("agent_id = ? AND tool_name = ?", agentID, toolName).First(&existing)
	if result.Error == nil {
		if existing.ActionType == "always_allow" && !existing.Paused {
			log.Printf("🛡️ AutoSaveToolPermission: permission already exists and is active")
			return
		}
		existing.ActionType = "always_allow"
		existing.Paused = false
		existing.PausedAt = nil
		if err := database.DB.Save(&existing).Error; err != nil {
			log.Printf("❌ AutoSaveToolPermission: failed to update permission: %v", err)
		} else {
			log.Printf("🛡️ AutoSaveToolPermission: permission updated successfully")
		}
		return
	}
	perm := database.ToolPermission{
		AgentID:    agentID,
		ToolName:   toolName,
		ActionType: "always_allow",
	}
	if err := database.DB.Create(&perm).Error; err != nil {
		log.Printf("❌ AutoSaveToolPermission: failed to create permission: %v", err)
	} else {
		log.Printf("🛡️ AutoSaveToolPermission: permission created successfully in database")
	}
}
