package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"aigent/internal/ai"
	"aigent/internal/audit"
	"aigent/internal/database"
	"aigent/internal/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ChatHandler struct {
	Brain *ai.Brain
}

type ChatRequest struct {
	Message       string `json:"message"`
	ModelOverride string `json:"model_override,omitempty"`
}

type ResetLLMOverrideRequest struct{}

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

	if req.ModelOverride != "" {
		session.LLMModelOverride = req.ModelOverride
		database.DB.Model(&session).Where("id = ?", session.ID).Update("llm_model_override", req.ModelOverride)
	}

	// Cancelar cualquier acción pendiente antes de procesar el nuevo mensaje
	var pendingActions []database.PendingAction
	if err := database.DB.Where("session_id = ? AND status = ?", session.ID, "PENDING").Find(&pendingActions).Error; err == nil && len(pendingActions) > 0 {
		activeSess, sessErr := ai.GetSessionManager().GetOrCreateSession(session.ID)
		for _, p := range pendingActions {
			p.Status = "REJECTED"
			database.DB.Save(&p)
			
			// Registrar en el chat que fue cancelada para mantener la validez del historial
			if sessErr == nil {
				activeSess.AddMessage("tool", `{"status":"rejected","error":"Acción cancelada por el inicio de un nuevo mensaje"}`, p.ToolCallID, "")
			} else {
				// Fallback si no está en caché
				sysMsg := database.ChatMessage{
					SessionID:  session.ID,
					Role:       "tool",
					Content:    `{"status":"rejected","error":"Acción cancelada por el inicio de un nuevo mensaje"}`,
					ToolCallID: p.ToolCallID,
				}
				database.DB.Create(&sysMsg)
			}
		}
	}

	// 3. Ejecutar The Brain Loop
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	respMsg, _, err := h.Brain.ProcessChatInteraction(ctx, session.ID, nil, req.Message)
	if err != nil {
		errText := err.Error()
		sysMsg := database.ChatMessage{
			SessionID: session.ID,
			Role:      "system",
			Content:   "❌ Error: " + errText,
		}
		if saveErr := database.DB.Create(&sysMsg).Error; saveErr != nil {
			log.Printf("⚠️ Failed to persist chat error message: %v", saveErr)
		}
		return c.JSON(fiber.Map{
			"status":                "error",
			"error":                 errText,
			"response":              "",
			"tool_calls":            []interface{}{},
			"requires_confirmation": false,
			"pending_action_id":     0,
			"waiting_tool":          nil,
			"provider_switched":     false,
			"provider_switch":       nil,
		})
	}

	// Extraer y procesar artefactos
	cleanContent, savedArtifacts := processAndSaveArtifacts(session.ID, respMsg.Content)

	// 6. Manejar si requiere confirmación
	var pendingID uint
	if respMsg.RequiresConfirmation {
		var firstPending database.PendingAction
		if err := database.DB.Where("session_id = ? AND status = ?", session.ID, "PENDING").Order("id asc").First(&firstPending).Error; err == nil {
			pendingID = firstPending.ID
		}
	}

	return c.JSON(fiber.Map{
		"response":              cleanContent,
		"artifacts":             savedArtifacts,
		"tool_calls":            respMsg.ToolCalls,
		"status":                "ok",
		"requires_confirmation": respMsg.RequiresConfirmation,
		"pending_action_id":     pendingID,
		"waiting_tool":          respMsg.WaitingToolCall,
		"provider_switched":     respMsg.ProviderSwitched,
		"provider_switch":       respMsg.ProviderSwitch,
	})
}

