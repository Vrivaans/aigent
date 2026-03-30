package mcpcommon

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CallToolResultToJSON convierte un CallToolResult del SDK al mismo estilo que handsai.Client.CallTool
// (JSON útil para el LLM: texto extraído o objeto con "result").
func CallToolResultToJSON(res *mcp.CallToolResult) (json.RawMessage, error) {
	if res == nil {
		return json.RawMessage(`{"error":"empty tool result"}`), nil
	}
	if res.IsError {
		var msg string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok && tc != nil {
				msg = tc.Text
				break
			}
		}
		if msg == "" {
			msg = "tool error"
		}
		return nil, fmt.Errorf("%s", msg)
	}

	if res.StructuredContent != nil {
		raw, err := json.Marshal(res.StructuredContent)
		if err == nil {
			return raw, nil
		}
	}

	var texts []string
	var hasInternalErr bool
	for _, c := range res.Content {
		switch v := c.(type) {
		case *mcp.TextContent:
			if v == nil {
				continue
			}
			if v.Text != "" {
				texts = append(texts, v.Text)
				lower := strings.ToLower(v.Text)
				if strings.Contains(lower, "error executing tool") ||
					strings.Contains(lower, "cannot invoke") ||
					strings.Contains(lower, "exception:") {
					hasInternalErr = true
				}
			}
		default:
			if raw, err := json.Marshal(c); err == nil {
				texts = append(texts, string(raw))
			}
		}
	}

	if hasInternalErr && len(texts) > 0 {
		return nil, fmt.Errorf("tool returned internal error: %v", texts)
	}

	if len(texts) == 0 {
		out, _ := json.Marshal(map[string]string{"result": ""})
		return out, nil
	}

	combined := texts[0]
	if len(texts) > 1 {
		all, _ := json.Marshal(texts)
		combined = string(all)
	}

	var js json.RawMessage
	if err := json.Unmarshal([]byte(combined), &js); err == nil {
		return js, nil
	}
	wrapped, _ := json.Marshal(map[string]string{"result": combined})
	return json.RawMessage(wrapped), nil
}
