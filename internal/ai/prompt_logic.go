package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"aigent/internal/database"
	"aigent/internal/handsai"
	"aigent/internal/mcpstdio"
	"aigent/internal/mcpstream"
	tasksvc "aigent/internal/tasks"
	"aigent/internal/utils"

	"github.com/pgvector/pgvector-go"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
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

	b.Registry.Register(ToolDef{
		Name:        "list_workflows",
		Description: "Lista todos los workflows de automatización deterministas de RuleGo guardados en la base de datos de AIgent, incluyendo sus IDs, nombres y descripciones.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
			var wfs []database.Workflow
			if err := database.DB.Find(&wfs).Error; err != nil {
				return nil, fmt.Errorf("failed to query workflows: %w", err)
			}
			type wfItem struct {
				ID          uint   `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Enabled     bool   `json:"enabled"`
				Cron        string `json:"cron_expression,omitempty"`
			}
			var items []wfItem
			for _, wf := range wfs {
				items = append(items, wfItem{
					ID:          wf.ID,
					Name:        wf.Name,
					Description: wf.Description,
					Enabled:     wf.Enabled,
					Cron:        wf.CronExpression,
				})
			}
			resBytes, _ := json.Marshal(items)
			return resBytes, nil
		},
		Sensitive: false,
	})

	b.Registry.Register(ToolDef{
		Name:        "read_workflow",
		Description: "Obtiene el JSON de definición completo de un workflow específico a partir de su ID para poder leer su estructura actual de RuleChain.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"number","description":"ID numérico del workflow a consultar"}},"required":["id"]}`),
		Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
			var wfID uint
			if v, ok := args["id"]; ok {
				switch n := v.(type) {
				case float64:
					wfID = uint(n)
				case int:
					wfID = uint(n)
				}
			}
			if wfID == 0 {
				return nil, fmt.Errorf("id is required and must be a valid number")
			}
			var wf database.Workflow
			if err := database.DB.First(&wf, wfID).Error; err != nil {
				return nil, fmt.Errorf("failed to find workflow with ID %d: %w", wfID, err)
			}
			resBytes, _ := json.Marshal(wf)
			return resBytes, nil
		},
		Sensitive: false,
	})

	b.Registry.Register(ToolDef{
		Name:        "ask_agent",
		Description: "Despierta a un agente específico por su nombre o ID, iniciando un ciclo de ejecución inteligente donde el agente puede usar sus propias herramientas (ej. Playwright, Trello, Odoo) para resolver la tarea y devolver el resultado final.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"prompt":{"type":"string","description":"La instrucción o tarea en lenguaje natural que debe realizar el agente (ej. 'Entra a la página de cotizaciones de dólar, busca el precio de venta y devuélvemelo')."},"agent_id":{"type":"number","description":"ID numérico del agente a despertar. Si se omite, se buscará por el nombre o se usará el agente por defecto."},"agent_name":{"type":"string","description":"Nombre exacto del agente a despertar (ej: 'Navegador Web', 'Asistente de Odoo')."},"auto_approve":{"type":"boolean","description":"Si es true, pre-aprueba automáticamente todas las herramientas sensibles llamadas por este agente en esta corrida, evitando pausarse por aprobación humana."}},"required":["prompt"]}`),
		Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
			prompt, _ := args["prompt"].(string)
			agentIDFloat, _ := args["agent_id"].(float64)
			agentName, _ := args["agent_name"].(string)
			autoApprove, _ := args["auto_approve"].(bool)

			if prompt == "" {
				return nil, fmt.Errorf("prompt is required")
			}

			var agent database.Agent
			var found bool

			// 1. Buscar agente por ID
			if agentIDFloat > 0 {
				if err := database.DB.First(&agent, uint(agentIDFloat)).Error; err == nil {
					found = true
				}
			}

			// 2. Buscar agente por nombre
			if !found && agentName != "" {
				if err := database.DB.Where("name = ?", agentName).First(&agent).Error; err == nil {
					found = true
				}
			}

			// 3. Fallback al agente por defecto (ID = 1 o el marcado como default)
			if !found {
				var defaultAgent database.Agent
				if err := database.DB.Where("is_default = ?", true).First(&defaultAgent).Error; err == nil {
					agent = defaultAgent
					found = true
				} else {
					// Fallback al ID 1
					if err2 := database.DB.First(&agent, 1).Error; err2 == nil {
						found = true
					}
				}
			}

			if !found {
				return nil, fmt.Errorf("could not find any active agent to run this task")
			}

			// Crear una nueva sesión en la base de datos para este agente
			session := database.Session{
				Title:   "Workflow Run: " + agent.Name,
				AgentID: agent.ID,
			}
			if err := database.DB.Create(&session).Error; err != nil {
				return nil, fmt.Errorf("failed to create agent session: %w", err)
			}

			// Si se solicitó auto_approve, inyectarlo en el contexto
			if autoApprove {
				ctx = context.WithValue(ctx, AutoApproveToolsKey, true)
			}

			// Ejecutar la interacción del chat
			respMsg, _, err := b.ProcessChatInteraction(ctx, session.ID, nil, prompt)
			if err != nil {
				return nil, fmt.Errorf("error during agent execution: %w", err)
			}

			type AgentResult struct {
				SessionID uint   `json:"session_id"`
				Response  string `json:"response"`
				Status    string `json:"status"`
			}

			status := "completed"
			if respMsg.RequiresConfirmation || respMsg.WaitingToolCall != nil {
				status = "waiting_approval"
			}

			resBytes, _ := json.Marshal(AgentResult{
				SessionID: session.ID,
				Response:  respMsg.Content,
				Status:    status,
			})
			return resBytes, nil
		},
		Sensitive: false,
	})

	b.Registry.Register(ToolDef{
		Name:        "save_workflow",
		Description: "Crea o actualiza un workflow de automatización determinista de RuleGo en la base de datos de AIgent. Los workflows son Grafos Acíclicos Dirigidos (DAGs) estructurados en formato RuleChain JSON de RuleGo. Permite encadenar herramientas de forma secuencial.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Nombre descriptivo único del workflow"},"description":{"type":"string","description":"Descripción detallada del workflow"},"cron_expression":{"type":"string","description":"Expresión cron opcional para disparar el flujo automáticamente (ej: @hourly, @daily, 0 9 * * *). Si se omite, se ejecutará manualmente."},"definition":{"type":"string","description":"Definición completa en texto del JSON estructurado de RuleChain para RuleGo"}},"required":["name","description","definition"]}`),
		Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
			name, _ := args["name"].(string)
			description, _ := args["description"].(string)
			cronExpr, _ := args["cron_expression"].(string)
			definition, _ := args["definition"].(string)

			if name == "" || definition == "" {
				return nil, fmt.Errorf("name and definition are required")
			}

			// Normalizar y validar JSON de la definición
			normalizedDef, err := utils.NormalizeRuleChainJSON(definition)
			if err != nil {
				return nil, fmt.Errorf("invalid workflow definition: %w", err)
			}

			// Buscar si ya existe para actualizarlo, o crear uno nuevo
			var existing database.Workflow
			if err := database.DB.Where("name = ?", name).First(&existing).Error; err == nil {
				existing.Description = description
				existing.CronExpression = cronExpr
				existing.Definition = normalizedDef
				if err := database.DB.Save(&existing).Error; err != nil {
					return nil, fmt.Errorf("failed to update workflow: %w", err)
				}
				// Recargar flujos en caliente en RuleGo
				_ = ReloadWorkflows()
				return []byte(fmt.Sprintf(`{"status":"success","action":"updated","workflow_id":%d}`, existing.ID)), nil
			}

			wf := database.Workflow{
				Name:           name,
				Description:    description,
				CronExpression: cronExpr,
				Definition:     normalizedDef,
				Enabled:        true,
			}

			if err := database.DB.Create(&wf).Error; err != nil {
				return nil, fmt.Errorf("failed to create workflow: %w", err)
			}

			// Recargar flujos en caliente en RuleGo
			_ = ReloadWorkflows()

			return []byte(fmt.Sprintf(`{"status":"success","action":"created","workflow_id":%d}`, wf.ID)), nil
		},
		Sensitive: false,
	})

	b.Registry.Register(ToolDef{
		Name:        "get_workflow_guide",
		Description: "Devuelve la especificación del formato JSON de RuleGo, ejemplos detallados de flujos y la lista actual de herramientas activas disponibles que se pueden integrar en los flujos.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
			var toolsList strings.Builder
			for _, t := range b.Registry.List() {
				if t.Name == "schedule_task" || t.Name == "save_workflow" || t.Name == "get_workflow_guide" || t.Name == "list_workflows" || t.Name == "read_workflow" {
					continue
				}
				toolsList.WriteString(fmt.Sprintf("- `%s`: %s\n", t.Name, t.Description))
			}

			guide := fmt.Sprintf(`# Guía de Formato de Workflows de RuleGo para AIgent

