package ai

import (
	"context"
	"testing"

	"aigent/internal/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPolicyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.ApprovalPolicy{}, &database.ToolPermission{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() { database.DB = nil })
	return db
}

func TestFindSensitiveToolCallsWithPolicyPattern(t *testing.T) {
	setupPolicyTestDB(t)
	if err := database.DB.Create(&database.ApprovalPolicy{
		ToolPattern:      "odoo_*",
		Environment:      "*",
		RequiresApproval: true,
		MinRole:          "operator",
	}).Error; err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	b := &Brain{Registry: NewToolRegistry()}
	b.Registry.Register(ToolDef{Name: "odoo_write", Sensitive: false})

	toolCalls := []ToolCall{
		{ID: "call_1", Function: FunctionCall{Name: "odoo_write"}},
	}
	sensitive := b.findSensitiveToolCalls(context.Background(), toolCalls, map[string]string{
		"odoo_write": "odoo_write",
	}, 1)
	if len(sensitive) != 1 {
		t.Fatalf("expected 1 sensitive call from policy, got %d", len(sensitive))
	}
}

func TestAlwaysAllowBypassesPolicyRequirement(t *testing.T) {
	setupPolicyTestDB(t)
	if err := database.DB.Create(&database.ApprovalPolicy{
		ToolPattern:      "odoo_*",
		Environment:      "*",
		RequiresApproval: true,
		MinRole:          "operator",
	}).Error; err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	oldChecker := permissionChecker
	permissionChecker = func(agentID uint, toolName string) bool {
		return toolName == "odoo_write"
	}
	t.Cleanup(func() { permissionChecker = oldChecker })

	b := &Brain{Registry: NewToolRegistry()}
	b.Registry.Register(ToolDef{Name: "odoo_write", Sensitive: false})

	toolCalls := []ToolCall{
		{ID: "call_1", Function: FunctionCall{Name: "odoo_write"}},
	}
	sensitive := b.findSensitiveToolCalls(context.Background(), toolCalls, map[string]string{
		"odoo_write": "odoo_write",
	}, 1)
	if len(sensitive) != 0 {
		t.Fatalf("expected always_allow to bypass policy, got %d sensitive calls", len(sensitive))
	}
}

func TestRegistrySensitiveStillWorksWithoutPolicy(t *testing.T) {
	setupPolicyTestDB(t)

	b := &Brain{Registry: NewToolRegistry()}
	b.Registry.Register(ToolDef{Name: "dangerous_tool", Sensitive: true})

	toolCalls := []ToolCall{
		{ID: "call_1", Function: FunctionCall{Name: "dangerous_tool"}},
	}
	sensitive := b.findSensitiveToolCalls(context.Background(), toolCalls, map[string]string{
		"dangerous_tool": "dangerous_tool",
	}, 1)
	if len(sensitive) != 1 {
		t.Fatalf("expected registry sensitive default, got %d", len(sensitive))
	}
}
