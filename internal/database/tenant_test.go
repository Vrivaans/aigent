package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&Tenant{},
		&User{},
		&Session{},
		&LLMProvider{},
		&Agent{},
		&HandsAIConfig{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func TestEnsureDefaultTenantCreatesDefault(t *testing.T) {
	db := openTenantTestDB(t)

	tenant, err := EnsureDefaultTenant(db)
	if err != nil {
		t.Fatalf("EnsureDefaultTenant: %v", err)
	}
	if tenant.Slug != DefaultTenantSlug {
		t.Fatalf("slug = %q", tenant.Slug)
	}

	again, err := EnsureDefaultTenant(db)
	if err != nil {
		t.Fatalf("EnsureDefaultTenant second call: %v", err)
	}
	if again.ID != tenant.ID {
		t.Fatalf("expected same tenant id, got %d vs %d", again.ID, tenant.ID)
	}
}

func TestBackfillTenantIDsAssignsDefault(t *testing.T) {
	db := openTenantTestDB(t)
	defaultTenant, err := EnsureDefaultTenant(db)
	if err != nil {
		t.Fatalf("EnsureDefaultTenant: %v", err)
	}

	user := User{Username: "tenant-user", PasswordHash: "hash", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	agent := Agent{Name: "Scoped", Description: "test"}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	provider := LLMProvider{Name: "p1", BaseURL: "http://localhost", APIKey: "x"}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	session := Session{Title: "s1", AgentID: agent.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}
	hands := HandsAIConfig{Username: "bridge", URL: "http://localhost", Token: "enc"}
	if err := db.Create(&hands).Error; err != nil {
		t.Fatalf("create handsai: %v", err)
	}

	if err := BackfillTenantIDs(db); err != nil {
		t.Fatalf("BackfillTenantIDs: %v", err)
	}

	assertTenantID := func(t *testing.T, label string, got *uint) {
		t.Helper()
		if got == nil || *got != defaultTenant.ID {
			t.Fatalf("%s tenant_id = %v, want %d", label, got, defaultTenant.ID)
		}
	}

	var reloadedUser User
	if err := db.First(&reloadedUser, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	assertTenantID(t, "user", reloadedUser.TenantID)

	var reloadedAgent Agent
	if err := db.First(&reloadedAgent, agent.ID).Error; err != nil {
		t.Fatalf("reload agent: %v", err)
	}
	assertTenantID(t, "agent", reloadedAgent.TenantID)

	var reloadedProvider LLMProvider
	if err := db.First(&reloadedProvider, provider.ID).Error; err != nil {
		t.Fatalf("reload provider: %v", err)
	}
	assertTenantID(t, "provider", reloadedProvider.TenantID)

	var reloadedSession Session
	if err := db.First(&reloadedSession, session.ID).Error; err != nil {
		t.Fatalf("reload session: %v", err)
	}
	assertTenantID(t, "session", reloadedSession.TenantID)

	var reloadedHands HandsAIConfig
	if err := db.First(&reloadedHands, hands.ID).Error; err != nil {
		t.Fatalf("reload handsai: %v", err)
	}
	assertTenantID(t, "hands_ai", reloadedHands.TenantID)
}

func TestResolveUserTenantIDUsesDefaultWhenUnset(t *testing.T) {
	db := openTenantTestDB(t)
	defaultTenant, err := EnsureDefaultTenant(db)
	if err != nil {
		t.Fatalf("EnsureDefaultTenant: %v", err)
	}

	user := User{Username: "no-tenant", PasswordHash: "hash", IsActive: true}
	tid, err := ResolveUserTenantID(db, &user)
	if err != nil {
		t.Fatalf("ResolveUserTenantID: %v", err)
	}
	if tid != defaultTenant.ID {
		t.Fatalf("tenant id = %d, want %d", tid, defaultTenant.ID)
	}
}
