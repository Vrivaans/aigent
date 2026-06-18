package handlers

import (
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
	rows := []database.AuditEvent{
		{
			ID:           1,
			OccurredAt:   mustParseTime(t, "2026-06-01T10:00:00Z"),
			ActorUserID:  &actorID,
			Action:       "provider.create",
			ResourceType: "provider",
			ResourceID:   "42",
		},
	}
	csvBytes, err := auditEventsToCSV(rows)
	if err != nil {
		t.Fatalf("auditEventsToCSV: %v", err)
	}
	out := string(csvBytes)
	if !containsAll(out, "id,occurred_at", "provider.create", "42") {
		t.Fatalf("unexpected csv: %q", out)
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
