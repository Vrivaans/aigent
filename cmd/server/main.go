package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	"aigent/internal/ai"
	"aigent/internal/auth"
	"aigent/internal/database"
	"aigent/internal/handlers"
	"aigent/internal/handsai"
	"aigent/internal/scheduler"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Cargar variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Println("Note: No .env file found, using system environment variables")
	}

	// 1.5 Validar llave de cifrado y credenciales
	encryptionKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(encryptionKey) != 32 {
		log.Fatalf("FATAL: DB_ENCRYPTION_KEY must be exactly 32 characters long (for AES-256). Current length: %d", len(encryptionKey))
	}
	adminUser := os.Getenv("ADMIN_USERNAME")
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminUser == "" || adminPass == "" {
		log.Fatal("FATAL: ADMIN_USERNAME and ADMIN_PASSWORD must be set in .env")
	}

	// 2. Initializar Base de Datos
	dbCfg := database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "aigent"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
	if err := database.ConnectDB(dbCfg); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 3. Inicializar integraciones (HandsAI y LLM)
	handsaiCfg := handsai.Config{
		BaseURL: getEnv("HANDSAI_URL", "http://localhost:8080/mcp"),
		Token:   getEnv("HANDSAI_TOKEN", ""),
	}
	
	brain := ai.NewBrain(
		"", 
		"", 
		handsaiCfg,
		nil,
	)

	// 4. Levantar Cron Worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go scheduler.StartCronWorker(ctx, brain)

	// 5. Inicializar Fiber App y Rutas
	app := fiber.New()
	app.Use(cors.New())
	app.Use(logger.New())

	api := app.Group("/api")

	// Public routes
	api.Post("/login", handlers.HandleLogin)

	api.Get("/debug/tools", func(c *fiber.Ctx) error {
		raw, err := brain.HandsAI.GetTools(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		var parsed interface{}
		json.Unmarshal(raw, &parsed)
		return c.JSON(fiber.Map{
			"status": "ok",
			"raw": string(raw),
			"parsed": parsed,
		})
	})

	// Protected routes (require JWT)
	api.Use(auth.NewAuthMiddleware())

	api.Get("/active-tools", func(c *fiber.Ctx) error {
		_ = brain.SyncTools(c.Context())
		return c.JSON(brain.Registry.List())
	})
	chatHandler := &handlers.ChatHandler{Brain: brain}
	api.Get("/sessions", chatHandler.GetSessions)
	api.Post("/sessions", chatHandler.CreateSession)
	api.Post("/sessions/:id/chat", chatHandler.HandleChat)
	api.Post("/sessions/:id/confirm/:pending_id", chatHandler.HandleConfirm)
	api.Get("/sessions/:id/chat", chatHandler.HandleGetHistory)

	// LLM Provider Management
	api.Get("/providers", handlers.HandleListProviders)
	api.Post("/providers", handlers.HandleCreateProvider)
	api.Patch("/providers/:id", handlers.HandleUpdateProvider)
	api.Patch("/providers/:id/set-default", handlers.HandleSetDefaultProvider)
	api.Delete("/providers/:id", handlers.HandleDeleteProvider)
	api.Post("/providers/test", handlers.HandleTestProviderConfig)
	api.Post("/providers/:id/test", handlers.HandleTestProvider)

	taskHandler := &handlers.TaskHandler{}
	api.Get("/tasks", taskHandler.GetTasks)
	api.Delete("/tasks/:id", taskHandler.DeleteTask)

	ruleHandler := &handlers.RuleHandler{}
	api.Get("/rules", ruleHandler.GetRules)
	api.Post("/rules", ruleHandler.CreateRule)
	api.Delete("/rules/:id", ruleHandler.DeleteRule)

	// Serve Static Angular Files
	app.Static("/", "./web/dist/web/browser")

	// SPA Catch-all
	app.Get("/*", func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "API route not found"})
		}
		return c.SendFile("./web/dist/web/browser/index.html")
	})

	// 6. Iniciar Servidor
	port := getEnv("PORT", "3000")
	log.Printf("🚀 Starting AIgent Server on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Fiber failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
