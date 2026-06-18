package database

import (
	"errors"
	"fmt"
	"log"
	"os"

	"aigent/internal/auth"

	"gorm.io/gorm"
)

var ErrInvalidCredentials = errors.New("invalid username or password")

// SeedAdminUser creates the first admin user from ADMIN_USERNAME / ADMIN_PASSWORD when the table is empty.
func SeedAdminUser(db *gorm.DB) error {
	var count int64
	if err := db.Model(&User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	adminUser := os.Getenv("ADMIN_USERNAME")
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminUser == "" || adminPass == "" {
		return fmt.Errorf("users table is empty: ADMIN_USERNAME and ADMIN_PASSWORD must be set for initial seed")
	}

	hash, err := auth.HashPassword(adminPass)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	var tenantID *uint
	if id, err := DefaultTenantID(db); err == nil {
		tenantID = &id
	}

	user := User{
		TenantID:     tenantID,
		Username:     adminUser,
		PasswordHash: hash,
		IsActive:     true,
	}
	if err := db.Create(&user).Error; err != nil {
		return fmt.Errorf("failed to seed admin user: %w", err)
	}

	log.Printf("Seeded initial admin user %q (id=%d)", adminUser, user.ID)
	return nil
}

// AuthenticateUser validates credentials against the users table.
func AuthenticateUser(db *gorm.DB, username, password string) (*User, error) {
	var user User
	err := db.Where("username = ? AND is_active = ?", username, true).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !auth.CheckPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	return &user, nil
}

// SetUserRoles replaces all role assignments for a user.
func SetUserRoles(db *gorm.DB, userID uint, roleNames []string) error {
	if len(roleNames) == 0 {
		return fmt.Errorf("at least one role is required")
	}

	var roles []Role
	if err := db.Where("name IN ?", roleNames).Find(&roles).Error; err != nil {
		return err
	}
	if len(roles) != len(roleNames) {
		return fmt.Errorf("one or more roles were not found")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&UserRole{}).Error; err != nil {
			return err
		}
		for _, role := range roles {
			if err := tx.Create(&UserRole{UserID: userID, RoleID: role.ID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListRoles returns all roles ordered by name.
func ListRoles(db *gorm.DB) ([]Role, error) {
	var roles []Role
	if err := db.Order("name asc").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}
