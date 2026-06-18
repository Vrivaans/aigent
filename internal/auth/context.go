package auth

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

const (
	localUserID    = "user_id"
	localUsername  = "username"
	localRoles     = "roles"
	localTenantID  = "tenant_id"
)

// PermissionChecker resolves whether a user may perform resource/action.
// Wired from main after database init to avoid import cycles.
var PermissionChecker func(userID uint, resource, action string) (bool, error)

// SetRequestUser stores authenticated user data in Fiber locals.
func SetRequestUser(c *fiber.Ctx, claims *Claims) {
	c.Locals(localUserID, claims.UserID)
	c.Locals(localUsername, claims.Username)
	c.Locals(localRoles, claims.Roles)
	c.Locals(localTenantID, claims.TenantID)
}

// GetUserID returns the authenticated user id from Fiber locals.
func GetUserID(c *fiber.Ctx) (uint, bool) {
	v := c.Locals(localUserID)
	if v == nil {
		return 0, false
	}
	id, ok := v.(uint)
	return id, ok && id > 0
}

// GetRoles returns role names from Fiber locals.
func GetRoles(c *fiber.Ctx) []string {
	v := c.Locals(localRoles)
	if v == nil {
		return nil
	}
	roles, ok := v.([]string)
	if !ok {
		return nil
	}
	return roles
}

// GetTenantID returns tenant_id from JWT claims stored in Fiber locals.
func GetTenantID(c *fiber.Ctx) (uint, bool) {
	v := c.Locals(localTenantID)
	if v == nil {
		return 0, false
	}
	id, ok := v.(uint)
	return id, ok && id > 0
}

// enforcePermission writes JSON error responses when access is denied.
// Returns true when the request may proceed.
func enforcePermission(c *fiber.Ctx, resource, action string) bool {
	userID, ok := GetUserID(c)
	if !ok {
		log.Printf("⚠️ RBAC denied: missing user context path=%s resource=%s action=%s", c.Path(), resource, action)
		_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized: user context missing",
		})
		return false
	}
	if PermissionChecker == nil {
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Permission checker not configured",
		})
		return false
	}

	allowed, err := PermissionChecker(userID, resource, action)
	if err != nil {
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Permission check failed",
		})
		return false
	}
	if !allowed {
		log.Printf("⚠️ RBAC denied: user_id=%d resource=%s action=%s path=%s", userID, resource, action, c.Path())
		_ = c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Forbidden: insufficient permissions",
		})
		return false
	}
	return true
}

// RequirePermission checks DB-backed permissions for the current user.
// Returns an error response via Fiber when denied; nil when allowed.
func RequirePermission(c *fiber.Ctx, resource, action string) error {
	if enforcePermission(c, resource, action) {
		return nil
	}
	return fiber.ErrForbidden
}
