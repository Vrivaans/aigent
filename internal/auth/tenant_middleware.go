package auth

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// TenantResolver resolves the effective tenant for a request when JWT omits tenant_id.
var TenantResolver func(userID uint, claimTenantID uint) (uint, error)

// TenantMiddleware ensures tenant_id is present in Fiber locals after authentication.
func TenantMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if id, ok := GetTenantID(c); ok && id > 0 {
			return c.Next()
		}

		userID, ok := GetUserID(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: user context missing",
			})
		}
		if TenantResolver == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Tenant resolver not configured",
			})
		}

		tid, err := TenantResolver(userID, 0)
		if err != nil {
			log.Printf("⚠️ Tenant resolve failed for user_id=%d: %v", userID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to resolve tenant",
			})
		}
		c.Locals(localTenantID, tid)
		return c.Next()
	}
}
