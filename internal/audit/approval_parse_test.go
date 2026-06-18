package audit

import (
	"encoding/json"
	"testing"
	"time"

	"aigent/internal/database"
)

func TestApprovalPayloadIncludesResolutionFields(t *testing.T) {
	resolverID := uint(5)
	resolvedAt := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	msgID := uint(99)
	payload := ApprovalPayload(database.PendingAction{
		ID:               1,
		SessionID:        10,
		ToolName:         "odoo_write",
		ToolCallID:       "call_abc",
		Status:           "APPROVED",
		ResolvedByUserID: &resolverID,
		ResolvedAt:       &resolvedAt,
	}, &msgID)
	if payload == nil {
		t.Fatal("expected payload")
	}

	fields := parseApprovalPayload(payload)
	if fields == nil {
		t.Fatal("expected parsed fields")
	}
	if fields.ToolName != "odoo_write" || fields.Status != "APPROVED" {
		t.Fatalf("unexpected fields: %+v", fields)
	}
	if fields.SessionID != 10 {
		t.Fatalf("session_id = %d", fields.SessionID)
	}
	if fields.ChatMessageID == nil || *fields.ChatMessageID != 99 {
		t.Fatalf("chat_message_id = %v", fields.ChatMessageID)
	}
	if fields.ResolvedByUserID == nil || *fields.ResolvedByUserID != 5 {
		t.Fatalf("resolved_by_user_id = %v", fields.ResolvedByUserID)
	}
	if fields.ResolvedAt == "" {
		t.Fatal("expected resolved_at")
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(*payload), &raw); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if raw["tool_call_id"] != "call_abc" {
		t.Fatalf("tool_call_id = %v", raw["tool_call_id"])
	}
}
