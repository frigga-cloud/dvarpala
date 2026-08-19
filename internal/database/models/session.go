package models

import (
	"gorm.io/gorm"
	"time"
)

// Session represents user web application sessions for maintaining login state
type Session struct {
	// Primary identifier for the session record
	ID uint `gorm:"primaryKey"`

	// Unique session identifier used in cookies and tokens
	SessionID string `gorm:"uniqueIndex;not null;size:255"`

	// Foreign key to the user who owns this session
	UserID uint `gorm:"not null;index"`

	// IP address from which the session was created
	IPAddress string `gorm:"size:45;not null"`

	// Browser user agent string for security validation
	UserAgent string `gorm:"size:500"`

	// Timestamp when session expires and becomes invalid
	ExpiresAt time.Time `gorm:"not null"`

	// Timestamp of last activity in this session (for idle timeout)
	LastUsedAt time.Time `gorm:"not null"`

	// Timestamp when session was created
	CreatedAt time.Time

	// Timestamp when session was last updated
	UpdatedAt time.Time

	// Soft delete timestamp - when session was terminated
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Associations
	// Reference to the user who owns this session
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// TableName returns the table name for Session
func (Session) TableName() string {
	return "sessions"
}
