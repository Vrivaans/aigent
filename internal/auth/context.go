package auth

import (
	"github.com/gofiber/fiber/v2"
)

const (
	localUserID   = "user_id"
	localUsername = "username"
	localRoles    = "roles"
)

// PermissionChecker resolves whether a user may perform resource/action.
// Wired from main after database init to avoid import cycles.
var PermissionChecker func(userID uint, resource, action string) (bool, error)

// SetRequestUser stores authenticated user data in Fiber locals.
func SetRequestUser(c *fiber.Ctx, claims *Claims) {
	c.Locals(localUserID, claims.UserID)
	c.Locals(localUsername, claims.Username)
	c.Locals(localRoles, claims.Roles)
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

// RequirePermission checks DB-backed permissions for the current user.
// Returns an error response via Fiber when denied; nil when allowed.
func RequirePermission(c *fiber.Ctx, resource, action string) error {
	userID, ok := GetUserID(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized: user context missing",
		})
	}
	if PermissionChecker == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Permission checker not configured",
		})
	}

	allowed, err := PermissionChecker(userID, resource, action)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Permission check failed",
		})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Forbidden: insufficient permissions",
		})
	}
	return nil
}
