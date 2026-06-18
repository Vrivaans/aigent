package auth

import "github.com/gofiber/fiber/v2"

// RequirePermissionMiddleware returns Fiber middleware that enforces resource/action permissions.
func RequirePermissionMiddleware(resource, action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !enforcePermission(c, resource, action) {
			return nil
		}
		return c.Next()
	}
}
