package database

import (
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"
)

const (
	DefaultTenantSlug = "default"
	DefaultTenantName = "Default"
)

// EnsureDefaultTenant creates the default tenant row when missing.
func EnsureDefaultTenant(db *gorm.DB) (*Tenant, error) {
	var tenant Tenant
	err := db.Where("slug = ?", DefaultTenantSlug).First(&tenant).Error
	if err == nil {
		return &tenant, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	tenant = Tenant{Slug: DefaultTenantSlug, Name: DefaultTenantName}
	if err := db.Create(&tenant).Error; err != nil {
		return nil, fmt.Errorf("create default tenant: %w", err)
	}
	log.Printf("Seeded default tenant %q (id=%d)", DefaultTenantSlug, tenant.ID)
	return &tenant, nil
}

// DefaultTenantID returns the id of the default tenant, creating it if needed.
func DefaultTenantID(db *gorm.DB) (uint, error) {
	tenant, err := EnsureDefaultTenant(db)
	if err != nil {
		return 0, err
	}
	return tenant.ID, nil
}

// BackfillTenantIDs assigns the default tenant to core rows with NULL tenant_id.
func BackfillTenantIDs(db *gorm.DB) error {
	tenantID, err := DefaultTenantID(db)
	if err != nil {
		return err
	}

	tables := []string{"users", "sessions", "llm_providers", "agents", "hands_ai_configs"}
	for _, table := range tables {
		res := db.Exec(
			fmt.Sprintf("UPDATE %s SET tenant_id = ? WHERE tenant_id IS NULL", table),
			tenantID,
		)
		if res.Error != nil {
			return fmt.Errorf("backfill tenant_id on %s: %w", table, res.Error)
		}
		if res.RowsAffected > 0 {
			log.Printf("Backfilled tenant_id=%d on %d row(s) in %s", tenantID, res.RowsAffected, table)
		}
	}
	return nil
}

// ResolveUserTenantID returns the user's tenant or the default tenant id.
func ResolveUserTenantID(db *gorm.DB, user *User) (uint, error) {
	if user == nil {
		return 0, fmt.Errorf("user is nil")
	}
	if user.TenantID != nil && *user.TenantID > 0 {
		return *user.TenantID, nil
	}
	return DefaultTenantID(db)
}
