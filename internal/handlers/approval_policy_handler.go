package handlers

import (
	"errors"
	"strings"

	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ApprovalPolicyHandler struct{}

type ApprovalPolicyRequest struct {
	ToolPattern      string `json:"tool_pattern"`
	Environment      string `json:"environment"`
	RequiresApproval *bool  `json:"requires_approval"`
	MinRole          string `json:"min_role"`
}

func (h *ApprovalPolicyHandler) List(c *fiber.Ctx) error {
	policies, err := database.ListApprovalPolicies(database.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(policies)
}

func (h *ApprovalPolicyHandler) Create(c *fiber.Ctx) error {
	var req ApprovalPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	pattern := strings.TrimSpace(req.ToolPattern)
	if pattern == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tool_pattern is required"})
	}

	env := strings.TrimSpace(req.Environment)
	if env == "" {
		env = "*"
	}
	minRole := strings.TrimSpace(req.MinRole)
	if minRole == "" {
		minRole = "operator"
	}
	requires := true
	if req.RequiresApproval != nil {
		requires = *req.RequiresApproval
	}

	policy := database.ApprovalPolicy{
		ToolPattern:      pattern,
		Environment:      env,
		RequiresApproval: requires,
		MinRole:          minRole,
	}
	if err := database.DB.Create(&policy).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(policy)
}

func (h *ApprovalPolicyHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	res := database.DB.Delete(&database.ApprovalPolicy{}, id)
	if res.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": res.Error.Error()})
	}
	if res.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Policy not found"})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}

func (h *ApprovalPolicyHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var policy database.ApprovalPolicy
	if err := database.DB.First(&policy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Policy not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var req ApprovalPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if pattern := strings.TrimSpace(req.ToolPattern); pattern != "" {
		policy.ToolPattern = pattern
	}
	if env := strings.TrimSpace(req.Environment); env != "" {
		policy.Environment = env
	}
	if req.RequiresApproval != nil {
		policy.RequiresApproval = *req.RequiresApproval
	}
	if minRole := strings.TrimSpace(req.MinRole); minRole != "" {
		policy.MinRole = minRole
	}
	if err := database.DB.Save(&policy).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(policy)
}
