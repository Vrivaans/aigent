package main

import (
	"context"
	"log"
	"os"
	"strings"

	"aigent/internal/ai"
	"aigent/internal/auth"
	"aigent/internal/database"
	"aigent/internal/handlers"
	"aigent/internal/handsai"
	"aigent/internal/mcpstdio"
	"aigent/internal/scheduler"
	"aigent/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

func main() {
	//go run cmd/server/main.go
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
	// La configuración de HandsAI viene EXCLUSIVAMENTE de la base de datos.
	// No hay fallback a variables de entorno — debe configurarse desde la UI.
	handsaiCfg := handsai.Config{}

	var handsaiDB database.HandsAIConfig
	if err := database.DB.First(&handsaiDB).Error; err == nil && handsaiDB.URL != "" {
		plainToken, decErr := utils.Decrypt(handsaiDB.Token, encryptionKey)
		if decErr != nil {
			log.Printf("⚠️  Failed to decrypt HandsAI token from DB: %v. HandsAI will be disabled.", decErr)
		} else {
			handsaiCfg.BaseURL = handsaiDB.URL
			handsaiCfg.Token = plainToken
			log.Printf("📦 HandsAI config loaded from database: %s", handsaiCfg.BaseURL)
		}
	} else {
		log.Println("ℹ️  No HandsAI config found in database. Configure it from the Providers page.")
	}

	brain := ai.NewBrain(
		"",
		"",
		handsaiCfg,
		nil,
	)

	mcpStdioMgr := mcpstdio.NewManager()
	brain.McpStdio = mcpStdioMgr
	mcpStdioMgr.ReloadFromDB(context.Background())
	defer mcpStdioMgr.CloseAll()

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
		reqCtx := c.Context()
		_ = brain.SyncTools(reqCtx)

		var regList []fiber.Map
		for _, t := range brain.Registry.List() {
			regList = append(regList, fiber.Map{
				"name":        t.Name,
				"description": t.Description,
				"sensitive":   t.Sensitive,
			})
		}

		handsaiRaw := ""
		handsaiErr := ""
		if brain.HandsAI != nil && brain.HandsAI.IsConfigured() {
			raw, err := brain.HandsAI.GetTools(reqCtx)
			if err != nil {
				handsaiErr = err.Error()
			} else {
				handsaiRaw = string(raw)
			}
		} else {
			handsaiErr = "handsai not configured"
		}

		var stdioDbg []fiber.Map
		if brain.McpStdio != nil {
			for _, e := range brain.McpStdio.ListEntries() {
				entry := fiber.Map{
					"alias":      e.Alias,
					"server_id":  e.ID,
					"connected":  e.Session != nil,
					"tool_count": 0,
					"list_error": "",
				}
				if e.Session != nil {
					tools, err := e.Session.ListTools(reqCtx)
					if err != nil {
						entry["list_error"] = err.Error()
					} else {
						entry["tool_count"] = len(tools)
					}
				}
				stdioDbg = append(stdioDbg, entry)
			}
		}

		return c.JSON(fiber.Map{
			"status":         "ok",
			"registry_tools": regList,
			"handsai": fiber.Map{
				"raw":   handsaiRaw,
				"error": handsaiErr,
			},
			"mcp_stdio": stdioDbg,
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
	api.Delete("/sessions/:id", chatHandler.DeleteSession)
	api.Patch("/sessions/:id/agent", chatHandler.UpdateSessionAgent)
	api.Post("/sessions/:id/chat", chatHandler.HandleChat)
	api.Post("/sessions/:id/confirm/:pending_id", chatHandler.HandleConfirm)
	api.Get("/sessions/:id/chat", chatHandler.HandleGetHistory)

	// LLM Provider Management
	agentHandler := &handlers.AgentHandler{}
	admin := api.Group("/admin")

	api.Get("/providers", handlers.HandleListProviders)
	api.Post("/providers", handlers.HandleCreateProvider)
	api.Patch("/providers/:id", handlers.HandleUpdateProvider)
	api.Patch("/providers/:id/set-default", handlers.HandleSetDefaultProvider)
	api.Delete("/providers/:id", handlers.HandleDeleteProvider)
	api.Post("/providers/test", handlers.HandleTestProviderConfig)
	api.Post("/providers/:id/test", handlers.HandleTestProvider)

	// Agent management
	admin.Get("/agents", agentHandler.GetAgents)
	admin.Post("/agents", agentHandler.CreateAgent)
	admin.Put("/agents/:id", agentHandler.UpdateAgent)
	admin.Delete("/agents/:id", agentHandler.DeleteAgent)

	// HandsAI Config Management
	configHandler := &handlers.ConfigHandler{Brain: brain}
	api.Get("/config/handsai", configHandler.GetHandsAIConfig)
	api.Patch("/config/handsai", configHandler.UpdateHandsAIConfig)
	api.Delete("/config/handsai", configHandler.DeleteHandsAIConfig)

	mcpStdioHandler := &handlers.McpStdioConfigHandler{Brain: brain, Manager: mcpStdioMgr}
	api.Get("/config/mcp-stdio", mcpStdioHandler.List)
	api.Post("/config/mcp-stdio", mcpStdioHandler.Create)
	api.Post("/config/mcp-stdio/test", mcpStdioHandler.TestDryRun)
	api.Patch("/config/mcp-stdio/:id", mcpStdioHandler.Update)
	api.Delete("/config/mcp-stdio/:id", mcpStdioHandler.Delete)
	api.Post("/config/mcp-stdio/:id/test", mcpStdioHandler.TestSaved)

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
