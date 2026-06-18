package audit

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const localCorrelationID = "correlation_id"

// CorrelationMiddleware assigns a request correlation ID from X-Request-ID or a new UUID.
func CorrelationMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := strings.TrimSpace(c.Get("X-Request-ID"))
		if id == "" {
			id = uuid.NewString()
		}
		c.Locals(localCorrelationID, id)
		c.Set("X-Request-ID", id)
		return c.Next()
	}
}

// CorrelationID returns the correlation ID for the current request.
func CorrelationID(c *fiber.Ctx) string {
	if v, ok := c.Locals(localCorrelationID).(string); ok {
		return v
	}
	return ""
}
