package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"aigent/internal/database"
)

func (b *Brain) findSensitiveToolCall(toolCalls []ToolCall, sanitizedToOriginal map[string]string) *ToolCall {
	for i, tc := range toolCalls {
		realName, ok := sanitizedToOriginal[tc.Function.Name]
		if !ok {
			realName = tc.Function.Name
		}
		if tDef, exists := b.Registry.Get(realName); exists && tDef.Sensitive {
			return &toolCalls[i]
		}
	}
	return nil
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
