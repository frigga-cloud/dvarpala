package database

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgreSQLHandler handles database operations for PostgreSQL
type PostgreSQLHandler struct {
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
	AdminEmail   string
	AdminName    string
	db           *sql.DB
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
	AdminEmail   string
	AdminName    string
}

// NewPostgreSQLHandler creates a new PostgreSQL handler instance
func NewPostgreSQLHandler(config DatabaseConfig) *PostgreSQLHandler {
	return &PostgreSQLHandler{
		Host:         config.Host,
		Port:         config.Port,
		Username:     config.Username,
		Password:     config.Password,
		DatabaseName: config.DatabaseName,
		AdminEmail:   config.AdminEmail,
		AdminName:    config.AdminName,
	}
}

// Connect establishes connection to PostgreSQL database
func (pg *PostgreSQLHandler) Connect() error {
	// First connect to postgres database to create dvarpala database if needed
	postgresConnStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
		pg.Host, pg.Port, pg.Username, pg.Password)

	postgresDB, err := sql.Open("postgres", postgresConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %v", err)
	}
	defer postgresDB.Close()

	// Test connection
	if err := postgresDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping postgres database: %v", err)
	}

	fmt.Printf("✅ Connected to PostgreSQL server\n")

	// Create dvarpala database if it doesn't exist
	if err := pg.createDatabaseIfNotExists(postgresDB); err != nil {
		return err
	}

	// Now connect to dvarpala database
	dvarpalConnStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		pg.Host, pg.Port, pg.Username, pg.Password, pg.DatabaseName)

	pg.db, err = sql.Open("postgres", dvarpalConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s database: %v", pg.DatabaseName, err)
	}

	// Test connection to dvarpala database
	if err := pg.db.Ping(); err != nil {
		return fmt.Errorf("failed to ping %s database: %v", pg.DatabaseName, err)
	}

	fmt.Printf("✅ Connected to %s database\n", pg.DatabaseName)
	return nil
}

// Close closes the database connection
func (pg *PostgreSQLHandler) Close() error {
	if pg.db != nil {
		return pg.db.Close()
	}
	return nil
}

// createDatabaseIfNotExists creates the dvarpala database if it doesn't exist
func (pg *PostgreSQLHandler) createDatabaseIfNotExists(postgresDB *sql.DB) error {
	// Check if database exists
	var exists bool
	query := "SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = $1)"
	err := postgresDB.QueryRow(query, pg.DatabaseName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %v", err)
	}

	if !exists {
		fmt.Printf("🏗️ Creating database: %s\n", pg.DatabaseName)
		createDBQuery := fmt.Sprintf("CREATE DATABASE %s", pg.DatabaseName)
		if _, err := postgresDB.Exec(createDBQuery); err != nil {
			return fmt.Errorf("failed to create database %s: %v", pg.DatabaseName, err)
		}
		fmt.Printf("✅ Database %s created successfully\n", pg.DatabaseName)
	} else {
		fmt.Printf("✅ Database %s already exists\n", pg.DatabaseName)
	}

	return nil
}

// ExecuteSQL executes a single SQL statement
func (pg *PostgreSQLHandler) ExecuteSQL(sql string) error {
	if pg.db == nil {
		return fmt.Errorf("database connection not established")
	}

	_, err := pg.db.Exec(sql)
	return err
}

// ExecuteSQLFile executes SQL commands from a file
func (pg *PostgreSQLHandler) ExecuteSQLFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read SQL file %s: %v", filePath, err)
	}

	sqlContent := string(content)
	
	// Split by semicolon and execute each statement
	statements := strings.Split(sqlContent, ";")
	
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue // Skip empty lines and comments
		}

		if err := pg.ExecuteSQL(stmt); err != nil {
			return fmt.Errorf("failed to execute statement %d in file %s: %v\nStatement: %s", 
				i+1, filepath.Base(filePath), err, stmt)
		}
	}

	fmt.Printf("✅ Executed SQL file: %s\n", filepath.Base(filePath))
	return nil
}

// RunMigrations executes all migration files in order
func (pg *PostgreSQLHandler) RunMigrations(migrationsPath string) error {
	files, err := pg.getMigrationFiles(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to get migration files: %v", err)
	}

	fmt.Printf("🔄 Running %d migration files...\n", len(files))

	for i, file := range files {
		fmt.Printf("  [%d/%d] Executing %s...\n", i+1, len(files), filepath.Base(file))
		if err := pg.ExecuteSQLFile(file); err != nil {
			return fmt.Errorf("migration failed at %s: %v", filepath.Base(file), err)
		}
	}

	fmt.Printf("✅ All migrations completed successfully\n")
	return nil
}

