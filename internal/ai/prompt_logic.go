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
	"aigent/internal/utils"

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
		Description: "Programa una tarea recurrente (@hourly, * * * * *, etc.) que invocará otras herramientas de forma autónoma.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Nombre de la tarea"},"cron_expression":{"type":"string","description":"Expresión en palabras, ej: @hourly, o cada 1 minuto (* * * * *)"},"tool_name":{"type":"string","description":"La herramienta a correr"},"payload":{"type":"object","description":"Argumentos para la herramienta"}},"required":["name","cron_expression","tool_name","payload"]}`),
		Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
			payloadRaw, _ := json.Marshal(args["payload"])
			newTask := database.Task{
				Name:           fmt.Sprintf("%v", args["name"]),
				CronExpression: fmt.Sprintf("%v", args["cron_expression"]),
				ToolName:       fmt.Sprintf("%v", args["tool_name"]),
				Payload:        payloadRaw,
			}
			if err := database.DB.Create(&newTask).Error; err != nil {
				return nil, fmt.Errorf("failed to save scheduled task: %w", err)
			}
			return []byte(fmt.Sprintf(`{"status":"success","task_id":%d}`, newTask.ID)), nil
		},
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

	var provider database.LLMProvider
	useDefault := true

	if session.Agent != nil && session.Agent.LLMProviderID != nil {
		if session.Agent.LLMProvider.ID != 0 {
			provider = session.Agent.LLMProvider
			useDefault = false
		} else {
			// fallback in case preload didn't work properly
			if err := database.DB.First(&provider, *session.Agent.LLMProviderID).Error; err == nil {
				useDefault = false
			} else {
				log.Printf("⚠️ Provider ID %d not found for Agent %s. Falling back to default.", *session.Agent.LLMProviderID, session.Agent.Name)
			}
		}
	}

	if useDefault {
		if err := database.DB.Where("is_default = ? AND is_active = ?", true, true).First(&provider).Error; err != nil {
			agentName := "desconocido"
			if session.Agent != nil {
				agentName = session.Agent.Name
			}
			return nil, nil, fmt.Errorf("El agente '%s' no tiene un modelo específico, y no hay un proveedor global por defecto. Configura uno en la pestaña Agentes o Proveedores", agentName)
		}
	}

	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	apiKey, err := utils.Decrypt(provider.APIKey, masterKey)
	if err != nil {
		return nil, nil, fmt.Errorf("error al descifrar la API Key del proveedor '%s': %w", provider.Name, err)
	}

	// Cliente LLM local — seguro para concurrencia, no comparte estado con b.LLM
	llmClient := NewClient(apiKey, provider.BaseURL)

	defaultModel := provider.DefaultModel
	if defaultModel == "" {
		defaultModel = "meta-llama/llama-3.3-70b-instruct"
	}
	log.Printf("🌐 Provider: %s | Model: %s | URL: %s", provider.Name, defaultModel, provider.BaseURL)

	// 1. Obtener reglas dinámicas (Globales + Específicas del Agente)
	// Globales = reglas sin ninguna entrada en rule_agents
	// Específicas = reglas que tienen una entrada en rule_agents para este agente
	var rules []database.Rule
	if err := database.DB.
		Preload("Agents").
		Where(`id NOT IN (SELECT rule_id FROM rule_agents) OR id IN (SELECT rule_id FROM rule_agents WHERE agent_id = ?)`, session.AgentID).
		Order("importance desc").
		Find(&rules).Error; err != nil {
		log.Printf("Warning: Failed to fetch rules: %v", err)
	}
	var rulesText string
	for _, r := range rules {
		agentScope := "GLOBAL"
		if len(r.Agents) > 0 {
			agentScope = "ESPECÍFICA"
		}
		rulesText += fmt.Sprintf("- [%s] (%s) %s\n", r.Category, agentScope, r.Content)
	}
	systemPrompt := `Eres AIgent, un asistente operativo con capacidad de ejecución real.
REGLAS ACTUALES DEL USUARIO:
` + rulesText + `

