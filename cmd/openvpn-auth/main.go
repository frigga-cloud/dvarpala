package main

import (
	"os"
)

func main() {
	// OpenVPN authentication script
	// Called by OpenVPN server for user authentication
	
	username := os.Getenv("username")
	password := os.Getenv("password")
	_ = os.Getenv("untrusted_ip") // clientIP - not used in this basic version

	// Simple authentication logic - will be enhanced later
	if username == "temp_user" && password == "temp_portal_access" {
		os.Exit(0) // Success - allow captive portal access
	} else {
		os.Exit(1) // Failure
	}
}
