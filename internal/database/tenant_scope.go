package database

import "gorm.io/gorm"

// ForTenant scopes queries to rows owned by tenantID.
func ForTenant(db *gorm.DB, tenantID uint) *gorm.DB {
	if tenantID == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("tenant_id = ?", tenantID)
}

// BelongsToTenant reports whether a row's tenant_id matches the request tenant.
func BelongsToTenant(recordTenantID *uint, tenantID uint) bool {
	if tenantID == 0 || recordTenantID == nil || *recordTenantID == 0 {
		return false
	}
	return *recordTenantID == tenantID
}

// TenantPtr returns a pointer suitable for model TenantID fields.
func TenantPtr(tenantID uint) *uint {
	if tenantID == 0 {
		return nil
	}
	id := tenantID
	return &id
}
