package mcpstdio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"aigent/internal/mcpcommon"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Session mantiene una sesión MCP stdio viva (un subproceso).
type Session struct {
	mu sync.Mutex
	cs *mcp.ClientSession
}

// Connect arranca command con args y variables de entorno extra (sobre el entorno del proceso).
func Connect(ctx context.Context, command string, args []string, extraEnv map[string]string) (*Session, error) {
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(command, args...)
	if len(extraEnv) > 0 {
		env := os.Environ()
		for k, v := range extraEnv {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	transport := &mcp.CommandTransport{Command: cmd}
	client := mcp.NewClient(&mcp.Implementation{Name: "aigent", Version: "1.0"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})

	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	return &Session{cs: cs}, nil
}

// ListTools devuelve todas las herramientas expuestas por el servidor (paginación interna del SDK).
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

// CallTool invoca una herramienta por nombre MCP y devuelve JSON compatible con el flujo del agente.
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

// Close cierra la sesión y termina el subproceso.
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
