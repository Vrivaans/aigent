package handlers

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"aigent/internal/database"
)

func TestValidateExportRowCount(t *testing.T) {
	if err := validateExportRowCount(100); err != nil {
		t.Fatalf("expected nil for 100 rows, got %v", err)
	}
	if err := validateExportRowCount(auditExportMaxRows); err != nil {
		t.Fatalf("expected nil at max, got %v", err)
	}
	if err := validateExportRowCount(auditExportMaxRows + 1); err == nil {
		t.Fatal("expected error when exceeding max rows")
	}
}

func TestAuditEventsToCSV(t *testing.T) {
	actorID := uint(7)
	resolverID := uint(3)
	msgID := uint(55)
	resolvedAt := "2026-06-01T11:00:00Z"
	payload := `{"id":1,"session_id":10,"tool_name":"odoo_write","tool_call_id":"call_1","chat_message_id":55,"status":"APPROVED","resolved_by_user_id":3,"resolved_at":"2026-06-01T11:00:00Z"}`
	rows := []database.AuditEvent{
		{
			ID:           1,
			OccurredAt:   mustParseTime(t, "2026-06-01T10:00:00Z"),
			ActorUserID:  &actorID,
			Action:       "provider.create",
			ResourceType: "provider",
			ResourceID:   "42",
		},
		{
			ID:           2,
			OccurredAt:   mustParseTime(t, "2026-06-01T11:00:00Z"),
			ActorUserID:  &actorID,
			Action:       "approval.approve",
			ResourceType: "pending_action",
			ResourceID:   "1",
			PayloadAfter: &payload,
		},
	}
	csvBytes, err := auditEventsToCSV(rows)
	if err != nil {
		t.Fatalf("auditEventsToCSV: %v", err)
	}
	out := string(csvBytes)
	if !containsAll(out, "id,occurred_at", "provider.create", "42", "approval_tool_name", "odoo_write", "APPROVED") {
		t.Fatalf("unexpected csv: %q", out)
	}
	if !containsAll(out, strconv.FormatUint(uint64(resolverID), 10), resolvedAt, "10", strconv.FormatUint(uint64(msgID), 10)) {
		t.Fatalf("csv missing approval resolution fields: %q", out)
	}
}

func mustParseTime(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed.UTC()
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
