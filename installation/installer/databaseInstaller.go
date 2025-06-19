// Package main provides standalone database installation script for Dvarpala VPN system
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func mainDatabaseInstaller() {
	var (
		dbHost     = flag.String("host", "localhost", "Database host")
		dbPort     = flag.Int("port", 5432, "Database port")
		dbUser     = flag.String("user", "dvarpala", "Database user")
		dbPassword = flag.String("password", "", "Database password")
		dbName     = flag.String("dbname", "dvarpala", "Database name")
		dropTables = flag.Bool("drop", false, "Drop existing tables before creating new ones")
		seedData   = flag.Bool("seed", false, "Seed initial data after table creation")
		force      = flag.Bool("force", false, "Force operation without confirmation prompts")
	)
	flag.Parse()

	fmt.Println("🚀 Dvarpala VPN System Database Installation")
	fmt.Println("==========================================")

	// Connect to database
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		*dbHost, *dbPort, *dbUser, *dbPassword, *dbName)
	fmt.Printf("🔌 Connecting to database: %s@%s:%d/%s\n", *dbUser, *dbHost, *dbPort, *dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Failed to ping database: %v", err)
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
		if err := dropAllTables(db); err != nil {
			log.Printf("⚠️  Warning: Failed to drop some tables: %v", err)
		} else {
			fmt.Println("✅ Existing tables dropped")
		}
	}

	// Create/migrate tables
	fmt.Println("🏗️  Creating database tables...")
	if err := createTables(db); err != nil {
		log.Fatalf("❌ Failed to create database tables: %v", err)
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

	fmt.Println("\n🎉 Dvarpala VPN System Database Installation Complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Start the Dvarpala server: ./bin/dvarpala-server")
	fmt.Println("2. Access the web interface at: http://localhost:8080")
	fmt.Println("3. Configure OAuth providers in your config file")
	fmt.Println("4. Set up OpenVPN integration")
}

// dropAllTables drops all existing tables
func dropAllTables(db *sql.DB) error {
	tables := []string{
		"group_network_routes", "group_permissions", "user_groups",
		"ip_whitelists", "oauth_states", "sessions", "network_routes",
		"vpn_configs", "vpn_sessions", "audit_logs", "resources", "groups", "users",
	}

	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}
	return nil
}

