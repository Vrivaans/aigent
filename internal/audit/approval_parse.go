package audit

import (
	"encoding/json"
	"strings"
)

// ApprovalFields holds parsed approval metadata from audit payload_after JSON.
type ApprovalFields struct {
	ToolName         string
	Status           string
	SessionID        uint
	ToolCallID       string
	ChatMessageID    *uint
	ResolvedByUserID *uint
	ResolvedAt       string
}

// ParseApprovalFields extracts approval metadata from payload_after JSON.
func ParseApprovalFields(payload *string) *ApprovalFields {
	return parseApprovalPayload(payload)
}

// IsApprovalAction reports whether an audit action relates to HITL approvals.
func IsApprovalAction(action string) bool {
	return isApprovalAction(action)
}

func parseApprovalPayload(payload *string) *ApprovalFields {
	if payload == nil {
		return nil
	}
	raw := strings.TrimSpace(*payload)
	if raw == "" {
		return nil
	}
	var p approvalAuditPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil
	}
	fields := &ApprovalFields{
		ToolName:         p.ToolName,
		Status:           p.Status,
		SessionID:        p.SessionID,
		ToolCallID:       p.ToolCallID,
		ChatMessageID:    p.ChatMessageID,
		ResolvedByUserID: p.ResolvedByUserID,
	}
	if p.ResolvedAt != nil {
		fields.ResolvedAt = *p.ResolvedAt
	}
	return fields
}

func isApprovalAction(action string) bool {
	return strings.HasPrefix(action, "approval.")
}