package database

import (
	"fmt"
	"log"
	"os"
	"strings"

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

// validatePostgresURI catches incomplete URIs like postgresql://user:pass (no @host).
// Keyword DSNs (host=... user=...) are left unchanged.
func validatePostgresURI(s string) error {
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "postgres://") && !strings.HasPrefix(lower, "postgresql://") {
		return nil
	}
	idx := strings.Index(s, "://")
	if idx < 0 {
		return nil
	}
	afterScheme := s[idx+3:]
	if !strings.Contains(afterScheme, "@") {
		return fmt.Errorf(
			`invalid DATABASE_URL: URI must include "@hostname" after user and password (e.g. postgresql://user:pass@db-host:5432/dbname); ` +
				`if the password contains ":" or "@", percent-encode those characters in the userinfo part`,
		)
	}
	return nil
}

// ConnectDB initilizes the standard postgres connection using GORM
func ConnectDB(cfg Config) error {
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable" // Default para desarrollo local
	}

	dsnFromEnv := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	dsnFromParts := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	var db *gorm.DB
	var err error

	if dsnFromEnv != "" {
		if verr := validatePostgresURI(dsnFromEnv); verr != nil {
			log.Printf("DATABASE_URL: %v — using DB_* variables instead", verr)
			dsnFromEnv = ""
		}
	}

	if dsnFromEnv != "" {
		db, err = gorm.Open(postgres.Open(dsnFromEnv), &gorm.Config{})
		if err != nil {
			log.Printf("DATABASE_URL failed (%v), falling back to DB_* variables", err)
			db, err = gorm.Open(postgres.Open(dsnFromParts), &gorm.Config{})
		}
	} else {
		db, err = gorm.Open(postgres.Open(dsnFromParts), &gorm.Config{})
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
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
