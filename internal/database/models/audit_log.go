package models

import (
	"time"
	"gorm.io/gorm"
)

type AuditLog struct {
	ID           uint           `gorm:"primaryKey"`
	UserID       *uint          `gorm:"index"`
	Action       string         `gorm:"not null;size:100"`
	ResourceType string         `gorm:"size:50"`
	ResourceID   *uint
	IPAddress    string         `gorm:"size:45"`
	UserAgent    string         `gorm:"size:500"`
	Details      string         `gorm:"type:jsonb"`
	CreatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	
	// Associations
	User *User `gorm:"foreignKey:UserID"`
}