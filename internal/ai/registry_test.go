package ai

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"aigent/internal/database"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDef{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters:  json.RawMessage(`{"type":"object"}`),
		Sensitive:   false,
	})

	tool, exists := r.Get("test_tool")
	if !exists {
		t.Fatal("Expected tool to exist after Register")
	}
	if tool.Name != "test_tool" {
		t.Fatalf("Expected name 'test_tool', got %q", tool.Name)
	}
	if tool.Sensitive {
		t.Fatal("Expected non-sensitive tool")
	}
}

func TestRegistryGetNonExistent(t *testing.T) {
	r := NewToolRegistry()
	_, exists := r.Get("does_not_exist")
	if exists {
		t.Fatal("Expected false for non-existent tool")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDef{Name: "tool_a"})
	r.Register(ToolDef{Name: "tool_b"})
	r.Register(ToolDef{Name: "tool_c"})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("Expected 3 tools, got %d", len(list))
	}
}

func TestRegistryClear(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDef{Name: "tool_a"})
	r.Register(ToolDef{Name: "tool_b"})

	r.Clear()

	list := r.List()
	if len(list) != 0 {
		t.Fatalf("Expected 0 tools after Clear, got %d", len(list))
	}
}

func TestRegistryGetBySanitized(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDef{Name: "my-tool-with-dashes"})

	tool, exists := r.GetBySanitized("my_tool_with_dashes")
	if !exists {
		t.Fatal("Expected to find tool via sanitized name")
	}
	if tool.Name != "my-tool-with-dashes" {
		t.Fatalf("Expected original name 'my-tool-with-dashes', got %q", tool.Name)
	}
}

func TestRegistryGetBySanitizedDirectMatch(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDef{Name: "simple_name"})

	tool, exists := r.GetBySanitized("simple_name")
	if !exists {
		t.Fatal("Expected to find tool via direct match")
	}
	if tool.Name != "simple_name" {
		t.Fatalf("Expected 'simple_name', got %q", tool.Name)
	}
}

func TestRegistryGetBySanitizedNotFound(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDef{Name: "existing_tool"})

	_, exists := r.GetBySanitized("non_existent_tool")
	if exists {
		t.Fatal("Expected false for non-matching sanitized name")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dashes", "with_dashes"},
		{"with spaces", "with_spaces"},
		{"UPPERCASE", "UPPERCASE"},
		{"123numbers", "123numbers"},
		{"mix-ed_case 123", "mix_ed_case_123"},
		{"special!@#$%^&*()chars", "special__________chars"},
		{"esp-añol", "esp_a_ol"},
	}

	for _, tt := range tests {
		result := sanitizeName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeJSONSchema(t *testing.T) {
	input := json.RawMessage(`{
		"type": "object",
		"properties": {
			"my-prop": {"type": "string"},
			"another-prop!": {"type": "number"}
		},
		"required": ["my-prop"]
	}`)

	result, argMap := sanitizeJSONSchema(input)

	var schema map[string]interface{}
	if err := json.Unmarshal(result, &schema); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties in schema")
	}

	if _, exists := props["my_prop"]; !exists {
		t.Error("Expected sanitized key 'my_prop'")
	}
	if _, exists := props["another_prop_"]; !exists {
		t.Error("Expected sanitized key 'another_prop_'")
	}

	if argMap["my_prop"] != "my-prop" {
		t.Errorf("Expected argMap['my_prop'] = 'my-prop', got %q", argMap["my_prop"])
	}
}

func TestFindSensitiveToolCall(t *testing.T) {
	b := &Brain{Registry: NewToolRegistry()}

	b.Registry.Register(ToolDef{
		Name:      "safe_tool",
		Sensitive: false,
	})
	b.Registry.Register(ToolDef{
		Name:      "dangerous_tool",
		Sensitive: true,
	})

	toolCalls := []ToolCall{
		{ID: "call_1", Function: FunctionCall{Name: "safe_tool"}},
		{ID: "call_2", Function: FunctionCall{Name: "dangerous_tool"}},
	}

	sensitive := b.findSensitiveToolCall(toolCalls, map[string]string{
		"safe_tool":       "safe_tool",
		"dangerous_tool":  "dangerous_tool",
	}, 1)
	if sensitive == nil {
		t.Fatal("Expected to find sensitive tool call")
	}
	if sensitive.ID != "call_2" {
		t.Fatalf("Expected call_2 to be sensitive, got %q", sensitive.ID)
	}
}

