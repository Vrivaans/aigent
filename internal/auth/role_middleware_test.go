package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequireRoleMiddleware(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		SetRequestUser(c, &Claims{UserID: 1, Username: "u", Roles: GetRolesFromHeader(c)})
		return c.Next()
	})
	app.Get("/admin", RequireRoleMiddleware("admin"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	okReq := httptest.NewRequest("GET", "/admin", nil)
	okReq.Header.Set("X-Roles", "admin")
	okResp, err := app.Test(okReq)
	if err != nil || okResp.StatusCode != fiber.StatusOK {
		t.Fatalf("admin role: err=%v status=%d", err, okResp.StatusCode)
	}

	denyReq := httptest.NewRequest("GET", "/admin", nil)
	denyReq.Header.Set("X-Roles", "viewer")
	denyResp, err := app.Test(denyReq)
	if err != nil || denyResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("viewer role: err=%v status=%d", err, denyResp.StatusCode)
	}
}

func GetRolesFromHeader(c *fiber.Ctx) []string {
	raw := c.Get("X-Roles")
	if raw == "" {
		return nil
	}
	return []string{raw}
}