Los workflows son Grafos Acíclicos Dirigidos (DAGs) definidos como cadenas de reglas (RuleChains) en formato JSON.

## Estructura General del JSON
El JSON debe tener dos secciones principales:
1. "ruleChain": Contiene metadatos del flujo ("id", "name").
2. "metadata": Contiene la lista de "nodes" y "connections".

Ejemplo de estructura base:
{
  "ruleChain": {
    "id": "nombre_unico_id",
    "name": "Nombre Descriptivo"
  },
  "metadata": {
    "nodes": [],
    "connections": []
  }
}

## Tipos de Nodo Admitidos

### 1. Herramientas del Sistema (type: "aigent/tool")
Representa la ejecución de cualquier herramienta del sistema (MCP, local skills, etc.).
- **type**: "aigent/tool"
- **configuration**: Debe incluir "toolName" indicando la herramienta exacta a ejecutar.
Ejemplo:
{
  "id": "n1",
  "type": "aigent/tool",
  "name": "Listar órdenes",
  "configuration": {
    "toolName": "odoo_sale_order_list"
  }
}

### 2. Transformación JavaScript (type: "jsTransform")
Ejecuta código JavaScript ES5 para transformar el mensaje ("msg") o los metadatos ("metadata").
- **type**: "jsTransform"
- **configuration**: Debe incluir "jsScript" con una función JS que retorne un objeto con {'msg': msg, 'metadata': metadata, 'msgType': msgType}.
Ejemplo:
{
  "id": "n2",
  "type": "jsTransform",
  "name": "Formatear datos",
  "configuration": {
    "jsScript": "msg.total = msg.amount * 1.21; return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
  }
}

