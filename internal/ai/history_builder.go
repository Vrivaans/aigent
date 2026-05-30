package ai

import (
	"encoding/json"

	"aigent/internal/database"
)

func buildRuntimeMessages(systemPrompt string, chatHistory []database.ChatMessage, newUserMsg string) []ChatMessage {
	respondedToolCallIDs := make(map[string]bool)
	for _, dbMsg := range chatHistory {
		if dbMsg.Role == "tool" && dbMsg.ToolCallID != "" {
			respondedToolCallIDs[dbMsg.ToolCallID] = true
		}
	}

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
				var pairedCalls []ToolCall
				for _, tc := range tCalls {
					if respondedToolCallIDs[tc.ID] {
						pairedCalls = append(pairedCalls, tc)
					}
				}
				if len(pairedCalls) > 0 {
					m.ToolCalls = pairedCalls
				}
			}
		}
		messages = append(messages, m)
	}

	if newUserMsg != "" {
		messages = append(messages, ChatMessage{Role: "user", Content: newUserMsg})
	}

	return messages
}
