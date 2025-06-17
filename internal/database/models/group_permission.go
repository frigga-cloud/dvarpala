// Package models provides group permission relationship model definitions
package models

import (
	"time"
)

// GroupPermission represents the many-to-many relationship between groups and resource permissions
type GroupPermission struct {
	// Foreign key to group that receives the permission
	GroupID uint `gorm:"primaryKey;not null"`
	
	// Foreign key to resource that the permission applies to
	ResourceID uint `gorm:"primaryKey;not null"`
	
	// Type of permission granted (read, write, admin, ssh, full)
	PermissionType PermissionType `gorm:"primaryKey;not null;size:20"`
	
	// Timestamp when permission was granted
	CreatedAt time.Time `gorm:"autoCreateTime"`
	
	// Timestamp when permission was last updated
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	
	// Foreign key constraints
	// Reference to the group that has the permission
	Group Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
	
	// Reference to the resource the permission applies to
	Resource Resource `gorm:"foreignKey:ResourceID;constraint:OnDelete:CASCADE"`
}

// TableName returns the table name for GroupPermission
func (GroupPermission) TableName() string {
	return "group_permissions"
}