Instrucciones Críticas:
1. Tu propósito no es solo hablar, sino EJECUTAR acciones para el usuario.
2. Cada vez que tengas usar una herramienta leé y entendé sus descripciones para formar correctamente los flujos de ejecución si son necesarios.
3. BAJO NINGUNA CIRCUNSTANCIA respondas con un bloque de código JSON de ejemplo.
4. NUNCA menciones que "no tienes acceso directo" o que "estás simulando". Tus herramientas SON reales.
5. NO expliques qué parámetros vas a usar, solo ejecuta la acción.
6. Cuando recibas el resultado de una herramienta (rol "tool"), léelo y responde en lenguaje natural con un resumen útil.
7. Sé proactivo. Si puedes resolver algo con una herramienta, hazlo de una vez.
8. Cuando el usuario pida una acción, ejecutá las tools necesarias de inmediato sin pedir confirmación ni explicar el plan primero.
9. Cuando termines de completar un flujo de ejecución de tools (serie de ejecuciones de tools encadenadas, como varios post, get, etc), es MUY IMPORTANTE que siempre me hagas un resumen muy corto de lo que hiciste e informes si algo salió mal.
10. Guarda lo que creas necesario en las memorias de HandsAI como Knowledge (conocimiento a largo plazo) o como Intent (intención del agente, vos), según corresponda.`
	// 2. Sincronizar Herramientas MCP
	if err := b.SyncTools(ctx); err != nil {
		log.Printf("⚠️ SyncTools Warning: %v", err)
	}

	// 3. Preparar listado de herramientas y mapeo sanitizado -> original
	// 3a. Obtener qué tools están autorizadas explícitamente para este agente
	allowedTools := make(map[string]bool)
	if session.Agent != nil {
		for _, at := range session.Agent.Tools {
			allowedTools[at.ToolName] = true
		}
	}

	sanitizedToOriginal := make(map[string]string)
	var openRouterTools []Tool
	for _, rt := range b.Registry.List() {
		// Filtrar solo las herramientas permitidas
		if session.Agent != nil && len(session.Agent.Tools) > 0 {
			if !allowedTools[rt.Name] {
				continue // Ignorar herramientas que no están seleccionadas en el Agente
			}
		} else if session.Agent != nil && !session.Agent.IsDefault && len(session.Agent.Tools) == 0 {
			// Si el Agente fue creado, pero se le desactivaron todas las tools explícitamente.
			continue
		}

		shortName := sanitizeName(rt.Name)
		sanitizedToOriginal[shortName] = rt.Name

		params := rt.Parameters
		if len(params) == 0 || string(params) == "null" || string(params) == "{}" {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		openRouterTools = append(openRouterTools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        shortName,
				Description: rt.Description,
				Parameters:  params,
			},
		})
	}

	// 4. Reconstruir historial de mensajes EN MEMORIA desde la base de datos
	// Paso 1: Construir set de tool_call IDs que tienen una respuesta en el historial
	respondedToolCallIDs := make(map[string]bool)
	for _, dbMsg := range chatHistory {
		if dbMsg.Role == "tool" && dbMsg.ToolCallID != "" {
			respondedToolCallIDs[dbMsg.ToolCallID] = true
		}
	}

	// Paso 2: Añadir mensajes filtrando tool_calls huérfanos (sin respuesta)
	// Google/Vertex exige que cada tool_call tenga exactamente una tool response.
	messages := []ChatMessage{{Role: "system", Content: systemPrompt}}
	for _, dbMsg := range chatHistory {
		content := dbMsg.Content
		if content == "" {
			content = " " // Google/Vertex rechaza content vacío
		}
		m := ChatMessage{
			Role:    dbMsg.Role,
			Content: content,
		}
		if dbMsg.Role == "tool" {
			m.ToolCallID = dbMsg.ToolCallID
		}
		if dbMsg.Role == "assistant" && dbMsg.RawToolCalls != "" {
			var tCalls []ToolCall
			if err := json.Unmarshal([]byte(dbMsg.RawToolCalls), &tCalls); err == nil {
				// Filtrar solo los tool_calls que tienen respuesta
				var pairedCalls []ToolCall
				for _, tc := range tCalls {
					if respondedToolCallIDs[tc.ID] {
						pairedCalls = append(pairedCalls, tc)
					}
				}
				if len(pairedCalls) > 0 {
					m.ToolCalls = pairedCalls
				} else {
					// Todos huérfanos — no incluir como assistant con tool_calls
					// (solo incluir el texto si hay)
					m.ToolCalls = nil
				}
			}
		}
		messages = append(messages, m)
	}
	// Solo añadir mensaje de usuario si no está vacío
	if newUserMsg != "" {
		messages = append(messages, ChatMessage{Role: "user", Content: newUserMsg})
	}

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

		resp, err := llmClient.CreateChatCompletion(ctx, req)
		if err != nil {
			log.Printf("❌ LLM API Error (%s): %v", provider.Name, err)
			return nil, nil, fmt.Errorf("llm inference failed: %w", err)
		}
		if resp == nil || len(resp.Choices) == 0 {
			return nil, nil, fmt.Errorf("no response from llm")
		}

		msg := resp.Choices[0].Message
		log.Printf("📩 LLM response: content=%d chars, tool_calls=%d", len(msg.Content), len(msg.ToolCalls))

		// ── CASO A: Sin herramientas → respuesta final del usuario ──────────────
		if len(msg.ToolCalls) == 0 {
			return &msg, dbMsgsToSave, nil
		}

		// ── CASO B: Hay herramientas ──────────────────────────────────────────────
		// Detectar si alguna requiere confirmación
		hasSensitive := false
		var sensitiveTC *ToolCall
		for i, tc := range msg.ToolCalls {
			realName, ok := sanitizedToOriginal[tc.Function.Name]
			if !ok {
				realName = tc.Function.Name
			}
			if tDef, exists := b.Registry.Get(realName); exists && tDef.Sensitive {
				hasSensitive = true
				sensitiveTC = &msg.ToolCalls[i]
				break
			}
		}

		if hasSensitive {
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
		// 1. Añadir mensaje del asistente (con tool_calls) al contexto en memoria
		// Nunca content vacío: Google/Vertex lo rechaza con INVALID_ARGUMENT
		assistantContent := msg.Content
		if assistantContent == "" {
			assistantContent = " "
		}
		rawTools, _ := json.Marshal(msg.ToolCalls)
		messages = append(messages, ChatMessage{
			Role:      "assistant",
			Content:   assistantContent,
			ToolCalls: msg.ToolCalls,
		})
		// Acumular para DB (se guardará al final por el handler)
		dbMsgsToSave = append(dbMsgsToSave, database.ChatMessage{
			SessionID:    sessionID,
			Role:         "assistant",
			Content:      msg.Content, // guardamos el original en DB
			RawToolCalls: string(rawTools),
		})

		// 2. Ejecutar cada herramienta y añadir resultado al contexto en memoria
		for _, tc := range msg.ToolCalls {
			realName, ok := sanitizedToOriginal[tc.Function.Name]
			if !ok {
				realName = tc.Function.Name
			}
			tDef, exists := b.Registry.Get(realName)
			if !exists {
				log.Printf("⚠️ Tool not found in registry: %s", realName)
				continue
			}

			var args map[string]interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			finalArgs := make(map[string]interface{})
			for k, v := range args {
				if origK, ok := tDef.ArgMapping[k]; ok {
					finalArgs[origK] = v
				} else {
					finalArgs[k] = v
				}
			}

			log.Printf("🦾 Executing tool: %s with args: %v", realName, finalArgs)
			result, execErr := tDef.Execute(ctx, finalArgs)
			resultStr := string(result)
			if execErr != nil {
				resultStr = fmt.Sprintf(`{"error": "%s"}`, execErr.Error())
				log.Printf("❌ Tool error: %v", execErr)
			} else {
				log.Printf("✅ Tool result: %s", resultStr)
			}

			// Añadir al contexto en memoria (CLAVE para que el LLM lo lea)
			messages = append(messages, ChatMessage{
				Role:       "tool",
				Content:    resultStr,
				ToolCallID: tc.ID,
			})
			// Acumular para DB
			dbMsgsToSave = append(dbMsgsToSave, database.ChatMessage{
				SessionID:  sessionID,
				Role:       "tool",
				Content:    resultStr,
				ToolCallID: tc.ID,
			})
		}
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
