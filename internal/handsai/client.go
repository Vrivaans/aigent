package handsai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Tipos JSON-RPC 2.0 requeridos por HandsAI Java
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type CallToolRequestParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// Config representa la configuración del cliente HandsAI
type Config struct {
	BaseURL    string // e.g., "http://localhost:8080/mcp" o la URL de prod
	Token      string // Token de autenticación para el bridge
	HTTPClient *http.Client
}

type Client struct {
	cfg         Config
	reqID       int64
	permHandler PermissionHandler
	mu          sync.RWMutex
}

// PermissionHandler intercepta llamadas para validar permisos.
// Si retorna false, la ejecución de la tool se deniega.
type PermissionHandler func(ctx context.Context, toolName string, params map[string]interface{}) bool

// DefaultPermissionHandler previene ejecución unicamente logueando el intento por ahora.
func DefaultPermissionHandler(ctx context.Context, name string, p map[string]interface{}) bool {
	sensitiveKeywords := []string{"create", "update", "delete", "post", "write", "send"}
	nameLower := strings.ToLower(name)
	for _, kw := range sensitiveKeywords {
		if strings.Contains(nameLower, kw) {
			// En la implementación real para el Hackaton, aquí se lanzará un aviso al frontend
			// mediante WebSockets para pausar la ejecución de OpenRouter hasta tener un "yes".
			log.Printf("⚠️ REQUIRES CONFIRMATION: Tool '%s' is a sensitive action.", name)
			return true
		}
	}
	return true
}

func NewClient(cfg Config, permHandler PermissionHandler) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if permHandler == nil {
		permHandler = DefaultPermissionHandler
	}
	return &Client{
		cfg:         cfg,
		permHandler: permHandler,
		reqID:       0,
	}
}

// UpdateConfig updates the client's configuration in a thread-safe way.
func (c *Client) UpdateConfig(baseURL, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.BaseURL = baseURL
	c.cfg.Token = token
}

// IsConfigured returns true if the base URL and token are set.
func (c *Client) IsConfigured() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg.BaseURL != "" && c.cfg.Token != ""
}

// GetTools obtiene la lista dinámica de herramientas de la base del backend Java.
func (c *Client) GetTools(ctx context.Context) (json.RawMessage, error) {
	c.mu.RLock()
	baseURL := c.cfg.BaseURL
	token := c.cfg.Token
	httpClient := c.cfg.HTTPClient
	c.mu.RUnlock()

	// HandsAI expone GET /mcp/tools/list
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/tools/list", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("X-HandsAI-Token", token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parseamos asumiendo que el Java siempre envuelve la respuesta en JSON-RPC
	var rpcResp JSONRPCMessage
	if err := json.Unmarshal(body, &rpcResp); err == nil {
		if rpcResp.Error != nil {
			return nil, fmt.Errorf("rpc error: %s", rpcResp.Error.Message)
		}
		if len(rpcResp.Result) > 0 {
			return rpcResp.Result, nil
		}
	}

	// Fallback/Direct check: Si no es JSON-RPC o no tiene resultado, buscamos errores crudos
	var rawErr map[string]interface{}
	if err := json.Unmarshal(body, &rawErr); err == nil {
		if eStr, ok := rawErr["error"].(string); ok {
			return nil, fmt.Errorf("backend error: %s", eStr)
		}
	}

	return body, nil
}

func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (json.RawMessage, error) {
	if args == nil {
		args = make(map[string]interface{})
	}
	if !c.permHandler(ctx, name, args) {
		return nil, errors.New("tool execution denied by user/policy")
	}

	id := atomic.AddInt64(&c.reqID, 1)

	params := CallToolRequestParams{
		Name:      name,
		Arguments: args,
	}

	paramsRaw, _ := json.Marshal(params)

	rpcReq := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params:  paramsRaw,
	}

	reqBytes, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	httpClient := c.cfg.HTTPClient
	token := c.cfg.Token
	baseURL := c.cfg.BaseURL
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/tools/call", bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("X-HandsAI-Token", token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code (HTTP %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp JSONRPCMessage
	if err := json.Unmarshal(body, &rpcResp); err == nil {
		if rpcResp.Error != nil {
			return nil, fmt.Errorf("handsai rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
		}
		if len(rpcResp.Result) > 0 {
			// El resultado MCP tiene la forma: {"content":[{"type":"text","text":"..."}],"isError":false}
			// Necesitamos extraer el texto de cada content item y devolverlo como el resultado real.
			var mcpResult struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			}

			if err := json.Unmarshal(rpcResp.Result, &mcpResult); err == nil {
				if mcpResult.IsError {
					return nil, fmt.Errorf("tool returned error: %s", func() string {
						if len(mcpResult.Content) > 0 {
							return mcpResult.Content[0].Text
						}
						return "unknown error"
					}())
				}

				// Concatenar todos los textos del resultado y detectar errores de Java no flaggeados
				var texts []string
				var hasJavaError bool
				for _, c := range mcpResult.Content {
					if c.Type == "text" && c.Text != "" {
						texts = append(texts, c.Text)
						lower := strings.ToLower(c.Text)
						if strings.Contains(lower, "error executing tool") ||
							strings.Contains(lower, "cannot invoke") ||
							strings.Contains(lower, "exception:") {
							hasJavaError = true
						}
					}
				}

				if hasJavaError {
					return nil, fmt.Errorf("tool returned internal error: %v", texts)
				}

				if len(texts) > 0 {
					combined := texts[0]
					if len(texts) > 1 {
						// Si hay múltiples, unirlos en un array JSON
						allTexts, _ := json.Marshal(texts)
						combined = string(allTexts)
					}
					// Intentar devolver como JSON; si no es JSON válido, wrappearlo
					var js json.RawMessage
					if err := json.Unmarshal([]byte(combined), &js); err == nil {
						return js, nil
					}
					// No es JSON — devolver como string JSON
					wrapped, _ := json.Marshal(map[string]string{"result": combined})
					return json.RawMessage(wrapped), nil
				}
			}

			// El resultado no tiene content[], devolver el result bruto
			return rpcResp.Result, nil
		}
	}

	// Fallback: intentar detectar error en respuesta cruda
	var rawErr map[string]interface{}
	if err := json.Unmarshal(body, &rawErr); err == nil {
		if eStr, ok := rawErr["error"].(string); ok {
			return nil, fmt.Errorf("backend error: %s", eStr)
		}
	}

	// Sin resultado útil — loguear el body crudo para debug
	log.Printf("⚠️ CallTool '%s': no content in response. Raw body: %s", name, string(body))
	return body, nil
}
