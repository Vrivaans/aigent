package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
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

func setupConfirmTestDB(t *testing.T) (*gorm.DB, uint, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&database.User{},
		&database.Agent{},
		&database.Session{},
		&database.PendingAction{},
		&database.AuditEvent{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	operator := database.User{Username: "operator1", PasswordHash: "x", IsActive: true}
	if err := db.Create(&operator).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	agent := database.Agent{Name: "General", IsDefault: true}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	session := database.Session{Title: "Test", AgentID: agent.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	toReject := database.PendingAction{
		SessionID:  session.ID,
		ToolName:   "sensitive_tool",
		Arguments:  `{}`,
		ToolCallID: "call_reject",
		Status:     "PENDING",
	}
	keepPending := database.PendingAction{
		SessionID:  session.ID,
		ToolName:   "other_tool",
		Arguments:  `{}`,
		ToolCallID: "call_keep",
		Status:     "PENDING",
	}
	if err := db.Create(&toReject).Error; err != nil {
		t.Fatalf("create pending reject: %v", err)
	}
	if err := db.Create(&keepPending).Error; err != nil {
		t.Fatalf("create pending keep: %v", err)
	}

	database.DB = db
	restoreAudit := audit.SetDBForTest(db)
	t.Cleanup(func() {
		restoreAudit()
		database.DB = nil
	})

	return db, operator.ID, toReject.ID
}

func TestHandleConfirmRejectSetsResolver(t *testing.T) {
	db, operatorID, pendingID := setupConfirmTestDB(t)
	handler := &handlers.ChatHandler{}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{
			UserID:   operatorID,
			Username: "operator1",
			Roles:    []string{"operator"},
		})
		return c.Next()
	})
	app.Post("/sessions/:id/confirm/:pending_id", handler.HandleConfirm)

	body, _ := json.Marshal(map[string]bool{"approved": false})
	req := httptest.NewRequest("POST", "/sessions/1/confirm/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(respBody))
	}

	var action database.PendingAction
	if err := db.First(&action, pendingID).Error; err != nil {
		t.Fatalf("load pending action: %v", err)
	}
	if action.Status != "REJECTED" {
		t.Fatalf("status = %q", action.Status)
	}
	if action.ResolvedByUserID == nil || *action.ResolvedByUserID != operatorID {
		t.Fatalf("resolved_by_user_id = %v", action.ResolvedByUserID)
	}
	if action.ResolvedAt == nil {
		t.Fatal("expected resolved_at to be set")
	}

	var auditRows []database.AuditEvent
	if err := db.Where("action = ?", "approval.reject").Find(&auditRows).Error; err != nil {
		t.Fatalf("find audit rows: %v", err)
	}
	if len(auditRows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(auditRows))
	}
	if auditRows[0].ActorUserID == nil || *auditRows[0].ActorUserID != operatorID {
		t.Fatalf("audit actor_user_id = %v", auditRows[0].ActorUserID)
	}
	fields := audit.ParseApprovalFields(auditRows[0].PayloadAfter)
	if fields == nil {
		t.Fatal("expected parsed approval fields in audit payload")
	}
	if fields.ResolvedByUserID == nil || *fields.ResolvedByUserID != operatorID {
		t.Fatalf("payload resolved_by_user_id = %v", fields.ResolvedByUserID)
	}
	if fields.ResolvedAt == "" {
		t.Fatal("expected resolved_at in audit payload")
	}
}

func TestHandleConfirmAuditLinksChatMessage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&database.User{},
		&database.Agent{},
		&database.Session{},
		&database.ChatMessage{},
		&database.PendingAction{},
		&database.AuditEvent{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	operator := database.User{Username: "operator1", PasswordHash: "x", IsActive: true}
	if err := db.Create(&operator).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	agent := database.Agent{Name: "General", IsDefault: true}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	session := database.Session{Title: "Audit link", AgentID: agent.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}
	toolCalls := `[{"id":"call_link","type":"function","function":{"name":"tool","arguments":"{}"}}]`
	msg := database.ChatMessage{
		SessionID:    session.ID,
		Role:         "assistant",
		Content:      "approve me",
		RawToolCalls: toolCalls,
	}
	if err := db.Create(&msg).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}
	pending := database.PendingAction{
		SessionID:  session.ID,
		ToolName:   "tool",
		Arguments:  `{}`,
		ToolCallID: "call_link",
		Status:     "PENDING",
	}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("create pending: %v", err)
	}

	database.DB = db
	restore := audit.SetDBForTest(db)
	t.Cleanup(func() {
		restore()
		database.DB = nil
	})

	handler := &handlers.ChatHandler{}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{
			UserID:   operator.ID,
			Username: "operator1",
			Roles:    []string{"operator"},
		})
		return c.Next()
	})
	app.Post("/sessions/:id/confirm/:pending_id", handler.HandleConfirm)

	body, _ := json.Marshal(map[string]bool{"approved": false})
	req := httptest.NewRequest("POST", "/sessions/1/confirm/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(respBody))
	}

	var auditRows []database.AuditEvent
	if err := db.Where("action = ?", "approval.reject").Find(&auditRows).Error; err != nil {
		t.Fatalf("find audit rows: %v", err)
	}
	if len(auditRows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(auditRows))
	}
	fields := audit.ParseApprovalFields(auditRows[0].PayloadAfter)
	if fields == nil || fields.ChatMessageID == nil || *fields.ChatMessageID != msg.ID {
		t.Fatalf("expected chat_message_id=%d in audit payload, got %+v", msg.ID, fields)
	}
}

func TestHandleGetHistoryExposesResolver(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.Agent{}, &database.Session{}, &database.ChatMessage{}, &database.PendingAction{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() { database.DB = nil })

	agent := database.Agent{Name: "General", IsDefault: true}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	session := database.Session{Title: "History", AgentID: agent.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	resolverID := uint(42)
	resolvedAt := time.Date(2026, 6, 18, 15, 0, 0, 0, time.UTC)
	toolCalls := `[{"id":"call_1","type":"function","function":{"name":"tool","arguments":"{}"}}]`
	msg := database.ChatMessage{
		SessionID:    session.ID,
		Role:         "assistant",
		Content:      "needs approval",
		RawToolCalls: toolCalls,
	}
	if err := db.Create(&msg).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}
	action := database.PendingAction{
		SessionID:        session.ID,
		ToolName:         "tool",
		Arguments:        `{}`,
		ToolCallID:       "call_1",
		Status:           "APPROVED",
		ResolvedByUserID: &resolverID,
		ResolvedAt:       &resolvedAt,
	}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("create resolved action: %v", err)
	}

	handler := &handlers.ChatHandler{}
	app := fiber.New()
	app.Get("/sessions/:id/chat", handler.HandleGetHistory)

	req := httptest.NewRequest("GET", "/sessions/1/chat", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var history []handlers.ChatMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
	if history[0].ResolvedByUserID == nil || *history[0].ResolvedByUserID != resolverID {
		t.Fatalf("resolved_by_user_id = %v", history[0].ResolvedByUserID)
	}
	if history[0].ResolvedAt == nil {
		t.Fatal("expected resolved_at in history response")
	}
}
