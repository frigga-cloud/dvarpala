// Package main provides comprehensive installation script for Dvarpala VPN system
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"dvarpala/internal/config"
	"dvarpala/internal/database"
	"dvarpala/internal/database/models"
)

func main() {
	var (
		configPath = flag.String("config", "configs/environments/development.yaml", "Configuration file path")
		dropTables = flag.Bool("drop", false, "Drop existing tables before creating new ones")
		seedData   = flag.Bool("seed", false, "Seed initial data after table creation")
		force      = flag.Bool("force", false, "Force operation without confirmation prompts")
	)
	flag.Parse()

	fmt.Println("🚀 Dvarpala VPN System Installation")
	fmt.Println("===================================")

	// Load configuration
	fmt.Printf("📖 Loading configuration from: %s\n", *configPath)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}
	fmt.Printf("✅ Configuration loaded successfully\n")

	// Connect to database
	fmt.Printf("🔌 Connecting to database: %s@%s:%d/%s\n", 
		cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	fmt.Printf("✅ Database connection established\n")

	// Check if we should drop existing tables
	if *dropTables {
		if !*force {
			fmt.Print("⚠️  This will DROP ALL EXISTING TABLES! Are you sure? (yes/no): ")
			var response string
			fmt.Scanln(&response)
			if response != "yes" && response != "y" {
				fmt.Println("❌ Operation cancelled")
				os.Exit(0)
			}
		}
		
		fmt.Println("🗑️  Dropping existing tables...")
		if err := database.DropAllTables(db.DB); err != nil {
			log.Printf("⚠️  Warning: Failed to drop some tables: %v", err)
		} else {
			fmt.Println("✅ Existing tables dropped")
		}
	}

	// Create/migrate tables
	fmt.Println("🏗️  Creating database tables...")
	if err := database.AutoMigrate(db.DB); err != nil {
		log.Fatalf("❌ Failed to migrate database: %v", err)
	}
	fmt.Println("✅ Database tables created/updated successfully")

	// Display created tables
	fmt.Println("\n📋 Core tables created:")
	tables := []string{
		"users", "groups", "resources", "audit_logs",
		"vpn_sessions", "vpn_configs", "network_routes",
		"sessions", "oauth_states", "ip_whitelists",
		"user_groups", "group_permissions", "group_network_routes",
	}
	for _, table := range tables {
		fmt.Printf("   ✓ %s\n", table)
	}

	// Create default data
	fmt.Println("\n🌱 Creating essential default data...")
	if err := createDefaultData(db); err != nil {
		log.Printf("⚠️  Warning: Failed to create some default data: %v", err)
	} else {
		fmt.Println("✅ Default data created")
	}

	// Seed additional data if requested
	if *seedData {
		fmt.Println("\n🌱 Seeding additional development data...")
		if err := seedDevelopmentData(db); err != nil {
			log.Printf("⚠️  Warning: Failed to seed some data: %v", err)
		} else {
			fmt.Println("✅ Development data seeded")
		}
	}

	// Validate installation
	fmt.Println("\n🔍 Validating installation...")
	if err := validateInstallation(db); err != nil {
		log.Fatalf("❌ Installation validation failed: %v", err)
	}
	fmt.Println("✅ Installation validation passed")

	fmt.Println("\n🎉 Dvarpala VPN System Installation Complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Start the Dvarpala server: ./bin/dvarpala-server")
	fmt.Println("2. Access the web interface at: http://localhost:8080")
	fmt.Println("3. Configure OAuth providers in your config file")
	fmt.Println("4. Set up OpenVPN integration")
}

