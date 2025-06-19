package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// Resource management API for Dvarpala selective blocking

type BlockedResource struct {
	ID           int       `json:"id"`
	ResourceType string    `json:"resource_type"`
	ResourceValue string   `json:"resource_value"`
	Description  string    `json:"description"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	IsActive     bool      `json:"is_active"`
}

type VPCResource struct {
	ID             int       `json:"id"`
	ResourceID     string    `json:"resource_id"`
	ResourceType   string    `json:"resource_type"`
	IPAddress      string    `json:"ip_address,omitempty"`
	DomainName     string    `json:"domain_name,omitempty"`
	PortRange      string    `json:"port_range,omitempty"`
	CloudProvider  string    `json:"cloud_provider"`
	VPCID          string    `json:"vpc_id"`
	AutoDiscovered bool      `json:"auto_discovered"`
	LastScan       time.Time `json:"last_scan"`
	IsActive       bool      `json:"is_active"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type ResourceRequest struct {
	ResourceType  string `json:"resource_type"`
	ResourceValue string `json:"resource_value"`
	Description   string `json:"description"`
	CreatedBy     string `json:"created_by"`
}

type BulkResourceRequest struct {
	Resources []ResourceRequest `json:"resources"`
	CreatedBy string            `json:"created_by"`
}

var db *sql.DB

// Database connection
func initDB() error {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "dvarpala")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "dvarpala")

	connectionString := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbName)
	
	if dbPassword != "" {
		connectionString += fmt.Sprintf(" password=%s", dbPassword)
	}

	var err error
	db, err = sql.Open("postgres", connectionString)
	if err != nil {
		return err
	}

	return db.Ping()
}

// Utility functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func respondJSON(w http.ResponseWriter, data APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	if !data.Success {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(data)
}

func validateResourceType(resourceType string) bool {
	validTypes := []string{"ip", "domain", "cidr", "vpc_resource"}
	for _, validType := range validTypes {
		if resourceType == validType {
			return true
		}
	}
	return false
}

func validateIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func validateCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

func validateDomain(domain string) bool {
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	return domainRegex.MatchString(domain)
}

func validateResource(resourceType, resourceValue string) error {
	if !validateResourceType(resourceType) {
		return fmt.Errorf("invalid resource type: %s", resourceType)
	}

	switch resourceType {
	case "ip":
		if !validateIP(resourceValue) {
			return fmt.Errorf("invalid IP address: %s", resourceValue)
		}
	case "cidr":
		if !validateCIDR(resourceValue) {
			return fmt.Errorf("invalid CIDR notation: %s", resourceValue)
		}
	case "domain":
		if !validateDomain(resourceValue) {
			return fmt.Errorf("invalid domain name: %s", resourceValue)
		}
	}

	return nil
}

// API Handlers

