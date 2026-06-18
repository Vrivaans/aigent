package auth

import (
	"log"
	"slices"

	"github.com/gofiber/fiber/v2"
)

// RequireRoleMiddleware restricts access to users with at least one of the given roles.
func RequireRoleMiddleware(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRoles := GetRoles(c)
		for _, role := range userRoles {
			if slices.Contains(allowedRoles, role) {
				return c.Next()
			}
		}

		userID, _ := GetUserID(c)
		log.Printf("⚠️ RBAC denied: user_id=%d lacks roles %v path=%s", userID, allowedRoles, c.Path())
		_ = c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Forbidden: admin role required",
		})
		return nil
	}
}
