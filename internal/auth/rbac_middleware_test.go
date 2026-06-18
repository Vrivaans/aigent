package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequirePermissionMiddlewareAllows(t *testing.T) {
	PermissionChecker = func(userID uint, resource, action string) (bool, error) {
		return userID == 2 && resource == "agents" && action == "read", nil
	}
	t.Cleanup(func() { PermissionChecker = nil })

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		SetRequestUser(c, &Claims{UserID: 2, Username: "op", Roles: []string{"operator"}})
		return c.Next()
	})
	app.Get("/agents", RequirePermissionMiddleware("agents", "read"), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	req := httptest.NewRequest("GET", "/agents", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRequirePermissionMiddlewareForbidden(t *testing.T) {
	PermissionChecker = func(userID uint, resource, action string) (bool, error) {
		return false, nil
	}
	t.Cleanup(func() { PermissionChecker = nil })

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		SetRequestUser(c, &Claims{UserID: 3, Username: "viewer", Roles: []string{"viewer"}})
		return c.Next()
	})
	app.Post("/providers", RequirePermissionMiddleware("providers", "write"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusCreated)
	})

	req := httptest.NewRequest("POST", "/providers", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRequirePermissionMiddlewareMissingUser(t *testing.T) {
	PermissionChecker = func(userID uint, resource, action string) (bool, error) {
		return true, nil
	}
	t.Cleanup(func() { PermissionChecker = nil })

	app := fiber.New()
	app.Delete("/agents/1", RequirePermissionMiddleware("agents", "write"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest("DELETE", "/agents/1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
