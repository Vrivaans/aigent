package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenUsageCachedTokensOpenAIFormat(t *testing.T) {
	raw := `{"prompt_tokens":1200,"completion_tokens":40,"total_tokens":1240,"prompt_tokens_details":{"cached_tokens":1024}}`
	var u TokenUsage
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := u.CachedTokens(); got != 1024 {
		t.Fatalf("CachedTokens = %d, want 1024", got)
	}
}

func TestTokenUsageCachedTokensAnthropicFormat(t *testing.T) {
	raw := `{"prompt_tokens":5000,"completion_tokens":120,"total_tokens":5120,"cache_read_input_tokens":4096}`
	var u TokenUsage
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := u.CachedTokens(); got != 4096 {
		t.Fatalf("CachedTokens = %d, want 4096", got)
	}
}

func TestTokenUsageCachedTokensZeroWhenAbsent(t *testing.T) {
	var u TokenUsage
	u.PromptTokens = 100
	if got := u.CachedTokens(); got != 0 {
		t.Fatalf("CachedTokens = %d, want 0", got)
	}
}

func TestCreateChatCompletionParsesUsageFromMock(t *testing.T) {
	body := `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":2000,"completion_tokens":10,"total_tokens":2010,"prompt_tokens_details":{"cached_tokens":1536}}}`
	srv := mockOpenRouterServer(t, body)
	defer srv.Close()

	client := NewClient("test-key", srv.URL)
	resp, err := client.CreateChatCompletion(t.Context(), ChatCompletionRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	if resp.Usage == nil {
		t.Fatal("expected usage block in response")
	}
	if resp.Usage.CachedTokens() != 1536 {
		t.Fatalf("cached_tokens = %d", resp.Usage.CachedTokens())
	}
}

func mockOpenRouterServer(t *testing.T, responseBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseBody))
	}))
}