// GET /api/resources - List all blocked resources
func listBlockedResources(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT id, resource_type, resource_value, description, created_by, created_at, updated_at, is_active 
		FROM blocked_resources 
		ORDER BY created_at DESC
	`)
	if err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Failed to fetch resources: " + err.Error(),
		})
		return
	}
	defer rows.Close()

	var resources []BlockedResource
	for rows.Next() {
		var resource BlockedResource
		err := rows.Scan(&resource.ID, &resource.ResourceType, &resource.ResourceValue,
			&resource.Description, &resource.CreatedBy, &resource.CreatedAt,
			&resource.UpdatedAt, &resource.IsActive)
		if err != nil {
			continue
		}
		resources = append(resources, resource)
	}

	respondJSON(w, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Found %d blocked resources", len(resources)),
		Data:    resources,
	})
}

// POST /api/resources - Add new blocked resource
func addBlockedResource(w http.ResponseWriter, r *http.Request) {
	var req ResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Invalid JSON payload: " + err.Error(),
		})
		return
	}

	// Validate input
	if err := validateResource(req.ResourceType, req.ResourceValue); err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if req.CreatedBy == "" {
		req.CreatedBy = "api-user"
	}

	// Insert into database
	var resourceID int
	err := db.QueryRow(`
		INSERT INTO blocked_resources (resource_type, resource_value, description, created_by, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id
	`, req.ResourceType, req.ResourceValue, req.Description, req.CreatedBy).Scan(&resourceID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			respondJSON(w, APIResponse{
				Success: false,
				Error:   "Resource already exists in blocked list",
			})
		} else {
			respondJSON(w, APIResponse{
				Success: false,
				Error:   "Failed to add resource: " + err.Error(),
			})
		}
		return
	}

	// Log the action
	db.Exec(`
		INSERT INTO resource_audit_log (action_type, resource_type, resource_value, performed_by, client_ip)
		VALUES ('ADD', $1, $2, $3, $4)
	`, req.ResourceType, req.ResourceValue, req.CreatedBy, getClientIP(r))

	// Update network configuration
	updateNetworkConfig()

	respondJSON(w, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully added %s %s to blocked resources", req.ResourceType, req.ResourceValue),
		Data:    map[string]interface{}{"id": resourceID},
	})
}

// DELETE /api/resources/{id} - Remove blocked resource
func removeBlockedResource(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	resourceID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Invalid resource ID",
		})
		return
	}

	// Get resource details before deletion for logging
	var resourceType, resourceValue string
	err = db.QueryRow(`
		SELECT resource_type, resource_value FROM blocked_resources WHERE id = $1
	`, resourceID).Scan(&resourceType, &resourceValue)

	if err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Resource not found",
		})
		return
	}

	// Mark as inactive instead of deleting
	result, err := db.Exec(`
		UPDATE blocked_resources SET is_active = false, updated_at = CURRENT_TIMESTAMP 
		WHERE id = $1
	`, resourceID)

	if err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Failed to remove resource: " + err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Resource not found",
		})
		return
	}

	// Log the action
	createdBy := r.Header.Get("X-User") // Assume user passed in header
	if createdBy == "" {
		createdBy = "api-user"
	}

	db.Exec(`
		INSERT INTO resource_audit_log (action_type, resource_type, resource_value, performed_by, client_ip)
		VALUES ('REMOVE', $1, $2, $3, $4)
	`, resourceType, resourceValue, createdBy, getClientIP(r))

	// Update network configuration
	updateNetworkConfig()

	respondJSON(w, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully removed %s %s from blocked resources", resourceType, resourceValue),
	})
}

// POST /api/resources/bulk - Add multiple resources
func addBulkBlockedResources(w http.ResponseWriter, r *http.Request) {
	var req BulkResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Invalid JSON payload: " + err.Error(),
		})
		return
	}

	if len(req.Resources) == 0 {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "No resources provided",
		})
		return
	}

	if req.CreatedBy == "" {
		req.CreatedBy = "api-user"
	}

	// Validate all resources first
	for i, resource := range req.Resources {
		if err := validateResource(resource.ResourceType, resource.ResourceValue); err != nil {
			respondJSON(w, APIResponse{
				Success: false,
				Error:   fmt.Sprintf("Resource %d validation failed: %s", i+1, err.Error()),
			})
			return
		}
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Failed to start transaction: " + err.Error(),
		})
		return
	}
	defer tx.Rollback()

	addedCount := 0
	var addedResources []string

	for _, resource := range req.Resources {
		var resourceID int
		err := tx.QueryRow(`
			INSERT INTO blocked_resources (resource_type, resource_value, description, created_by, is_active)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (resource_type, resource_value) WHERE is_active = true
			DO NOTHING
			RETURNING id
		`, resource.ResourceType, resource.ResourceValue, resource.Description, req.CreatedBy).Scan(&resourceID)

		if err == nil {
			addedCount++
			addedResources = append(addedResources, fmt.Sprintf("%s:%s", resource.ResourceType, resource.ResourceValue))

			// Log each addition
			tx.Exec(`
				INSERT INTO resource_audit_log (action_type, resource_type, resource_value, performed_by, client_ip)
				VALUES ('BULK_ADD', $1, $2, $3, $4)
			`, resource.ResourceType, resource.ResourceValue, req.CreatedBy, getClientIP(r))
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Failed to commit transaction: " + err.Error(),
		})
		return
	}

	// Update network configuration
	updateNetworkConfig()

	respondJSON(w, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully added %d out of %d resources", addedCount, len(req.Resources)),
		Data:    map[string]interface{}{"added_resources": addedResources},
	})
}

// GET /api/vpc-resources - List VPC resources
func listVPCResources(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT id, resource_id, resource_type, ip_address, domain_name, port_range, 
		       cloud_provider, vpc_id, auto_discovered, last_scan, is_active
		FROM vpc_resources 
		WHERE is_active = true
		ORDER BY last_scan DESC
	`)
	if err != nil {
		respondJSON(w, APIResponse{
			Success: false,
			Error:   "Failed to fetch VPC resources: " + err.Error(),
		})
		return
	}
	defer rows.Close()

	var resources []VPCResource
	for rows.Next() {
		var resource VPCResource
		var ipAddress, domainName, portRange sql.NullString

		err := rows.Scan(&resource.ID, &resource.ResourceID, &resource.ResourceType,
			&ipAddress, &domainName, &portRange, &resource.CloudProvider,
			&resource.VPCID, &resource.AutoDiscovered, &resource.LastScan, &resource.IsActive)
		if err != nil {
			continue
		}

		resource.IPAddress = ipAddress.String
		resource.DomainName = domainName.String
		resource.PortRange = portRange.String
		resources = append(resources, resource)
	}

	respondJSON(w, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Found %d VPC resources", len(resources)),
		Data:    resources,
	})
}

