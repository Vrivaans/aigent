package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	sessionID := c.Params("id")

	var req ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON format"})
	}

	// Validate Session
	var session database.Session
	if err := database.DB.First(&session, sessionID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Session not found"})
	}

	// Update title if it's the first message
	if session.Title == "Nueva conversación" && len(req.Message) > 0 {
		title := req.Message
		if len(title) > 30 {
			title = title[:30] + "..."
		}
		database.DB.Model(&session).Update("title", title)
	}

	// 1. Save user message history
	userMsg := database.ChatMessage{
		SessionID: session.ID,
		Role:      "user",
		Content:   req.Message,
	}
	if err := database.DB.Create(&userMsg).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save history"})
	}

	// 2. Traer el historial de ESTA sesion (10 mensajes anteriores)
	var history []database.ChatMessage
	database.DB.Where("session_id = ?", sessionID).Order("created_at asc").Limit(10).Find(&history)

	// 3. Ejecutar The Brain Loop
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	respMsg, intermediates, err := h.Brain.ProcessChatInteraction(ctx, session.ID, history, req.Message)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 4. Persistir mensajes intermedios (tool_calls y resultados) acumulados por el Brain
	for i := range intermediates {
		database.DB.Create(&intermediates[i])
	}

	// 5. Guardar la respuesta FINAL del asistente
	var rawTools string
	if len(respMsg.ToolCalls) > 0 {
		b, _ := json.Marshal(respMsg.ToolCalls)
		rawTools = string(b)
	}
	asstMsg := database.ChatMessage{
		SessionID:    session.ID,
		Role:         "assistant",
		Content:      respMsg.Content,
		RawToolCalls: rawTools,
	}
	database.DB.Create(&asstMsg)

	// 6. Manejar si requiere confirmación
	var pendingID uint
	if respMsg.RequiresConfirmation && respMsg.WaitingToolCall != nil {
		pending := database.PendingAction{
			SessionID:  session.ID,
			ToolName:   respMsg.WaitingToolCall.Function.Name,
			Arguments:  respMsg.WaitingToolCall.Function.Arguments,
			ToolCallID: respMsg.WaitingToolCall.ID,
			Status:     "PENDING",
		}
		database.DB.Create(&pending)
		pendingID = pending.ID
	}

	return c.JSON(fiber.Map{
		"response":              asstMsg.Content,
		"tool_calls":            respMsg.ToolCalls,
		"status":                "ok",
		"requires_confirmation": respMsg.RequiresConfirmation,
		"pending_action_id":     pendingID,
		"waiting_tool":          respMsg.WaitingToolCall,
	})
}

type ConfirmRequest struct {
	Approved bool `json:"approved"`
}