func (h *ChatHandler) HandleChatStream(c *fiber.Ctx) error {
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

	if req.ModelOverride != "" {
		session.LLMModelOverride = req.ModelOverride
		database.DB.Model(&session).Where("id = ?", session.ID).Update("llm_model_override", req.ModelOverride)
	}

	// Cancelar cualquier acción pendiente antes de procesar el nuevo mensaje
	var pendingActions []database.PendingAction
	if err := database.DB.Where("session_id = ? AND status = ?", session.ID, "PENDING").Find(&pendingActions).Error; err == nil && len(pendingActions) > 0 {
		activeSess, sessErr := ai.GetSessionManager().GetOrCreateSession(session.ID)
		for _, p := range pendingActions {
			p.Status = "REJECTED"
			database.DB.Save(&p)
			
			// Registrar en el chat que fue cancelada para mantener la validez del historial
			if sessErr == nil {
				activeSess.AddMessage("tool", `{"status":"rejected","error":"Acción cancelada por el inicio de un nuevo mensaje"}`, p.ToolCallID, "")
			} else {
				// Fallback si no está en caché
				sysMsg := database.ChatMessage{
					SessionID:  session.ID,
					Role:       "tool",
					Content:    `{"status":"rejected","error":"Acción cancelada por el inicio de un nuevo mensaje"}`,
					ToolCallID: p.ToolCallID,
				}
				database.DB.Create(&sysMsg)
			}
		}
	}

	// Configurar los headers de SSE
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Helper para enviar eventos formateados de SSE
		sendEvent := func(eventType string, data interface{}) {
			dataBytes, err := json.Marshal(data)
			if err != nil {
				return
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(dataBytes))
			_ = w.Flush()
		}

		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
		defer cancel()

		respMsg, err := h.Brain.ProcessChatInteractionStream(ctx, session.ID, req.Message, sendEvent)
		if err != nil {
			log.Printf("⚠️ Stream interaction error: %v", err)
			if errors.Is(err, context.Canceled) {
				// Don't save system message or send events (client is already disconnected)
				return
			}
			sendEvent("error", map[string]string{"message": err.Error()})

			sysMsg := database.ChatMessage{
				SessionID: session.ID,
				Role:      "system",
				Content:   "❌ Error: " + err.Error(),
			}
			database.DB.Create(&sysMsg)
			return
		}

		// Extraer y guardar artefactos si hay respuesta textual
		if respMsg != nil && respMsg.Content != "" {
			_, _ = processAndSaveArtifacts(session.ID, respMsg.Content)
		}

		sendEvent("done", map[string]interface{}{"status": "ok"})
	})

	return nil
}