### 3. Filtro JavaScript (type: "jsFilter")
Filtra mensajes según una condición lógica en JS.
- **type**: "jsFilter"
- **configuration**: Debe incluir "jsScript" que retorne true o false.
- **Conexiones**: Las conexiones de salida deben tener type "True" (si pasa el filtro) o "False" (si no pasa).
Ejemplo:
{
  "id": "n3",
  "type": "jsFilter",
  "name": "Filtrar por monto",
  "configuration": {
    "jsScript": "return msg.amount > 1000;"
  }
}

### 4. Switch de Ruta JavaScript (type: "jsSwitch")
Enruta el mensaje a uno o más caminos según una condición JS.
- **type**: "jsSwitch"
- **configuration**: Debe incluir "jsScript" que retorne una lista de nombres de relaciones (ej. ['RutaA', 'RutaB']).
- **Conexiones**: Las conexiones de salida deben coincidir en su "type" con las relaciones retornadas.
Ejemplo:
{
  "id": "n4",
  "type": "jsSwitch",
  "name": "Decidir ruta",
  "configuration": {
    "jsScript": "if (msg.priority == 'high') return ['Urgente']; return ['Normal'];"
  }
}

## Conexiones y Relaciones
Las conexiones enlazan nodos. Campos requeridos:
- **fromId**: El ID del nodo de origen.
- **toId**: El ID del nodo de destino.
- **type**: Tipo de relación.
  - Para "aigent/tool" y "jsTransform": usar "Success" o "Failure".
  - Para "jsFilter": usar "True" o "False".
  - Para "jsSwitch": el nombre del canal devuelto por el JS.

