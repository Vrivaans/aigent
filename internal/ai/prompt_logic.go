package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"aigent/internal/database"
	"aigent/internal/handsai"
)

// Brain es el orquestador principal que une el LLM (OpenRouter) con el motor de acciones (HandsAI)
type Brain struct {
	LLM     *OpenRouterClient
	HandsAI *handsai.Client
}

func NewBrain(llmKey string, handsaiCfg handsai.Config, permHandler handsai.PermissionHandler) *Brain {
	return &Brain{
		LLM:     NewClient(llmKey),
		HandsAI: handsai.NewClient(handsaiCfg, permHandler),
	}
}

// ProcessChatInteraction ejecuta The Brain Loop: Rules + Tools -> LLM -> Execution
func (b *Brain) ProcessChatInteraction(ctx context.Context, chatHistory []database.ChatMessage, newUserMsg string) (*ChoiceMessage, error) {
	// 1. Obtener reglas dinámicas para enriquecer el contexto
	var rules []database.Rule
	if err := database.DB.Find(&rules).Error; err != nil {
		log.Printf("Warning: Failed to fetch rules: %v", err)
	}

	var rulesText string
	for _, r := range rules {
		rulesText += fmt.Sprintf("- [%s] %s\n", r.Category, r.Content)
	}

	systemPrompt := `Eres AIgent, un operador digital asistente persistente.
Siempre que te den una instrucción que aplique a futuro, programala usando herramientas cron o informala.
REGLAS ACTUALES DEL USUARIO:
` + rulesText + `
Sigue estas reglas al pie de la letra. Analiza los requests y utiliza las herramientas integradas para satisfacer el requerimiento.`

	// 2. Traer herramientas disponibles dinámicamente desde HandsAI Java API
	handsaiToolsRaw, err := b.HandsAI.GetTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tools from HandsAI: %w", err)
	}

	// 3. Mapear la firma de herramientas de HandsAI a la firma estándar de OpenAI/OpenRouter
	var mcpTools []struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema"` 
	}
	if err := json.Unmarshal(handsaiToolsRaw, &mcpTools); err != nil {
		log.Printf("Warning: Failed to parse MCP tools: %v", err)
	}

	var openRouterTools []Tool
	
	// Inject Native "schedule_task" tool for background cron operations	
	openRouterTools = append(openRouterTools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "schedule_task",
			Description: "Programa una tarea recurrente para ser ejecutada autónomamente en el futuro utilizando las otras herramientas. Utilizala cuando el usuario pida recordar o revisar algo cada cierto tiempo.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Nombre de la tarea"},"cron_expression":{"type":"string","description":"Expresión en palabras, ej: @hourly, o cada 1 minuto (* * * * *)"},"tool_name":{"type":"string","description":"La herramienta a correr"},"payload":{"type":"object","description":"Argumentos para la herramienta"}},"required":["name","cron_expression","tool_name","payload"]}`),
		},
	})

	for _, mt := range mcpTools {
		openRouterTools = append(openRouterTools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        mt.Name,
				Description: mt.Description,
				Parameters:  mt.InputSchema,
			},
		})
	}

	// 4. Transformar el historial de base de datos a mensajes de IA
	messages := []ChatMessage{{Role: "system", Content: systemPrompt}}
	for _, msg := range chatHistory {
		messages = append(messages, ChatMessage{Role: msg.Role, Content: msg.Content})
	}
	messages = append(messages, ChatMessage{Role: "user", Content: newUserMsg})

	req := ChatCompletionRequest{
		Messages: messages,
		Tools:    openRouterTools,
	}

	// 5. Inferencia (Brain Loop - pensar decidir actuar)
	resp, err := b.LLM.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm inference failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from llm")
	}

	msg := resp.Choices[0].Message

	// 6. Tool Proxy: Si el LLM decide usar herramientas HandsAI, las ejecutamos automáticamente
	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			log.Printf("🦾 LLM decided to call tool: %s", tc.Function.Name)
			
			var args map[string]interface{}
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

			// Manejo de la Tool Nativa: schedule_task
			if tc.Function.Name == "schedule_task" {
				payloadRaw, _ := json.Marshal(args["payload"])
				newTask := database.Task{
					Name:           fmt.Sprintf("%v", args["name"]),
					CronExpression: fmt.Sprintf("%v", args["cron_expression"]),
					ToolName:       fmt.Sprintf("%v", args["tool_name"]),
					Payload:        payloadRaw,
				}
				if err := database.DB.Create(&newTask).Error; err != nil {
					log.Printf("❌ Failed to save scheduled task: %v", err)
					msg.Content += fmt.Sprintf("\n(Error programando la tarea: %v)", err)
				} else {
					log.Printf("✅ Task Scheduled Successfully: %s", newTask.Name)
					msg.Content += fmt.Sprintf("\n(Tarea '%s' programada y guardada en base de datos. Se visualizará en el dashboard.)", newTask.Name)
				}
				continue
			}

			// Ejecutamos herramientas de HandsAI (POST REST)
			result, err := b.HandsAI.CallTool(ctx, tc.Function.Name, args)
			
			if err != nil {
				log.Printf("❌ Tool execution failed/denied: %v", err)
				msg.Content += fmt.Sprintf("\n(Intenté ejecutar %s pero falló o requiere confirmación: %v)", tc.Function.Name, err)
			} else {
				log.Printf("✅ Tool execution success: %s", string(result))
				// Hackathon MVP shortcut: we just append the success text to the agent's internal thought/message 
				// In a full implementation, we'd send a tool_result message back to OpenRouter to let it process the response.
				msg.Content += fmt.Sprintf("\n(Ejecuté herramienta %s con éxito: %s)", tc.Function.Name, string(result))
			}
		}
	}

	return &msg, nil
}
