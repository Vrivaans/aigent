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

	if err := autoMigrate(db); err != nil {
		return err
	}

	return nil
}

// autoMigrate run structure migrations safely and dynamically
func autoMigrate(db *gorm.DB) error {
	log.Println("Running AutoMigration for GORM models...")

	// 1. Migrate LLMProvider first (base for Agent)
	if err := db.AutoMigrate(&LLMProvider{}); err != nil {
		return err
	}

	// 2. Migrate Agent and its tools
	if err := db.AutoMigrate(&Agent{}, &AgentTool{}); err != nil {
		return err
	}

	// 3. Seed the default agent
	if err := SeedDefaultAgent(db); err != nil {
		return err
	}

	// Sincronizar secuencia de IDs de PostgreSQL (vital si se insertó con ID explícito 1)
	db.Exec(`SELECT setval('agents_id_seq', (SELECT COALESCE(MAX(id), 1) FROM agents));`)

	// 4. Migrate the rest (Session, Rules, etc.)
	return db.AutoMigrate(
		&Rule{},
		&Task{},
		&Session{},
		&ChatMessage{},
		&PendingAction{},
		&HandsAIConfig{},
		&McpStdioServer{},
		&McpStreamServer{},
	)
}

// SeedDefaultAgent ensures the "General" agent exists in the database
func SeedDefaultAgent(db *gorm.DB) error {
	var count int64
	db.Model(&Agent{}).Where("id = ?", 1).Count(&count)
	if count == 0 {
		log.Println("Seeding default 'General' agent...")
		agent := Agent{
			ID:          1,
			Name:        "General",
			Description: "Agente multipropósito con acceso completo a todas las herramientas configuradas.",
			IsDefault:   true,
		}
		if err := db.Create(&agent).Error; err != nil {
			return fmt.Errorf("failed to seed default agent: %w", err)
		}
	}
	return nil
}
