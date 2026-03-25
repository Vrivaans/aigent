package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenRouterClient struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// DefaultModel define un modelo de OpenRouter con alta eficacia para Tool Calling
const DefaultModel = "meta-llama/llama-3.3-70b-instruct"

func NewClient(apiKey, baseURL string) *OpenRouterClient {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	// Normalizar: quitar slashes y espacios finales para evitar URLs como /v1//chat/completions
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	return &OpenRouterClient{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
	}
}

// Request and Response structs para interactuar con la API estructurada (compatible con OpenAI/OpenRouter)
type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []Tool        `json:"tools,omitempty"`
}

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"` // Convertimos content en texto para simplificar
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// Tool definitions (estandar OpenAI usado por OpenRouter)
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"` // Objeto JSON Schema
}

type ChatCompletionResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message ChoiceMessage `json:"message"`
}

type ChoiceMessage struct {
	Role                 string     `json:"role"`
	Content              string     `json:"content"`
	ToolCalls            []ToolCall `json:"tool_calls,omitempty"`
	RequiresConfirmation bool       `json:"requires_confirmation,omitempty"`
	WaitingToolCall      *ToolCall  `json:"waiting_tool_call,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (c *OpenRouterClient) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if req.Model == "" {
		req.Model = DefaultModel
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "http://localhost:3000") // Required by OpenRouter API policies
	httpReq.Header.Set("X-Title", "AIgent")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openrouter API error %d: %s", resp.StatusCode, string(b))
	}

	var completion ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return nil, fmt.Errorf("failed to decode openrouter response: %w", err)
	}

	return &completion, nil
}
