package handlers_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"aigent/internal/database"
	"aigent/internal/handlers"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetApprovalHistoryReturnsUsername(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.User{}, &database.Agent{}, &database.Session{}, &database.PendingAction{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	resolver := database.User{Username: "alice", PasswordHash: "x", IsActive: true}
	if err := db.Create(&resolver).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	agent := database.Agent{Name: "General", IsDefault: true}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	session := database.Session{Title: "Done", AgentID: agent.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}
	resolvedAt := time.Date(2026, 6, 18, 16, 0, 0, 0, time.UTC)
	action := database.PendingAction{
		SessionID:        session.ID,
		ToolName:         "tool_x",
		Arguments:        `{}`,
		ToolCallID:       "call_x",
		Status:           "APPROVED",
		ResolvedByUserID: &resolver.ID,
		ResolvedAt:       &resolvedAt,
	}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("create action: %v", err)
	}

	database.DB = db
	t.Cleanup(func() { database.DB = nil })

	handler := &handlers.ChatHandler{}
	app := fiber.New()
	app.Get("/approvals/history", handler.GetApprovalHistory)

	req := httptest.NewRequest("GET", "/approvals/history", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var items []handlers.ApprovalHistoryItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ResolvedByUsername != "alice" {
		t.Fatalf("resolved_by_username = %q", items[0].ResolvedByUsername)
	}
	if items[0].Status != "APPROVED" {
		t.Fatalf("status = %q", items[0].Status)
	}
}
