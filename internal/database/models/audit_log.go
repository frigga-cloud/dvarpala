package models

import (
	"time"
	"gorm.io/gorm"
)

// AuditLog tracks all user actions and system events for security and compliance
type AuditLog struct {
	// Primary identifier for the audit log entry
	ID uint `gorm:"primaryKey"`
	
	// Foreign key to the user who performed the action (nullable for system actions)
	UserID *uint `gorm:"index"`
	
	// Description of the action performed (e.g., "user_login", "vpn_connect", "resource_access")
	Action string `gorm:"not null;size:100"`
	
	// Type of resource affected by the action (e.g., "user", "group", "vpn", "dashboard")
	ResourceType string `gorm:"size:50"`
	
	// ID of the specific resource affected (nullable if action doesn't target specific resource)
	ResourceID *uint
	
	// IP address from which the action was performed (IPv4/IPv6 compatible)
	IPAddress string `gorm:"size:45"`
	
	// Browser/client user agent string for additional context
	UserAgent string `gorm:"size:500"`
	
	// Additional structured data about the action in JSON format
	// Example: {"method": "oauth", "provider": "google", "success": true}
	Details string `gorm:"type:jsonb"`
	
	// Timestamp when the action occurred
	CreatedAt time.Time
	
	// Soft delete timestamp (audit logs should rarely be deleted)
	DeletedAt gorm.DeletedAt `gorm:"index"`
	
	// Associations
	// Reference to the user who performed the action
	User *User `gorm:"foreignKey:UserID"`
}