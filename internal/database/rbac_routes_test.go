package database_test

import (
	"net/http/httptest"
	"testing"

	"aigent/internal/auth"
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRBACTestDB(t *testing.T) (*gorm.DB, uint, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.User{}, &database.Role{}, &database.RolePermission{}, &database.UserRole{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := database.SeedRolesAndPermissions(db); err != nil {
		t.Fatalf("seed roles: %v", err)
	}

	var viewerRole database.Role
	if err := db.Where("name = ?", "viewer").First(&viewerRole).Error; err != nil {
		t.Fatalf("viewer role: %v", err)
	}
	viewer := database.User{Username: "viewer1", PasswordHash: "x", IsActive: true}
	if err := db.Create(&viewer).Error; err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	if err := db.Create(&database.UserRole{UserID: viewer.ID, RoleID: viewerRole.ID}).Error; err != nil {
		t.Fatalf("assign viewer role: %v", err)
	}

	var operatorRole database.Role
	if err := db.Where("name = ?", "operator").First(&operatorRole).Error; err != nil {
		t.Fatalf("operator role: %v", err)
	}
	operator := database.User{Username: "operator1", PasswordHash: "x", IsActive: true}
	if err := db.Create(&operator).Error; err != nil {
		t.Fatalf("create operator: %v", err)
	}
	if err := db.Create(&database.UserRole{UserID: operator.ID, RoleID: operatorRole.ID}).Error; err != nil {
		t.Fatalf("assign operator role: %v", err)
	}

	return db, viewer.ID, operator.ID
}

func TestViewerCannotPostProviders(t *testing.T) {
	db, viewerID, operatorID := setupRBACTestDB(t)
	auth.PermissionChecker = func(userID uint, resource, action string) (bool, error) {
		return database.UserHasPermission(db, userID, resource, action)
	}
	t.Cleanup(func() { auth.PermissionChecker = nil })

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if c.Get("X-Test-User") == "operator" {
			auth.SetRequestUser(c, &auth.Claims{UserID: operatorID, Username: "operator1", Roles: []string{"operator"}})
		} else {
			auth.SetRequestUser(c, &auth.Claims{UserID: viewerID, Username: "viewer1", Roles: []string{"viewer"}})
		}
		return c.Next()
	})
	app.Post("/api/providers", auth.RequirePermissionMiddleware("providers", "write"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusCreated)
	})

	viewerReq := httptest.NewRequest("POST", "/api/providers", nil)
	viewerResp, err := app.Test(viewerReq)
	if err != nil {
		t.Fatalf("viewer request: %v", err)
	}
	if viewerResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("viewer POST /api/providers: expected 403, got %d", viewerResp.StatusCode)
	}

	opReq := httptest.NewRequest("POST", "/api/providers", nil)
	opReq.Header.Set("X-Test-User", "operator")
	opResp, err := app.Test(opReq)
	if err != nil {
		t.Fatalf("operator request: %v", err)
	}
	if opResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("operator POST /api/providers: expected 201, got %d", opResp.StatusCode)
	}
}

func TestViewerHasChatReadOnly(t *testing.T) {
	db, viewerID, _ := setupRBACTestDB(t)

	okRead, err := database.UserHasPermission(db, viewerID, "chat", "read")
	if err != nil || !okRead {
		t.Fatalf("viewer should have chat:read, ok=%v err=%v", okRead, err)
	}
	okWrite, err := database.UserHasPermission(db, viewerID, "chat", "write")
	if err != nil {
		t.Fatalf("chat:write check: %v", err)
	}
	if okWrite {
		t.Fatal("viewer should not have chat:write")
	}
}
