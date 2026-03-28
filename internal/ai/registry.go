package ai

import (
	"context"
	"encoding/json"
)

// ActionFunc represents the actual logic of a tool execution
type ActionFunc func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error)

// ToolDef is a unified representation of any capability (Native or MCP)
type ToolDef struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  json.RawMessage   `json:"parameters"`
	Execute     ActionFunc        `json:"-"`
	ArgMapping  map[string]string `json:"-"` // Mapeo de nombre_sanitizado -> nombre_original_mcp
	Sensitive   bool              `json:"sensitive"`
}

// ToolRegistry manages the available capabilities of the AIgent Brain
type ToolRegistry struct {
	tools map[string]ToolDef
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolDef),
	}
}

func (r *ToolRegistry) Register(def ToolDef) {
	r.tools[def.Name] = def
}

// Clear removes all tools from the registry (used before a fresh sync).
func (r *ToolRegistry) Clear() {
	r.tools = make(map[string]ToolDef)
}

func (r *ToolRegistry) Get(name string) (ToolDef, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// GetBySanitized busca una herramienta por su nombre sanitizado (guiones → guiones_bajos).
// Útil para resolver el nombre almacenado en PendingAction al nombre original en el Registry.
func (r *ToolRegistry) GetBySanitized(sanitizedName string) (ToolDef, bool) {
	// Intento directo primero (por si el nombre original coincide)
	if t, ok := r.tools[sanitizedName]; ok {
		return t, ok
	}
	// Búsqueda fuzzy: comparar nombre sanitizado de cada tool
	for _, t := range r.tools {
		if sanitizeName(t.Name) == sanitizedName {
			return t, true
		}
	}
	return ToolDef{}, false
}

func (r *ToolRegistry) List() []ToolDef {
	var list []ToolDef
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}
