package models

import (
	"time"
	"gorm.io/gorm"
)

// SessionStatus represents the current state of a VPN session
type SessionStatus string

const (
	SessionStatusActive       SessionStatus = "active"       // Session is currently connected
	SessionStatusDisconnected SessionStatus = "disconnected" // Session ended normally
	SessionStatusExpired      SessionStatus = "expired"      // Session timed out
)

// VPNSession tracks individual VPN connections for monitoring and billing
type VPNSession struct {
	// Primary identifier for the VPN session
	ID uint `gorm:"primaryKey"`
	
	// Foreign key to the user who initiated this VPN session
	UserID uint `gorm:"not null;index"`
	
	// Client's IP address in the VPN network (assigned by VPN server)
	ClientIP string `gorm:"size:45;not null"`
	
	// VPN server's IP address that handled this connection
	ServerIP string `gorm:"size:45"`
	
	// Current status of the VPN session
	Status SessionStatus `gorm:"default:active"`
	
	// Timestamp when VPN connection was established
	ConnectedAt time.Time `gorm:"not null"`
	
	// Timestamp when VPN connection ended (nullable for active sessions)
	DisconnectedAt *time.Time
	
	// Total bytes received by client through VPN tunnel
	BytesIn uint64 `gorm:"default:0"`
	
	// Total bytes sent by client through VPN tunnel
	BytesOut uint64 `gorm:"default:0"`
	
	// Timestamp when session record was created
	CreatedAt time.Time
	
	// Timestamp when session record was last updated
	UpdatedAt time.Time
	
	// Soft delete timestamp (sessions are rarely deleted for audit purposes)
	DeletedAt gorm.DeletedAt `gorm:"index"`
	
	// Associations
	// Reference to the user who owns this VPN session
	User User `gorm:"foreignKey:UserID"`
}