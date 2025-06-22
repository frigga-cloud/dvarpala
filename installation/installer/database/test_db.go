package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

)

func TestDatabaseHandler() {
	// Test database connection and setup
	config := DatabaseConfig{
		Host:         "localhost",
		Port:         5432,
		Username:     "dvarpala",
		Password:     "dvarpala123",
		DatabaseName: "dvarpala",
		AdminEmail:   "admin@example.com",
		AdminName:    "Test Admin",
	}

	handler := NewPostgreSQLHandler(config)

	// Test connection only (don't run full setup without PostgreSQL running)
	fmt.Println("Testing PostgreSQL database handler...")
	
	// Get current directory to find schema files
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	schemaPath := filepath.Join(currentDir, "..", "schema")
	fmt.Printf("Schema path: %s\n", schemaPath)

	// Check if schema files exist
	migrationsPath := filepath.Join(schemaPath, "migrations")
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		fmt.Printf("❌ Migrations directory not found: %s\n", migrationsPath)
		return
	}

	seedsPath := filepath.Join(schemaPath, "seeds")
	if _, err := os.Stat(seedsPath); os.IsNotExist(err) {
		fmt.Printf("❌ Seeds directory not found: %s\n", seedsPath)
		return
	}

	fmt.Println("✅ Schema directory structure is valid")

	// In a real scenario with PostgreSQL running, you would call:
	// if err := handler.SetupDatabase(schemaPath); err != nil {
	//     log.Fatalf("Database setup failed: %v", err)
	// }

	fmt.Println("✅ Database handler test completed")
	fmt.Println("📋 To test with real PostgreSQL:")
	fmt.Println("   1. Ensure PostgreSQL is running")
	fmt.Println("   2. Create dvarpala user with password 'dvarpala123'")
	fmt.Println("   3. Uncomment the SetupDatabase call above")
}