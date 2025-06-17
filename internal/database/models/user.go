package models

import (
	"time"
	"gorm.io/gorm"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
)

type User struct {
	ID           uint           `gorm:"primaryKey"`
	Email        string         `gorm:"uniqueIndex;not null"`
	FullName     string         `gorm:"size:255"`
	Department   string         `gorm:"size:100"`
	Status       UserStatus     `gorm:"default:active"`
	OAuthProvider string        `gorm:"size:50"`
	LastLogin    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	
	// Associations
	Groups       []Group      `gorm:"many2many:user_groups;"`
	VPNSessions  []VPNSession
	AuditLogs    []AuditLog
}