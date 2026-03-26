package auth

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func NewAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			log.Printf("⚠️ Unauthorized access attempt: no token provided for path %s", c.Path())
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: No token provided",
			})
		}

		// Support "Bearer <token>"
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		_, err := ValidateToken(tokenString)
		if err != nil {
			log.Printf("⚠️ Unauthorized access attempt: invalid token for path %s. Error: %v", c.Path(), err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Invalid or expired token",
			})
		}

		return c.Next()
	}
}
