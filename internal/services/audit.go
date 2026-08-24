// Package services provides audit logging functionality
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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

// AuditQuery narrows a search of the trail. A zero value returns the most
// recent activity of any kind.
type AuditQuery struct {
	// Email restricts to one person. Matched through the users table rather
	// than the details column, so it finds records written before that person
	// was identified only if they were attributed at the time.
	Email string

	// Action restricts to one kind of event, e.g. "authentication_failed".
	Action string

	// Since restricts to recent activity.
	Since time.Duration

	// Limit caps the number of records returned. Zero means 50.
	Limit int
}

// AuditRecord is one entry, with the actor resolved to an address.
type AuditRecord struct {
	models.AuditLog

	// Email of the user the record is attributed to, empty when the action
	// happened before anyone was identified - a failed login, for instance,
	// or an installation step.
	Email string
}

// List returns audit records, newest first.
//
// Reading only. Nothing in this package can alter or remove a record: an
// audit trail that the application can edit is not evidence of anything.
func (s *AuditService) List(ctx context.Context, q AuditQuery) ([]AuditRecord, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	// The actor's address comes from a join rather than from the details
	// column, because details is written by whichever call site produced the
	// record and is not consistent between them.
	tx := s.db.WithContext(ctx).
		Table("audit_logs").
		Select("audit_logs.*, users.email AS email").
		Joins("LEFT JOIN users ON users.id = audit_logs.user_id").
		Where("audit_logs.deleted_at IS NULL").
		Order("audit_logs.id DESC").
		Limit(limit)

	if e := strings.ToLower(strings.TrimSpace(q.Email)); e != "" {
		// Either attributed to that user, or naming them in the details -
		// which is how a refusal before identification records who was
		// turned away.
		tx = tx.Where("LOWER(users.email) = ? OR audit_logs.details->>'email' = ?", e, e)
	}
	if a := strings.TrimSpace(q.Action); a != "" {
		tx = tx.Where("audit_logs.action = ?", a)
	}
	if q.Since > 0 {
		tx = tx.Where("audit_logs.created_at >= ?", time.Now().Add(-q.Since))
	}

	var records []AuditRecord
	if err := tx.Scan(&records).Error; err != nil {
		return nil, fmt.Errorf("reading the audit trail: %w", err)
	}
	return records, nil
}

// Actions lists the kinds of event present in the trail, with how many of
// each. Useful for discovering what can be filtered on without having to read
// the source.
func (s *AuditService) Actions(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Action string
		Count  int64
	}

	if err := s.db.WithContext(ctx).
		Table("audit_logs").
		Select("action, COUNT(*) AS count").
		Where("deleted_at IS NULL").
		Group("action").
		Order("count DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("counting audit actions: %w", err)
	}

	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Action] = r.Count
	}
	return out, nil
}
