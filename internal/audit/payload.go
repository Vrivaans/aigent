package audit

import (
	"encoding/json"
	"strconv"

	"aigent/internal/database"
)

func jsonPtr(v any) *string {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func UintID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

type providerAuditPayload struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	DefaultModel string `json:"default_model"`
	ProviderType string `json:"provider_type"`
	IsEmbeddings bool   `json:"is_embeddings"`
	IsActive     bool   `json:"is_active"`
	IsDefault    bool   `json:"is_default"`
}

func ProviderPayload(p database.LLMProvider) *string {
	return jsonPtr(providerAuditPayload{
		ID:           p.ID,
		Name:         p.Name,
		BaseURL:      p.BaseURL,
		DefaultModel: p.DefaultModel,
		ProviderType: p.ProviderType,
		IsEmbeddings: p.IsEmbeddings,
		IsActive:     p.IsActive,
		IsDefault:    p.IsDefault,
	})
}

type mcpStdioAuditPayload struct {
	ID      uint   `json:"id"`
	Alias   string `json:"alias"`
	Command string `json:"command"`
	Enabled bool   `json:"enabled"`
}

func McpStdioPayload(s database.McpStdioServer) *string {
	return jsonPtr(mcpStdioAuditPayload{
		ID:      s.ID,
		Alias:   s.Alias,
		Command: s.Command,
		Enabled: s.Enabled,
	})
}

type mcpStreamAuditPayload struct {
	ID                   uint   `json:"id"`
	Alias                string `json:"alias"`
	BaseURL              string `json:"base_url"`
	DisableStandaloneSSE bool   `json:"disable_standalone_sse"`
	Enabled              bool   `json:"enabled"`
}

func McpStreamPayload(s database.McpStreamServer) *string {
	return jsonPtr(mcpStreamAuditPayload{
		ID:                   s.ID,
		Alias:                s.Alias,
		BaseURL:              s.BaseURL,
		DisableStandaloneSSE: s.DisableStandaloneSSE,
		Enabled:              s.Enabled,
	})
}

type permissionAuditPayload struct {
	ID         uint   `json:"id"`
	AgentID    uint   `json:"agent_id"`
	ToolName   string `json:"tool_name"`
	ActionType string `json:"action_type"`
	Paused     bool   `json:"paused"`
}

func PermissionPayload(p database.ToolPermission) *string {
	return jsonPtr(permissionAuditPayload{
		ID:         p.ID,
		AgentID:    p.AgentID,
		ToolName:   p.ToolName,
		ActionType: p.ActionType,
		Paused:     p.Paused,
	})
}

type approvalAuditPayload struct {
	ID               uint   `json:"id"`
	SessionID        uint   `json:"session_id"`
	ToolName         string `json:"tool_name"`
	Status           string `json:"status"`
	ResolvedByUserID *uint  `json:"resolved_by_user_id,omitempty"`
}

func ApprovalPayload(p database.PendingAction) *string {
	return jsonPtr(approvalAuditPayload{
		ID:               p.ID,
		SessionID:        p.SessionID,
		ToolName:         p.ToolName,
		Status:           p.Status,
		ResolvedByUserID: p.ResolvedByUserID,
	})
}

type loginAuditPayload struct {
	Username string `json:"username"`
	Reason   string `json:"reason,omitempty"`
}

func LoginFailurePayload(username, reason string) *string {
	return jsonPtr(loginAuditPayload{Username: username, Reason: reason})
}

func LoginSuccessPayload(username string) *string {
	return jsonPtr(loginAuditPayload{Username: username})
}
