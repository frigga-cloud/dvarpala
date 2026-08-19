package models

import (
	"gorm.io/gorm"
	"time"
)

// VPNConfigStatus represents the status of VPN configuration
type VPNConfigStatus string

const (
	VPNConfigStatusActive   VPNConfigStatus = "active"   // Configuration is valid and usable
	VPNConfigStatusInactive VPNConfigStatus = "inactive" // Configuration is disabled
	VPNConfigStatusRevoked  VPNConfigStatus = "revoked"  // Configuration has been revoked for security
)

// VPNConfig stores VPN client configurations and certificates for users
type VPNConfig struct {
	// Primary identifier for the VPN configuration
	ID uint `gorm:"primaryKey"`

	// Foreign key to the user who owns this configuration
	UserID uint `gorm:"not null;index"`

	// Human-readable name for this configuration (e.g., "Mobile Device", "Laptop")
	ConfigName string `gorm:"not null;size:100"`

	// Client certificate in PEM format for VPN authentication
	ClientCert string `gorm:"type:text"`

	// Client private key in PEM format (encrypted in production)
	ClientKey string `gorm:"type:text"`

	// Certificate Authority certificate in PEM format
	CACert string `gorm:"type:text"`

	// Complete OpenVPN configuration file content
	ConfigData string `gorm:"type:text"`

	// Current status of this configuration
	Status VPNConfigStatus `gorm:"default:active"`

	// Timestamp when configuration expires (nullable for permanent configs)
	ExpiresAt *time.Time

	// Timestamp when configuration was last used for VPN connection
	LastUsedAt *time.Time

	// Timestamp when configuration was downloaded by user
	DownloadedAt *time.Time

	// Timestamp when configuration was created
	CreatedAt time.Time

	// Timestamp when configuration was last updated
	UpdatedAt time.Time

	// Soft delete timestamp - when configuration was deactivated
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Associations
	// Reference to the user who owns this configuration
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// TableName returns the table name for VPNConfig
func (VPNConfig) TableName() string {
	return "vpn_configs"
}
