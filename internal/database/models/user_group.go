// Package models provides user group relationship model definitions
package models

import (
	"time"
)

// UserGroup represents the many-to-many relationship between users and groups
type UserGroup struct {
	// Foreign key to user who is a member of the group
	UserID uint `gorm:"primaryKey;not null"`

	// Foreign key to group that the user belongs to
	GroupID uint `gorm:"primaryKey;not null"`

	// Timestamp when user was added to the group
	CreatedAt time.Time `gorm:"autoCreateTime"`

	// Timestamp when membership was last updated
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	// Foreign key constraints
	// Reference to the user
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`

	// Reference to the group
	Group Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
}

// TableName returns the table name for UserGroup
func (UserGroup) TableName() string {
	return "user_groups"
}
