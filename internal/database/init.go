package database

import (
	"fmt"
	"log"

	"dvarpala/internal/database/models"
	"gorm.io/gorm"
)

// InitializeDatabase initializes the database with required tables and data
func InitializeDatabase(db *gorm.DB) error {
	log.Println("Initializing database...")

	// Run migrations
	if err := AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	log.Println("Database migration completed")

	// Create essential default data
	if err := createEssentialData(db); err != nil {
		return fmt.Errorf("failed to create essential data: %w", err)
	}
	log.Println("Essential data created")

	return nil
}

// createEssentialData creates the minimum required data for the system to function
func createEssentialData(db *gorm.DB) error {
	// Create default system groups if they don't exist
	defaultGroups := []models.Group{
		{Name: "system_admins", Description: "System administrators"},
		{Name: "vpn_users", Description: "VPN users"},
	}

	for _, group := range defaultGroups {
		var existingGroup models.Group
		if err := db.Where("name = ?", group.Name).First(&existingGroup).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&group).Error; err != nil {
					return fmt.Errorf("failed to create group %s: %w", group.Name, err)
				}
				log.Printf("Created default group: %s", group.Name)
			} else {
				return fmt.Errorf("error checking group %s: %w", group.Name, err)
			}
		}
	}

	// Create default system resources
	defaultResources := []models.Resource{
		{
			Name:        "System Dashboard",
			Type:        models.ResourceTypeDashboard,
			URL:         "/admin",
			Description: "System administration dashboard",
		},
		{
			Name:        "User Portal",
			Type:        models.ResourceTypeDashboard,
			URL:         "/portal",
			Description: "User self-service portal",
		},
	}

	for _, resource := range defaultResources {
		var existingResource models.Resource
		if err := db.Where("name = ?", resource.Name).First(&existingResource).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&resource).Error; err != nil {
					return fmt.Errorf("failed to create resource %s: %w", resource.Name, err)
				}
				log.Printf("Created default resource: %s", resource.Name)
			} else {
				return fmt.Errorf("error checking resource %s: %w", resource.Name, err)
			}
		}
	}

	// Create default network routes
	defaultRoutes := []models.NetworkRoute{
		{
			Name:        "Captive Portal",
			Destination: "8.8.8.8/32",
			RouteType:   models.RouteTypeCaptivePortal,
			Priority:    1,
			Description: "DNS access for captive portal",
		},
	}

	for _, route := range defaultRoutes {
		var existingRoute models.NetworkRoute
		if err := db.Where("name = ?", route.Name).First(&existingRoute).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&route).Error; err != nil {
					return fmt.Errorf("failed to create route %s: %w", route.Name, err)
				}
				log.Printf("Created default route: %s", route.Name)
			} else {
				return fmt.Errorf("error checking route %s: %w", route.Name, err)
			}
		}
	}

	return nil
}
