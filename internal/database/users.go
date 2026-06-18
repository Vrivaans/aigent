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

	user := User{
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
