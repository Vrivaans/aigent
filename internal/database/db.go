package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// ConnectDB initilizes the standard postgres connection using GORM
func ConnectDB(cfg Config) error {
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable" // Default para desarrollo local
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to local database: %w", err)
	}

	DB = db
	log.Println("Connected to PostgreSQL successfully")

	return autoMigrate(db)
}

// autoMigrate run structure migrations safely and dynamically
func autoMigrate(db *gorm.DB) error {
	log.Println("Running AutoMigration for GORM models...")
	return DB.AutoMigrate(
		&Rule{},
		&Task{},
		&Session{},
		&ChatMessage{},
		&PendingAction{},
		&LLMProvider{},
		&HandsAIConfig{},
	)
}
