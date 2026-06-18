package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
)

type AuditHandler struct{}

type AuditEventResponse struct {
	ID            uint    `json:"id"`
	OccurredAt    string  `json:"occurred_at"`
	ActorUserID   *uint   `json:"actor_user_id,omitempty"`
	Action        string  `json:"action"`
	ResourceType  string  `json:"resource_type"`
	ResourceID    string  `json:"resource_id"`
	SessionID     *uint   `json:"session_id,omitempty"`
	IP            string  `json:"ip,omitempty"`
	UserAgent     string  `json:"user_agent,omitempty"`
	PayloadBefore *string `json:"payload_before,omitempty"`
	PayloadAfter  *string `json:"payload_after,omitempty"`
	CorrelationID string  `json:"correlation_id,omitempty"`
}

type AuditEventsListResponse struct {
	Items  []AuditEventResponse `json:"items"`
	Total  int64                `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

func toAuditEventResponse(row database.AuditEvent) AuditEventResponse {
	return AuditEventResponse{
		ID:            row.ID,
		OccurredAt:    row.OccurredAt.UTC().Format(time.RFC3339),
		ActorUserID:   row.ActorUserID,
		Action:        row.Action,
		ResourceType:  row.ResourceType,
		ResourceID:    row.ResourceID,
		SessionID:     row.SessionID,
		IP:            row.IP,
		UserAgent:     row.UserAgent,
		PayloadBefore: row.PayloadBefore,
		PayloadAfter:  row.PayloadAfter,
		CorrelationID: row.CorrelationID,
	}
}

func parseAuditTime(raw string, endOfDay bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		if layout == "2006-01-02" && endOfDay {
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q", raw)
}

func (h *AuditHandler) ListEvents(c *fiber.Ctx) error {
	limit := 50
	offset := 0
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid limit"})
		}
		if n > 200 {
			n = 200
		}
		limit = n
	}
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid offset"})
		}
		offset = n
	}

	q := database.DB.Model(&database.AuditEvent{})

	if from := strings.TrimSpace(c.Query("from")); from != "" {
		t, err := parseAuditTime(from, false)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from date"})
		}
		q = q.Where("occurred_at >= ?", t)
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		t, err := parseAuditTime(to, true)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to date"})
		}
		q = q.Where("occurred_at <= ?", t)
	}
	if actor := strings.TrimSpace(c.Query("actor_user_id")); actor != "" {
		id, err := strconv.ParseUint(actor, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid actor_user_id"})
		}
		q = q.Where("actor_user_id = ?", uint(id))
	}
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		q = q.Where("action = ?", action)
	}
	if resourceType := strings.TrimSpace(c.Query("resource_type")); resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var rows []database.AuditEvent
	if err := q.Order("occurred_at desc, id desc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	items := make([]AuditEventResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, toAuditEventResponse(row))
	}

	return c.JSON(AuditEventsListResponse{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}