## Lista de Herramientas Disponibles en este Agente (para "toolName")
Puedes usar cualquiera de estas herramientas activas en los nodos de tipo "aigent/tool":
%s`, toolsList.String())

			result := map[string]string{
				"guide": guide,
			}
			jsonResult, _ := json.Marshal(result)
			return jsonResult, nil
		},
		Sensitive: false,
	})
}

// SyncTools fetches tools from HandsAI and registers them in the local Registry.
// It returns nil if the sync should proceed gracefully even if tools are unavailable.
func (b *Brain) SyncTools(ctx context.Context) error {
	// 1. Limpiar el registry y volver a registrar las tools nativas y dinámicas (Skills).
	// Esto garantiza que no queden herramientas "fantasma" de una sesión previa de HandsAI.
	b.Registry.Clear()
	b.registerNativeTools()
	if err := LoadSkills("./skills", b.Registry); err != nil {
		log.Printf("⚠️ SyncTools: Error loading dynamic skills: %v", err)
	}

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
	ctx = context.WithValue(ctx, "session_id", sessionID)
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
	if newUserMsg != "" {
		if ragContext := b.retrieveRAGContext(ctx, newUserMsg); ragContext != "" {
			systemPrompt += ragContext
			log.Printf("📚 RAG context retrieved and appended to system prompt:\n%s", ragContext)
		}
	}

	// 2. Sincronizar Herramientas MCP
	if err := b.SyncTools(ctx); err != nil {
		log.Printf("⚠️ SyncTools Warning: %v", err)
	}

	toolCtx := b.prepareAgentToolContext(session)
	sanitizedToOriginal := toolCtx.SanitizedToOriginal
	openRouterTools := toolCtx.OpenRouterTools

	// 3. Obtener sesión activa en memoria
	activeSess, err := GetSessionManager().GetOrCreateSession(sessionID)
	if err != nil {
		return nil, nil, err
	}

	// 4. Agregar mensaje de usuario si existe
	if newUserMsg != "" {
		activeSess.AddMessage("user", newUserMsg, "", "")
	}

	// Cargar archivos de contexto de sesión
	var sessionFiles []database.SessionFile
	if err := database.DB.Where("session_id = ?", sessionID).Find(&sessionFiles).Error; err != nil {
		log.Printf("⚠️ SmartContextCache: Failed to fetch session files for session #%d: %v", sessionID, err)
	}

	maxIterations := 5
	for i := 0; i < maxIterations; i++ {
		currentProvider := providerCandidates[activeProviderIdx]
		defaultModel := modelForActiveProvider(&session, currentProvider)
		apiKey, decErr := utils.Decrypt(currentProvider.APIKey, masterKey)
		if decErr != nil {
			return nil, nil, fmt.Errorf("no se pudo desencriptar API key del proveedor: %w", decErr)
		}
		sccPlan, layer2Content := b.prepareSCC(ctx, &session, sessionFiles, currentProvider, defaultModel, apiKey)

		// Reconstruir la lista de mensajes optimizada y compactada en memoria para esta iteración
		sessMsgs := activeSess.GetMessages()
		runtimeMessages := buildRuntimeMessagesWithCache(systemPrompt, layer2Content, sccPlan, sessMsgs, "")
		optimizedMessages := pruneMessagesInMemory(runtimeMessages, activeSess.ContextSummary)

		log.Printf("🤖 [Iter %d/%d] Calling LLM with %d messages (optimized), %d tools", i+1, maxIterations, len(optimizedMessages), len(openRouterTools))

		req := ChatCompletionRequest{
			Model:         defaultModel,
			Messages:      optimizedMessages,
			Tools:         openRouterTools,
			CachedContent: sccPlan.CachedContentName,
		}

		// Debug: mostrar qué mensajes llevan de contexto al LLM en esta iteración
		var ctxSummary []string
		for _, m := range optimizedMessages {
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
			// Persistir el mensaje final del asistente en memoria y DB
			activeSess.AddMessage("assistant", msg.Content, "", "")
			// Disparar compactación asíncrona en segundo plano si aplica
			b.triggerAsyncCompaction(sessionID, activeSess, defaultModel)
			return &msg, nil, nil
		}

		// ── CASO B: Hay herramientas ──────────────────────────────────────────────
		sensitiveTCs := b.findSensitiveToolCalls(ctx, msg.ToolCalls, sanitizedToOriginal, session.AgentID)
		if len(sensitiveTCs) > 0 {
			// Persistir el mensaje del asistente con las tool_calls en memoria y DB
			rawTools, _ := json.Marshal(msg.ToolCalls)
			activeSess.AddMessage("assistant", msg.Content, "", string(rawTools))

			// Crear la PendingAction para cada herramienta sensible
			for _, tc := range sensitiveTCs {
				pending := database.PendingAction{
					SessionID:  session.ID,
					ToolName:   tc.Function.Name,
					Arguments:  tc.Function.Arguments,
					ToolCallID: tc.ID,
					Status:     "PENDING",
				}
				database.DB.Create(&pending)
				log.Printf("🔒 Tool '%s' (ID %s) requires confirmation. Created PendingAction #%d.", tc.Function.Name, tc.ID, pending.ID)
			}
			
			// Ejecutar inmediatamente todas las herramientas en el mismo mensaje que NO sean sensibles
			for _, tc := range msg.ToolCalls {
				if b.isToolCallSensitive(ctx, tc, sanitizedToOriginal, session.AgentID) {
					continue
				}

				realName, ok := sanitizedToOriginal[tc.Function.Name]
				if !ok {
					realName = tc.Function.Name
				}
				tDef, exists := b.Registry.Get(realName)
				if !exists {
					log.Printf("⚠️ Tool not found in registry during confirm pre-execution: %s", realName)
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

				log.Printf("🦾 Executing non-sensitive tool pre-confirmation: %s", realName)
				result, execErr := tDef.Execute(ctx, finalArgs)
				resultStr := string(result)
				if execErr != nil {
					resultStr = fmt.Sprintf(`{"error": "%s"}`, execErr.Error())
				}
				activeSess.AddMessage("tool", resultStr, tc.ID, "")
			}

			msg.RequiresConfirmation = true
			msg.WaitingToolCall = &sensitiveTCs[0]
			return &msg, nil, nil
		}

		// ── Ejecución inmediata (no sensibles) ───────────────────────────────────
		// Guardar el mensaje del asistente con las tool_calls
		rawTools, _ := json.Marshal(msg.ToolCalls)
		activeSess.AddMessage("assistant", msg.Content, "", string(rawTools))

		// Ejecutar y guardar las respuestas de las tools
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

			activeSess.AddMessage("tool", resultStr, tc.ID, "")
		}
	}

	return &ChoiceMessage{Content: "Proceso completado."}, nil, nil
}

func truncateToolOutput(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= 30 {
		return content
	}
	firstPart := strings.Join(lines[:15], "\n")
	lastPart := strings.Join(lines[len(lines)-15:], "\n")
	return fmt.Sprintf("%s\n\n... [Sigue output de herramienta truncado por antigüedad (%d líneas omitidas)] ...\n\n%s", firstPart, len(lines)-30, lastPart)
}

func findSafeCutIndex(messages []ChatMessage, targetLimit int) int {
	if len(messages) <= targetLimit {
		return 0
	}

	cutIndex := len(messages) - targetLimit

	// Recopilamos todos los tool_call_ids de los mensajes con rol "tool" que mantendremos
	keptToolCallIDs := make(map[string]bool)
	for i := cutIndex; i < len(messages); i++ {
		if messages[i].Role == "tool" && messages[i].ToolCallID != "" {
			keptToolCallIDs[messages[i].ToolCallID] = true
		}
	}

	// Corremos el corte hacia atrás para incluir cualquier assistant que originó esos tool_call_ids
	for {
		shifted := false
		for i := 0; i < cutIndex; i++ {
			if messages[i].Role == "assistant" && len(messages[i].ToolCalls) > 0 {
				hasKeptCall := false
				for _, tc := range messages[i].ToolCalls {
					if keptToolCallIDs[tc.ID] {
						hasKeptCall = true
						break
					}
				}
				if hasKeptCall {
					cutIndex = i
					shifted = true
					break
				}
			}
		}
		if !shifted {
			break
		}
	}

	// Por seguridad, nunca debemos iniciar el historial podado con un mensaje de rol "tool"
	for cutIndex < len(messages) && messages[cutIndex].Role == "tool" {
		if cutIndex > 0 {
			cutIndex--
		} else {
			break
		}
	}

	return cutIndex
}

func pruneMessagesInMemory(messages []ChatMessage, contextSummary string) []ChatMessage {
	recentLimit := 8
	if len(messages) <= recentLimit {
		if contextSummary != "" {
			return append([]ChatMessage{{Role: "system", Content: "Resumen técnico de la conversación previa:\n" + contextSummary}}, messages...)
		}
		return messages
	}

	pruned := make([]ChatMessage, len(messages))
	copy(pruned, messages)

	oldIndexCut := findSafeCutIndex(pruned, recentLimit)

	for i := 0; i < oldIndexCut; i++ {
		msg := &pruned[i]
		if msg.Role == "tool" && len(msg.Content) > 2000 {
			msg.Content = truncateToolOutput(msg.Content)
		}
	}

	if oldIndexCut > 0 && contextSummary != "" {
		result := []ChatMessage{{Role: "system", Content: "Resumen técnico de la conversación previa:\n" + contextSummary}}
		result = append(result, pruned[oldIndexCut:]...)
		return result
	}

	return pruned[oldIndexCut:]
}

func (b *Brain) triggerAsyncCompaction(sessionID uint, activeSess *ActiveSession, defaultModel string) {
	activeSess.mu.RLock()
	msgCount := len(activeSess.Messages)
	summaryExists := activeSess.ContextSummary != ""
	activeSess.mu.RUnlock()

	if msgCount > 25 && (!summaryExists || msgCount%10 == 0) {
		go func() {
			log.Printf("🔄 SessionManager: Starting async compaction for session #%d", sessionID)

			activeSess.mu.RLock()
			if len(activeSess.Messages) <= 8 {
				activeSess.mu.RUnlock()
				return
			}
			oldMessages := make([]ChatMessage, len(activeSess.Messages)-8)
			copy(oldMessages, activeSess.Messages[:len(activeSess.Messages)-8])
			previousSummary := activeSess.ContextSummary
			activeSess.mu.RUnlock()

			var sb strings.Builder
			sb.WriteString("Resume el progreso de la conversación previa de forma técnica y estructurada. ")
			sb.WriteString("Describe brevemente el objetivo final, qué tareas se han completado, qué está en progreso y qué archivos han sido creados o modificados. ")
			sb.WriteString("Mantén estrictamente nombres de archivos, funciones, variables, comandos y errores exactos si son relevantes. ")
			sb.WriteString("Sé conciso, usa viñetas en Markdown.\n\n")

			if previousSummary != "" {
				sb.WriteString(fmt.Sprintf("Resumen previo a actualizar:\n%s\n\n", previousSummary))
			}

			sb.WriteString("Historial de mensajes a resumir:\n")
			for _, m := range oldMessages {
				content := m.Content
				if m.Role == "tool" && len(content) > 1000 {
					content = truncateToolOutput(content)
				}
				sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, content))
			}

			req := ChatCompletionRequest{
				Model: defaultModel,
				Messages: []ChatMessage{
					{Role: "user", Content: sb.String()},
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			resp, err := b.LLM.CreateChatCompletion(ctx, req)
			if err != nil {
				log.Printf("⚠️ SessionManager: Async compaction LLM call failed for session #%d: %v", sessionID, err)
				return
			}

			if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != "" {
				newSummary := resp.Choices[0].Message.Content
				activeSess.SaveContextSummary(newSummary)
				log.Printf("✅ SessionManager: Async compaction succeeded for session #%d. New summary length: %d chars", sessionID, len(newSummary))
			}
		}()
	}
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

func (b *Brain) ProcessChatInteractionStream(ctx context.Context, sessionID uint, newUserMsg string, onEvent func(eventType string, data interface{})) (*ChoiceMessage, error) {
	ctx = context.WithValue(ctx, "session_id", sessionID)
	// 0. Obtener Sesión para saber el Agente asociado
	var session database.Session
	if err := database.DB.Preload("Agent").Preload("Agent.LLMProvider").Preload("Agent.Tools").First(&session, sessionID).Error; err != nil {
		return nil, fmt.Errorf("no se encontró la sesión: %w", err)
	}

	providerCandidates, activeProviderIdx, err := b.resolveProviderCandidates(&session)
	if err != nil {
		return nil, err
	}
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")

	currentProvider := providerCandidates[activeProviderIdx]
	defaultModel := modelForActiveProvider(&session, currentProvider)
	log.Printf("🌐 Stream Provider inicial: %s | Model: %s | URL: %s", currentProvider.Name, defaultModel, currentProvider.BaseURL)

	systemPrompt := buildSystemPromptForSession(session)
	if newUserMsg != "" {
		if ragContext := b.retrieveRAGContext(ctx, newUserMsg); ragContext != "" {
			systemPrompt += ragContext
			log.Printf("📚 Stream RAG context retrieved and appended to system prompt:\n%s", ragContext)
		}
	}

	// 2. Sincronizar Herramientas MCP
	if err := b.SyncTools(ctx); err != nil {
		log.Printf("⚠️ SyncTools Warning: %v", err)
	}

	toolCtx := b.prepareAgentToolContext(session)
	sanitizedToOriginal := toolCtx.SanitizedToOriginal
	openRouterTools := toolCtx.OpenRouterTools

	// 3. Obtener sesión activa en memoria
	activeSess, err := GetSessionManager().GetOrCreateSession(sessionID)
	if err != nil {
		return nil, err
	}

	// 4. Agregar mensaje de usuario si existe
	if newUserMsg != "" {
		activeSess.AddMessage("user", newUserMsg, "", "")
	}

	// Cargar archivos de contexto de sesión
	var sessionFiles []database.SessionFile
	if err := database.DB.Where("session_id = ?", sessionID).Find(&sessionFiles).Error; err != nil {
		log.Printf("⚠️ SmartContextCache: Failed to fetch session files for stream session #%d: %v", sessionID, err)
	}

	maxIterations := 5
	var lastChoice *ChoiceMessage

	for i := 0; i < maxIterations; i++ {
		if ctx.Err() != nil {
			log.Printf("⚠️ Stream: Context cancelled before starting iteration %d. Aborting.", i+1)
			return nil, ctx.Err()
		}
		currentProvider := providerCandidates[activeProviderIdx]
		defaultModel := modelForActiveProvider(&session, currentProvider)
		apiKey, decErr := utils.Decrypt(currentProvider.APIKey, masterKey)
		if decErr != nil {
			return nil, fmt.Errorf("no se pudo desencriptar API key del proveedor: %w", decErr)
		}
		sccPlan, layer2Content := b.prepareSCC(ctx, &session, sessionFiles, currentProvider, defaultModel, apiKey)

		// Reconstruir la lista de mensajes optimizada y compactada en memoria para esta iteración
		sessMsgs := activeSess.GetMessages()
		runtimeMessages := buildRuntimeMessagesWithCache(systemPrompt, layer2Content, sccPlan, sessMsgs, "")
		optimizedMessages := pruneMessagesInMemory(runtimeMessages, activeSess.ContextSummary)

		log.Printf("🤖 [Stream Iter %d/%d] Calling LLM with %d messages (optimized), %d tools", i+1, maxIterations, len(optimizedMessages), len(openRouterTools))

		req := ChatCompletionRequest{
			Model:         defaultModel,
			Messages:      optimizedMessages,
			Tools:         openRouterTools,
			CachedContent: sccPlan.CachedContentName,
		}

		streamBody, switchNotice, err := b.createChatCompletionStreamWithFallback(ctx, req, &session, providerCandidates, &activeProviderIdx, masterKey)
		if err != nil {
			return nil, err
		}
		if switchNotice != nil {
			onEvent("provider_switch", switchNotice)
		}

		// Leer de forma incremental los tokens/tool_calls
		var accumulatedContent strings.Builder
		var accumulatedReasoning strings.Builder
		accumulatedToolCallsMap := make(map[int]*ToolCall)

		reader := bufio.NewReader(streamBody)
		for {
			if ctx.Err() != nil {
				log.Printf("⚠️ Stream: Context cancelled during stream body read loop. Aborting.")
				streamBody.Close()
				return nil, ctx.Err()
			}
			lineBytes, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				streamBody.Close()
				return nil, fmt.Errorf("error reading stream body: %w", err)
			}

			line := strings.TrimSpace(string(lineBytes))
			if line == "" {
				continue
			}

			if strings.HasPrefix(line, ":") { // comentario
				continue
			}

			if line == "data: [DONE]" {
				break
			}

			if strings.HasPrefix(line, "data: ") {
				dataStr := line[6:]
				var deltaResp ChatCompletionStreamResponse
				if err := json.Unmarshal([]byte(dataStr), &deltaResp); err == nil {
					if len(deltaResp.Choices) > 0 {
						choice := deltaResp.Choices[0]
						delta := choice.Delta
						reasoningText := delta.Reasoning
						if reasoningText == "" {
							reasoningText = delta.ReasoningContent
						}
						if reasoningText != "" {
							accumulatedReasoning.WriteString(reasoningText)
							onEvent("reasoning", map[string]string{"text": reasoningText})
						}
						if delta.Content != "" {
							accumulatedContent.WriteString(delta.Content)
							onEvent("token", map[string]string{"text": delta.Content})
						}
						for _, ts := range delta.ToolCalls {
							accumulateToolCallSnippet(accumulatedToolCallsMap, ts)
						}
					}
				}
			}
		}
		streamBody.Close()

		if ctx.Err() != nil {
			log.Printf("⚠️ Stream: Context cancelled after stream body read loop. Aborting.")
			return nil, ctx.Err()
		}

		// Reconstruir la lista de ToolCalls finalizada
		var toolCalls []ToolCall
		for j := 0; j < len(accumulatedToolCallsMap); j++ {
			if tc, exists := accumulatedToolCallsMap[j]; exists {
				toolCalls = append(toolCalls, *tc)
			}
		}

		lastChoice = &ChoiceMessage{
			Role:             "assistant",
			Content:          accumulatedContent.String(),
			Reasoning:        accumulatedReasoning.String(),
			ReasoningContent: accumulatedReasoning.String(),
			ToolCalls:        toolCalls,
		}

		// ── CASO A: Sin herramientas → respuesta final
		if len(toolCalls) == 0 {
			activeSess.AddMessage("assistant", lastChoice.Content, "", "")
			b.triggerAsyncCompaction(sessionID, activeSess, defaultModel)
			return lastChoice, nil
		}

		// ── CASO B: Hay herramientas
		sensitiveTCs := b.findSensitiveToolCalls(ctx, lastChoice.ToolCalls, sanitizedToOriginal, session.AgentID)
		if len(sensitiveTCs) > 0 {
			// Persistir el mensaje del asistente con las tool_calls en memoria y DB
			rawTools, _ := json.Marshal(lastChoice.ToolCalls)
			activeSess.AddMessage("assistant", lastChoice.Content, "", string(rawTools))

			// Crear la PendingAction para cada herramienta sensible
			var firstPendingID uint
			for i, tc := range sensitiveTCs {
				pending := database.PendingAction{
					SessionID:  session.ID,
					ToolName:   tc.Function.Name,
					Arguments:  tc.Function.Arguments,
					ToolCallID: tc.ID,
					Status:     "PENDING",
				}
				database.DB.Create(&pending)
				log.Printf("🔒 Stream: Tool '%s' (ID %s) requires confirmation. Created PendingAction #%d.", tc.Function.Name, tc.ID, pending.ID)
				if i == 0 {
					firstPendingID = pending.ID
				}
			}

			// Pre-ejecutar herramientas que no sean sensibles
			for _, tc := range lastChoice.ToolCalls {
				if b.isToolCallSensitive(ctx, tc, sanitizedToOriginal, session.AgentID) {
					continue
				}

				realName, ok := sanitizedToOriginal[tc.Function.Name]
				if !ok {
					realName = tc.Function.Name
				}
				tDef, exists := b.Registry.Get(realName)
				if !exists {
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

				onEvent("tool_start", map[string]interface{}{"name": realName, "arguments": finalArgs})
				result, execErr := tDef.Execute(ctx, finalArgs)
				resultStr := string(result)
				if execErr != nil {
					resultStr = fmt.Sprintf(`{"error": "%s"}`, execErr.Error())
				}
				onEvent("tool_end", map[string]interface{}{"name": realName, "result": resultStr})
				activeSess.AddMessage("tool", resultStr, tc.ID, "")
			}

			lastChoice.RequiresConfirmation = true
			lastChoice.WaitingToolCall = &sensitiveTCs[0]

			onEvent("confirmation_required", map[string]interface{}{
				"pending_action_id": firstPendingID,
				"waiting_tool":      &sensitiveTCs[0],
			})
			return lastChoice, nil
		}

		// ── Ejecución inmediata (todas no sensibles)
		rawTools, _ := json.Marshal(lastChoice.ToolCalls)
		activeSess.AddMessage("assistant", lastChoice.Content, "", string(rawTools))

		for _, tc := range lastChoice.ToolCalls {
			realName, ok := sanitizedToOriginal[tc.Function.Name]
			if !ok {
				realName = tc.Function.Name
			}
			tDef, exists := b.Registry.Get(realName)
			if !exists {
				log.Printf("⚠️ Stream Tool not found in registry: %s", realName)
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

			onEvent("tool_start", map[string]interface{}{"name": realName, "arguments": finalArgs})
			result, execErr := tDef.Execute(ctx, finalArgs)
			resultStr := string(result)
			if execErr != nil {
				resultStr = fmt.Sprintf(`{"error": "%s"}`, execErr.Error())
			}
			onEvent("tool_end", map[string]interface{}{"name": realName, "result": resultStr})
			activeSess.AddMessage("tool", resultStr, tc.ID, "")
		}
	}

	return lastChoice, nil
}

func accumulateToolCallSnippet(accumulated map[int]*ToolCall, snippet ToolCallSnippet) {
	if snippet.Index == nil {
		return
	}
	idx := *snippet.Index
	tc, exists := accumulated[idx]
	if !exists {
		tc = &ToolCall{
			ID:   snippet.ID,
			Type: snippet.Type,
			Function: FunctionCall{
				Name:      "",
				Arguments: "",
			},
		}
		accumulated[idx] = tc
	}
	if snippet.ID != "" {
		tc.ID = snippet.ID
	}
	if snippet.Type != "" {
		tc.Type = snippet.Type
	}
	if snippet.Function != nil {
		if snippet.Function.Name != "" {
			tc.Function.Name += snippet.Function.Name
		}
		if snippet.Function.Arguments != "" {
			tc.Function.Arguments += snippet.Function.Arguments
		}
	}
}

// retrieveRAGContext searches for similar document chunks in the database and returns a formatted system context.
func (b *Brain) retrieveRAGContext(ctx context.Context, queryText string) string {
	if queryText == "" {
		return ""
	}

	// 1. Get active provider designated for embeddings
	var provider database.LLMProvider
	if err := database.DB.Where("is_active = ? AND is_embeddings = ?", true, true).First(&provider).Error; err != nil {
		log.Printf("⚠️ RAG: No active LLM provider found to generate embeddings: %v", err)
		return ""
	}

	embeddingModel := "text-embedding-3-small"
	if provider.DefaultModel != "" {
		embeddingModel = provider.DefaultModel
	} else {
		provName := strings.ToLower(provider.Name)
		provURL := strings.ToLower(provider.BaseURL)
		if strings.Contains(provName, "gemini") || strings.Contains(provURL, "gemini") {
			embeddingModel = "text-embedding-004"
		}
	}

	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	apiKey, err := utils.Decrypt(provider.APIKey, masterKey)
	if err != nil {
		log.Printf("⚠️ RAG: Failed to decrypt provider API key: %v", err)
		return ""
	}

	log.Printf("🔍 RAG: Generating query embedding for: %q using model: %s (%s)", queryText, embeddingModel, provider.Name)

	// 2. Generate embedding for query
	client := NewClient(apiKey, provider.BaseURL)
	vector, err := client.CreateEmbeddings(ctx, queryText, embeddingModel)
	if err != nil {
		log.Printf("⚠️ RAG: Failed to generate query embedding: %v", err)
		return ""
	}

	log.Printf("🔍 RAG: Successfully generated query embedding vector of size %d. Querying vector database...", len(vector))

	// 3. Query PostgreSQL using pgvector cosine similarity
	var chunks []database.DocumentChunk
	limit := 3
	err = database.DB.Order(gorm.Expr("embedding <=> ?", pgvector.NewVector(vector))).
		Limit(limit).
		Find(&chunks).Error
	if err != nil {
		log.Printf("⚠️ RAG: Database vector search failed: %v", err)
		return ""
	}

	log.Printf("🔍 RAG: Vector search returned %d relevant chunks", len(chunks))

	if len(chunks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n=== CONTEXTO DE CONOCIMIENTO RELEVANTE (RAG) ===\n")
	sb.WriteString("Usa la siguiente información del repositorio de conocimiento para responder la pregunta del usuario:\n")
	for i, chunk := range chunks {
		preview := chunk.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		log.Printf("📚 RAG Chunk %d [Source: %s]: %s", i+1, chunk.Source, preview)
		sb.WriteString(fmt.Sprintf("--- FRAGMENTO %d (Origen: %s) ---\n%s\n", i+1, chunk.Source, chunk.Content))
	}
	sb.WriteString("================================================\n")
	return sb.String()
}