// POST /api/discover-vpc-resources - Trigger VPC resource discovery
func discoverVPCResources(w http.ResponseWriter, r *http.Request) {
	// This would integrate with cloud provider APIs to discover resources
	// For now, return success message
	respondJSON(w, APIResponse{
		Success: true,
		Message: "VPC resource discovery triggered (implementation pending)",
	})
}

// Helper functions
func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return strings.Split(forwarded, ",")[0]
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func updateNetworkConfig() {
	// Update iptables rules
	if err := exec.Command("/opt/dvarpala/scripts/update-iptables-blocking.sh").Run(); err != nil {
		log.Printf("Failed to update iptables: %v", err)
	}

	// Update DNS blocking
	if err := exec.Command("/opt/dvarpala/scripts/update-dns-blocking.sh").Run(); err != nil {
		log.Printf("Failed to update DNS blocking: %v", err)
	}

	log.Println("Network configuration updated successfully")
}

// Middleware for API authentication (basic implementation)
func authenticateAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			respondJSON(w, APIResponse{
				Success: false,
				Error:   "API key required",
			})
			return
		}

		// Validate API key against database
		var isValid bool
		err := db.QueryRow(`
			SELECT COUNT(*) > 0 FROM api_tokens 
			WHERE token_hash = $1 AND is_active = true AND (expires_at IS NULL OR expires_at > NOW())
		`, apiKey).Scan(&isValid)

		if err != nil || !isValid {
			respondJSON(w, APIResponse{
				Success: false,
				Error:   "Invalid or expired API key",
			})
			return
		}

		// Update last used timestamp
		db.Exec("UPDATE api_tokens SET last_used = NOW() WHERE token_hash = $1", apiKey)

		next.ServeHTTP(w, r)
	}
}

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, X-User")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Main function
func main() {
	// Initialize database
	if err := initDB(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Setup routes
	r := mux.NewRouter()
	
	// API routes with authentication
	api := r.PathPrefix("/api").Subrouter()
	api.Use(corsMiddleware)

	// Resource management endpoints
	api.HandleFunc("/resources", authenticateAPI(listBlockedResources)).Methods("GET")
	api.HandleFunc("/resources", authenticateAPI(addBlockedResource)).Methods("POST")
	api.HandleFunc("/resources/{id}", authenticateAPI(removeBlockedResource)).Methods("DELETE")
	api.HandleFunc("/resources/bulk", authenticateAPI(addBulkBlockedResources)).Methods("POST")

	// VPC resource endpoints
	api.HandleFunc("/vpc-resources", authenticateAPI(listVPCResources)).Methods("GET")
	api.HandleFunc("/discover-vpc-resources", authenticateAPI(discoverVPCResources)).Methods("POST")

	// Health check endpoint (no auth required)
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, APIResponse{
			Success: true,
			Message: "Dvarpala Resource Management API is healthy",
		})
	}).Methods("GET")

	// Start server
	port := getEnv("API_PORT", "8081")
	log.Printf("Starting Dvarpala Resource Management API on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}