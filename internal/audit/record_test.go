package audit_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aigent/internal/audit"
	"aigent/internal/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.AuditEvent{}); err != nil {
		t.Fatalf("automigrate AuditEvent: %v", err)
	}
	return db
}

func TestRecordCreatesRow(t *testing.T) {
	db := openAuditTestDB(t)
	restore := audit.SetDBForTest(db)
	defer restore()

	before := `{"name":"old"}`
	after := `{"name":"new"}`
	actorID := uint(7)
	sessionID := uint(3)
	at := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)

	err := audit.Record(context.Background(), audit.Event{
		ActorUserID:   &actorID,
		Action:        "provider.create",
		ResourceType:  "provider",
		ResourceID:    "42",
		SessionID:     &sessionID,
		IP:            "127.0.0.1",
		UserAgent:     "test-agent/1.0",
		PayloadBefore: &before,
		PayloadAfter:  &after,
		CorrelationID: "corr-abc",
		OccurredAt:    at,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var rows []database.AuditEvent
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("find audit events: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row.Action != "provider.create" {
		t.Fatalf("action = %q", row.Action)
	}
	if row.ResourceType != "provider" || row.ResourceID != "42" {
		t.Fatalf("resource = %s/%s", row.ResourceType, row.ResourceID)
	}
	if row.ActorUserID == nil || *row.ActorUserID != 7 {
		t.Fatalf("actor_user_id = %v", row.ActorUserID)
	}
	if row.CorrelationID != "corr-abc" {
		t.Fatalf("correlation_id = %q", row.CorrelationID)
	}
	if !row.OccurredAt.Equal(at) {
		t.Fatalf("occurred_at = %v", row.OccurredAt)
	}
}

func TestRecordRequiresActionAndResourceType(t *testing.T) {
	db := openAuditTestDB(t)
	restore := audit.SetDBForTest(db)
	defer restore()

	err := audit.Record(context.Background(), audit.Event{Action: "", ResourceType: "provider"})
	if err == nil {
		t.Fatal("expected error for empty action")
	}

	err = audit.Record(context.Background(), audit.Event{Action: "provider.create", ResourceType: ""})
	if err == nil {
		t.Fatal("expected error for empty resource_type")
	}
}

func TestNoUpdateHelperExported(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}

	fset := token.NewFileSet()
	for _, name := range []string{"event.go", "record.go"} {
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !fn.Name.IsExported() {
				continue
			}
			n := fn.Name.Name
			if strings.HasPrefix(n, "Update") || strings.HasPrefix(n, "Delete") {
				t.Fatalf("audit package must not export mutating helper %q", n)
			}
		}
	}
}
