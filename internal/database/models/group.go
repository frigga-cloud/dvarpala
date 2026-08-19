package models

import (
	"gorm.io/gorm"
	"time"
)

// Group represents a collection of users with shared access permissions and network routes
// Supports hierarchical structure for organizational alignment
type Group struct {
	// Primary identifier for the group
	ID uint `gorm:"primaryKey"`

	// Unique name of the group (e.g., "administrators", "engineering", "vpn_users")
	Name string `gorm:"uniqueIndex;not null;size:100"`

	// Human-readable description of the group's purpose and scope
	Description string `gorm:"type:text"`

	// Foreign key to parent group for hierarchical organization (nullable for root groups)
	ParentID *uint `gorm:"index"`

	// Timestamp when group was created
	CreatedAt time.Time

	// Timestamp when group was last modified
	UpdatedAt time.Time

	// Soft delete timestamp - when group was deactivated
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Self-referencing associations for hierarchy
	// Reference to parent group (null for root-level groups)
	Parent *Group `gorm:"foreignKey:ParentID"`

	// Child groups under this group
	Children []Group `gorm:"foreignKey:ParentID"`

	// Many-to-many associations
	// Users who are members of this group
	Users []User `gorm:"many2many:user_groups;"`

	// Permissions granted to this group (deprecated - use GroupPermission junction table)
	Permissions []Permission `gorm:"many2many:group_permissions;"`
}
