// Package main provides test user creation functionality for development
package main

import (
	"fmt"
	"log"
	"os"

	"dvarpala/internal/config"
	"dvarpala/internal/database"
	"dvarpala/internal/database/models"
)

func main() {
	// Load configuration
	configPath := "configs/environments/development.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate tables
	if err := database.AutoMigrate(db.DB); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Create test users
	testUsers := []models.User{
		{
			Email:         "admin@yourcompany.com",
			FullName:      "System Administrator",
			Department:    "IT",
			Status:        models.UserStatusActive,
			OAuthProvider: "google",
		},
		{
			Email:         "john.doe@yourcompany.com",
			FullName:      "John Doe",
			Department:    "Engineering",
			Status:        models.UserStatusActive,
			OAuthProvider: "google",
		},
		{
			Email:         "jane.smith@yourcompany.com",
			FullName:      "Jane Smith",
			Department:    "Marketing",
			Status:        models.UserStatusActive,
			OAuthProvider: "microsoft",
		},
		{
			Email:         "bob.wilson@yourcompany.com",
			FullName:      "Bob Wilson",
			Department:    "Sales",
			Status:        models.UserStatusActive,
			OAuthProvider: "github",
		},
	}

	// Create test groups
	testGroups := []models.Group{
		{
			Name:        "administrators",
			Description: "System administrators with full access",
		},
		{
			Name:        "engineers",
			Description: "Engineering team members",
		},
		{
			Name:        "marketing",
			Description: "Marketing team members",
		},
		{
			Name:        "sales",
			Description: "Sales team members",
		},
	}

	// Create test resources
	testResources := []models.Resource{
		{
			Name:        "Admin Dashboard",
			Type:        models.ResourceTypeDashboard,
			URL:         "https://admin.yourcompany.com",
			Description: "Administrative dashboard",
		},
		{
			Name:        "Development Server",
			Type:        models.ResourceTypeVM,
			IPAddress:   "10.0.1.100",
			Port:        22,
			Description: "Development environment server",
		},
		{
			Name:        "Production Database",
			Type:        models.ResourceTypeDatabase,
			IPAddress:   "10.0.2.50",
			Port:        5432,
			Description: "Production PostgreSQL database",
		},
		{
			Name:        "API Gateway",
			Type:        models.ResourceTypeService,
			URL:         "https://api.yourcompany.com",
			Description: "Main API gateway",
		},
	}

	fmt.Println("Creating test data...")

	// Insert groups first
	for _, group := range testGroups {
		if err := db.FirstOrCreate(&group, models.Group{Name: group.Name}).Error; err != nil {
			log.Printf("Failed to create group %s: %v", group.Name, err)
		} else {
			fmt.Printf("✓ Created group: %s\n", group.Name)
		}
	}

	// Insert resources
	for _, resource := range testResources {
		if err := db.FirstOrCreate(&resource, models.Resource{Name: resource.Name}).Error; err != nil {
			log.Printf("Failed to create resource %s: %v", resource.Name, err)
		} else {
			fmt.Printf("✓ Created resource: %s\n", resource.Name)
		}
	}

	// Insert users
	for _, user := range testUsers {
		if err := db.FirstOrCreate(&user, models.User{Email: user.Email}).Error; err != nil {
			log.Printf("Failed to create user %s: %v", user.Email, err)
		} else {
			fmt.Printf("✓ Created user: %s (%s)\n", user.Email, user.FullName)
		}
	}

	// Assign users to groups
	assignments := map[string]string{
		"admin@yourcompany.com":      "administrators",
		"john.doe@yourcompany.com":   "engineers",
		"jane.smith@yourcompany.com": "marketing",
		"bob.wilson@yourcompany.com": "sales",
	}

	for userEmail, groupName := range assignments {
		var user models.User
		var group models.Group

		if err := db.Where("email = ?", userEmail).First(&user).Error; err != nil {
			log.Printf("User %s not found: %v", userEmail, err)
			continue
		}

		if err := db.Where("name = ?", groupName).First(&group).Error; err != nil {
			log.Printf("Group %s not found: %v", groupName, err)
			continue
		}

		// Associate user with group
		if err := db.Model(&user).Association("Groups").Append(&group); err != nil {
			log.Printf("Failed to assign user %s to group %s: %v", userEmail, groupName, err)
		} else {
			fmt.Printf("✓ Assigned %s to %s group\n", userEmail, groupName)
		}
	}

	fmt.Println("\nTest data creation completed successfully!")
	fmt.Println("You can now use these test users for development and testing.")
}
