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
	"sync/atomic"
	"time"
)

// Tipos JSON-RPC 2.0 requeridos por HandsAI Java
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
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
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// Config representa la configuración del cliente HandsAI
type Config struct {
	BaseURL    string // e.g., "http://localhost:8080/mcp" o la URL de prod
	HTTPClient *http.Client
}

type Client struct {
	cfg         Config
	reqID       int64
	permHandler PermissionHandler
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

// GetTools obtiene la lista dinámica de herramientas de la base del backend Java.
func (c *Client) GetTools(ctx context.Context) (json.RawMessage, error) {
	// HandsAI expone GET /mcp/tools/list
	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.BaseURL+"/tools/list", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.HTTPClient.Do(req)
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
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		// Fallback: Si resulta que responde un JSON crudo sin formato JSON-RPC
		return body, nil
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", rpcResp.Error.Message)
	}

	// Si tiene formato JSON-RPC, retornamos el Result donde está el array de Tools
	if len(rpcResp.Result) > 0 {
		return rpcResp.Result, nil
	}

	return body, nil
}

// CallTool lanza la peticion a POST /mcp/tools/call empacando el payload como JSON-RPC
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (json.RawMessage, error) {
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.BaseURL+"/tools/call", bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.HTTPClient.Do(req)
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
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		// Fallback
		return body, nil
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("handsai rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Puede que result esté vacío pero que tools haya operado correctamente
	if rpcResp.Result == nil {
		return []byte(`{"status":"success"}`), nil
	}

	return rpcResp.Result, nil
}
