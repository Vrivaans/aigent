package handlers_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aigent/internal/audit"
	"aigent/internal/auth"
	"aigent/internal/database"
	"aigent/internal/handlers"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuditListTestDB(t *testing.T) (*gorm.DB, uint, uint, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.User{}, &database.Role{}, &database.RolePermission{}, &database.UserRole{}, &database.AuditEvent{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := database.SeedRolesAndPermissions(db); err != nil {
		t.Fatalf("seed roles: %v", err)
	}

	var viewerRole, auditorRole database.Role
	if err := db.Where("name = ?", "viewer").First(&viewerRole).Error; err != nil {
		t.Fatalf("viewer role: %v", err)
	}
	if err := db.Where("name = ?", "auditor").First(&auditorRole).Error; err != nil {
		t.Fatalf("auditor role: %v", err)
	}

	viewer := database.User{Username: "viewer1", PasswordHash: "x", IsActive: true}
	auditor := database.User{Username: "auditor1", PasswordHash: "x", IsActive: true}
	if err := db.Create(&viewer).Error; err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	if err := db.Create(&auditor).Error; err != nil {
		t.Fatalf("create auditor: %v", err)
	}
	if err := db.Create(&database.UserRole{UserID: viewer.ID, RoleID: viewerRole.ID}).Error; err != nil {
		t.Fatalf("assign viewer: %v", err)
	}
	if err := db.Create(&database.UserRole{UserID: auditor.ID, RoleID: auditorRole.ID}).Error; err != nil {
		t.Fatalf("assign auditor: %v", err)
	}

	actorID := uint(99)
	events := []database.AuditEvent{
		{OccurredAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), Action: "provider.create", ResourceType: "provider", ResourceID: "1", ActorUserID: &actorID},
		{OccurredAt: time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC), Action: "auth.login.success", ResourceType: "auth", ResourceID: "login", ActorUserID: &actorID},
		{OccurredAt: time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC), Action: "provider.delete", ResourceType: "provider", ResourceID: "2", ActorUserID: &actorID},
	}
	for i := range events {
		if err := db.Create(&events[i]).Error; err != nil {
			t.Fatalf("seed audit event: %v", err)
		}
	}

	database.DB = db
	t.Cleanup(func() { database.DB = nil })

	auth.PermissionChecker = func(userID uint, resource, action string) (bool, error) {
		return database.UserHasPermission(db, userID, resource, action)
	}
	t.Cleanup(func() { auth.PermissionChecker = nil })

	return db, viewer.ID, auditor.ID, actorID
}

func TestViewerCannotAccessAuditEvents(t *testing.T) {
	_, viewerID, _, _ := setupAuditListTestDB(t)

	handler := &handlers.AuditHandler{}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{UserID: viewerID, Username: "viewer1", Roles: []string{"viewer"}})
		return c.Next()
	})
	app.Get("/api/audit/events", auth.RequirePermissionMiddleware("audit", "read"), handler.ListEvents)

	req := httptest.NewRequest("GET", "/api/audit/events", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAuditorCanListAuditEventsWithFilters(t *testing.T) {
	db, _, auditorID, actorID := setupAuditListTestDB(t)

	handler := &handlers.AuditHandler{}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{UserID: auditorID, Username: "auditor1", Roles: []string{"auditor"}})
		return c.Next()
	})
	app.Get("/api/audit/events", auth.RequirePermissionMiddleware("audit", "read"), handler.ListEvents)

	req := httptest.NewRequest("GET", "/api/audit/events?resource_type=provider&actor_user_id=99&limit=10&offset=0", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var payload handlers.AuditEventsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Total != 2 {
		t.Fatalf("expected total=2, got %d", payload.Total)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(payload.Items))
	}
	for _, item := range payload.Items {
		if item.ResourceType != "provider" {
			t.Fatalf("unexpected resource_type %q", item.ResourceType)
		}
		if item.ActorUserID == nil || *item.ActorUserID != actorID {
			t.Fatalf("unexpected actor_user_id %v", item.ActorUserID)
		}
	}

	var count int64
	if err := db.Model(&database.AuditEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 rows in db, got %d", count)
	}
}

func TestAuditorCanExportCSV(t *testing.T) {
	_, _, auditorID, _ := setupAuditListTestDB(t)

	handler := &handlers.AuditHandler{}
	app := fiber.New()
	app.Use(audit.CorrelationMiddleware())
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{UserID: auditorID, Username: "auditor1", Roles: []string{"auditor"}})
		return c.Next()
	})
	app.Get("/api/audit/events/export", auth.RequirePermissionMiddleware("audit", "export"), handler.ExportEvents)

	req := httptest.NewRequest("GET", "/api/audit/events/export?format=csv", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("expected text/csv, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "provider.create") {
		t.Fatalf("csv missing expected action: %q", string(body))
	}
}

func TestExportCreatesAuditEvent(t *testing.T) {
	db, _, auditorID, _ := setupAuditListTestDB(t)
	restore := audit.SetDBForTest(db)
	defer restore()

	handler := &handlers.AuditHandler{}
	app := fiber.New()
	app.Use(audit.CorrelationMiddleware())
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{UserID: auditorID, Username: "auditor1", Roles: []string{"auditor"}})
		return c.Next()
	})
	app.Get("/api/audit/events/export", auth.RequirePermissionMiddleware("audit", "export"), handler.ExportEvents)

	req := httptest.NewRequest("GET", "/api/audit/events/export?format=csv", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	_, _ = io.ReadAll(resp.Body)

	var exportRows []database.AuditEvent
	if err := db.Where("action = ?", "audit.export").Find(&exportRows).Error; err != nil {
		t.Fatalf("find export audit: %v", err)
	}
	if len(exportRows) != 1 {
		t.Fatalf("expected 1 audit.export row, got %d", len(exportRows))
	}
}

func TestViewerCannotExportAuditEvents(t *testing.T) {
	_, viewerID, _, _ := setupAuditListTestDB(t)

	handler := &handlers.AuditHandler{}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{UserID: viewerID, Username: "viewer1", Roles: []string{"viewer"}})
		return c.Next()
	})
	app.Get("/api/audit/events/export", auth.RequirePermissionMiddleware("audit", "export"), handler.ExportEvents)

	req := httptest.NewRequest("GET", "/api/audit/events/export?format=csv", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}
