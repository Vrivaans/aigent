package handlers_test

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"aigent/internal/auth"
	"aigent/internal/database"
	"aigent/internal/handlers"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTenantScopeTestDB(t *testing.T) (*gorm.DB, database.Tenant, database.Tenant) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.Tenant{}, &database.Agent{}, &database.Session{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	tenantA := database.Tenant{Slug: "tenant-a", Name: "Tenant A"}
	tenantB := database.Tenant{Slug: "tenant-b", Name: "Tenant B"}
	if err := db.Create(&tenantA).Error; err != nil {
		t.Fatalf("create tenant A: %v", err)
	}
	if err := db.Create(&tenantB).Error; err != nil {
		t.Fatalf("create tenant B: %v", err)
	}

	database.DB = db
	t.Cleanup(func() { database.DB = nil })
	return db, tenantA, tenantB
}

func TestCrossTenantSessionHistoryForbidden(t *testing.T) {
	_, tenantA, tenantB := setupTenantScopeTestDB(t)

	agentB := database.Agent{Name: "Agent B", TenantID: database.TenantPtr(tenantB.ID)}
	if err := database.DB.Create(&agentB).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	sessionB := database.Session{Title: "Secret", AgentID: agentB.ID, TenantID: database.TenantPtr(tenantB.ID)}
	if err := database.DB.Create(&sessionB).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{
			UserID:   1,
			Username: "user-a",
			Roles:    []string{"operator"},
			TenantID: tenantA.ID,
		})
		return c.Next()
	})

	handler := &handlers.ChatHandler{}
	app.Get("/api/sessions/:id/chat", handler.HandleGetHistory)

	req := httptest.NewRequest("GET", "/api/sessions/"+strconv.FormatUint(uint64(sessionB.ID), 10)+"/chat", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}
