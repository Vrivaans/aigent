package audit

import "time"

// Event describes a single append-only audit log entry.
type Event struct {
	ActorUserID   *uint
	Action        string
	ResourceType  string
	ResourceID    string
	SessionID     *uint
	IP            string
	UserAgent     string
	PayloadBefore *string
	PayloadAfter  *string
	CorrelationID string
	OccurredAt    time.Time
}
