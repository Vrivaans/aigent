package handlers

import (
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
)

type RuleHandler struct{}

type CreateRuleRequest struct {
	AgentIDs   []uint `json:"agent_ids"`
	Category   string `json:"category"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

func (h *RuleHandler) GetRules(c *fiber.Ctx) error {
	var rules []database.Rule
	if err := database.DB.Preload("Agents").Order("importance desc").Find(&rules).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(rules)
}

func (h *RuleHandler) CreateRule(c *fiber.Ctx) error {
	var req CreateRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid format"})
	}

	rule := database.Rule{
		Category:   req.Category,
		Content:    req.Content,
		Importance: req.Importance,
	}

	if err := database.DB.Create(&rule).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Assign agents via many2many if any specified
	if len(req.AgentIDs) > 0 {
		var agents []database.Agent
		if err := database.DB.Where("id IN ?", req.AgentIDs).Find(&agents).Error; err == nil {
			database.DB.Model(&rule).Association("Agents").Replace(agents)
		}
	}

	database.DB.Preload("Agents").First(&rule, rule.ID)

	return c.JSON(rule)
}

func (h *RuleHandler) DeleteRule(c *fiber.Ctx) error {
	id := c.Params("id")
	// Clean up join table entries first
	var rule database.Rule
	if err := database.DB.First(&rule, id).Error; err == nil {
		database.DB.Model(&rule).Association("Agents").Clear()
	}
	if err := database.DB.Delete(&database.Rule{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}
