package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"aigent/internal/database"
)

type ContextKey string

const IsScheduledTaskKey ContextKey = "is_scheduled_task"
const AutoApproveToolsKey ContextKey = "auto_approve_tools"

func (b *Brain) findSensitiveToolCall(ctx context.Context, toolCalls []ToolCall, sanitizedToOriginal map[string]string, agentID uint) *ToolCall {
	for i, tc := range toolCalls {
		realName, ok := sanitizedToOriginal[tc.Function.Name]
		if !ok {
			realName = tc.Function.Name
		}
		tDef, exists := b.Registry.Get(realName)
		if !exists || !tDef.Sensitive {
			continue
		}

		if hasAutoAllowPermission(ctx, agentID, realName) {
			continue
		}

		return &toolCalls[i]
	}
	return nil
}

func (b *Brain) findSensitiveToolCalls(ctx context.Context, toolCalls []ToolCall, sanitizedToOriginal map[string]string, agentID uint) []ToolCall {
	var sensitive []ToolCall
	for _, tc := range toolCalls {
		realName, ok := sanitizedToOriginal[tc.Function.Name]
		if !ok {
			realName = tc.Function.Name
		}
		tDef, exists := b.Registry.Get(realName)
		if !exists || !tDef.Sensitive {
			continue
		}

		if hasAutoAllowPermission(ctx, agentID, realName) {
			continue
		}

		sensitive = append(sensitive, tc)
	}
	return sensitive
}

func (b *Brain) isToolCallSensitive(ctx context.Context, tc ToolCall, sanitizedToOriginal map[string]string, agentID uint) bool {
	realName, ok := sanitizedToOriginal[tc.Function.Name]
	if !ok {
		realName = tc.Function.Name
	}
	tDef, exists := b.Registry.Get(realName)
	if !exists || !tDef.Sensitive {
		return false
	}
	return !hasAutoAllowPermission(ctx, agentID, realName)
}

var permissionChecker = func(agentID uint, toolName string) bool {
	var perm database.ToolPermission
	log.Printf("🔍 DB permission check: querying permission for agent #%d and tool '%s'", agentID, toolName)
	result := database.DB.Where("agent_id = ? AND tool_name = ? AND action_type = ? AND paused = ?",
		agentID, toolName, "always_allow", false).First(&perm)
	if result.Error != nil {
		log.Printf("🔍 DB permission check: result error: %v", result.Error)
		return false
	}
	log.Printf("🔍 DB permission check: found active permission ID #%d", perm.ID)
	return true
}

func hasAutoAllowPermission(ctx context.Context, agentID uint, toolName string) bool {
	if ctx != nil && ctx.Value(AutoApproveToolsKey) == true {
		log.Printf("🔒 Permission Bypass: Tool '%s' is auto-approved for this workflow/session context", toolName)
		return true
	}
	if database.DB == nil {
		log.Printf("🔍 DB permission check: database.DB is nil!")
		return false
	}
	return permissionChecker(agentID, toolName)
}

func appendAssistantToolCallContext(
	messages []ChatMessage,
	dbMsgsToSave []database.ChatMessage,
	sessionID uint,
	msg ChoiceMessage,
) ([]ChatMessage, []database.ChatMessage) {
	assistantContent := msg.Content
	if assistantContent == "" {
		assistantContent = " " // Google/Vertex rechaza content vacío.
	}

	rawTools, _ := json.Marshal(msg.ToolCalls)
	messages = append(messages, ChatMessage{
		Role:      "assistant",
		Content:   assistantContent,
		ToolCalls: msg.ToolCalls,
	})
	dbMsgsToSave = append(dbMsgsToSave, database.ChatMessage{
		SessionID:    sessionID,
		Role:         "assistant",
		Content:      msg.Content,
		RawToolCalls: string(rawTools),
	})

	return messages, dbMsgsToSave
}

func (b *Brain) executeImmediateToolCalls(
	ctx context.Context,
	sessionID uint,
	toolCalls []ToolCall,
	sanitizedToOriginal map[string]string,
) ([]ChatMessage, []database.ChatMessage) {
	var runtimeMessages []ChatMessage
	var dbMessages []database.ChatMessage

	for _, tc := range toolCalls {
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

		runtimeMessages = append(runtimeMessages, ChatMessage{
			Role:       "tool",
			Content:    resultStr,
			ToolCallID: tc.ID,
		})
		dbMessages = append(dbMessages, database.ChatMessage{
			SessionID:  sessionID,
			Role:       "tool",
			Content:    resultStr,
			ToolCallID: tc.ID,
		})
	}

	return runtimeMessages, dbMessages
}
