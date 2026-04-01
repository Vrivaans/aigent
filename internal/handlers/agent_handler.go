package handlers

import (
	"aigent/internal/ai"
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type AgentHandler struct {
	Brain *ai.Brain
}

type CreateAgentRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	LLMProviderID *uint    `json:"llm_provider_id"`
	Tools         []string `json:"tools"` // List of tool names
}

func (h *AgentHandler) GetAgents(c *fiber.Ctx) error {
	var agents []database.Agent
	if err := database.DB.Preload("LLMProvider").Order("id asc").Find(&agents).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if len(agents) == 0 {
		return c.JSON(agents)
	}
	ids := make([]uint, len(agents))
	for i := range agents {
		ids[i] = agents[i].ID
	}
	var rows []struct {
		AgentID uint  `gorm:"column:agent_id"`
		Cnt     int64 `gorm:"column:cnt"`
	}
	if err := database.DB.Model(&database.AgentTool{}).
		Select("agent_id, count(*) as cnt").
		Where("agent_id IN ?", ids).
		Group("agent_id").
		Scan(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	byID := make(map[uint]int64, len(rows))
	for _, r := range rows {
		byID[r.AgentID] = r.Cnt
	}
	for i := range agents {
		agents[i].ToolsCount = int(byID[agents[i].ID])
		if agents[i].IsDefault && h.Brain != nil {
			agents[i].ToolsCount = len(h.Brain.Registry.List())
		}
	}
	return c.JSON(agents)
}

func (h *AgentHandler) GetAgent(c *fiber.Ctx) error {
	id := c.Params("id")
	var agent database.Agent
	if err := database.DB.Preload("Tools").Preload("LLMProvider").First(&agent, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Agent not found"})
	}
	return c.JSON(agent)
}

func (h *AgentHandler) CreateAgent(c *fiber.Ctx) error {
	var req CreateAgentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid format"})
	}

	agent := database.Agent{
		Name:          req.Name,
		Description:   req.Description,
		LLMProviderID: req.LLMProviderID,
	}
	
	if req.LLMProviderID != nil && *req.LLMProviderID == 0 {
		agent.LLMProviderID = nil
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&agent).Error; err != nil {
			return err
		}

		for _, toolName := range req.Tools {
			at := database.AgentTool{
				AgentID:  agent.ID,
				ToolName: toolName,
			}
			if err := tx.Create(&at).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(agent)
}

func (h *AgentHandler) UpdateAgent(c *fiber.Ctx) error {
	id := c.Params("id")
	var req CreateAgentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid format"})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var agent database.Agent
		if err := tx.First(&agent, id).Error; err != nil {
			return err
		}

		agent.Name = req.Name
		agent.Description = req.Description
		agent.LLMProviderID = req.LLMProviderID

		if req.LLMProviderID != nil && *req.LLMProviderID == 0 {
			agent.LLMProviderID = nil
		}
		
		if err := tx.Save(&agent).Error; err != nil {
			return err
		}

		// El agente General (IsDefault) no persiste selección: siempre usa todas las tools del registry.
		if agent.IsDefault {
			return tx.Where("agent_id = ?", agent.ID).Delete(&database.AgentTool{}).Error
		}

		// Update tools: Delete old and insert new
		if err := tx.Where("agent_id = ?", agent.ID).Delete(&database.AgentTool{}).Error; err != nil {
			return err
		}

		for _, toolName := range req.Tools {
			at := database.AgentTool{
				AgentID:  agent.ID,
				ToolName: toolName,
			}
			if err := tx.Create(&at).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "updated"})
}

func (h *AgentHandler) DeleteAgent(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "1" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot delete General agent"})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Clean up tools first
		if err := tx.Where("agent_id = ?", id).Delete(&database.AgentTool{}).Error; err != nil {
			return err
		}
		
		// Asignar todas las sesiones (chats) de este agente al Agente General (ID: 1) para no perder el historial
		if err := tx.Model(&database.Session{}).Where("agent_id = ?", id).Update("agent_id", 1).Error; err != nil {
			return err
		}

		if err := tx.Delete(&database.Agent{}, id).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "deleted"})
}
