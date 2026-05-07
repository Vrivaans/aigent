package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"aigent/internal/database"
	"aigent/internal/handsai"
	"aigent/internal/mcpstdio"
	"aigent/internal/mcpstream"
	tasksvc "aigent/internal/tasks"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Brain es el orquestador principal que une el LLM (OpenRouter) con el motor de acciones (HandsAI)
type Brain struct {
	LLM        *OpenRouterClient
	HandsAI    *handsai.Client
	Registry   *ToolRegistry
	McpStdio   *mcpstdio.Manager
	McpStream  *mcpstream.Manager
	toolPermit handsai.PermissionHandler
}

// mcpExecutable abstrae sesiones MCP stdio y stream (mismos métodos hacia el agente).
type mcpExecutable interface {
	ListTools(ctx context.Context) ([]*mcpsdk.Tool, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (json.RawMessage, error)
}

func isRecoverableProviderError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	needles := []string{
		"insufficient",
		"insufficient_quota",
		"quota",
		"credit",
		"credits",
		"payment required",
		"429",
		"rate limit",
		"model_not_found",
		"not_found_error",
		"does not exist",
		"you do not have access to it",
		"401",
		"403",
		"unauthorized",
		"invalid api key",
		"authentication",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

func (b *Brain) resolveProviderCandidates(session *database.Session) ([]database.LLMProvider, int, error) {
	var preferred database.LLMProvider
	preferredSet := false

	if session.LLMProviderOverrideID != nil {
		if err := database.DB.Where("id = ? AND is_active = ?", *session.LLMProviderOverrideID, true).First(&preferred).Error; err == nil {
			preferredSet = true
		}
	}

	if !preferredSet && session.Agent != nil && session.Agent.LLMProviderID != nil {
		if session.Agent.LLMProvider.ID != 0 && session.Agent.LLMProvider.IsActive {
			preferred = session.Agent.LLMProvider
			preferredSet = true
		} else if err := database.DB.Where("id = ? AND is_active = ?", *session.Agent.LLMProviderID, true).First(&preferred).Error; err == nil {
			preferredSet = true
		}
	}

	if !preferredSet {
		if err := database.DB.Where("is_default = ? AND is_active = ?", true, true).First(&preferred).Error; err == nil {
			preferredSet = true
		}
	}

	if !preferredSet {
		agentName := "desconocido"
		if session.Agent != nil {
			agentName = session.Agent.Name
		}
		return nil, 0, fmt.Errorf("El agente '%s' no tiene un modelo específico, y no hay un proveedor global por defecto. Configura uno en la pestaña Agentes o Proveedores", agentName)
	}

	var others []database.LLMProvider
	if err := database.DB.Where("is_active = ? AND id <> ?", true, preferred.ID).Order("is_default desc, id asc").Find(&others).Error; err != nil {
		return nil, 0, err
	}

	candidates := make([]database.LLMProvider, 0, len(others)+1)
	candidates = append(candidates, preferred)
	candidates = append(candidates, others...)
	return candidates, 0, nil
}

// ReloadMCPIntegrations reconecta servidores MCP desde la BD (stdio + stream).
func (b *Brain) ReloadMCPIntegrations(ctx context.Context) {
	if b.McpStdio != nil {
		b.McpStdio.ReloadFromDB(ctx)
	}
	if b.McpStream != nil {
		b.McpStream.ReloadFromDB(ctx)
	}
}

func NewBrain(llmKey, llmBaseURL string, handsaiCfg handsai.Config, permHandler handsai.PermissionHandler) *Brain {
	ph := permHandler
	if ph == nil {
		ph = handsai.DefaultPermissionHandler
	}
	b := &Brain{
		LLM:        NewClient(llmKey, llmBaseURL),
		HandsAI:    handsai.NewClient(handsaiCfg, ph),
		Registry:   NewToolRegistry(),
		toolPermit: ph,
	}
	b.registerNativeTools()
	return b
}

// registerNativeTools re-registers all built-in (Go) tools in the registry.
// Called on startup and on every SyncTools to avoid losing native tools after a clear.
func (b *Brain) registerNativeTools() {
	b.Registry.Register(ToolDef{
		Name:        "schedule_task",
		Description: "Programa una tarea recurrente que ejecutará un agente con un prompt en lenguaje natural a la frecuencia indicada.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Nombre de la tarea"},"cron_expression":{"type":"string","description":"Expresion cron, ej: @hourly, @daily, 0 9 * * *"},"agent_id":{"type":"number","description":"ID del agente que ejecutara la tarea (default: 1)"},"prompt":{"type":"string","description":"Instruccion en lenguaje natural que el agente ejecutara"}},"required":["name","cron_expression","prompt"]}`),
		Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
			agentID := uint(1)
			if v, ok := args["agent_id"]; ok {
				switch n := v.(type) {
				case float64:
					agentID = uint(n)
				case int:
					agentID = uint(n)
				}
			}
			newTask, err := tasksvc.CreateScheduledTask(tasksvc.CreateTaskInput{
				Name:           fmt.Sprintf("%v", args["name"]),
				CronExpression: fmt.Sprintf("%v", args["cron_expression"]),
				AgentID:        agentID,
				Prompt:         fmt.Sprintf("%v", args["prompt"]),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to save scheduled task: %w", err)
			}
			return []byte(fmt.Sprintf(`{"status":"success","task_id":%d}`, newTask.ID)), nil
		},
		Sensitive: true,
	})
}

// SyncTools fetches tools from HandsAI and registers them in the local Registry.
// It returns nil if the sync should proceed gracefully even if tools are unavailable.
func (b *Brain) SyncTools(ctx context.Context) error {
	// 1. Limpiar el registry y volver a registrar las tools nativas.
	// Esto garantiza que no queden herramientas "fantasma" de una sesión previa de HandsAI.
	b.Registry.Clear()
	b.registerNativeTools()

	// 2. Registrar tools de HandsAI si está configurado. Si falla, seguimos con stdio.
	if b.HandsAI == nil || !b.HandsAI.IsConfigured() {
		log.Printf("⚠️ SyncTools: HandsAI not configured, skipping HandsAI tools.")
	} else {
		handsaiToolsRaw, err := b.HandsAI.GetTools(ctx)
		if err != nil {
			log.Printf("❌ Failed to fetch tools from HandsAI: %v", err)
		} else {
			var mcpResponse struct {
				Tools []struct {
					Name        string          `json:"name"`
					Description string          `json:"description"`
					InputSchema json.RawMessage `json:"inputSchema"`
				} `json:"tools"`
			}
			if err := json.Unmarshal(handsaiToolsRaw, &mcpResponse); err != nil {
				// Algunos bridges devuelven la lista directo como array
				var directArray []struct {
					Name        string          `json:"name"`
					Description string          `json:"description"`
					InputSchema json.RawMessage `json:"inputSchema"`
				}
				if errArray := json.Unmarshal(handsaiToolsRaw, &directArray); errArray == nil {
					mcpResponse.Tools = directArray
				} else {
					log.Printf("❌ SyncTools: Failed to parse MCP tools (tried object and array): %v. Body: %s", err, string(handsaiToolsRaw))
				}
			}
			for _, mt := range mcpResponse.Tools {
				origName := mt.Name

				// Clasificación de sensibilidad: solo operaciones de escritura/destructivas requieren confirmación.
				isSensitive := false
				lowerName := strings.ToLower(origName)

				// 1. Si el nombre contiene un verbo de lectura, nunca es sensible
				readVerbs := []string{"get", "list", "read", "search", "ver", "buscar", "view", "fetch", "home", "notificacion"}
				isReadOnly := false
				for _, rv := range readVerbs {
					if strings.Contains(lowerName, rv) {
						isReadOnly = true
						break
					}
				}

				if !isReadOnly {
					// 2. Si no es lectura, verificar si es una operación de escritura
					writeVerbs := []string{
						"create", "delete", "update", "post", "publicar", "social_post",
						"save", "move", "add", "approve", "send", "dar_like",
						"schedule", "moltbook_create", "moltbook_verify",
						"jules_create", "jules_approve", "odoo_crm_create",
						"odoo_crm_update", "odoo_project_task_create",
					}
					for _, wv := range writeVerbs {
						if strings.Contains(lowerName, wv) {
							isSensitive = true
							break
						}
					}
				}

				// Sanitizar el esquema de entrada (recursivamente)
				sanitizedSchema, argMap := sanitizeJSONSchema(mt.InputSchema)

				b.Registry.Register(ToolDef{
					Name:        origName,
					Description: mt.Description,
					Parameters:  sanitizedSchema,
					ArgMapping:  argMap,
					Sensitive:   isSensitive,
					Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
						return b.HandsAI.CallTool(ctx, origName, args)
					},
				})
			}
		}
	}

	// 3. Servidores MCP stdio locales: nombres en registry con prefijo alias_
	if b.McpStdio != nil {
		for _, ent := range b.McpStdio.ListEntries() {
			b.registerMcpAliasTools(ctx, "stdio", ent.Alias, ent.Session)
		}
	}

	// 4. Servidores MCP remotos (HTTP streamable / SSE)
	if b.McpStream != nil {
		for _, ent := range b.McpStream.ListEntries() {
			b.registerMcpAliasTools(ctx, "stream", ent.Alias, ent.Session)
		}
	}
	return nil
}

func (b *Brain) registerMcpAliasTools(ctx context.Context, kind, alias string, sess mcpExecutable) {
	mcpTools, err := sess.ListTools(ctx)
	if err != nil {
		log.Printf("❌ SyncTools: MCP %s [%s] list tools failed: %v", kind, alias, err)
		return
	}
	for _, mt := range mcpTools {
		if mt == nil {
			continue
		}
		origName := mt.Name
		schemaBytes := json.RawMessage(`{"type":"object","properties":{}}`)
		if mt.InputSchema != nil {
			if sb, err := json.Marshal(mt.InputSchema); err == nil {
				schemaBytes = sb
			}
		}

		isSensitive := false
		lowerName := strings.ToLower(origName)
		readVerbs := []string{"get", "list", "read", "search", "ver", "buscar", "view", "fetch", "home", "notificacion"}
		isReadOnly := false
		for _, rv := range readVerbs {
			if strings.Contains(lowerName, rv) {
				isReadOnly = true
				break
			}
		}
		if !isReadOnly {
			writeVerbs := []string{
				"create", "delete", "update", "post", "publicar", "social_post",
				"save", "move", "add", "approve", "send", "dar_like",
				"schedule", "moltbook_create", "moltbook_verify",
				"jules_create", "jules_approve", "odoo_crm_create",
				"odoo_crm_update", "odoo_project_task_create",
				"write",
			}
			for _, wv := range writeVerbs {
				if strings.Contains(lowerName, wv) {
					isSensitive = true
					break
				}
			}
		}

		sanitizedSchema, argMap := sanitizeJSONSchema(schemaBytes)
		regName := sanitizeName(alias) + "_" + sanitizeName(origName)
		sessCopy := sess
		mcpToolNameCopy := origName
		regNameCopy := regName

		b.Registry.Register(ToolDef{
			Name:        regNameCopy,
			Description: mt.Description,
			Parameters:  sanitizedSchema,
			ArgMapping:  argMap,
			Sensitive:   isSensitive,
			Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
				if b.toolPermit != nil && !b.toolPermit(ctx, regNameCopy, args) {
					return nil, errors.New("tool execution denied by user/policy")
				}
				return sessCopy.CallTool(ctx, mcpToolNameCopy, args)
			},
		})
	}
}

func sanitizeJSONSchema(raw json.RawMessage) (json.RawMessage, map[string]string) {
	var schema interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return raw, nil
	}

	argMap := make(map[string]string)
	sanitized := sanitizeRecursive(schema, argMap)

	// Post-procesamiento: Si es un objeto sin propiedades, Gemini puede quejarse.
	// Si el esquema raíz es un objeto y no tiene propiedades, lo dejamos como opcional o vacío.
	if sMap, ok := sanitized.(map[string]interface{}); ok {
		if props, ok := sMap["properties"].(map[string]interface{}); ok && len(props) == 0 {
			delete(sMap, "properties")
			delete(sMap, "required")
		}
	}

	res, _ := json.Marshal(sanitized)
	return res, argMap
}

func sanitizeRecursive(val interface{}, argMap map[string]string) interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		// REGLA GEMINI: Solo permitir keywords soportadas
		allowedKeywords := map[string]bool{
			"type":                 true,
			"properties":           true,
			"items":                true,
			"required":             true,
			"description":          true,
			"enum":                 true,
			"additionalProperties": true,
		}

		newMap := make(map[string]interface{})
		for k, child := range v {
			if !allowedKeywords[k] {
				continue // Strip unsupported keywords like pattern, format, allOf, etc.
			}

			newK := k
			if k == "properties" {
				// Solo sanitizamos los nombres de las propiedades reales
				props, ok := child.(map[string]interface{})
				if ok {
					newProps := make(map[string]interface{})
					for propK, propV := range props {
						sanitizedK := sanitizeName(propK)
						newProps[sanitizedK] = sanitizeRecursive(propV, argMap)
						if argMap != nil {
							argMap[sanitizedK] = propK
						}
					}
					newMap[k] = newProps
					continue
				}
			}

			if k == "required" {
				reqs, ok := child.([]interface{})
				if ok {
					newReqs := make([]interface{}, 0, len(reqs))
					for _, r := range reqs {
						if rStr, ok := r.(string); ok {
							newReqs = append(newReqs, sanitizeName(rStr))
						} else {
							newReqs = append(newReqs, r)
						}
					}
					newMap[k] = newReqs
					continue
				}
			}

			// REGLA GEMINI: 'type' debe ser un string simple, no un array (ej. ["string", "null"])
			if k == "type" {
				if types, ok := child.([]interface{}); ok {
					// Buscamos el primer tipo que NO sea null
					found := false
					for _, t := range types {
						if tStr, ok := t.(string); ok && tStr != "null" {
							newMap[k] = tStr
							found = true
							break
						}
					}
					if !found {
						newMap[k] = "string" // Fallback seguro
					}
					continue
				}
				if tStr, ok := child.(string); ok && tStr == "null" {
					newMap[k] = "string" // Google/Gemini no permite type: null
					continue
				}
			}

			// REGLA GEMINI: Remover 'null' de los enums
			if k == "enum" {
				if enums, ok := child.([]interface{}); ok {
					newEnums := make([]interface{}, 0, len(enums))
					for _, e := range enums {
						if e != nil {
							newEnums = append(newEnums, e)
						}
					}
					newMap[k] = newEnums
					continue
				}
			}

			// Recursión para el resto de campos (ej. items en arrays)
			newMap[newK] = sanitizeRecursive(child, argMap)
		}
		return newMap
	case []interface{}:
		newSlice := make([]interface{}, len(v))
		for i, item := range v {
			newSlice[i] = sanitizeRecursive(item, argMap)
		}
		return newSlice
	default:
		return v
	}
}

// ProcessChatInteraction ejecuta The Brain Loop: Rules + Tools -> LLM -> Execution
func (b *Brain) ProcessChatInteraction(ctx context.Context, sessionID uint, chatHistory []database.ChatMessage, newUserMsg string) (*ChoiceMessage, []database.ChatMessage, error) {
	// 0. Obtener Sesión para saber el Agente asociado
	var session database.Session
	if err := database.DB.Preload("Agent").Preload("Agent.LLMProvider").Preload("Agent.Tools").First(&session, sessionID).Error; err != nil {
		return nil, nil, fmt.Errorf("no se encontró la sesión: %w", err)
	}

	providerCandidates, activeProviderIdx, err := b.resolveProviderCandidates(&session)
	if err != nil {
		return nil, nil, err
	}
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")

	currentProvider := providerCandidates[activeProviderIdx]
	defaultModel := modelForActiveProvider(&session, currentProvider)
	log.Printf("🌐 Provider inicial: %s | Model: %s | URL: %s", currentProvider.Name, defaultModel, currentProvider.BaseURL)

	systemPrompt := buildSystemPromptForSession(session)
	// 2. Sincronizar Herramientas MCP
	if err := b.SyncTools(ctx); err != nil {
		log.Printf("⚠️ SyncTools Warning: %v", err)
	}

	toolCtx := b.prepareAgentToolContext(session)
	sanitizedToOriginal := toolCtx.SanitizedToOriginal
	openRouterTools := toolCtx.OpenRouterTools

	messages := buildRuntimeMessages(systemPrompt, chatHistory, newUserMsg)

	// 5. Agent Loop — SOLO en memoria. Sin escrituras a DB aquí.
	// Los mensajes intermedios se acumulan en esta slice y en dbMsgsToSave.
	// El handler se encarga de persistirlos al final.
	var dbMsgsToSave []database.ChatMessage
	maxIterations := 5

	for i := 0; i < maxIterations; i++ {
		log.Printf("🤖 [Iter %d/%d] Calling LLM with %d messages, %d tools", i+1, maxIterations, len(messages), len(openRouterTools))

		req := ChatCompletionRequest{
			Model:    defaultModel,
			Messages: messages,
			Tools:    openRouterTools,
		}

		// Debug: mostrar qué mensajes llevan de contexto al LLM en esta iteración
		var ctxSummary []string
		for _, m := range messages {
			if m.Role == "tool" {
				ctxSummary = append(ctxSummary, fmt.Sprintf("tool(id=%s,len=%d)", m.ToolCallID, len(m.Content)))
			} else {
				ctxSummary = append(ctxSummary, fmt.Sprintf("%s(%d calls, %d chars)", m.Role, len(m.ToolCalls), len(m.Content)))
			}
		}
		log.Printf("📚 [Iter %d] Context: %v", i+1, ctxSummary)

		resp, switchNotice, err := b.createChatCompletionWithFallback(ctx, req, &session, providerCandidates, &activeProviderIdx, masterKey)
		if err != nil {
			return nil, nil, err
		}
		if resp == nil || len(resp.Choices) == 0 {
			return nil, nil, fmt.Errorf("no response from llm")
		}

		msg := resp.Choices[0].Message
		if switchNotice != nil {
			msg.ProviderSwitched = true
			msg.ProviderSwitch = switchNotice
		}
		log.Printf("📩 LLM response: content=%d chars, tool_calls=%d", len(msg.Content), len(msg.ToolCalls))

		// ── CASO A: Sin herramientas → respuesta final del usuario ──────────────
		if len(msg.ToolCalls) == 0 {
			return &msg, dbMsgsToSave, nil
		}

		// ── CASO B: Hay herramientas ──────────────────────────────────────────────
		sensitiveTC := b.findSensitiveToolCall(msg.ToolCalls, sanitizedToOriginal, session.AgentID)
		if sensitiveTC != nil {
			// NO añadimos a dbMsgsToSave aquí — el handler de chat guarda respMsg
			// como el mensaje final del asistente (con RawToolCalls).
			// Si lo agregáramos acá también, habría dos asistentes con el mismo tool_call
			// y solo una respuesta tool → el filtro de huérfanos borraría uno pero el contexto quedaría mal.
			log.Printf("🔒 Tool '%s' requires confirmation, returning for user approval.", sensitiveTC.Function.Name)
			msg.RequiresConfirmation = true
			msg.WaitingToolCall = sensitiveTC
			return &msg, dbMsgsToSave, nil
		}

		// ── Ejecución inmediata (no sensibles) ───────────────────────────────────
		messages, dbMsgsToSave = appendAssistantToolCallContext(messages, dbMsgsToSave, sessionID, msg)
		toolMessages, toolDBMessages := b.executeImmediateToolCalls(ctx, sessionID, msg.ToolCalls, sanitizedToOriginal)
		messages = append(messages, toolMessages...)
		dbMsgsToSave = append(dbMsgsToSave, toolDBMessages...)
		// Loop continúa: el LLM leerá [assistant(tool_calls) → tool(result)]
	}

	return &ChoiceMessage{Content: "Proceso completado."}, dbMsgsToSave, nil
}

func sanitizeName(name string) string {
	var res strings.Builder
	for _, r := range name {
		// Algunos modelos (especialmente Gemini) son muy estrictos: solo [a-zA-Z0-9_]
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			res.WriteRune(r)
		} else {
			res.WriteRune('_')
		}
	}
	return res.String()
}

func getToolNames(tools []Tool) []string {
	var names []string
	for _, t := range tools {
		names = append(names, t.Function.Name)
	}
	return names
}
