// Package main provides comprehensive seed data functionality for development
package main

import (
	"fmt"
	"log"
	"os"
	"time"

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

	fmt.Println("Seeding comprehensive development data...")

	// Create additional test groups with hierarchy
	parentGroups := []models.Group{
		{Name: "engineering", Description: "Engineering department"},
		{Name: "business", Description: "Business department"},
		{Name: "operations", Description: "Operations department"},
	}

	// Insert parent groups first
	for _, group := range parentGroups {
		if err := db.FirstOrCreate(&group, models.Group{Name: group.Name}).Error; err != nil {
			log.Printf("Failed to create parent group %s: %v", group.Name, err)
		} else {
			fmt.Printf("✓ Created parent group: %s\n", group.Name)
		}
	}

	// Get engineering parent group ID for child groups
	var engineeringGroup models.Group
	db.Where("name = ?", "engineering").First(&engineeringGroup)

	childGroups := []models.Group{
		{Name: "frontend-devs", Description: "Frontend developers", ParentID: &engineeringGroup.ID},
		{Name: "backend-devs", Description: "Backend developers", ParentID: &engineeringGroup.ID},
		{Name: "devops", Description: "DevOps engineers", ParentID: &engineeringGroup.ID},
	}

	// Insert child groups
	for _, group := range childGroups {
		if err := db.FirstOrCreate(&group, models.Group{Name: group.Name}).Error; err != nil {
			log.Printf("Failed to create child group %s: %v", group.Name, err)
		} else {
			fmt.Printf("✓ Created child group: %s\n", group.Name)
		}
	}

	// Create comprehensive test resources
	resources := []models.Resource{
		{Name: "Development API", Type: models.ResourceTypeService, URL: "https://dev-api.yourcompany.com", Description: "Development API endpoint"},
		{Name: "Staging API", Type: models.ResourceTypeService, URL: "https://staging-api.yourcompany.com", Description: "Staging API endpoint"},
		{Name: "Production API", Type: models.ResourceTypeService, URL: "https://api.yourcompany.com", Description: "Production API endpoint"},
		{Name: "Dev Database", Type: models.ResourceTypeDatabase, IPAddress: "10.0.1.50", Port: 5432, Description: "Development PostgreSQL database"},
		{Name: "Test Server", Type: models.ResourceTypeVM, IPAddress: "10.0.1.101", Port: 22, Description: "Test environment server"},
		{Name: "Build Server", Type: models.ResourceTypeVM, IPAddress: "10.0.1.102", Port: 22, Description: "CI/CD build server"},
		{Name: "Monitoring Dashboard", Type: models.ResourceTypeDashboard, URL: "https://monitor.yourcompany.com", Description: "System monitoring dashboard"},
		{Name: "Analytics Dashboard", Type: models.ResourceTypeDashboard, URL: "https://analytics.yourcompany.com", Description: "Business analytics dashboard"},
	}

	for _, resource := range resources {
		if err := db.FirstOrCreate(&resource, models.Resource{Name: resource.Name}).Error; err != nil {
			log.Printf("Failed to create resource %s: %v", resource.Name, err)
		} else {
			fmt.Printf("✓ Created resource: %s\n", resource.Name)
		}
	}

	// Create additional test users
	additionalUsers := []models.User{
		{Email: "alice.dev@yourcompany.com", FullName: "Alice Developer", Department: "Engineering", Status: models.UserStatusActive, OAuthProvider: "google"},
		{Email: "bob.frontend@yourcompany.com", FullName: "Bob Frontend", Department: "Engineering", Status: models.UserStatusActive, OAuthProvider: "github"},
		{Email: "charlie.backend@yourcompany.com", FullName: "Charlie Backend", Department: "Engineering", Status: models.UserStatusActive, OAuthProvider: "google"},
		{Email: "diana.devops@yourcompany.com", FullName: "Diana DevOps", Department: "Engineering", Status: models.UserStatusActive, OAuthProvider: "microsoft"},
		{Email: "eve.suspended@yourcompany.com", FullName: "Eve Suspended", Department: "Engineering", Status: models.UserStatusSuspended, OAuthProvider: "google"},
	}

	for _, user := range additionalUsers {
		if err := db.FirstOrCreate(&user, models.User{Email: user.Email}).Error; err != nil {
			log.Printf("Failed to create user %s: %v", user.Email, err)
		} else {
			fmt.Printf("✓ Created user: %s (%s)\n", user.Email, user.FullName)
		}
	}

	// Create sample VPN sessions
	now := time.Now()
	vpnSessions := []models.VPNSession{
		{UserID: 1, ClientIP: "192.168.1.100", ServerIP: "10.0.0.1", Status: models.SessionStatusActive, ConnectedAt: now.Add(-2 * time.Hour), BytesIn: 1024000, BytesOut: 512000},
		{UserID: 2, ClientIP: "192.168.1.101", ServerIP: "10.0.0.1", Status: models.SessionStatusDisconnected, ConnectedAt: now.Add(-4 * time.Hour), DisconnectedAt: &[]time.Time{now.Add(-1 * time.Hour)}[0], BytesIn: 2048000, BytesOut: 1024000},
		{UserID: 3, ClientIP: "192.168.1.102", ServerIP: "10.0.0.1", Status: models.SessionStatusActive, ConnectedAt: now.Add(-30 * time.Minute), BytesIn: 512000, BytesOut: 256000},
	}

	for _, session := range vpnSessions {
		if err := db.Create(&session).Error; err != nil {
			log.Printf("Failed to create VPN session: %v", err)
		} else {
			fmt.Printf("✓ Created VPN session for user ID %d\n", session.UserID)
		}
	}

	// Create sample audit logs
	auditLogs := []models.AuditLog{
		{UserID: &[]uint{1}[0], Action: "user_login", ResourceType: "authentication", IPAddress: "192.168.1.100", Details: `{"method": "oauth", "provider": "google"}`},
		{UserID: &[]uint{2}[0], Action: "vpn_connect", ResourceType: "vpn", IPAddress: "192.168.1.101", Details: `{"server": "10.0.0.1", "protocol": "openvpn"}`},
		{UserID: &[]uint{1}[0], Action: "resource_access", ResourceType: "dashboard", IPAddress: "192.168.1.100", Details: `{"resource": "admin_dashboard", "action": "view"}`},
	}

	for _, auditLog := range auditLogs {
		if err := db.Create(&auditLog).Error; err != nil {
			log.Printf("Failed to create audit log: %v", err)
		} else {
			fmt.Printf("✓ Created audit log for action: %s\n", auditLog.Action)
		}
	}

	fmt.Println("\nComprehensive seed data creation completed!")
	fmt.Println("Database now contains:")
	fmt.Println("- User accounts with different statuses")
	fmt.Println("- Hierarchical group structure")
	fmt.Println("- Various resource types")
	fmt.Println("- Sample VPN sessions")
	fmt.Println("- Audit trail entries")
}
