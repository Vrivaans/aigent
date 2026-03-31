package mcpstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"aigent/internal/mcpcommon"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Session mantiene una sesión MCP streamable HTTP (POST + SSE).
type Session struct {
	mu sync.Mutex
	cs *mcp.ClientSession
}

type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := h.base
	if base == nil {
		base = http.DefaultTransport
	}
	for k, v := range h.headers {
		if strings.TrimSpace(k) == "" || v == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	return base.RoundTrip(req)
}

// Connect abre una sesión contra endpoint (URL base del servidor MCP streamable).
func Connect(ctx context.Context, endpoint string, headers map[string]string, disableStandaloneSSE bool) (*Session, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("empty endpoint URL")
	}

	client := &http.Client{
		Transport: &headerRoundTripper{
			base:    http.DefaultTransport,
			headers: headers,
		},
		// Sin timeout global: el stream SSE puede ser largo.
		Timeout: 0,
	}

	transport := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           client,
		DisableStandaloneSSE: disableStandaloneSSE,
	}
	clientImpl := mcp.NewClient(&mcp.Implementation{Name: "aigent", Version: "1.0"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})

	cs, err := clientImpl.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	return &Session{cs: cs}, nil
}

// ListTools devuelve todas las herramientas expuestas por el servidor.
func (s *Session) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var tools []*mcp.Tool
	for t, err := range s.cs.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, nil
}

// CallTool invoca una herramienta por nombre MCP.
func (s *Session) CallTool(ctx context.Context, name string, args map[string]interface{}) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var arguments any
	if len(args) > 0 {
		arguments = args
	}
	res, err := s.cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		return nil, err
	}
	return mcpcommon.CallToolResultToJSON(res)
}

// Close cierra la sesión HTTP/SSE.
func (s *Session) Close() error {
	if s == nil || s.cs == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.cs.Close()
	s.cs = nil
	return err
}
