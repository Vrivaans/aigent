package handlers

import (
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
)

type RuleHandler struct{}

type CreateRuleRequest struct {
	Category   string `json:"category"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

func (h *RuleHandler) GetRules(c *fiber.Ctx) error {
	var rules []database.Rule
	if err := database.DB.Order("importance desc").Find(&rules).Error; err != nil {
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

	return c.JSON(rule)
}

func (h *RuleHandler) DeleteRule(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Delete(&database.Rule{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}
