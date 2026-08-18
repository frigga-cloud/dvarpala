// Package services provides audit logging functionality
package services

import (
	"context"
	"encoding/json"
	"log"

	"dvarpala/internal/database/models"

	"gorm.io/gorm"
)

// AuditService records actions to the audit_logs table.
type AuditService struct {
	db *gorm.DB
}

// NewAuditService creates an audit service backed by the given database.
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// Entry describes a single auditable action.
type Entry struct {
	// ID of the user who performed the action (nil for system actions)
	UserID *uint

	// What happened, e.g. "user_created", "user_deactivated"
	Action string

	// What kind of thing it happened to, e.g. "user", "group"
	ResourceType string

	// ID of the affected record, if any
	ResourceID *uint

	// Where the request came from (empty for CLI actions)
	IPAddress string
	UserAgent string

	// Any extra context, serialised to the jsonb details column
	Details map[string]interface{}
}

// Log writes an audit record.
//
// Auditing must never block the action it describes, so a failure here is
// logged and swallowed rather than returned. If the audit trail becomes a
// compliance requirement this should change to a returned error.
func (s *AuditService) Log(ctx context.Context, e Entry) {
	entry := models.AuditLog{
		UserID:       e.UserID,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		IPAddress:    e.IPAddress,
		UserAgent:    e.UserAgent,
		Details:      marshalDetails(e.Details),
	}

	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		log.Printf("audit: failed to record %q: %v", e.Action, err)
	}
}

// marshalDetails renders an entry's context for the jsonb details column.
//
// The column is jsonb and PostgreSQL rejects an empty string for one, so an
// entry carrying no extra context gets an empty object rather than nothing at
// all. Getting this wrong fails silently, because Log swallows write errors.
func marshalDetails(details map[string]interface{}) string {
	if details == nil {
		return "{}"
	}

	b, err := json.Marshal(details)
	if err != nil {
		return "{}"
	}
	return string(b)
}
