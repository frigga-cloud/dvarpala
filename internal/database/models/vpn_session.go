package models

import (
	"time"
	"gorm.io/gorm"
)

type SessionStatus string

const (
	SessionStatusActive      SessionStatus = "active"
	SessionStatusDisconnected SessionStatus = "disconnected"
	SessionStatusExpired     SessionStatus = "expired"
)

type VPNSession struct {
	ID         uint           `gorm:"primaryKey"`
	UserID     uint           `gorm:"not null;index"`
	ClientIP   string         `gorm:"size:45;not null"`
	ServerIP   string         `gorm:"size:45"`
	Status     SessionStatus  `gorm:"default:active"`
	ConnectedAt time.Time     `gorm:"not null"`
	DisconnectedAt *time.Time
	BytesIn    uint64         `gorm:"default:0"`
	BytesOut   uint64         `gorm:"default:0"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	
	// Associations
	User User `gorm:"foreignKey:UserID"`
}