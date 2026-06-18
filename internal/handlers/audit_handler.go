package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"aigent/internal/audit"
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const auditExportMaxRows = 10000

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

func applyAuditEventFilters(c *fiber.Ctx, q *gorm.DB) (*gorm.DB, error) {
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		t, err := parseAuditTime(from, false)
		if err != nil {
			return nil, fmt.Errorf("invalid from date")
		}
		q = q.Where("occurred_at >= ?", t)
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		t, err := parseAuditTime(to, true)
		if err != nil {
			return nil, fmt.Errorf("invalid to date")
		}
		q = q.Where("occurred_at <= ?", t)
	}
	if actor := strings.TrimSpace(c.Query("actor_user_id")); actor != "" {
		id, err := strconv.ParseUint(actor, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid actor_user_id")
		}
		q = q.Where("actor_user_id = ?", uint(id))
	}
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		q = q.Where("action = ?", action)
	}
	if resourceType := strings.TrimSpace(c.Query("resource_type")); resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}
	return q, nil
}

func validateExportRowCount(count int64) error {
	if count > auditExportMaxRows {
		return fmt.Errorf(
			"export would return %d rows (maximum %d); narrow your filters",
			count, auditExportMaxRows,
		)
	}
	return nil
}

func auditEventsToCSV(rows []database.AuditEvent) ([]byte, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)
	if err := w.Write([]string{
		"id", "occurred_at", "actor_user_id", "action", "resource_type", "resource_id",
		"session_id", "ip", "user_agent", "correlation_id", "payload_before", "payload_after",
	}); err != nil {
		return nil, err
	}
	for _, row := range rows {
		record := []string{
			strconv.FormatUint(uint64(row.ID), 10),
			row.OccurredAt.UTC().Format(time.RFC3339),
			uintPtrCSV(row.ActorUserID),
			row.Action,
			row.ResourceType,
			row.ResourceID,
			uintPtrCSV(row.SessionID),
			row.IP,
			row.UserAgent,
			row.CorrelationID,
			strPtrCSV(row.PayloadBefore),
			strPtrCSV(row.PayloadAfter),
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func uintPtrCSV(v *uint) string {
	if v == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*v), 10)
}

func strPtrCSV(v *string) string {
	if v == nil {
		return ""
	}
	return *v
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

	q, err := applyAuditEventFilters(c, database.DB.Model(&database.AuditEvent{}))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
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

func (h *AuditHandler) ExportEvents(c *fiber.Ctx) error {
	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	if format != "csv" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "format must be csv"})
	}

	q, err := applyAuditEventFilters(c, database.DB.Model(&database.AuditEvent{}))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := validateExportRowCount(total); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var rows []database.AuditEvent
	if err := q.Order("occurred_at desc, id desc").Limit(auditExportMaxRows).Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	csvBytes, err := auditEventsToCSV(rows)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate CSV"})
	}

	payload := fmt.Sprintf(`{"row_count":%d,"format":"csv"}`, len(rows))
	audit.Emit(c, audit.Event{
		Action:       "audit.export",
		ResourceType: "audit",
		ResourceID:   "export",
		PayloadAfter: &payload,
	})

	filename := fmt.Sprintf("audit-export-%s.csv", time.Now().UTC().Format("20060102-150405"))
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(csvBytes)
}
