package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMatchToolPattern(t *testing.T) {
	tests := []struct {
		pattern string
		tool    string
		want    bool
	}{
		{"odoo_*", "odoo_write", true},
		{"odoo_*", "odoo_read", true},
		{"odoo_*", "slack_post", false},
		{"*", "anything", true},
		{"exact_tool", "exact_tool", true},
		{"exact_tool", "exact_tool_x", false},
	}
	for _, tt := range tests {
		if got := MatchToolPattern(tt.pattern, tt.tool); got != tt.want {
			t.Fatalf("MatchToolPattern(%q, %q) = %v, want %v", tt.pattern, tt.tool, got, tt.want)
		}
	}
}

func TestToolMatchesApprovalPolicy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ApprovalPolicy{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := db.Create(&ApprovalPolicy{
		ToolPattern:      "odoo_*",
		Environment:      "*",
		RequiresApproval: true,
		MinRole:          "operator",
	}).Error; err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	matches, err := ToolMatchesApprovalPolicy(db, "odoo_create")
	if err != nil {
		t.Fatalf("ToolMatchesApprovalPolicy: %v", err)
	}
	if !matches {
		t.Fatal("expected odoo_create to match odoo_* policy")
	}

	matches, err = ToolMatchesApprovalPolicy(db, "safe_tool")
	if err != nil {
		t.Fatalf("ToolMatchesApprovalPolicy safe: %v", err)
	}
	if matches {
		t.Fatal("expected safe_tool not to match odoo_* policy")
	}
}
