package models

import (
	"time"
	"gorm.io/gorm"
)

type Group struct {
	ID          uint           `gorm:"primaryKey"`
	Name        string         `gorm:"uniqueIndex;not null;size:100"`
	Description string         `gorm:"type:text"`
	ParentID    *uint          `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	
	// Self-referencing for hierarchy
	Parent      *Group  `gorm:"foreignKey:ParentID"`
	Children    []Group `gorm:"foreignKey:ParentID"`
	
	// Associations
	Users       []User       `gorm:"many2many:user_groups;"`
	Permissions []Permission `gorm:"many2many:group_permissions;"`
}