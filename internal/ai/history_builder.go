package ai

func buildRuntimeMessages(systemPrompt string, chatHistory []ChatMessage, newUserMsg string) []ChatMessage {
	messages := []ChatMessage{{Role: "system", Content: systemPrompt}}
	
	for _, msg := range chatHistory {
		content := msg.Content
		if content == "" {
			content = " "
		}
		m := ChatMessage{
			Role:       msg.Role,
			Content:    content,
			ToolCallID: msg.ToolCallID,
			ToolCalls:  msg.ToolCalls,
		}
		messages = append(messages, m)
	}

	if newUserMsg != "" {
		messages = append(messages, ChatMessage{Role: "user", Content: newUserMsg})
	}

	return messages
}
