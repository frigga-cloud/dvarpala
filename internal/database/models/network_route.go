package models

import (
	"time"
	"gorm.io/gorm"
)

// RouteType represents the access level of network route
type RouteType string

const (
	RouteTypeCaptivePortal RouteType = "captive_portal" // Limited access for authentication
	RouteTypeFullAccess    RouteType = "full_access"    // Complete network access
	RouteTypeRestricted    RouteType = "restricted"     // Limited access to specific resources
)

// NetworkRoute defines network routing rules for VPN clients based on group membership
type NetworkRoute struct {
	// Primary identifier for the network route
	ID uint `gorm:"primaryKey"`
	
	// Human-readable name for this route (e.g., "Internal Network", "Internet Access")
	Name string `gorm:"not null;size:100"`
	
	// Destination network in CIDR notation (e.g., "10.0.0.0/8", "0.0.0.0/0")
	Destination string `gorm:"not null;size:50"`
	
	// Gateway IP address for routing (nullable for direct routes)
	Gateway string `gorm:"size:45"`
	
	// Type of access this route provides
	RouteType RouteType `gorm:"not null"`
	
	// Priority for route ordering (lower number = higher priority)
	Priority int `gorm:"default:100"`
	
	// Flag to enable/disable this route
	IsActive bool `gorm:"default:true"`
	
	// Detailed description of route purpose and restrictions
	Description string `gorm:"size:255"`
	
	// Timestamp when route was created
	CreatedAt time.Time
	
	// Timestamp when route was last updated
	UpdatedAt time.Time
	
	// Soft delete timestamp - when route was deactivated
	DeletedAt gorm.DeletedAt `gorm:"index"`
	
	// Many-to-many associations
	// Groups that have access to this network route
	Groups []Group `gorm:"many2many:group_network_routes;"`
}

// TableName returns the table name for NetworkRoute
func (NetworkRoute) TableName() string {
	return "network_routes"
}

// GroupNetworkRoute represents the junction table for groups and network routes
type GroupNetworkRoute struct {
	// Foreign key to group that has access to the route
	GroupID uint `gorm:"primaryKey;not null"`
	
	// Foreign key to network route that the group can access
	NetworkRouteID uint `gorm:"primaryKey;not null"`
	
	// Timestamp when access was granted
	CreatedAt time.Time `gorm:"autoCreateTime"`
	
	// Foreign key constraints
	// Reference to the group
	Group Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
	
	// Reference to the network route
	NetworkRoute NetworkRoute `gorm:"foreignKey:NetworkRouteID;constraint:OnDelete:CASCADE"`
}

// TableName returns the table name for GroupNetworkRoute
func (GroupNetworkRoute) TableName() string {
	return "group_network_routes"
}