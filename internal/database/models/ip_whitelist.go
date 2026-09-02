package models

import (
	"gorm.io/gorm"
	"time"
)

// IPWhitelistType represents the scope of IP whitelist entry
type IPWhitelistType string

const (
	IPWhitelistTypeUser   IPWhitelistType = "user"   // IP restriction for specific user
	IPWhitelistTypeGroup  IPWhitelistType = "group"  // IP restriction for group members
	IPWhitelistTypeGlobal IPWhitelistType = "global" // System-wide IP restriction
)

// IPWhitelist stores allowed IP addresses for enhanced access control
type IPWhitelist struct {
	// Primary identifier for the IP whitelist entry
	ID uint `gorm:"primaryKey"`

	// Single IP address that is allowed (IPv4/IPv6 compatible)
	IPAddress string `gorm:"not null;size:45;index"`

	// CIDR notation for IP range (e.g., "192.168.1.0/24")
	CIDR string `gorm:"size:50"`

	// Scope of this whitelist entry (user, group, or global)
	Type IPWhitelistType `gorm:"not null"`

	// Foreign key to user (only for user-specific entries)
	UserID *uint `gorm:"index"`

	// Foreign key to group (only for group-specific entries)
	GroupID *uint `gorm:"index"`

	// Human-readable description of this IP restriction
	Description string `gorm:"size:255"`

	// Flag to enable/disable this whitelist entry
	IsActive bool `gorm:"default:true"`

	// Timestamp when this IP restriction expires (nullable for permanent)
	ExpiresAt *time.Time

	// Timestamp when IP restriction was created
	CreatedAt time.Time

	// Timestamp when IP restriction was last updated
	UpdatedAt time.Time

	// Soft delete timestamp - when IP restriction was removed
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Associations
	// Reference to user for user-specific restrictions
	User *User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`

	// Reference to group for group-specific restrictions
	Group *Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
}

// TableName returns the table name for IPWhitelist
func (IPWhitelist) TableName() string {
	return "ip_whitelists"
}
