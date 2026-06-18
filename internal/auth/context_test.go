package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestGetUserIDFromLocals(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		SetRequestUser(c, &Claims{UserID: 7, Username: "u", Roles: []string{"admin"}})
		id, ok := GetUserID(c)
		if !ok || id != 7 {
			t.Fatalf("GetUserID = %d, ok=%v", id, ok)
		}
		roles := GetRoles(c)
		if len(roles) != 1 || roles[0] != "admin" {
			t.Fatalf("GetRoles = %v", roles)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRequirePermission(t *testing.T) {
	PermissionChecker = func(userID uint, resource, action string) (bool, error) {
		return userID == 1 && resource == "agents" && action == "read", nil
	}
	t.Cleanup(func() { PermissionChecker = nil })

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		SetRequestUser(c, &Claims{UserID: 1, Username: "u", Roles: []string{"viewer"}})
		if err := RequirePermission(c, "agents", "read"); err != nil {
			return err
		}
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/deny", func(c *fiber.Ctx) error {
		SetRequestUser(c, &Claims{UserID: 1, Username: "u", Roles: []string{"viewer"}})
		return RequirePermission(c, "providers", "write")
	})

	okReq := httptest.NewRequest("GET", "/", nil)
	okResp, err := app.Test(okReq)
	if err != nil || okResp.StatusCode != fiber.StatusOK {
		t.Fatalf("allowed request: err=%v status=%d", err, okResp.StatusCode)
	}

	denyReq := httptest.NewRequest("GET", "/deny", nil)
	denyResp, err := app.Test(denyReq)
	if err != nil || denyResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("denied request: err=%v status=%d", err, denyResp.StatusCode)
	}
}