// createTables creates all required database tables
func createTables(db *sql.DB) error {
	schemas := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			full_name VARCHAR(255) NOT NULL,
			department VARCHAR(255),
			status VARCHAR(50) DEFAULT 'active',
			oauth_provider VARCHAR(50),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Groups table
		`CREATE TABLE IF NOT EXISTS groups (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) UNIQUE NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Resources table
		`CREATE TABLE IF NOT EXISTS resources (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) UNIQUE NOT NULL,
			type VARCHAR(50) NOT NULL,
			url VARCHAR(512),
			ip_address INET,
			port INTEGER,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// VPN Sessions table
		`CREATE TABLE IF NOT EXISTS vpn_sessions (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			client_ip INET,
			vpn_ip INET,
			connected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			disconnected_at TIMESTAMP,
			bytes_sent BIGINT DEFAULT 0,
			bytes_received BIGINT DEFAULT 0,
			status VARCHAR(50) DEFAULT 'active'
		)`,

		// Audit Logs table
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			action VARCHAR(255) NOT NULL,
			resource_type VARCHAR(100),
			resource_id INTEGER,
			details JSONB,
			ip_address INET,
			user_agent TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Network Routes table
		`CREATE TABLE IF NOT EXISTS network_routes (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) UNIQUE NOT NULL,
			destination CIDR NOT NULL,
			route_type VARCHAR(50) NOT NULL,
			priority INTEGER DEFAULT 0,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// VPN Configs table
		`CREATE TABLE IF NOT EXISTS vpn_configs (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			config_name VARCHAR(255) NOT NULL,
			config_data TEXT NOT NULL,
			certificate_data TEXT,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP
		)`,

		// Sessions table
		`CREATE TABLE IF NOT EXISTS sessions (
			id VARCHAR(255) PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			session_data JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL
		)`,

		// OAuth States table
		`CREATE TABLE IF NOT EXISTS oauth_states (
			state VARCHAR(255) PRIMARY KEY,
			provider VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL
		)`,

		// IP Whitelists table
		`CREATE TABLE IF NOT EXISTS ip_whitelists (
			id SERIAL PRIMARY KEY,
			ip_address CIDR NOT NULL,
			description TEXT,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Junction tables
		`CREATE TABLE IF NOT EXISTS user_groups (
			user_id INTEGER REFERENCES users(id),
			group_id INTEGER REFERENCES groups(id),
			PRIMARY KEY (user_id, group_id)
		)`,

		`CREATE TABLE IF NOT EXISTS group_permissions (
			group_id INTEGER REFERENCES groups(id),
			resource_id INTEGER REFERENCES resources(id),
			permission VARCHAR(100) NOT NULL,
			PRIMARY KEY (group_id, resource_id, permission)
		)`,

		`CREATE TABLE IF NOT EXISTS group_network_routes (
			group_id INTEGER REFERENCES groups(id),
			network_route_id INTEGER REFERENCES network_routes(id),
			PRIMARY KEY (group_id, network_route_id)
		)`,
	}

	for _, schema := range schemas {
		if _, err := db.Exec(schema); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// createDefaultData creates essential system data
func createDefaultData(db *sql.DB) error {
	// Create default system groups
	systemGroups := []struct {
		Name        string
		Description string
	}{
		{"system_admins", "System administrators with full access"},
		{"vpn_users", "Standard VPN users"},
		{"guests", "Guest users with limited access"},
	}

	for _, group := range systemGroups {
		_, err := db.Exec("INSERT INTO groups (name, description) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING",
			group.Name, group.Description)
		if err != nil {
			return fmt.Errorf("failed to create group %s: %w", group.Name, err)
		}
	}

	// Create default resources
	systemResources := []struct {
		Name        string
		Type        string
		URL         *string
		IPAddress   *string
		Port        *int
		Description string
	}{
		{"System Administration", "dashboard", stringPtr("/admin"), nil, nil, "System administration dashboard"},
		{"User Dashboard", "dashboard", stringPtr("/dashboard"), nil, nil, "User dashboard and VPN status"},
		{"VPN Server", "service", nil, stringPtr("10.0.0.1"), intPtr(1194), "Main VPN server"},
	}

	for _, resource := range systemResources {
		_, err := db.Exec(`INSERT INTO resources (name, type, url, ip_address, port, description) 
			VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (name) DO NOTHING`,
			resource.Name, resource.Type, resource.URL, resource.IPAddress, resource.Port, resource.Description)
		if err != nil {
			return fmt.Errorf("failed to create resource %s: %w", resource.Name, err)
		}
	}

	// Create default network routes
	defaultRoutes := []struct {
		Name        string
		Destination string
		RouteType   string
		Priority    int
		Description string
	}{
		{"Captive Portal Access", "0.0.0.0/0", "captive_portal", 1, "Limited access for captive portal authentication"},
		{"Full Network Access", "10.0.0.0/8", "full_access", 10, "Full access to internal network"},
		{"Internet Access", "0.0.0.0/0", "full_access", 20, "Internet access through VPN"},
	}

	for _, route := range defaultRoutes {
		_, err := db.Exec(`INSERT INTO network_routes (name, destination, route_type, priority, description) 
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (name) DO NOTHING`,
			route.Name, route.Destination, route.RouteType, route.Priority, route.Description)
		if err != nil {
			return fmt.Errorf("failed to create route %s: %w", route.Name, err)
		}
	}

	return nil
}

// seedDevelopmentData creates additional data for development/testing
func seedDevelopmentData(db *sql.DB) error {
	// Create test users
	testUsers := []struct {
		Email         string
		FullName      string
		Department    string
		Status        string
		OAuthProvider string
	}{
		{"admin@dvarpala.local", "System Administrator", "IT", "active", "google"},
		{"user@dvarpala.local", "Test User", "Engineering", "active", "google"},
	}

	for _, user := range testUsers {
		_, err := db.Exec(`INSERT INTO users (email, full_name, department, status, oauth_provider) 
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (email) DO NOTHING`,
			user.Email, user.FullName, user.Department, user.Status, user.OAuthProvider)
		if err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.Email, err)
		}
	}

	// Create additional test resources
	testResources := []struct {
		Name        string
		Type        string
		IPAddress   *string
		Port        *int
		URL         *string
		Description string
	}{
		{"Test Server", "vm", stringPtr("10.0.1.100"), intPtr(22), nil, "Test environment server"},
		{"Dev Database", "database", stringPtr("10.0.1.50"), intPtr(5432), nil, "Development database"},
		{"API Gateway", "service", nil, nil, stringPtr("https://api.dvarpala.local"), "API gateway service"},
	}

	for _, resource := range testResources {
		_, err := db.Exec(`INSERT INTO resources (name, type, ip_address, port, url, description) 
			VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (name) DO NOTHING`,
			resource.Name, resource.Type, resource.IPAddress, resource.Port, resource.URL, resource.Description)
		if err != nil {
			return fmt.Errorf("failed to create resource %s: %w", resource.Name, err)
		}
	}

	return nil
}

// validateInstallation checks if the installation is valid
func validateInstallation(db *sql.DB) error {
	// Check if tables exist and can be queried
	tables := []string{"users", "groups", "resources", "vpn_sessions", "audit_logs", "user_groups", "group_permissions"}

	for _, table := range tables {
		var count int
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to query table %s: %w", table, err)
		}
	}

	// Get counts for statistics
	var userCount, groupCount, resourceCount int

	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	db.QueryRow("SELECT COUNT(*) FROM groups").Scan(&groupCount)
	db.QueryRow("SELECT COUNT(*) FROM resources").Scan(&resourceCount)

	fmt.Printf("📊 Database statistics:\n")
	fmt.Printf("   Users: %d\n", userCount)
	fmt.Printf("   Groups: %d\n", groupCount)
	fmt.Printf("   Resources: %d\n", resourceCount)

	return nil
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

// To use this database installer, call mainDatabaseInstaller() from elsewhere
// This file is designed to be used as a library, not run directly