func TestFindSensitiveToolCallNone(t *testing.T) {
	b := &Brain{Registry: NewToolRegistry()}

	b.Registry.Register(ToolDef{
		Name:      "safe_tool",
		Sensitive: false,
	})

	toolCalls := []ToolCall{
		{ID: "call_1", Function: FunctionCall{Name: "safe_tool"}},
	}

	sensitive := b.findSensitiveToolCall(toolCalls, map[string]string{
		"safe_tool": "safe_tool",
	}, 1)
	if sensitive != nil {
		t.Fatal("Expected nil for no sensitive tools")
	}
}

func TestFindSensitiveToolCallWithSanitizedNames(t *testing.T) {
	b := &Brain{Registry: NewToolRegistry()}

	b.Registry.Register(ToolDef{
		Name:      "dangerous-tool",
		Sensitive: true,
	})

	toolCalls := []ToolCall{
		{ID: "call_1", Function: FunctionCall{Name: "dangerous_tool"}},
	}

	sanitizedToOriginal := map[string]string{
		"dangerous_tool": "dangerous-tool",
	}

	sensitive := b.findSensitiveToolCall(toolCalls, sanitizedToOriginal, 1)
	if sensitive == nil {
		t.Fatal("Expected to find sensitive tool call via sanitized name mapping")
	}
}

func TestBuildRuntimeMessages(t *testing.T) {
	chatHistory := []database.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there", RawToolCalls: `[]`},
	}

	messages := buildRuntimeMessages("You are a helpful assistant.", chatHistory, "")

	if len(messages) < 2 {
		t.Fatalf("Expected at least 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("Expected first message to be system, got %q", messages[0].Role)
	}
	if messages[1].Role != "user" {
		t.Errorf("Expected second message to be user, got %q", messages[1].Role)
	}
}

func TestBuildRuntimeMessagesWithNewUserMsg(t *testing.T) {
	chatHistory := []database.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	messages := buildRuntimeMessages("System prompt.", chatHistory, "What's up?")

	lastMsg := messages[len(messages)-1]
	if lastMsg.Role != "user" {
		t.Errorf("Expected last message to be user, got %q", lastMsg.Role)
	}
	if lastMsg.Content != "What's up?" {
		t.Errorf("Expected content 'What's up?', got %q", lastMsg.Content)
	}
}

func TestExecuteImmediateToolCalls(t *testing.T) {
	b := &Brain{Registry: NewToolRegistry()}

	executed := false
	b.Registry.Register(ToolDef{
		Name:      "echo",
		Sensitive: false,
		Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
			executed = true
			return json.RawMessage(`{"echo": true}`), nil
		},
	})

	toolCalls := []ToolCall{
		{ID: "call_1", Function: FunctionCall{
			Name:      "echo",
			Arguments: `{}`,
		}},
	}

	runtimeMsgs, _ := b.executeImmediateToolCalls(context.Background(), 1, toolCalls, map[string]string{"echo": "echo"})

	if !executed {
		t.Fatal("Expected tool to be executed")
	}
	if len(runtimeMsgs) != 1 {
		t.Fatalf("Expected 1 runtime message, got %d", len(runtimeMsgs))
	}
	if runtimeMsgs[0].Role != "tool" {
		t.Errorf("Expected tool role, got %q", runtimeMsgs[0].Role)
	}
}

func TestExecuteImmediateToolCallsNotFound(t *testing.T) {
	b := &Brain{Registry: NewToolRegistry()}

	toolCalls := []ToolCall{
		{ID: "call_1", Function: FunctionCall{
			Name:      "nonexistent",
			Arguments: `{}`,
		}},
	}

	runtimeMsgs, _ := b.executeImmediateToolCalls(context.Background(), 1, toolCalls, map[string]string{})

	if len(runtimeMsgs) != 0 {
		t.Fatalf("Expected 0 runtime messages for unknown tool, got %d", len(runtimeMsgs))
	}
}

func TestIsRecoverableProviderError(t *testing.T) {
	tests := []struct {
		msg       string
		recoverable bool
	}{
		{"insufficient_quota", true},
		{"rate limit exceeded", true},
		{"429 too many requests", true},
		{"401 unauthorized", true},
		{"invalid api key", true},
		{"model_not_found", true},
		{"connection refused", false},
		{"timeout", false},
		{"null pointer", false},
	}

	for _, tt := range tests {
		err := errors.New(tt.msg)
		if got := isRecoverableProviderError(err); got != tt.recoverable {
			t.Errorf("isRecoverableProviderError(%q) = %v, want %v", tt.msg, got, tt.recoverable)
		}
	}
}
