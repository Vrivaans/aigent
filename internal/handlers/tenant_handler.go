package handlers

import (
	"errors"
	"strings"

	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type TenantHandler struct{}

type tenantRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func (h *TenantHandler) List(c *fiber.Ctx) error {
	var tenants []database.Tenant
	if err := database.DB.Order("slug asc").Find(&tenants).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tenants)
}

func (h *TenantHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	var tenant database.Tenant
	if err := database.DB.First(&tenant, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Tenant not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tenant)
}

func (h *TenantHandler) Create(c *fiber.Ctx) error {
	var req tenantRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.Name = strings.TrimSpace(req.Name)
	if req.Slug == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "slug and name are required"})
	}

	tenant := database.Tenant{Slug: req.Slug, Name: req.Name}
	if err := database.DB.Create(&tenant).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "slug already exists"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(tenant)
}

func (h *TenantHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var tenant database.Tenant
	if err := database.DB.First(&tenant, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Tenant not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var req tenantRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		tenant.Name = name
	}
	if slug := strings.TrimSpace(req.Slug); slug != "" && slug != tenant.Slug {
		if tenant.Slug == database.DefaultTenantSlug {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot rename default tenant slug"})
		}
		tenant.Slug = slug
	}

	if err := database.DB.Save(&tenant).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "slug already exists"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tenant)
}
