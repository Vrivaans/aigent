package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openRBACTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Role{}, &RolePermission{}, &UserRole{}); err != nil {
		t.Fatalf("automigrate rbac: %v", err)
	}
	return db
}

func TestSeedRolesAndPermissions(t *testing.T) {
	db := openRBACTestDB(t)
	if err := SeedRolesAndPermissions(db); err != nil {
		t.Fatalf("SeedRolesAndPermissions: %v", err)
	}

	var roleCount int64
	if err := db.Model(&Role{}).Count(&roleCount).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if roleCount != 4 {
		t.Fatalf("expected 4 roles, got %d", roleCount)
	}

	// Idempotent second call.
	if err := SeedRolesAndPermissions(db); err != nil {
		t.Fatalf("SeedRolesAndPermissions second call: %v", err)
	}
	if err := db.Model(&Role{}).Count(&roleCount).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if roleCount != 4 {
		t.Fatalf("expected still 4 roles, got %d", roleCount)
	}
}

func TestUserHasPermission(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "admin-pass")

	db := openRBACTestDB(t)
	if err := SeedAdminUser(db); err != nil {
		t.Fatalf("SeedAdminUser: %v", err)
	}
	if err := SeedRolesAndPermissions(db); err != nil {
		t.Fatalf("SeedRolesAndPermissions: %v", err)
	}

	var user User
	if err := db.Where("username = ?", "admin").First(&user).Error; err != nil {
		t.Fatalf("find admin user: %v", err)
	}

	ok, err := UserHasPermission(db, user.ID, "providers", "write")
	if err != nil {
		t.Fatalf("UserHasPermission admin: %v", err)
	}
	if !ok {
		t.Fatal("admin should have providers:write via *:* wildcard")
	}

	// Create viewer user with viewer role only.
	viewerRole := Role{}
	if err := db.Where("name = ?", "viewer").First(&viewerRole).Error; err != nil {
		t.Fatalf("find viewer role: %v", err)
	}
	viewer := User{Username: "viewer1", PasswordHash: "hash", IsActive: true}
	if err := db.Create(&viewer).Error; err != nil {
		t.Fatalf("create viewer user: %v", err)
	}
	if err := db.Create(&UserRole{UserID: viewer.ID, RoleID: viewerRole.ID}).Error; err != nil {
		t.Fatalf("assign viewer role: %v", err)
	}

	ok, err = UserHasPermission(db, viewer.ID, "chat", "read")
	if err != nil || !ok {
		t.Fatalf("viewer should have chat:read, ok=%v err=%v", ok, err)
	}

	ok, err = UserHasPermission(db, viewer.ID, "providers", "write")
	if err != nil {
		t.Fatalf("UserHasPermission viewer write: %v", err)
	}
	if ok {
		t.Fatal("viewer should not have providers:write")
	}
}