// createDefaultData creates essential system data
func createDefaultData(db *database.DB) error {
	// Create default system groups
	systemGroups := []models.Group{
		{Name: "system_admins", Description: "System administrators with full access"},
		{Name: "vpn_users", Description: "Standard VPN users"},
		{Name: "guests", Description: "Guest users with limited access"},
	}

	for _, group := range systemGroups {
		if err := db.FirstOrCreate(&group, models.Group{Name: group.Name}).Error; err != nil {
			return fmt.Errorf("failed to create group %s: %w", group.Name, err)
		}
	}

	// Create default resources
	systemResources := []models.Resource{
		{
			Name:        "System Administration",
			Type:        models.ResourceTypeDashboard,
			URL:         "/admin",
			Description: "System administration dashboard",
		},
		{
			Name:        "User Dashboard",
			Type:        models.ResourceTypeDashboard,
			URL:         "/dashboard",
			Description: "User dashboard and VPN status",
		},
		{
			Name:        "VPN Server",
			Type:        models.ResourceTypeService,
			IPAddress:   "10.0.0.1",
			Port:        1194,
			Description: "Main VPN server",
		},
	}

	for _, resource := range systemResources {
		if err := db.FirstOrCreate(&resource, models.Resource{Name: resource.Name}).Error; err != nil {
			return fmt.Errorf("failed to create resource %s: %w", resource.Name, err)
		}
	}

	// Create default network routes
	defaultRoutes := []models.NetworkRoute{
		{
			Name:        "Captive Portal Access",
			Destination: "0.0.0.0/0",
			RouteType:   models.RouteTypeCaptivePortal,
			Priority:    1,
			Description: "Limited access for captive portal authentication",
		},
		{
			Name:        "Full Network Access",
			Destination: "10.0.0.0/8",
			RouteType:   models.RouteTypeFullAccess,
			Priority:    10,
			Description: "Full access to internal network",
		},
		{
			Name:        "Internet Access",
			Destination: "0.0.0.0/0",
			RouteType:   models.RouteTypeFullAccess,
			Priority:    20,
			Description: "Internet access through VPN",
		},
	}

	for _, route := range defaultRoutes {
		if err := db.FirstOrCreate(&route, models.NetworkRoute{Name: route.Name}).Error; err != nil {
			return fmt.Errorf("failed to create route %s: %w", route.Name, err)
		}
	}

	return nil
}

// seedDevelopmentData creates additional data for development/testing
func seedDevelopmentData(db *database.DB) error {
	// Create test users
	testUsers := []models.User{
		{
			Email:         "admin@dvarpala.local",
			FullName:      "System Administrator",
			Department:    "IT",
			Status:        models.UserStatusActive,
			OAuthProvider: "google",
		},
		{
			Email:         "user@dvarpala.local",
			FullName:      "Test User",
			Department:    "Engineering",
			Status:        models.UserStatusActive,
			OAuthProvider: "google",
		},
	}

	for _, user := range testUsers {
		if err := db.FirstOrCreate(&user, models.User{Email: user.Email}).Error; err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.Email, err)
		}
	}

	// Create additional test resources
	testResources := []models.Resource{
		{Name: "Test Server", Type: models.ResourceTypeVM, IPAddress: "10.0.1.100", Port: 22, Description: "Test environment server"},
		{Name: "Dev Database", Type: models.ResourceTypeDatabase, IPAddress: "10.0.1.50", Port: 5432, Description: "Development database"},
		{Name: "API Gateway", Type: models.ResourceTypeService, URL: "https://api.dvarpala.local", Description: "API gateway service"},
	}

	for _, resource := range testResources {
		if err := db.FirstOrCreate(&resource, models.Resource{Name: resource.Name}).Error; err != nil {
			return fmt.Errorf("failed to create resource %s: %w", resource.Name, err)
		}
	}

	return nil
}

// validateInstallation checks if the installation is valid
func validateInstallation(db *database.DB) error {
	// Check if tables exist and have correct structure
	tables := []interface{}{
		&models.User{},
		&models.Group{},
		&models.Resource{},
		&models.VPNSession{},
		&models.AuditLog{},
		&models.UserGroup{},
		&models.GroupPermission{},
	}

	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf("table for %T does not exist", table)
		}
	}

	// Check if we can query basic tables
	var userCount, groupCount, resourceCount int64
	
	if err := db.Model(&models.User{}).Count(&userCount).Error; err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}
	
	if err := db.Model(&models.Group{}).Count(&groupCount).Error; err != nil {
		return fmt.Errorf("failed to count groups: %w", err)
	}
	
	if err := db.Model(&models.Resource{}).Count(&resourceCount).Error; err != nil {
		return fmt.Errorf("failed to count resources: %w", err)
	}

	fmt.Printf("📊 Database statistics:\n")
	fmt.Printf("   Users: %d\n", userCount)
	fmt.Printf("   Groups: %d\n", groupCount)
	fmt.Printf("   Resources: %d\n", resourceCount)

	return nil
}