package handlers

import (
	"errors"
	"strconv"

	"aigent/internal/auth"
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func requireTenantID(c *fiber.Ctx) (uint, error) {
	if id, ok := auth.GetTenantID(c); ok && id > 0 {
		return id, nil
	}
	return 0, fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: tenant context missing")
}

func scopedDB(c *fiber.Ctx) (*gorm.DB, uint, error) {
	tenantID, err := requireTenantID(c)
	if err != nil {
		return nil, 0, err
	}
	return database.ForTenant(database.DB, tenantID), tenantID, nil
}

func respondFiberError(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}

func loadSessionForTenant(c *fiber.Ctx, sessionID string) (database.Session, error) {
	tenantID, err := requireTenantID(c)
	if err != nil {
		return database.Session{}, err
	}

	var session database.Session
	if err := database.DB.First(&session, sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return session, fiber.NewError(fiber.StatusNotFound, "Session not found")
		}
		return session, err
	}
	if !database.BelongsToTenant(session.TenantID, tenantID) {
		return session, fiber.NewError(fiber.StatusForbidden, "Forbidden: session belongs to another tenant")
	}
	return session, nil
}

func loadSessionForTenantUint(c *fiber.Ctx, sessionID uint) (database.Session, error) {
	return loadSessionForTenant(c, strconv.FormatUint(uint64(sessionID), 10))
}

func loadAgentForTenant(c *fiber.Ctx, agentID string) (database.Agent, error) {
	tenantID, err := requireTenantID(c)
	if err != nil {
		return database.Agent{}, err
	}

	var agent database.Agent
	if err := database.DB.First(&agent, agentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return agent, fiber.NewError(fiber.StatusNotFound, "Agent not found")
		}
		return agent, err
	}
	if !database.BelongsToTenant(agent.TenantID, tenantID) {
		return agent, fiber.NewError(fiber.StatusForbidden, "Forbidden: agent belongs to another tenant")
	}
	return agent, nil
}

func loadUserForTenant(c *fiber.Ctx, userID string) (database.User, error) {
	tenantID, err := requireTenantID(c)
	if err != nil {
		return database.User{}, err
	}

	var user database.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user, fiber.NewError(fiber.StatusNotFound, "User not found")
		}
		return user, err
	}
	if !database.BelongsToTenant(user.TenantID, tenantID) {
		return user, fiber.NewError(fiber.StatusForbidden, "Forbidden: user belongs to another tenant")
	}
	return user, nil
}

func loadProviderForTenant(c *fiber.Ctx, providerID string) (database.LLMProvider, error) {
	tenantID, err := requireTenantID(c)
	if err != nil {
		return database.LLMProvider{}, err
	}

	var provider database.LLMProvider
	if err := database.DB.First(&provider, providerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return provider, fiber.NewError(fiber.StatusNotFound, "Provider not found")
		}
		return provider, err
	}
	if !database.BelongsToTenant(provider.TenantID, tenantID) {
		return provider, fiber.NewError(fiber.StatusForbidden, "Forbidden: provider belongs to another tenant")
	}
	return provider, nil
}

func tenantPendingQuery(db *gorm.DB, tenantID uint) *gorm.DB {
	return db.Joins("JOIN sessions ON sessions.id = pending_actions.session_id").
		Where("sessions.tenant_id = ?", tenantID)
}