type ConfirmRequest struct {
	Approved    bool `json:"approved"`
	AlwaysAllow bool `json:"always_allow"`
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
		sessionID := pending.SessionID
		audit.Emit(c, audit.Event{
			Action:       "approval.reject",
			ResourceType: "pending_action",
			ResourceID:   audit.UintID(pending.ID),
			SessionID:    &sessionID,
			PayloadAfter: audit.ApprovalPayload(pending),
		})

		// Agregar mensaje de herramienta rechazada en el historial para mantener la integridad de la API
		activeSess, err := ai.GetSessionManager().GetOrCreateSession(pending.SessionID)
		if err == nil {
			activeSess.AddMessage("tool", `{"status":"rejected","error":"Acción cancelada por el usuario"}`, pending.ToolCallID, "")
		}

		var pendingCount int64
		database.DB.Model(&database.PendingAction{}).Where("session_id = ? AND status = ?", pending.SessionID, "PENDING").Count(&pendingCount)

		if pendingCount > 0 {
			var nextPending database.PendingAction
			var nextPendingID uint
			if err := database.DB.Where("session_id = ? AND status = ?", pending.SessionID, "PENDING").Order("id asc").First(&nextPending).Error; err == nil {
				nextPendingID = nextPending.ID
			}
			return c.JSON(fiber.Map{
				"status":     "rejected",
				"partial":    true,
				"pending_id": nextPendingID,
				"response":   "⏳ Acción rechazada. Esperando otras aprobaciones pendientes...",
			})
		}

		// Re-inferencia tras el rechazo final
		newCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		msg, _, err := h.Brain.ProcessChatInteraction(newCtx, pending.SessionID, nil, "")
		var finalResponse string = "❌ Acción cancelada."
		var nextPendingID uint
		var savedArtifacts []database.Artifact
		if err == nil && msg != nil {
			cleanContent, arts := processAndSaveArtifacts(pending.SessionID, msg.Content)
			finalResponse = cleanContent
			savedArtifacts = arts

			if msg.RequiresConfirmation {
				var nextPending database.PendingAction
				if err := database.DB.Where("session_id = ? AND status = ?", pending.SessionID, "PENDING").Order("id asc").First(&nextPending).Error; err == nil {
					nextPendingID = nextPending.ID
				}
				finalResponse = "⏳ Acción cancelada. Pendiente de la siguiente confirmación..."
			}
		}

		return c.JSON(fiber.Map{
			"status":     "rejected",
			"response":   finalResponse,
			"artifacts":  savedArtifacts,
			"pending_id": nextPendingID,
		})
	}

	// EXECUTE TOOL
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	var args map[string]interface{}
	json.Unmarshal([]byte(pending.Arguments), &args)

	tDef, exists := h.Brain.Registry.GetBySanitized(pending.ToolName)
	if !exists {
		errText := "Tool no longer registered: " + pending.ToolName
		sysMsg := database.ChatMessage{
			SessionID: pending.SessionID,
			Role:      "system",
			Content:   "❌ Error: " + errText,
		}
		_ = database.DB.Create(&sysMsg).Error
		return c.JSON(fiber.Map{
			"status": "error",
			"error":  errText,
		})
	}

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
		errResMsg := database.ChatMessage{
			SessionID:  pending.SessionID,
			Role:       "tool",
			Content:    fmt.Sprintf("ERROR: %v", err),
			ToolCallID: pending.ToolCallID,
		}
		database.DB.Create(&errResMsg)
		sysMsg := database.ChatMessage{
			SessionID: pending.SessionID,
			Role:      "system",
			Content:   "❌ Error al ejecutar la herramienta: " + err.Error(),
		}
		database.DB.Create(&sysMsg)
		pending.Status = "REJECTED"
		database.DB.Save(&pending)

		return c.JSON(fiber.Map{
			"status": "error",
			"error":  err.Error(),
		})
	}

	pending.Status = "APPROVED"
	database.DB.Save(&pending)
	sessionID := pending.SessionID
	audit.Emit(c, audit.Event{
		Action:       "approval.approve",
		ResourceType: "pending_action",
		ResourceID:   audit.UintID(pending.ID),
		SessionID:    &sessionID,
		PayloadAfter: audit.ApprovalPayload(pending),
	})

	var session database.Session
	if err := database.DB.First(&session, pending.SessionID).Error; err == nil {
		if req.AlwaysAllow {
			AutoSaveToolPermission(c, session.AgentID, tDef.Name)
		}
	}

	// Guardamos el resultado de la tool para que el siguiente chat lo vea en contexto
	activeSess, err := ai.GetSessionManager().GetOrCreateSession(pending.SessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to access session cache"})
	}
	activeSess.AddMessage("tool", string(result), pending.ToolCallID, "")

	// Verificar si hay otras acciones PENDING para la misma sesión antes de la re-inferencia
	var pendingCount int64
	database.DB.Model(&database.PendingAction{}).Where("session_id = ? AND status = ?", pending.SessionID, "PENDING").Count(&pendingCount)

	if pendingCount > 0 {
		var nextPending database.PendingAction
		var nextPendingID uint
		if err := database.DB.Where("session_id = ? AND status = ?", pending.SessionID, "PENDING").Order("id asc").First(&nextPending).Error; err == nil {
			nextPendingID = nextPending.ID
		}
		return c.JSON(fiber.Map{
			"status":     "approved",
			"partial":    true,
			"result":     string(result),
			"response":   "⏳ Acción ejecutada. Esperando otras aprobaciones pendientes...",
			"pending_id": nextPendingID,
		})
	}

	// 7. RE-INFERENCIA: Reanudamos el bucle del agente al estar todo resuelto.
	newCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	msg, _, err := h.Brain.ProcessChatInteraction(newCtx, pending.SessionID, nil, "")
	if err != nil {
		log.Printf("⚠️ Error re-inferring after confirm: %v", err)
		errText := err.Error()
		sysMsg := database.ChatMessage{
			SessionID: pending.SessionID,
			Role:      "system",
			Content:   "❌ Error (re-inferencia tras confirmar): " + errText,
		}
		if saveErr := database.DB.Create(&sysMsg).Error; saveErr != nil {
			log.Printf("⚠️ Failed to persist reinference error: %v", saveErr)
		}
		return c.JSON(fiber.Map{
			"status":     "error",
			"error":      errText,
			"result":     string(result),
			"response":   "",
			"pending_id": 0,
		})
	}

	var finalResponse string = "✅ Acción ejecutada correctamente."
	var nextPendingID uint
	var savedArtifacts []database.Artifact
	if msg != nil {
		cleanContent, arts := processAndSaveArtifacts(pending.SessionID, msg.Content)
		finalResponse = cleanContent
		savedArtifacts = arts

		if msg.RequiresConfirmation {
			var nextPending database.PendingAction
			if err := database.DB.Where("session_id = ? AND status = ?", pending.SessionID, "PENDING").Order("id asc").First(&nextPending).Error; err == nil {
				nextPendingID = nextPending.ID
			}
			finalResponse = "⏳ Acción ejecutada. Pendiente de la siguiente confirmación..."
		}
	}

	return c.JSON(fiber.Map{
		"status":     "approved",
		"result":     string(result),
		"response":   finalResponse,
		"artifacts":  savedArtifacts,
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
	var recentHistory []database.ChatMessage
	if err := database.DB.Where("session_id = ?", sessionID).Order("created_at desc").Limit(50).Find(&recentHistory).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	var history []database.ChatMessage
	for i := len(recentHistory) - 1; i >= 0; i-- {
		history = append(history, recentHistory[i])
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
		cleanContent, _ := utils.ExtractArtifacts(msg.Content)
		msg.Content = cleanContent

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
	excludeCron := c.Query("exclude_cron") == "true"
	excludeWorkflows := c.Query("exclude_workflows") == "true"

	var sessions []database.Session
	query := database.DB.Preload("Agent")
	if excludeCron {
		query = query.Where("title NOT LIKE ?", "Cron: %")
	}
	if excludeWorkflows {
		query = query.Where("title NOT LIKE ? AND title NOT LIKE ?", "Workflow Run: %", "Workflow: %")
	}
	if err := query.Order("updated_at desc").Find(&sessions).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(sessions)
}

// CreateSession crea una nueva sesión de chat mapeada por default al Agent 1 (General)
func (h *ChatHandler) CreateSession(c *fiber.Ctx) error {
	session := database.Session{
		Title:   "Nueva conversación",
		AgentID: 1, // Por defecto al Agente General
	}
	if err := database.DB.Create(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create session"})
	}
	return c.JSON(session)
}

// UpdateSessionAgent permite a un usuario cambiar el agente de una sesión al vuelo
func (h *ChatHandler) UpdateSessionAgent(c *fiber.Ctx) error {
	id := c.Params("id")

	var req struct {
		AgentID uint `json:"agent_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var session database.Session
	if err := database.DB.First(&session, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Session not found"})
	}

	session.AgentID = req.AgentID
	if err := database.DB.Save(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update session agent"})
	}

	return c.JSON(fiber.Map{"status": "updated", "agent_id": session.AgentID})
}

// ResetSessionLLMOverride vuelve al provider/modelo por defecto del agente.
func (h *ChatHandler) ResetSessionLLMOverride(c *fiber.Ctx) error {
	id := c.Params("id")

	var session database.Session
	if err := database.DB.First(&session, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Session not found"})
	}

	if err := database.DB.Model(&database.Session{}).Where("id = ?", session.ID).Updates(map[string]interface{}{
		"llm_provider_override_id": nil,
		"llm_model_override":       "",
	}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to reset LLM override"})
	}

	return c.JSON(fiber.Map{"status": "reset"})
}

// DeleteSession borra una sesión y sus datos asociados (Hard Delete)
func (h *ChatHandler) DeleteSession(c *fiber.Ctx) error {
	id := c.Params("id")

	// Usar Transacción para asegurar que todo se borre o nada
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Borrar mensajes (Hard Delete)
		if err := tx.Unscoped().Where("session_id = ?", id).Delete(&database.ChatMessage{}).Error; err != nil {
			return err
		}

		// 2. Borrar acciones pendientes (Hard Delete)
		if err := tx.Unscoped().Where("session_id = ?", id).Delete(&database.PendingAction{}).Error; err != nil {
			return err
		}

		// 3. Borrar la sesión (Hard Delete)
		if err := tx.Unscoped().Delete(&database.Session{}, id).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete session: " + err.Error()})
	}

	return c.JSON(fiber.Map{"status": "deleted"})
}

func processAndSaveArtifacts(sessionID uint, content string) (string, []database.Artifact) {
	cleanContent, arts := utils.ExtractArtifacts(content)
	var savedArtifacts []database.Artifact
	for _, art := range arts {
		dbArt := database.Artifact{
			ID:        art.ID,
			SessionID: sessionID,
			Type:      art.Type,
			Title:     art.Title,
			Content:   art.Content,
		}
		if dbArt.ID == "" {
			dbArt.ID = fmt.Sprintf("diag-%d", time.Now().UnixNano())
		}
		if err := database.DB.Save(&dbArt).Error; err == nil {
			savedArtifacts = append(savedArtifacts, dbArt)
		} else {
			log.Printf("⚠️ Error saving artifact: %v", err)
		}
	}
	return cleanContent, savedArtifacts
}

// GetSessionArtifacts retrieves all artifacts created within a session
func (h *ChatHandler) GetSessionArtifacts(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	var artifacts []database.Artifact
	if err := database.DB.Where("session_id = ?", sessionID).Order("created_at asc").Find(&artifacts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(artifacts)
}

type PendingApprovalResponse struct {
	database.PendingAction
	SessionTitle string `json:"session_title"`
	TaskName     string `json:"task_name,omitempty"`
	TaskID       *uint  `json:"task_id,omitempty"`
}

// GetPendingApprovals retorna todas las aprobaciones pendientes del sistema
func (h *ChatHandler) GetPendingApprovals(c *fiber.Ctx) error {
	var pendings []database.PendingAction
	if err := database.DB.Where("status = ?", "PENDING").Order("created_at desc").Find(&pendings).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	response := make([]PendingApprovalResponse, len(pendings))
	for i, p := range pendings {
		var sess database.Session
		var taskName string
		var taskID *uint

		if err := database.DB.First(&sess, p.SessionID).Error; err == nil {
			if sess.TaskID != nil {
				var t database.Task
				if err := database.DB.First(&t, *sess.TaskID).Error; err == nil {
					taskName = t.Name
					taskID = sess.TaskID
				}
			}
			response[i] = PendingApprovalResponse{
				PendingAction: p,
				SessionTitle:  sess.Title,
				TaskName:      taskName,
				TaskID:        taskID,
			}
		} else {
			response[i] = PendingApprovalResponse{
				PendingAction: p,
				SessionTitle:  fmt.Sprintf("Sesión #%d", p.SessionID),
			}
		}
	}

	return c.JSON(response)
}

// DeleteMessagesFrom deletes a user message and all subsequent messages in a session.
func (h *ChatHandler) DeleteMessagesFrom(c *fiber.Ctx) error {
	sessionID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid session ID"})
	}
	messageID, err := c.ParamsInt("message_id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid message ID"})
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Delete all messages with ID >= messageID for this session
		if err := tx.Where("session_id = ? AND id >= ?", sessionID, messageID).Delete(&database.ChatMessage{}).Error; err != nil {
			return err
		}
		// 2. Delete any pending actions that are currently PENDING for this session
		if err := tx.Where("session_id = ? AND status = ?", sessionID, "PENDING").Delete(&database.PendingAction{}).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete messages: " + err.Error()})
	}

	// Clear memory cache so next request reloads from DB
	ai.GetSessionManager().ClearSession(uint(sessionID))

	return c.JSON(fiber.Map{"status": "deleted"})
}
