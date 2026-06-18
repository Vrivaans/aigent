package database

import (
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Tenant{}, &User{}); err != nil {
		t.Fatalf("automigrate User: %v", err)
	}
	if _, err := EnsureDefaultTenant(db); err != nil {
		t.Fatalf("EnsureDefaultTenant: %v", err)
	}
	return db
}

func TestSeedAdminUserCreatesFirstUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "admin-pass")

	db := openTestDB(t)
	if err := SeedAdminUser(db); err != nil {
		t.Fatalf("SeedAdminUser: %v", err)
	}

	var count int64
	if err := db.Model(&User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}

	// Second call is a no-op.
	if err := SeedAdminUser(db); err != nil {
		t.Fatalf("SeedAdminUser second call: %v", err)
	}
	if err := db.Model(&User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected still 1 user, got %d", count)
	}
}

func TestSeedAdminUserRequiresEnvWhenEmpty(t *testing.T) {
	os.Unsetenv("ADMIN_USERNAME")
	os.Unsetenv("ADMIN_PASSWORD")

	db := openTestDB(t)
	if err := SeedAdminUser(db); err == nil {
		t.Fatal("expected error when env vars missing and users empty")
	}
}

func TestAuthenticateUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "admin-pass")

	db := openTestDB(t)
	if err := SeedAdminUser(db); err != nil {
		t.Fatalf("SeedAdminUser: %v", err)
	}

	user, err := AuthenticateUser(db, "admin", "admin-pass")
	if err != nil {
		t.Fatalf("AuthenticateUser: %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("unexpected username %q", user.Username)
	}

	if _, err := AuthenticateUser(db, "admin", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if _, err := AuthenticateUser(db, "nobody", "admin-pass"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for unknown user, got %v", err)
	}
}

func TestSetUserRoles(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "admin-pass")

	db := openTestDB(t)
	if err := db.AutoMigrate(&Role{}, &RolePermission{}, &UserRole{}); err != nil {
		t.Fatalf("automigrate roles: %v", err)
	}
	if err := SeedRolesAndPermissions(db); err != nil {
		t.Fatalf("SeedRolesAndPermissions: %v", err)
	}

	user := User{Username: "newop", PasswordHash: "hash", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := SetUserRoles(db, user.ID, []string{"operator"}); err != nil {
		t.Fatalf("SetUserRoles: %v", err)
	}

	names, err := GetUserRoleNames(db, user.ID)
	if err != nil {
		t.Fatalf("GetUserRoleNames: %v", err)
	}
	if len(names) != 1 || names[0] != "operator" {
		t.Fatalf("expected [operator], got %v", names)
	}
}
