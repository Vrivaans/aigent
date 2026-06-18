package database

import (
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"
)

type permissionSpec struct {
	resource string
	action   string
}

var defaultRolePermissions = map[string][]permissionSpec{
	"admin": {
		{resource: "*", action: "*"},
	},
	"operator": {
		{resource: "agents", action: "read"},
		{resource: "agents", action: "write"},
		{resource: "providers", action: "read"},
		{resource: "providers", action: "write"},
		{resource: "mcp", action: "read"},
		{resource: "mcp", action: "write"},
		{resource: "permissions", action: "write"},
		{resource: "chat", action: "read"},
		{resource: "chat", action: "write"},
		{resource: "workflows", action: "read"},
		{resource: "workflows", action: "write"},
		{resource: "tasks", action: "read"},
		{resource: "tasks", action: "write"},
		{resource: "rules", action: "read"},
		{resource: "rules", action: "write"},
	},
	"auditor": {
		{resource: "audit", action: "read"},
		{resource: "audit", action: "export"},
		{resource: "chat", action: "read"},
		{resource: "agents", action: "read"},
		{resource: "permissions", action: "read"},
	},
	"viewer": {
		{resource: "chat", action: "read"},
		{resource: "agents", action: "read"},
	},
}

var defaultRoles = []Role{
	{Name: "admin", Description: "Full system access"},
	{Name: "operator", Description: "Configure providers, MCP, agents, and run chat"},
	{Name: "auditor", Description: "Read audit trail and view operational data"},
	{Name: "viewer", Description: "Read-only chat and agent visibility"},
}

// SeedRolesAndPermissions creates default roles, permissions, and assigns admin to the first user.
func SeedRolesAndPermissions(db *gorm.DB) error {
	var count int64
	if err := db.Model(&Role{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return assignAdminRoleToBootstrapUser(db)
	}

	for _, roleDef := range defaultRoles {
		role := roleDef
		if err := db.Create(&role).Error; err != nil {
			return fmt.Errorf("seed role %q: %w", role.Name, err)
		}

		for _, perm := range defaultRolePermissions[role.Name] {
			rp := RolePermission{
				RoleID:   role.ID,
				Resource: perm.resource,
				Action:   perm.action,
			}
			if err := db.Create(&rp).Error; err != nil {
				return fmt.Errorf("seed permission %s:%s for role %q: %w", perm.resource, perm.action, role.Name, err)
			}
		}
	}

	log.Println("Seeded default RBAC roles and permissions (admin, operator, auditor, viewer)")
	return assignAdminRoleToBootstrapUser(db)
}

func assignAdminRoleToBootstrapUser(db *gorm.DB) error {
	var adminRole Role
	if err := db.Where("name = ?", "admin").First(&adminRole).Error; err != nil {
		return fmt.Errorf("admin role not found: %w", err)
	}

	var bootstrapUser User
	if err := db.Order("id asc").First(&bootstrapUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	var existing int64
	if err := db.Model(&UserRole{}).Where("user_id = ?", bootstrapUser.ID).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}

	if err := db.Create(&UserRole{UserID: bootstrapUser.ID, RoleID: adminRole.ID}).Error; err != nil {
		return fmt.Errorf("assign admin role to user %d: %w", bootstrapUser.ID, err)
	}

	log.Printf("Assigned admin role to bootstrap user %q (id=%d)", bootstrapUser.Username, bootstrapUser.ID)
	return nil
}

// GetUserRoleNames returns role names assigned to a user.
func GetUserRoleNames(db *gorm.DB, userID uint) ([]string, error) {
	var roles []Role
	err := db.Table("roles").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names, nil
}

// UserHasPermission reports whether the user has the given resource/action via any assigned role.
// Supports wildcard permissions: *:*, resource:*, *:action.
func UserHasPermission(db *gorm.DB, userID uint, resource, action string) (bool, error) {
	var count int64
	err := db.Model(&RolePermission{}).
		Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ?", userID).
		Where(
			"(role_permissions.resource = ? AND role_permissions.action = ?) OR "+
				"(role_permissions.resource = ? AND role_permissions.action = ?) OR "+
				"(role_permissions.resource = ? AND role_permissions.action = ?) OR "+
				"(role_permissions.resource = ? AND role_permissions.action = ?)",
			resource, action, "*", "*", resource, "*", "*", action,
		).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
