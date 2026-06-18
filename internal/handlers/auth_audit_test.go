package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"aigent/internal/audit"
	"aigent/internal/auth"
	"aigent/internal/database"
	"aigent/internal/handlers"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuditHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.User{}, &database.AuditEvent{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	database.DB = db
	restore := audit.SetDBForTest(db)
	t.Cleanup(func() {
		restore()
		database.DB = nil
	})
	return db
}

func TestHandleLoginFailureWritesAuditEvent(t *testing.T) {
	db := setupAuditHandlerTestDB(t)

	app := fiber.New()
	app.Use(audit.CorrelationMiddleware())
	app.Post("/api/login", handlers.HandleLogin)

	body, _ := json.Marshal(map[string]string{
		"username": "nobody",
		"password": "wrong",
	})
	req := httptest.NewRequest("POST", "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "login-fail-corr")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", resp.StatusCode, readBody(t, resp))
	}

	var rows []database.AuditEvent
	if err := db.Where("action = ?", "auth.login.failure").Find(&rows).Error; err != nil {
		t.Fatalf("find audit: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(rows))
	}
	if rows[0].CorrelationID != "login-fail-corr" {
		t.Fatalf("correlation_id = %q", rows[0].CorrelationID)
	}
	if rows[0].PayloadAfter == nil || !bytes.Contains([]byte(*rows[0].PayloadAfter), []byte("nobody")) {
		t.Fatalf("payload_after = %v", rows[0].PayloadAfter)
	}
	if rows[0].PayloadAfter != nil && bytes.Contains([]byte(*rows[0].PayloadAfter), []byte("wrong")) {
		t.Fatal("password must not appear in audit payload")
	}
}

func TestHandleLoginSuccessWritesAuditEvent(t *testing.T) {
	db := setupAuditHandlerTestDB(t)
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "admin-pass")
	if err := database.SeedAdminUser(db); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	app := fiber.New()
	app.Use(audit.CorrelationMiddleware())
	app.Post("/api/login", handlers.HandleLogin)

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin-pass",
	})
	req := httptest.NewRequest("POST", "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d body=%s", resp.StatusCode, readBody(t, resp))
	}

	var rows []database.AuditEvent
	if err := db.Where("action = ?", "auth.login.success").Find(&rows).Error; err != nil {
		t.Fatalf("find audit: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(rows))
	}
	if rows[0].ActorUserID == nil {
		t.Fatal("expected actor_user_id on successful login audit")
	}
}

func TestHandleDeletePermissionWritesAuditEvent(t *testing.T) {
	db := setupAuditHandlerTestDB(t)
	perm := database.ToolPermission{
		AgentID:    1,
		ToolName:   "test_tool",
		ActionType: "always_allow",
	}
	if err := db.AutoMigrate(&database.ToolPermission{}); err != nil {
		t.Fatalf("automigrate permission: %v", err)
	}
	if err := db.Create(&perm).Error; err != nil {
		t.Fatalf("create permission: %v", err)
	}

	app := fiber.New()
	app.Use(audit.CorrelationMiddleware())
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{UserID: 1, Username: "admin", Roles: []string{"admin"}})
		return c.Next()
	})
	app.Delete("/api/permissions/:id", handlers.HandleDeletePermission)

	req := httptest.NewRequest("DELETE", "/api/permissions/1", nil)
	req.Header.Set("X-Request-ID", "perm-revoke-corr")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var rows []database.AuditEvent
	if err := db.Where("action = ?", "permission.revoke").Find(&rows).Error; err != nil {
		t.Fatalf("find audit: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(rows))
	}
	if rows[0].CorrelationID != "perm-revoke-corr" {
		t.Fatalf("correlation_id = %q", rows[0].CorrelationID)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}
