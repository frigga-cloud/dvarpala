package models

import (
	"time"
	"gorm.io/gorm"
)

// UserStatus represents the current status of a user account
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"    // User can access the system
	UserStatusInactive  UserStatus = "inactive"  // User account is disabled
	UserStatusSuspended UserStatus = "suspended" // User is temporarily blocked
)

// User represents a system user with OAuth authentication and VPN access
type User struct {
	// Primary identifier for the user
	ID uint `gorm:"primaryKey"`
	
	// User's email address - used as unique identifier and for OAuth validation
	// Must match allowed domains in configuration
	Email string `gorm:"uniqueIndex;not null"`
	
	// User's full display name from OAuth provider
	FullName string `gorm:"size:255"`
	
	// User's department/organization unit for grouping and access control
	Department string `gorm:"size:100"`
	
	// Current account status - determines if user can access the system
	Status UserStatus `gorm:"default:active"`
	
	// OAuth provider used for authentication (google, microsoft, github)
	OAuthProvider string `gorm:"size:50"`
	
	// Timestamp of user's last successful login (nullable for new users)
	LastLogin *time.Time
	
	// Timestamp when user account was created
	CreatedAt time.Time
	
	// Timestamp when user account was last modified
	UpdatedAt time.Time
	
	// Soft delete timestamp - when user account was deactivated
	DeletedAt gorm.DeletedAt `gorm:"index"`
	
	// Associations
	// Groups this user belongs to (many-to-many relationship)
	Groups []Group `gorm:"many2many:user_groups;"`
	
	// VPN sessions initiated by this user
	VPNSessions []VPNSession
	
	// Audit log entries for actions performed by this user
	AuditLogs []AuditLog
}