// RunSeeds executes all seed files in order
func (pg *PostgreSQLHandler) RunSeeds(seedsPath string) error {
	files, err := pg.getSeedFiles(seedsPath)
	if err != nil {
		return fmt.Errorf("failed to get seed files: %v", err)
	}

	fmt.Printf("🌱 Running %d seed files...\n", len(files))

	for i, file := range files {
		fmt.Printf("  [%d/%d] Executing %s...\n", i+1, len(files), filepath.Base(file))
		if err := pg.ExecuteSQLFile(file); err != nil {
			return fmt.Errorf("seed failed at %s: %v", filepath.Base(file), err)
		}
	}

	fmt.Printf("✅ All seeds completed successfully\n")
	return nil
}

// CreateAdminUser creates the admin user with dynamic values
func (pg *PostgreSQLHandler) CreateAdminUser() error {
	fmt.Printf("👤 Creating admin user: %s\n", pg.AdminEmail)

	// Insert admin user
	userSQL := `
		INSERT INTO users (email, full_name, is_admin, is_active) 
		VALUES ($1, $2, true, true) 
		ON CONFLICT (email) DO UPDATE SET
			full_name = EXCLUDED.full_name,
			is_admin = true,
			is_active = true,
			updated_at = CURRENT_TIMESTAMP`

	if err := pg.ExecuteSQL(fmt.Sprintf(userSQL, pg.AdminEmail, pg.AdminName)); err != nil {
		return fmt.Errorf("failed to create admin user: %v", err)
	}

	// Assign to administrators group
	groupSQL := `
		INSERT INTO user_groups (user_id, group_id, assigned_by)
		SELECT u.id, g.id, u.id
		FROM users u, groups g
		WHERE u.email = $1 AND g.name = 'administrators'
		ON CONFLICT (user_id, group_id) DO NOTHING`

	if err := pg.ExecuteSQL(fmt.Sprintf(groupSQL, pg.AdminEmail)); err != nil {
		return fmt.Errorf("failed to assign admin user to administrators group: %v", err)
	}

	fmt.Printf("✅ Admin user created and assigned to administrators group\n")
	return nil
}

// SetupDatabase runs the complete database setup process
func (pg *PostgreSQLHandler) SetupDatabase(schemaPath string) error {
	fmt.Printf("🗄️ Starting database setup...\n")

	// Connect to database
	if err := pg.Connect(); err != nil {
		return err
	}
	defer pg.Close()

	// Run migrations
	migrationsPath := filepath.Join(schemaPath, "migrations")
	if err := pg.RunMigrations(migrationsPath); err != nil {
		return err
	}

	// Run seeds
	seedsPath := filepath.Join(schemaPath, "seeds")
	if err := pg.RunSeeds(seedsPath); err != nil {
		return err
	}

	// Create admin user
	if err := pg.CreateAdminUser(); err != nil {
		return err
	}

	fmt.Printf("✅ Database setup completed successfully\n")
	return nil
}

// TestConnection tests the database connection
func (pg *PostgreSQLHandler) TestConnection() error {
	if err := pg.Connect(); err != nil {
		return err
	}
	defer pg.Close()

	var version string
	err := pg.db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to query database version: %v", err)
	}

	fmt.Printf("✅ Database connection successful\n")
	fmt.Printf("📋 PostgreSQL version: %s\n", version)
	return nil
}

// getMigrationFiles returns sorted list of migration files
func (pg *PostgreSQLHandler) getMigrationFiles(migrationsPath string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(migrationsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort files to ensure proper execution order
	sort.Strings(files)
	return files, nil
}

// getSeedFiles returns sorted list of seed files
func (pg *PostgreSQLHandler) getSeedFiles(seedsPath string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(seedsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort files to ensure proper execution order
	sort.Strings(files)
	return files, nil
}

// GetTableCount returns the number of tables in the database
func (pg *PostgreSQLHandler) GetTableCount() (int, error) {
	if pg.db == nil {
		return 0, fmt.Errorf("database connection not established")
	}

	var count int
	query := `SELECT COUNT(*) FROM information_schema.tables 
			  WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`
	
	err := pg.db.QueryRow(query).Scan(&count)
	return count, err
}

// VerifySchema verifies that all expected tables exist
func (pg *PostgreSQLHandler) VerifySchema() error {
	expectedTables := []string{
		"users", "groups", "user_groups", "sessions", 
		"vpn_access_logs", "system_settings",
	}

	for _, table := range expectedTables {
		var exists bool
		query := `SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		)`
		
		if err := pg.db.QueryRow(query, table).Scan(&exists); err != nil {
			return fmt.Errorf("failed to check table %s: %v", table, err)
		}

		if !exists {
			return fmt.Errorf("required table %s does not exist", table)
		}
	}

	count, err := pg.GetTableCount()
	if err != nil {
		return fmt.Errorf("failed to get table count: %v", err)
	}

	fmt.Printf("✅ Schema verification successful: %d tables found\n", count)
	return nil
}