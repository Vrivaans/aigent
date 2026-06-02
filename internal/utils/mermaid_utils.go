package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

type RuleGoChain struct {
	Metadata struct {
		Nodes []struct {
			ID            string                 `json:"id"`
			Type          string                 `json:"type"`
			Name          string                 `json:"name"`
			Configuration map[string]interface{} `json:"configuration"`
		} `json:"nodes"`
		Connections []struct {
			FromID string `json:"fromId"`
			ToID   string `json:"toId"`
			Type   string `json:"type"`
		} `json:"connections"`
	} `json:"metadata"`
}

type RuleGoConnectionHelper struct {
	FromID   string `json:"fromId"`
	From     string `json:"from"`
	ToID     string `json:"toId"`
	To       string `json:"to"`
	Type     string `json:"type"`
	Relation string `json:"relation"`
}

type RuleGoChainHelper struct {
	RuleChain map[string]interface{} `json:"ruleChain"`
	Metadata  struct {
		Nodes       []map[string]interface{} `json:"nodes"`
		Connections []RuleGoConnectionHelper `json:"connections"`
	} `json:"metadata"`
}

// NormalizeRuleChainJSON normaliza un JSON de RuleGo para asegurar compatibilidad con fromId/toId, jsTransform, y aigent/tool.
func NormalizeRuleChainJSON(definitionJSON string) (string, error) {
	if definitionJSON == "" {
		return "", fmt.Errorf("empty definition")
	}

	var helper RuleGoChainHelper
	if err := json.Unmarshal([]byte(definitionJSON), &helper); err != nil {
		return "", err
	}

	// Normalizar Nodos
	standardTypes := map[string]bool{
		"jsTransform": true,
		"jsFilter":    true,
		"jsSwitch":    true,
		"flow":        true,
		"aigent/tool": true,
	}

	var normalizedNodes []map[string]interface{}
	for _, node := range helper.Metadata.Nodes {
		nodeType, _ := node["type"].(string)
		
		// Corregir snake_case en componentes estándar
		if nodeType == "js_transform" {
			nodeType = "jsTransform"
			node["type"] = "jsTransform"
		} else if nodeType == "js_filter" {
			nodeType = "jsFilter"
			node["type"] = "jsFilter"
		} else if nodeType == "js_switch" {
			nodeType = "jsSwitch"
			node["type"] = "jsSwitch"
		}

		// Si no es un componente estándar y no contiene barra (/), asumimos que es una tool directa
		if !standardTypes[nodeType] && nodeType != "" && !strings.Contains(nodeType, "/") {
			node["type"] = "aigent/tool"
			config, ok := node["configuration"].(map[string]interface{})
			if !ok || config == nil {
				config = make(map[string]interface{})
			}
			if _, exists := config["toolName"]; !exists {
				config["toolName"] = nodeType
			}
			node["configuration"] = config
		}

		// Si es un componente JS, asegurar que use "jsScript" en lugar de "jsCode"
		if nodeType == "jsTransform" || nodeType == "jsFilter" || nodeType == "jsSwitch" {
			config, ok := node["configuration"].(map[string]interface{})
			if ok && config != nil {
				if jsCode, exists := config["jsCode"]; exists {
					config["jsScript"] = jsCode
					delete(config, "jsCode")
				}
				node["configuration"] = config
			}
		}

		normalizedNodes = append(normalizedNodes, node)
	}

	// Reconstruir conexiones normalizadas
	var normalizedConns []map[string]string
	for _, conn := range helper.Metadata.Connections {
		from := conn.FromID
		if from == "" {
			from = conn.From
		}
		to := conn.ToID
		if to == "" {
			to = conn.To
		}
		rel := conn.Type
		if rel == "" {
			rel = conn.Relation
		}
		if rel == "" {
			rel = "Success"
		}

		if from != "" && to != "" {
			normalizedConns = append(normalizedConns, map[string]string{
				"fromId": from,
				"toId":   to,
				"type":   rel,
			})
		}
	}

	// Si no hay ruleChain definido, inicializar uno por defecto
	if helper.RuleChain == nil {
		helper.RuleChain = map[string]interface{}{
			"id":   "workflow",
			"name": "Workflow",
		}
	}

	// Armar el mapa final
	finalMap := map[string]interface{}{
		"ruleChain": helper.RuleChain,
		"metadata": map[string]interface{}{
			"nodes":       normalizedNodes,
			"connections": normalizedConns,
		},
	}

	normalizedBytes, err := json.Marshal(finalMap)
	if err != nil {
		return "", err
	}

	return string(normalizedBytes), nil
}

// RuleGoToMermaid convierte una definición JSON de RuleGo a notación de grafos de Mermaid
func RuleGoToMermaid(definitionJSON string, currentNodeID string) (string, error) {
	if definitionJSON == "" {
		return "graph TD\n  empty[\"Esquema vacío\"]", nil
	}

	normalized, err := NormalizeRuleChainJSON(definitionJSON)
	if err != nil {
		return "", fmt.Errorf("normalization failed: %w", err)
	}

	var chain RuleGoChain
	if err := json.Unmarshal([]byte(normalized), &chain); err != nil {
		return "", fmt.Errorf("failed to unmarshal rulego JSON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// 1. Declarar los nodos con sus nombres y subtipos/herramientas si existen
	for _, node := range chain.Metadata.Nodes {
		name := node.Name
		if name == "" {
			name = node.ID
		}

		// Enriquecer el nombre visual si es una herramienta de AIgent
		if node.Type == "aigent/tool" {
			if tool, ok := node.Configuration["toolName"].(string); ok && tool != "" {
				name = fmt.Sprintf("🔧 %s (%s)", name, tool)
			}
		} else {
			name = fmt.Sprintf("⚡ %s (%s)", name, node.Type)
		}

		// Sanitizar comillas y caracteres especiales para evitar romper Mermaid
		name = strings.ReplaceAll(name, "\"", "'")
		name = strings.ReplaceAll(name, "[", "(")
		name = strings.ReplaceAll(name, "]", ")")

		sb.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", node.ID, name))
	}

	// 2. Declarar las conexiones
	for _, conn := range chain.Metadata.Connections {
		label := conn.Type
		if label == "" {
			label = "Success"
		}
		sb.WriteString(fmt.Sprintf("  %s -->|%s| %s\n", conn.FromID, label, conn.ToID))
	}

	// 3. Resaltar el nodo activo si aplica
	if currentNodeID != "" {
		sb.WriteString(fmt.Sprintf("  class %s activeNode;\n", currentNodeID))
		sb.WriteString("  classDef activeNode fill:#c8f04a,stroke:#050505,stroke-width:3px,color:#050505;\n")
	}

	// 4. Agregar estilo general para los nodos comunes
	sb.WriteString("  classDef default fill:#1a1a1c,stroke:#333,stroke-width:1px,color:#f0f0ec;\n")

	return sb.String(), nil
}
