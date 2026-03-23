package handlers

import (
	"context"
	"time"

	"aigent/internal/ai"
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
)

type ChatHandler struct {
	Brain *ai.Brain
}

type ChatRequest struct {
	Message string `json:"message"`
}

func (h *ChatHandler) HandleChat(c *fiber.Ctx) error {
	var req ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON format"})
	}

	// 1. Save user message history
	userMsg := database.ChatMessage{
		Role:    "user",
		Content: req.Message,
	}
	if err := database.DB.Create(&userMsg).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save history"})
	}

	// 2. Traer el historial reciente (10 mensajes)
	var history []database.ChatMessage
	database.DB.Order("created_at asc").Limit(10).Find(&history)

	// 3. Ejecutar The Brain Loop (Reglas + Tools -> INFERENCIA -> Proxy MCP)
	// Timeout de 2 min: La invocación LLM + Ejecución HandsAI puede tardar un poco
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	respMsg, err := h.Brain.ProcessChatInteraction(ctx, history, req.Message)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 4. Guardar respuesta final en historia
	asstMsg := database.ChatMessage{
		Role:    "assistant",
		Content: respMsg.Content,
	}
	database.DB.Create(&asstMsg)

	// Extraemos tool calls por si el frontend los quiere graficar lindo (como un pop-up que diga "Trello actualizado")
	return c.JSON(fiber.Map{
		"response": asstMsg.Content,
		"tool_calls": respMsg.ToolCalls,
	})
}

// GetHistory expone el chat al initial load del dashabord
func (h *ChatHandler) GetHistory(c *fiber.Ctx) error {
	var history []database.ChatMessage
	database.DB.Order("created_at asc").Limit(50).Find(&history)
	return c.JSON(history)
}
