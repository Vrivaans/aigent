package audit

import (
	"log"

	"aigent/internal/auth"

	"github.com/gofiber/fiber/v2"
)

// Emit records an audit event using request context (actor, IP, user agent, correlation ID).
// Failures are logged and do not interrupt the caller.
func Emit(c *fiber.Ctx, event Event) {
	if c == nil {
		return
	}

	if event.CorrelationID == "" {
		event.CorrelationID = CorrelationID(c)
	}
	if event.IP == "" {
		event.IP = c.IP()
	}
	if event.UserAgent == "" {
		event.UserAgent = c.Get("User-Agent")
	}
	if event.ActorUserID == nil {
		if uid, ok := auth.GetUserID(c); ok {
			event.ActorUserID = &uid
		}
	}

	if err := Record(c.Context(), event); err != nil {
		log.Printf("audit emit failed action=%s resource=%s: %v", event.Action, event.ResourceType, err)
	}
}

// EmitLogin records auth.login.* events (login route has no JWT actor yet).
func EmitLogin(c *fiber.Ctx, action string, actorUserID *uint, payload *string) {
	Emit(c, Event{
		ActorUserID:  actorUserID,
		Action:       action,
		ResourceType: "auth",
		ResourceID:   "login",
		PayloadAfter: payload,
	})
}