func (h *ChatHandler) HandleConfirm(c *fiber.Ctx) error {
	id := c.Params("pending_id")
	var req ConfirmRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid body"})
	}

	var pending database.PendingAction
	if err := database.DB.First(&pending, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Action not found"})
	}

	if !req.Approved {
		pending.Status = "REJECTED"
		database.DB.Save(&pending)
		return c.JSON(fiber.Map{"status": "rejected"})
	}

	// EXECUTE TOOL
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	var args map[string]interface{}
	json.Unmarshal([]byte(pending.Arguments), &args)

	// Necesitamos el Brain para obtener el Sanitize y el Registry
	// Pero el Handlers.ChatHandler ya tiene el Brain.
	// Ojo: En confirmación el nombre viene sanitizado de la DB.

	// Buscamos herramienta — el nombre en DB está sanitizado (guiones→guiones_bajos),
	// pero el Registry usa el nombre original del MCP (puede tener guiones).
	tDef, exists := h.Brain.Registry.GetBySanitized(pending.ToolName)
	if !exists {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Tool no longer registered: " + pending.ToolName})
	}

	// Mapeamos argumentos sanitizados a originales (como en ProcessChatInteraction)
	finalArgs := make(map[string]interface{})
	for k, v := range args {
		origK, ok := tDef.ArgMapping[k]
		if ok {
			finalArgs[origK] = v
		} else {
			finalArgs[k] = v
		}
	}

	result, err := tDef.Execute(ctx, finalArgs)
	if err != nil {
		// Log the error to DB so LLM knows it failed, then return 500 to frontend
		errResMsg := database.ChatMessage{
			SessionID:  pending.SessionID,
			Role:       "tool",
			Content:    fmt.Sprintf("ERROR: %v", err),
			ToolCallID: pending.ToolCallID,
		}
		database.DB.Create(&errResMsg)
		pending.Status = "REJECTED"
		database.DB.Save(&pending)

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	pending.Status = "APPROVED"
	database.DB.Save(&pending)

	// Guardamos el resultado de la tool para que el siguiente chat lo vea en contexto
	toolResMsg := database.ChatMessage{
		SessionID:  pending.SessionID,
		Role:       "tool",
		Content:    string(result),
		ToolCallID: pending.ToolCallID,
	}
	database.DB.Create(&toolResMsg)

	// 7. RE-INFERENCIA: Reanudamos el bucle del agente para ver si hay más pasos.
	// 7a. Obtener el historial actualizado (incluyendo el resultado que acabamos de guardar)
	var history []database.ChatMessage
	database.DB.Where("session_id = ?", pending.SessionID).Order("created_at asc").Limit(20).Find(&history)

	// 7b. Reanudar bucle
	newCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	msg, toSave, err := h.Brain.ProcessChatInteraction(newCtx, pending.SessionID, history, "")
	if err != nil {
		log.Printf("⚠️ Error re-inferring after confirm: %v", err)
		return c.JSON(fiber.Map{
			"status":   "approved",
			"result":   string(result),
			"response": "Ejecutado, pero falló la re-inferencia: " + err.Error(),
		})
	}

	// 8. Persistir mensajes intermedios generados en la re-inferencia
	for _, m := range toSave {
		database.DB.Create(&m)
	}

	// 9. Manejar la respuesta final de la re-inferencia
	var finalResponse string = "✅ Acción ejecutada correctamente."
	var nextPendingID uint
	if msg != nil {
		var rawTools string
		if len(msg.ToolCalls) > 0 {
			b, _ := json.Marshal(msg.ToolCalls)
			rawTools = string(b)
		}
		asstMsg := database.ChatMessage{
			SessionID:    pending.SessionID,
			Role:         "assistant",
			Content:      msg.Content,
			RawToolCalls: rawTools,
		}
		database.DB.Create(&asstMsg)
		finalResponse = msg.Content

		// Si la re-inferencia disparó OTRA acción sensible, crear el PendingAction
		if msg.RequiresConfirmation && msg.WaitingToolCall != nil {
			newPending := database.PendingAction{
				SessionID:  pending.SessionID,
				ToolName:   msg.WaitingToolCall.Function.Name,
				Arguments:  msg.WaitingToolCall.Function.Arguments,
				ToolCallID: msg.WaitingToolCall.ID,
				Status:     "PENDING",
			}
			database.DB.Create(&newPending)
			nextPendingID = newPending.ID
			finalResponse = "⏳ Acción ejecutada. Pendiente de la siguiente confirmación..."
		}
	}

	return c.JSON(fiber.Map{
		"status":     "approved",
		"result":     string(result),
		"response":   finalResponse,
		"pending_id": nextPendingID,
	})
}


type ChatMessageResponse struct {
	database.ChatMessage
	RequiresConfirmation bool        `json:"requires_confirmation"`
	PendingActionID      uint        `json:"pending_action_id"`
	WaitingTool          interface{} `json:"waiting_tool"`
}

// GetHistory expone el chat de una sesion enriquecido con acciones pendientes
func (h *ChatHandler) HandleGetHistory(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	var history []database.ChatMessage
	if err := database.DB.Where("session_id = ?", sessionID).Order("created_at asc").Limit(50).Find(&history).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Buscar acciones pendientes para esta sesión
	var pendingActions []database.PendingAction
	database.DB.Where("session_id = ? AND status = ?", sessionID, "PENDING").Find(&pendingActions)

	// Mapa para búsqueda rápida por ToolCallID
	pendingMap := make(map[string]database.PendingAction)
	for _, p := range pendingActions {
		pendingMap[p.ToolCallID] = p
	}

	// Enriquecer mensajes
	response := make([]ChatMessageResponse, len(history))
	for i, msg := range history {
		response[i] = ChatMessageResponse{
			ChatMessage: msg,
		}

		// Si el mensaje tiene tool calls, ver si alguno está pendiente
		if msg.Role == "assistant" && msg.RawToolCalls != "" {
			var tCalls []ai.ToolCall
			if err := json.Unmarshal([]byte(msg.RawToolCalls), &tCalls); err == nil {
				for _, tc := range tCalls {
					if p, ok := pendingMap[tc.ID]; ok {
						response[i].RequiresConfirmation = true
						response[i].PendingActionID = p.ID
						response[i].WaitingTool = tc
						break // Solo soportamos una acción pendiente por mensaje por ahora
					}
				}
			}
		}
	}

	return c.JSON(response)
}

// GetSessions devuelve todas las sesiones ordenadas
func (h *ChatHandler) GetSessions(c *fiber.Ctx) error {
	var sessions []database.Session
	database.DB.Order("updated_at desc").Find(&sessions)
	return c.JSON(sessions)
}

// CreateSession crea una nueva sesión de chat
func (h *ChatHandler) CreateSession(c *fiber.Ctx) error {
	session := database.Session{
		Title: "Nueva conversación",
	}
	if err := database.DB.Create(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create session"})
	}
	return c.JSON(session)
}
