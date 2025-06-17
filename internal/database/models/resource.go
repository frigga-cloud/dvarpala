package models

import (
	"time"
	"gorm.io/gorm"
)

type ResourceType string

const (
	ResourceTypeDashboard ResourceType = "dashboard"
	ResourceTypeVM        ResourceType = "vm"
	ResourceTypeDatabase  ResourceType = "database"
	ResourceTypeService   ResourceType = "service"
)

type Resource struct {
	ID          uint         `gorm:"primaryKey"`
	Name        string       `gorm:"not null;size:255"`
	Type        ResourceType `gorm:"not null"`
	URL         string       `gorm:"size:500"`
	IPAddress   string       `gorm:"size:45"` // IPv4/IPv6
	Port        int
	Description string       `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	
	// Associations
	Permissions []Permission `gorm:"many2many:group_permissions;"`
}