package ai

import (
	"encoding/json"

	"aigent/internal/database"
)

type agentToolContext struct {
	OpenRouterTools     []Tool
	SanitizedToOriginal map[string]string
}

func (b *Brain) prepareAgentToolContext(session database.Session) agentToolContext {
	allowedTools := make(map[string]bool)
	if session.Agent != nil {
		for _, at := range session.Agent.Tools {
			allowedTools[at.ToolName] = true
		}
	}

	ctx := agentToolContext{
		SanitizedToOriginal: make(map[string]string),
	}

	for _, rt := range b.Registry.List() {
		switch {
		case session.Agent == nil:
			// Sin agente asociado: exponer todo el registry.
		case session.Agent.IsDefault:
			// Agente General: siempre todas las herramientas del registry.
		case len(session.Agent.Tools) > 0:
			if !allowedTools[rt.Name] {
				continue
			}
		default:
			// Agente personalizado sin tools seleccionadas: ninguna.
			continue
		}

		shortName := sanitizeName(rt.Name)
		ctx.SanitizedToOriginal[shortName] = rt.Name

		params := rt.Parameters
		if len(params) == 0 || string(params) == "null" || string(params) == "{}" {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		ctx.OpenRouterTools = append(ctx.OpenRouterTools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        shortName,
				Description: rt.Description,
				Parameters:  params,
			},
		})
	}

	return ctx
}
