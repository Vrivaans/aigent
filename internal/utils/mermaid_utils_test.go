package utils

import (
	"strings"
	"testing"
)

func TestRuleGoToMermaid_Success(t *testing.T) {
	definitionJSON := `{
		"ruleChain": {
			"name": "Test Chain",
			"id": "tc1"
		},
		"metadata": {
			"nodes": [
				{
					"id": "n1",
					"type": "aigent/tool",
					"name": "Odoo Sale",
					"configuration": {
						"toolName": "odoo_sale_order_list"
					}
				},
				{
					"id": "n2",
					"type": "jsTransform",
					"name": "Filter Orders",
					"configuration": {}
				}
			],
			"connections": [
				{
					"fromId": "n1",
					"toId": "n2",
					"type": "Success"
				}
			]
		}
	}`

	// Test 1: Design time (no active node)
	mermaidStr, err := RuleGoToMermaid(definitionJSON, "")
	if err != nil {
		t.Fatalf("Failed to generate Mermaid: %v", err)
	}

	if !strings.Contains(mermaidStr, "graph TD") {
		t.Errorf("Expected graph definition, got: %s", mermaidStr)
	}
	if !strings.Contains(mermaidStr, `n1["🔧 Odoo Sale (odoo_sale_order_list)"]`) {
		t.Errorf("Expected n1 node definition, got: %s", mermaidStr)
	}
	if !strings.Contains(mermaidStr, `n2["⚡ Filter Orders (jsTransform)"]`) {
		t.Errorf("Expected n2 node definition, got: %s", mermaidStr)
	}
	if !strings.Contains(mermaidStr, "n1 -->|Success| n2") {
		t.Errorf("Expected connection, got: %s", mermaidStr)
	}
	if strings.Contains(mermaidStr, "activeNode") {
		t.Errorf("Did not expect activeNode style at design time")
	}

	// Test 2: Runtime (with active node)
	mermaidActiveStr, err := RuleGoToMermaid(definitionJSON, "n2")
	if err != nil {
		t.Fatalf("Failed to generate Mermaid for runtime: %v", err)
	}

	if !strings.Contains(mermaidActiveStr, "class n2 activeNode;") {
		t.Errorf("Expected class assignment for activeNode: %s", mermaidActiveStr)
	}
	if !strings.Contains(mermaidActiveStr, "classDef activeNode") {
		t.Errorf("Expected classDef for activeNode: %s", mermaidActiveStr)
	}
}

func TestRuleGoToMermaid_Empty(t *testing.T) {
	mermaidStr, err := RuleGoToMermaid("", "")
	if err != nil {
		t.Fatalf("Failed on empty input: %v", err)
	}
	if !strings.Contains(mermaidStr, "empty") {
		t.Errorf("Expected empty state graph, got: %s", mermaidStr)
	}
}

func TestRuleGoToMermaid_InvalidJSON(t *testing.T) {
	_, err := RuleGoToMermaid("{invalid-json}", "")
	if err == nil {
		t.Errorf("Expected error on invalid JSON, got nil")
	}
}

func TestNormalizeRuleChainJSON(t *testing.T) {
	invalidJSON := `{
		"metadata": {
			"nodes": [
				{
					"id": "1",
					"type": "odoo_sale_order_list",
					"name": "Odoo Sale",
					"configuration": {}
				},
				{
					"id": "2",
					"type": "js_transform",
					"name": "Filter",
					"configuration": {}
				}
			],
			"connections": [
				{
					"from": "1",
					"to": "2",
					"relation": "Success"
				}
			]
		}
	}`

	normalized, err := NormalizeRuleChainJSON(invalidJSON)
	if err != nil {
		t.Fatalf("Failed to normalize: %v", err)
	}

	if !strings.Contains(normalized, `"fromId":"1"`) {
		t.Errorf("Expected fromId connection mapping, got: %s", normalized)
	}
	if !strings.Contains(normalized, `"toId":"2"`) {
		t.Errorf("Expected toId connection mapping, got: %s", normalized)
	}
	if !strings.Contains(normalized, `"type":"Success"`) {
		t.Errorf("Expected type Success connection mapping, got: %s", normalized)
	}
	if !strings.Contains(normalized, `"type":"aigent/tool"`) {
		t.Errorf("Expected aigent/tool type rewriting, got: %s", normalized)
	}
	if !strings.Contains(normalized, `"toolName":"odoo_sale_order_list"`) {
		t.Errorf("Expected toolName mapping in configuration, got: %s", normalized)
	}
	if !strings.Contains(normalized, `"type":"jsTransform"`) {
		t.Errorf("Expected jsTransform rewriting, got: %s", normalized)
	}
}
