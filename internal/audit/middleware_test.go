package audit_test

import (
	"io"
	"net/http/httptest"
	"testing"

	"aigent/internal/audit"
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCorrelationMiddlewareUsesHeader(t *testing.T) {
	app := fiber.New()
	app.Use(audit.CorrelationMiddleware())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(audit.CorrelationID(c))
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "corr-from-client")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "corr-from-client" {
		t.Fatalf("correlation body = %q", string(body))
	}
	if resp.Header.Get("X-Request-ID") != "corr-from-client" {
		t.Fatalf("response header = %q", resp.Header.Get("X-Request-ID"))
	}
}

func TestEmitWritesEventWithCorrelation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.AuditEvent{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	restore := audit.SetDBForTest(db)
	defer restore()

	app := fiber.New()
	app.Use(audit.CorrelationMiddleware())
	app.Post("/action", func(c *fiber.Ctx) error {
		audit.Emit(c, audit.Event{
			Action:       "provider.create",
			ResourceType: "provider",
			ResourceID:   "9",
		})
		return c.SendStatus(fiber.StatusCreated)
	})

	req := httptest.NewRequest("POST", "/action", nil)
	req.Header.Set("X-Request-ID", "emit-test-corr")
	if _, err := app.Test(req); err != nil {
		t.Fatalf("request: %v", err)
	}

	var rows []database.AuditEvent
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(rows))
	}
	if rows[0].Action != "provider.create" {
		t.Fatalf("action = %q", rows[0].Action)
	}
	if rows[0].CorrelationID != "emit-test-corr" {
		t.Fatalf("correlation_id = %q", rows[0].CorrelationID)
	}
}
