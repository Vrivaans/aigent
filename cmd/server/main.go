package main

import (
	"context"
	"log"
	"os"

	"aigent/internal/ai"
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
	// 1. Cargar variables de entorno (ignora si no hay archivo, ideal para Docker)
	if err := godotenv.Load(); err != nil {
		log.Println("Note: No .env file found, using system environment variables")
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

	// 3. Inicializar integraciones (HandsAI y LLM OpenRouter)
	handsaiCfg := handsai.Config{
		BaseURL: getEnv("HANDSAI_URL", "http://localhost:8080/mcp"),
	}
	
	brain := ai.NewBrain(
		getEnv("OPENROUTER_API_KEY", ""),
		handsaiCfg,
		nil, // Usa DefaultPermissionHandler (bloquea sensitive actions por consola)
	)

	// 4. Levantar Cron Worker en background para tareas autonomas
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go scheduler.StartCronWorker(ctx, brain)

	// 5. Inicializar Fiber App y Rutas
	app := fiber.New()
	app.Use(cors.New())
	app.Use(logger.New())

	api := app.Group("/api")

	chatHandler := &handlers.ChatHandler{Brain: brain}
	api.Post("/chat", chatHandler.HandleChat)
	api.Get("/chat/history", chatHandler.GetHistory)

	taskHandler := &handlers.TaskHandler{}
	api.Get("/tasks", taskHandler.GetTasks)
	api.Delete("/tasks/:id", taskHandler.DeleteTask)

	ruleHandler := &handlers.RuleHandler{}
	api.Get("/rules", ruleHandler.GetRules)
	api.Post("/rules", ruleHandler.CreateRule)
	api.Delete("/rules/:id", ruleHandler.DeleteRule)

	// 6. Iniciar Servidor web
	port := getEnv("PORT", "3000")
	log.Printf("🚀 Starting AIgent Server on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Fiber failed: %v", err)
	}
}

// getEnv helper para leer variables con default
func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
