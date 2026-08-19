package models

import (
	"gorm.io/gorm"
	"time"
)

// ResourceType defines the category of protected resource
type ResourceType string

const (
	ResourceTypeDashboard ResourceType = "dashboard" // Web dashboard or UI interface
	ResourceTypeVM        ResourceType = "vm"        // Virtual machine or server
	ResourceTypeDatabase  ResourceType = "database"  // Database server
	ResourceTypeService   ResourceType = "service"   // API service or microservice
)

// Resource represents a protected system resource that requires access control
type Resource struct {
	// Primary identifier for the resource
	ID uint `gorm:"primaryKey"`

	// Human-readable name of the resource (e.g., "Admin Dashboard", "Production DB")
	Name string `gorm:"not null;size:255"`

	// Category of resource for appropriate access control handling
	Type ResourceType `gorm:"not null"`

	// Web URL for dashboard and service type resources
	URL string `gorm:"size:500"`

	// IP address for VM and database type resources (IPv4/IPv6 compatible)
	IPAddress string `gorm:"size:45"`

	// Network port for VM and database type resources
	Port int

	// Detailed description of the resource and its purpose
	Description string `gorm:"type:text"`

	// Timestamp when resource was registered
	CreatedAt time.Time

	// Timestamp when resource configuration was last updated
	UpdatedAt time.Time

	// Soft delete timestamp - when resource was decommissioned
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Associations
	// Permissions granted on this resource (deprecated - use GroupPermission junction table)
	Permissions []Permission `gorm:"many2many:group_permissions;"`
}
