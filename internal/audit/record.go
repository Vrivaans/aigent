package audit

import (
	"context"
	"strings"
	"time"

	"aigent/internal/database"

	"gorm.io/gorm"
)

var dbFn = func() *gorm.DB { return database.DB }

// SetDBForTest overrides the database handle used by Record. Returns a restore function.
func SetDBForTest(db *gorm.DB) func() {
	prev := dbFn
	dbFn = func() *gorm.DB { return db }
	return func() { dbFn = prev }
}

// Record persists an append-only audit event. Updates and deletes are intentionally unsupported.
func Record(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	action := strings.TrimSpace(event.Action)
	resourceType := strings.TrimSpace(event.ResourceType)
	if action == "" || resourceType == "" {
		return gorm.ErrInvalidData
	}

	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	row := database.AuditEvent{
		OccurredAt:    occurredAt,
		ActorUserID:   event.ActorUserID,
		Action:        action,
		ResourceType:  resourceType,
		ResourceID:    strings.TrimSpace(event.ResourceID),
		SessionID:     event.SessionID,
		IP:            strings.TrimSpace(event.IP),
		UserAgent:     strings.TrimSpace(event.UserAgent),
		PayloadBefore: event.PayloadBefore,
		PayloadAfter:  event.PayloadAfter,
		CorrelationID: strings.TrimSpace(event.CorrelationID),
	}

	return dbFn().WithContext(ctx).Create(&row).Error
}
