package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"aigent/internal/ai"
	"aigent/internal/audit"
	"aigent/internal/auth"
	"aigent/internal/database"
	"aigent/internal/handlers"
	"aigent/internal/handsai"
	"aigent/internal/mcpstdio"
	"aigent/internal/mcpstream"
	"aigent/internal/scheduler"
	"aigent/internal/secrets"
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

	// 1.5 Validar secretos y credenciales
	if err := auth.ValidateStartupSecrets(); err != nil {
		log.Fatalf("FATAL: %v", err)
	}
	encryptionKey := secrets.DBEncryptionKey()
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

	auth.PermissionChecker = func(userID uint, resource, action string) (bool, error) {
		return database.UserHasPermission(database.DB, userID, resource, action)
	}

	auth.TenantResolver = func(userID uint, claimTenantID uint) (uint, error) {
		if claimTenantID > 0 {
			return claimTenantID, nil
		}
		var user database.User
		if err := database.DB.First(&user, userID).Error; err != nil {
			return database.DefaultTenantID(database.DB)
		}
		return database.ResolveUserTenantID(database.DB, &user)
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

	mcpStreamMgr := mcpstream.NewManager()
	brain.McpStream = mcpStreamMgr
	mcpStreamMgr.ReloadFromDB(context.Background())
	defer mcpStreamMgr.CloseAll()

	initCtx, initCancel := context.WithTimeout(context.Background(), 60*time.Second)
	brain.ReloadMCPIntegrations(initCtx)
	if err := brain.SyncTools(initCtx); err != nil {
		log.Printf("⚠️ Initial SyncTools: %v", err)
	}
	initCancel()

	// 3.5 Inicializar motor RuleGo y cargar workflows activos
	ai.SetGlobalBrain(brain)
	if err := ai.ReloadWorkflows(); err != nil {
		log.Printf("⚠️ RuleGo Workflows initialization failed: %v", err)
	}

	// 3.6 Durable Execution: recuperar runs/tareas interrumpidas por un reinicio
	// y arrancar el worker de reconciliación (aprobaciones resueltas, zombies).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ai.StartDurableWorker(ctx, brain)

	// 4. Levantar Cron Worker
	go scheduler.StartCronWorker(ctx, brain)

	// 5. Inicializar Fiber App y Rutas
	app := fiber.New()
	app.Use(cors.New())
	app.Use(logger.New())
	app.Use(audit.CorrelationMiddleware())

	api := app.Group("/api")

	// Public routes
	api.Post("/login", handlers.HandleLogin)

	api.Get("/debug/tools", func(c *fiber.Ctx) error {
		reqCtx := c.Context()
		brain.ReloadMCPIntegrations(reqCtx)
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

		var streamDbg []fiber.Map
		if brain.McpStream != nil {
			for _, e := range brain.McpStream.ListEntries() {
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
				streamDbg = append(streamDbg, entry)
			}
		}

		return c.JSON(fiber.Map{
			"status":         "ok",
			"registry_tools": regList,
			"handsai": fiber.Map{
				"raw":   handsaiRaw,
				"error": handsaiErr,
			},
			"mcp_stdio":  stdioDbg,
			"mcp_stream": streamDbg,
		})
	})

	// Protected routes (require JWT)
	api.Use(auth.NewAuthMiddleware())
	api.Use(auth.TenantMiddleware())

	permRead := auth.RequirePermissionMiddleware
	permWrite := auth.RequirePermissionMiddleware

	api.Get("/active-tools", permRead("agents", "read"), func(c *fiber.Ctx) error {
		ctx := c.Context()
		if c.Query("refresh") == "true" || c.Query("sync") == "true" {
			brain.ReloadMCPIntegrations(ctx)
			_ = brain.SyncTools(ctx)
		}
		return c.JSON(brain.Registry.List())
	})

	chatHandler := &handlers.ChatHandler{Brain: brain}
	chatRead := api.Group("", permRead("chat", "read"))
	chatWrite := api.Group("", permWrite("chat", "write"))

	chatRead.Get("/sessions", chatHandler.GetSessions)
	chatWrite.Post("/sessions", chatHandler.CreateSession)
	chatWrite.Delete("/sessions/:id", chatHandler.DeleteSession)
	chatWrite.Patch("/sessions/:id/agent", chatHandler.UpdateSessionAgent)
	chatWrite.Patch("/sessions/:id/llm/reset", chatHandler.ResetSessionLLMOverride)
	chatWrite.Post("/sessions/:id/chat", chatHandler.HandleChat)
	chatWrite.Post("/sessions/:id/chat/stream", chatHandler.HandleChatStream)
	chatWrite.Delete("/sessions/:id/messages/:message_id", chatHandler.DeleteMessagesFrom)
	chatWrite.Post("/sessions/:id/confirm/:pending_id", chatHandler.HandleConfirm)
	chatRead.Get("/sessions/:id/chat", chatHandler.HandleGetHistory)
	chatRead.Get("/sessions/:id/artifacts", chatHandler.GetSessionArtifacts)
	chatRead.Get("/approvals", chatHandler.GetPendingApprovals)
	chatRead.Get("/approvals/history", chatHandler.GetApprovalHistory)

	// Centro de notificaciones (mensajes de agentes al usuario, linkeados a sesión)
	chatRead.Get("/notifications", handlers.GetNotifications)
	chatRead.Get("/notifications/unread-count", handlers.GetUnreadCount)
	chatWrite.Post("/notifications/:id/read", handlers.MarkNotificationRead)
	chatWrite.Post("/notifications/read-all", handlers.MarkAllNotificationsRead)

	chatWrite.Post("/sessions/:id/goals", handlers.UpdateSessionGoals)
	chatWrite.Post("/sessions/:id/workspace", handlers.UpdateSessionWorkspace)
	chatWrite.Post("/sessions/:id/files", handlers.UploadSessionFile)
	chatRead.Get("/sessions/:id/files", handlers.GetSessionFiles)
	chatWrite.Delete("/sessions/:id/files/:file_id", handlers.DeleteSessionFile)
	chatRead.Get("/workspace/browse", handlers.BrowseWorkspaceDirectories)

	agentHandler := &handlers.AgentHandler{Brain: brain}
	adminRead := api.Group("/admin", permRead("agents", "read"))
	adminWrite := api.Group("/admin", permWrite("agents", "write"))

	providersRead := api.Group("", permRead("providers", "read"))
	providersWrite := api.Group("", permWrite("providers", "write"))

	providersRead.Get("/providers", handlers.HandleListProviders)
	providersRead.Get("/providers/presets", handlers.HandleGetPrefilledProviders)
	providersWrite.Post("/providers", handlers.HandleCreateProvider)
	providersWrite.Patch("/providers/:id", handlers.HandleUpdateProvider)
	providersWrite.Patch("/providers/:id/set-default", handlers.HandleSetDefaultProvider)
	providersWrite.Delete("/providers/:id", handlers.HandleDeleteProvider)
	providersWrite.Post("/providers/test", handlers.HandleTestProviderConfig)
	providersWrite.Post("/providers/:id/test", handlers.HandleTestProvider)
	providersRead.Get("/providers/:id/models", handlers.HandleGetProviderModels)
	providersWrite.Post("/providers/:id/models/refresh", handlers.HandleRefreshProviderModels)
	providersRead.Get("/models", handlers.HandleGetAllModels)

	adminRead.Get("/agents", agentHandler.GetAgents)
	adminRead.Get("/agents/:id", agentHandler.GetAgent)
	adminWrite.Post("/agents", agentHandler.CreateAgent)
	adminWrite.Put("/agents/:id", agentHandler.UpdateAgent)
	adminWrite.Delete("/agents/:id", agentHandler.DeleteAgent)

	userHandler := &handlers.UserHandler{}
	adminOnly := api.Group("/admin", auth.RequireRoleMiddleware("admin"))
	adminOnly.Get("/users", userHandler.GetUsers)
	adminOnly.Post("/users", userHandler.CreateUser)
	adminOnly.Patch("/users/:id", userHandler.UpdateUser)
	adminOnly.Patch("/users/:id/roles", userHandler.UpdateUserRoles)
	adminOnly.Get("/roles", userHandler.GetRoles)

	approvalPolicyHandler := &handlers.ApprovalPolicyHandler{}
	adminOnly.Get("/approval-policies", approvalPolicyHandler.List)
	adminOnly.Post("/approval-policies", approvalPolicyHandler.Create)
	adminOnly.Patch("/approval-policies/:id", approvalPolicyHandler.Update)
	adminOnly.Delete("/approval-policies/:id", approvalPolicyHandler.Delete)

	tenantHandler := &handlers.TenantHandler{}
	adminOnly.Get("/tenants", tenantHandler.List)
	adminOnly.Get("/tenants/:id", tenantHandler.Get)
	adminOnly.Post("/tenants", tenantHandler.Create)
	adminOnly.Patch("/tenants/:id", tenantHandler.Update)

	configHandler := &handlers.ConfigHandler{Brain: brain}
	mcpRead := api.Group("", permRead("mcp", "read"))
	mcpWrite := api.Group("", permWrite("mcp", "write"))

	mcpRead.Get("/config/handsai", configHandler.GetHandsAIConfig)
	mcpWrite.Patch("/config/handsai", configHandler.UpdateHandsAIConfig)
	mcpWrite.Delete("/config/handsai", configHandler.DeleteHandsAIConfig)

	mcpStdioHandler := &handlers.McpStdioConfigHandler{Brain: brain, Manager: mcpStdioMgr}
	mcpRead.Get("/config/mcp-stdio", mcpStdioHandler.List)
	mcpWrite.Post("/config/mcp-stdio", mcpStdioHandler.Create)
	mcpWrite.Post("/config/mcp-stdio/test", mcpStdioHandler.TestDryRun)
	mcpWrite.Patch("/config/mcp-stdio/:id", mcpStdioHandler.Update)
	mcpWrite.Delete("/config/mcp-stdio/:id", mcpStdioHandler.Delete)
	mcpWrite.Post("/config/mcp-stdio/:id/test", mcpStdioHandler.TestSaved)

	mcpStreamHandler := &handlers.McpStreamConfigHandler{Brain: brain, Manager: mcpStreamMgr}
	mcpRead.Get("/config/mcp-stream", mcpStreamHandler.List)
	mcpWrite.Post("/config/mcp-stream", mcpStreamHandler.Create)
	mcpWrite.Post("/config/mcp-stream/test", mcpStreamHandler.TestDryRun)
	mcpWrite.Patch("/config/mcp-stream/:id", mcpStreamHandler.Update)
	mcpWrite.Delete("/config/mcp-stream/:id", mcpStreamHandler.Delete)
	mcpWrite.Post("/config/mcp-stream/:id/test", mcpStreamHandler.TestSaved)

	mcpCatalogHandler := &handlers.McpCatalogHandler{
		Brain:     brain,
		StdioMgr:  mcpStdioMgr,
		StreamMgr: mcpStreamMgr,
	}
	mcpRead.Get("/catalog/mcp", mcpCatalogHandler.List)
	mcpWrite.Post("/catalog/mcp/install", mcpCatalogHandler.Install)

	taskHandler := &handlers.TaskHandler{}
	tasksRead := api.Group("", permRead("tasks", "read"))
	tasksWrite := api.Group("", permWrite("tasks", "write"))
	tasksRead.Get("/tasks", taskHandler.GetTasks)
	tasksWrite.Post("/tasks", taskHandler.CreateTask)
	tasksWrite.Delete("/tasks/:id", taskHandler.DeleteTask)

	ruleHandler := &handlers.RuleHandler{}
	rulesRead := api.Group("", permRead("rules", "read"))
	rulesWrite := api.Group("", permWrite("rules", "write"))
	rulesRead.Get("/rules", ruleHandler.GetRules)
	rulesWrite.Post("/rules", ruleHandler.CreateRule)
	rulesWrite.Delete("/rules/:id", ruleHandler.DeleteRule)

	workflowHandler := &handlers.WorkflowHandler{}
	wfRead := api.Group("", permRead("workflows", "read"))
	wfWrite := api.Group("", permWrite("workflows", "write"))
	wfRead.Get("/workflows", workflowHandler.GetWorkflows)
	wfRead.Get("/workflows/:id", workflowHandler.GetWorkflow)
	wfWrite.Post("/workflows", workflowHandler.CreateWorkflow)
	wfWrite.Post("/workflows/reload", workflowHandler.ReloadWorkflows)
	wfWrite.Delete("/workflows/:id", workflowHandler.DeleteWorkflow)
	wfWrite.Post("/workflows/:id/run", workflowHandler.RunWorkflow)
	wfRead.Get("/workflows/:id/runs", workflowHandler.GetWorkflowRuns)
	wfRead.Get("/workflows/runs/:run_id", workflowHandler.GetWorkflowRun)

	permRoutesRead := api.Group("", permRead("permissions", "read"))
	permRoutesWrite := api.Group("", permWrite("permissions", "write"))
	permRoutesRead.Get("/permissions", handlers.HandleListPermissions)
	permRoutesWrite.Delete("/permissions/:id", handlers.HandleDeletePermission)
	permRoutesWrite.Post("/permissions/:id/pause", handlers.HandleTogglePausePermission)

	auditHandler := &handlers.AuditHandler{}
	auditRead := api.Group("", permRead("audit", "read"))
	auditRead.Get("/audit/events", auditHandler.ListEvents)
	auditExport := api.Group("", permRead("audit", "export"))
	auditExport.Get("/audit/events/export", auditHandler.ExportEvents)

	ragWrite := api.Group("", permWrite("providers", "write"))
	ragWrite.Post("/rag/upload", handlers.UploadKnowledgeFile)
	ragWrite.Get("/rag/search", handlers.SearchKnowledge)
	ragWrite.Post("/rag/search", handlers.SearchKnowledge)

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
