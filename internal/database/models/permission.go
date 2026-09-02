package models

import (
	"gorm.io/gorm"
	"time"
)

// PermissionType defines the level of access granted to a resource
type PermissionType string

const (
	PermissionRead  PermissionType = "read"  // View-only access to resource
	PermissionWrite PermissionType = "write" // Read and modify access to resource
	PermissionAdmin PermissionType = "admin" // Full administrative access to resource
	PermissionSSH   PermissionType = "ssh"   // SSH/remote access to VM resources
	PermissionFull  PermissionType = "full"  // Complete access to all resource functions
)

// Permission defines access level granted to a group for a specific resource
// NOTE: This model is deprecated in favor of GroupPermission junction table
type Permission struct {
	// Primary identifier for the permission
	ID uint `gorm:"primaryKey"`

	// Foreign key to the group that receives this permission
	GroupID uint `gorm:"not null;index"`

	// Foreign key to the resource that this permission applies to
	ResourceID uint `gorm:"not null;index"`

	// Type of access granted (read, write, admin, ssh, full)
	Type PermissionType `gorm:"not null"`

	// Timestamp when permission was granted
	CreatedAt time.Time

	// Timestamp when permission was last modified
	UpdatedAt time.Time

	// Soft delete timestamp - when permission was revoked
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Associations
	// Reference to the group that has this permission
	Group Group `gorm:"foreignKey:GroupID"`

	// Reference to the resource this permission applies to
	Resource Resource `gorm:"foreignKey:ResourceID"`
}
