package providers

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Common interfaces for abstraction
type VPCInfo interface {
	GetID() string
	GetSubnetID() string
	GetSecurityGroupID() string
}

type InstanceInfo interface {
	GetPublicIP() string
	GetPrivateIP() string
	GetInstanceID() string
	GetSSHKeyPath() string
}

// Base cloud provider interface
type CloudProvider interface {
	// Provider-specific methods that must be implemented
	SetupEnvironment() error
	ValidateAuthentication() error
	CreateOrGetVPC(vpcName string, config NetworkConfig) (VPCInfo, error)
	CreateInstance(vpcInfo VPCInfo, config InstanceConfig, vmName string) (InstanceInfo, error)
	CreateStorage(name string) error
	UploadConfiguration(name string, configData []byte, filename string) error

	// Common methods with base implementation
	InstallDvarpalaDirectly(instanceInfo InstanceInfo, config InstanceConfig, cloudProvider string) error
	Configure2StepVPNAccess(instanceInfo InstanceInfo, config InstanceConfig) error
	GetSSHUser() string
}

// Base struct with common functionality
type BaseCloudProvider struct {
	SSHUser string // "ubuntu" or "azureuser"
}

// Note: NetworkConfig and InstanceConfig are defined in the individual provider files

// SSH-related functions
func (base *BaseCloudProvider) GetSSHUser() string {
	return base.SSHUser
}

func (base *BaseCloudProvider) generateSSHKeyPair(keyPath string) error {
	// Generate SSH key pair
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", keyPath, "-N", "")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate SSH key pair: %v", err)
	}

	fmt.Printf("🔑 SSH key pair generated: %s\n", keyPath)
	return nil
}

func (base *BaseCloudProvider) waitForSSHAccess(vmIP, keyPath string) error {
	fmt.Printf("⏳ Waiting for SSH access to %s...\n", vmIP)

	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		// Test SSH connectivity with key
		sshCmd := exec.Command("ssh", "-i", keyPath, "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
			fmt.Sprintf("%s@%s", base.SSHUser, vmIP), "echo 'SSH Ready'")
		if sshCmd.Run() == nil {
			return nil
		}

		fmt.Printf("⏳ SSH not ready yet... attempt %d/%d\n", i+1, maxAttempts)
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("SSH access not available after %d attempts", maxAttempts)
}

func (base *BaseCloudProvider) executeSSHCommand(vmIP, command, keyPath string) error {
	cmd := exec.Command("ssh", "-i", keyPath, "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("%s@%s", base.SSHUser, vmIP), command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Command failed: %s\nOutput: %s\n", command, string(output))
		return err
	}

	return nil
}

// Installation configuration functions
func (base *BaseCloudProvider) getEasyRSAVarsCommand(config InstanceConfig) string {
	return fmt.Sprintf(`tee /home/$(whoami)/dvarpala/easy-rsa/vars > /dev/null << 'EOF'
set_var EASYRSA_REQ_COUNTRY    "US"
set_var EASYRSA_REQ_PROVINCE   "CA"
set_var EASYRSA_REQ_CITY       "San Francisco"
set_var EASYRSA_REQ_ORG        "Frigga Labs"
set_var EASYRSA_REQ_EMAIL      "%s"
set_var EASYRSA_REQ_OU         "Dvarpala VPN"
set_var EASYRSA_KEY_SIZE       2048
set_var EASYRSA_ALGO           rsa
set_var EASYRSA_CA_EXPIRE      3650
set_var EASYRSA_CERT_EXPIRE    365
EOF
echo "🔍 DEBUG: Easy-RSA vars file created" && ls -la /home/$(whoami)/dvarpala/easy-rsa/vars`, config.AdminEmail)
}

func (base *BaseCloudProvider) getOpenVPNServerConfigCommand() string {
	return `sudo tee /etc/openvpn/server/server.conf > /dev/null << 'EOF'
port 1194
proto udp
dev tun
ca ca.crt
cert server.crt
key server.key
dh none
ecdh-curve prime256v1
server 172.30.100.0 255.255.255.0
ifconfig-pool-persist /var/log/openvpn/ipp.txt

# Captive portal configuration - route only to captive portal initially
push "route 172.30.100.1 255.255.255.255"
push "dhcp-option DNS 172.30.100.1"

# Client connection scripts for 2-step authentication  
script-security 2
client-connect /opt/dvarpala/scripts/client-connect.sh
client-disconnect /opt/dvarpala/scripts/client-disconnect.sh

keepalive 10 120
tls-auth ta.key 0
cipher AES-256-GCM
user nobody
group nogroup
persist-key
persist-tun
status /var/log/openvpn/openvpn-status.log
log-append /var/log/openvpn/openvpn.log
verb 3
explicit-exit-notify 1
EOF

# Create OpenVPN client connection scripts for 2-step authentication
sudo mkdir -p /opt/dvarpala/scripts

# Client connect script - restrict to captive portal initially
sudo tee /opt/dvarpala/scripts/client-connect.sh > /dev/null << 'CONNECT_EOF'
#!/bin/bash
# OpenVPN client connect script for 2-step authentication
# Initially route only to captive portal (172.30.100.1:8080)

CLIENT_IP=$1
CLIENT_CN=$2

# Block all internet traffic initially - only allow captive portal
iptables -I FORWARD -s $CLIENT_IP -d 172.30.100.1 -j ACCEPT
iptables -I FORWARD -s $CLIENT_IP -j DROP

# Log connection
echo "$(date): Client $CLIENT_CN ($CLIENT_IP) connected - restricted to captive portal" >> /var/log/openvpn/client-connections.log
CONNECT_EOF

# Client disconnect script
sudo tee /opt/dvarpala/scripts/client-disconnect.sh > /dev/null << 'DISCONNECT_EOF'
#!/bin/bash
# OpenVPN client disconnect script

CLIENT_IP=$1
CLIENT_CN=$2

# Clean up iptables rules for this client
iptables -D FORWARD -s $CLIENT_IP -d 172.30.100.1 -j ACCEPT 2>/dev/null
iptables -D FORWARD -s $CLIENT_IP -j DROP 2>/dev/null
iptables -D FORWARD -s $CLIENT_IP -j ACCEPT 2>/dev/null

# Log disconnection
echo "$(date): Client $CLIENT_CN ($CLIENT_IP) disconnected" >> /var/log/openvpn/client-connections.log
DISCONNECT_EOF

# Make scripts executable
sudo chmod +x /opt/dvarpala/scripts/client-connect.sh
sudo chmod +x /opt/dvarpala/scripts/client-disconnect.sh

echo "✅ OpenVPN 2-step authentication configured"`
}

func (base *BaseCloudProvider) configureNginxMonitoring(vmIP, keyPath string) error {
	// Create the nginx configuration file using a heredoc to avoid quoting issues
	command := `sudo tee /etc/nginx/sites-available/dvarpala-monitoring > /dev/null << 'EOF'
server {
    listen 8080;
    server_name _;
    root /var/www/html;
    
    location /health {
        return 200 '{"status":"healthy","service":"dvarpala"}';
        add_header Content-Type application/json;
    }
    
    location /installation-progress {
        return 200 '{"current_step":"Installation completed","completed_steps":9,"total_steps":9}';
        add_header Content-Type application/json;
    }
    
    location /installation-status {
        return 200 'Installation completed successfully';
        add_header Content-Type text/plain;
    }
}
EOF`

	return base.executeSSHCommand(vmIP, command, keyPath)
}

func (base *BaseCloudProvider) generateAdminOVPN(vmIP, keyPath string) error {
	// Create the admin.ovpn file with real certificates
	command := `
# Check if certificate files exist before proceeding
ls -la /home/$(whoami)/dvarpala/easy-rsa/pki/ca.crt
ls -la /home/$(whoami)/dvarpala/easy-rsa/pki/issued/admin.crt
ls -la /home/$(whoami)/dvarpala/easy-rsa/pki/private/admin.key  
ls -la /home/$(whoami)/dvarpala/easy-rsa/pki/ta.key

wc -l /home/$(whoami)/dvarpala/easy-rsa/pki/ca.crt
wc -l /home/$(whoami)/dvarpala/easy-rsa/pki/issued/admin.crt
wc -l /home/$(whoami)/dvarpala/easy-rsa/pki/private/admin.key
wc -l /home/$(whoami)/dvarpala/easy-rsa/pki/ta.key

# Get the external IP address
EXTERNAL_IP=$(curl -s http://checkip.amazonaws.com)

# Create admin.ovpn with embedded certificates
tee /home/$(whoami)/dvarpala/certs/admin.ovpn > /dev/null << EOF
# Dvarpala VPN - Captive Portal Mode
# Browser will auto-open to: http://172.30.100.1:8080
# Complete authentication via web portal for full VPN access

client
dev tun
proto udp
remote $EXTERNAL_IP 1194
resolv-retry infinite
nobind
persist-key
persist-tun
remote-cert-tls server
cipher AES-256-GCM
verb 3

# Auto-open captive portal after connection
script-security 2
up "echo 'Opening captive portal...' && (open http://172.30.100.1:8080 2>/dev/null || xdg-open http://172.30.100.1:8080 2>/dev/null || start http://172.30.100.1:8080 2>/dev/null || echo 'Please open http://172.30.100.1:8080 manually')"

# Initial captive portal access credentials
# Username: portal, Password: access (for initial connection only)
auth-user-pass

<ca>
$(cat /home/$(whoami)/dvarpala/easy-rsa/pki/ca.crt)
</ca>

<cert>
$(cat /home/$(whoami)/dvarpala/easy-rsa/pki/issued/admin.crt)
</cert>

<key>
$(cat /home/$(whoami)/dvarpala/easy-rsa/pki/private/admin.key)
</key>

<tls-auth>
$(cat /home/$(whoami)/dvarpala/easy-rsa/pki/ta.key)
</tls-auth>
key-direction 1
EOF

# Create credentials file
tee /home/$(whoami)/dvarpala/certs/admin-credentials.txt > /dev/null << EOF
portal
access
EOF

# Set proper permissions
chmod 600 /home/$(whoami)/dvarpala/certs/admin.ovpn
chmod 600 /home/$(whoami)/dvarpala/certs/admin-credentials.txt

ls -la /home/$(whoami)/dvarpala/certs/admin.ovpn
wc -l /home/$(whoami)/dvarpala/certs/admin.ovpn
head -10 /home/$(whoami)/dvarpala/certs/admin.ovpn
tail -10 /home/$(whoami)/dvarpala/certs/admin.ovpn
`

	return base.executeSSHCommand(vmIP, command, keyPath)
}

func (base *BaseCloudProvider) setupPostgreSQLDatabase(vmIP, keyPath string, config InstanceConfig) error {
	fmt.Println("🗄️ Setting up PostgreSQL database...")

	// First, install PostgreSQL and create the dvarpala user via SSH
	setupCommands := []string{
		"sudo -u postgres createuser -s dvarpala 2>/dev/null || echo 'User already exists'",
		"sudo -u postgres psql -c \"ALTER USER dvarpala WITH PASSWORD 'dvarpala123';\"",
	}

	for _, cmd := range setupCommands {
		if err := base.executeSSHCommand(vmIP, cmd, keyPath); err != nil {
			return fmt.Errorf("failed to setup PostgreSQL user: %v", err)
		}
	}

	// Download schema files to VM
	schemaCommands := []string{
		"mkdir -p /home/$(whoami)/dvarpala/schema/migrations",
		"mkdir -p /home/$(whoami)/dvarpala/schema/seeds",
		"curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/main/installation/installer/schema/migrations/001_create_users_table.sql -o /home/$(whoami)/dvarpala/schema/migrations/001_create_users_table.sql",
		"curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/main/installation/installer/schema/migrations/002_create_groups_table.sql -o /home/$(whoami)/dvarpala/schema/migrations/002_create_groups_table.sql",
		"curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/main/installation/installer/schema/migrations/003_create_sessions_table.sql -o /home/$(whoami)/dvarpala/schema/migrations/003_create_sessions_table.sql",
		"curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/main/installation/installer/schema/seeds/001_default_admin_user.sql -o /home/$(whoami)/dvarpala/schema/seeds/001_default_admin_user.sql",
	}

	for _, cmd := range schemaCommands {
		if err := base.executeSSHCommand(vmIP, cmd, keyPath); err != nil {
			return fmt.Errorf("failed to download schema files: %v", err)
		}
	}

	// Create a Go program on the VM to handle database setup
	dbSetupProgram := fmt.Sprintf(`package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	_ "github.com/lib/pq"
)

func main() {
	// Connect to PostgreSQL
	connStr := "host=localhost port=5432 user=dvarpala password=dvarpala123 dbname=postgres sslmode=disable"
	postgresDB, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Printf("Failed to connect to postgres: %%v\n", err)
		os.Exit(1)
	}
	defer postgresDB.Close()

	// Create dvarpala database
	if _, err := postgresDB.Exec("CREATE DATABASE dvarpala"); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			fmt.Printf("Failed to create database: %%v\n", err)
			os.Exit(1)
		}
	}
	postgresDB.Close()

	// Connect to dvarpala database
	connStr = "host=localhost port=5432 user=dvarpala password=dvarpala123 dbname=dvarpala sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Printf("Failed to connect to dvarpala database: %%v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Execute migration files
	migrationsPath := "/home/$(whoami)/dvarpala/schema/migrations"
	if err := runMigrations(db, migrationsPath); err != nil {
		fmt.Printf("Migration failed: %%v\n", err)
		os.Exit(1)
	}

	// Execute seed files
	seedsPath := "/home/$(whoami)/dvarpala/schema/seeds"
	if err := runSeeds(db, seedsPath); err != nil {
		fmt.Printf("Seeds failed: %%v\n", err)
		os.Exit(1)
	}

	// Create admin user
	if err := createAdminUser(db, "%s", "%s"); err != nil {
		fmt.Printf("Failed to create admin user: %%v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Database setup completed successfully")
}

func runMigrations(db *sql.DB, migrationsPath string) error {
	files, err := getSQLFiles(migrationsPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := executeSQLFile(db, file); err != nil {
			return fmt.Errorf("migration failed at %%s: %%v", filepath.Base(file), err)
		}
		fmt.Printf("✅ Executed migration: %%s\n", filepath.Base(file))
	}
	return nil
}

func runSeeds(db *sql.DB, seedsPath string) error {
	files, err := getSQLFiles(seedsPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := executeSQLFile(db, file); err != nil {
			return fmt.Errorf("seed failed at %%s: %%v", filepath.Base(file), err)
		}
		fmt.Printf("✅ Executed seed: %%s\n", filepath.Base(file))
	}
	return nil
}

func getSQLFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func executeSQLFile(db *sql.DB, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	sqlContent := string(content)
	statements := strings.Split(sqlContent, ";")
	
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute statement: %%v\nStatement: %%s", err, stmt)
		}
	}
	return nil
}

func createAdminUser(db *sql.DB, email, name string) error {
	userSQL := ` + "`" + `INSERT INTO users (email, full_name, is_admin, is_active) 
		VALUES ($1, $2, true, true) 
		ON CONFLICT (email) DO UPDATE SET
			full_name = EXCLUDED.full_name,
			is_admin = true,
			is_active = true,
			updated_at = CURRENT_TIMESTAMP` + "`" + `

	if _, err := db.Exec(userSQL, email, name); err != nil {
		return err
	}

	groupSQL := ` + "`" + `INSERT INTO user_groups (user_id, group_id, assigned_by)
		SELECT u.id, g.id, u.id
		FROM users u, groups g
		WHERE u.email = $1 AND g.name = 'administrators'
		ON CONFLICT (user_id, group_id) DO NOTHING` + "`" + `

	if _, err := db.Exec(groupSQL, email); err != nil {
		return err
	}

	fmt.Printf("✅ Admin user created: %%s\n", email)
	return nil
}`, config.AdminEmail, config.AdminName)

	// Write the Go program to VM
	writeProgram := fmt.Sprintf("cat > /home/$(whoami)/dvarpala/db_setup.go << 'EOF'\n%s\nEOF", dbSetupProgram)
	if err := base.executeSSHCommand(vmIP, writeProgram, keyPath); err != nil {
		return fmt.Errorf("failed to write database setup program: %v", err)
	}

	// Install Go PostgreSQL driver and run the program
	setupDBCommands := []string{
		"cd /home/$(whoami)/dvarpala && go mod init dvarpala-db-setup",
		"cd /home/$(whoami)/dvarpala && go get github.com/lib/pq",
		"cd /home/$(whoami)/dvarpala && go run db_setup.go",
		"rm /home/$(whoami)/dvarpala/db_setup.go /home/$(whoami)/dvarpala/go.mod /home/$(whoami)/dvarpala/go.sum",
	}

	for _, cmd := range setupDBCommands {
		if err := base.executeSSHCommand(vmIP, cmd, keyPath); err != nil {
			return fmt.Errorf("failed to setup database: %v", err)
		}
	}

	return nil
}

func (base *BaseCloudProvider) getDvarpalaDeploymentCommand() string {
	return `
# Clone and build Dvarpala application
echo "📦 Downloading Dvarpala application..."
cd /home/$(whoami)/dvarpala
git clone https://github.com/friggalabs/dvarpala.git app
cd app

# Build the application
export PATH=$PATH:/usr/local/go/bin
go mod tidy
go build -o dvarpala ./cmd/server

# Create systemd service
sudo tee /etc/systemd/system/dvarpala.service > /dev/null << 'SERVICE_EOF'
[Unit]
Description=Dvarpala VPN Captive Portal
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/dvarpala/app
ExecStart=/home/ubuntu/dvarpala/app/dvarpala
Restart=always
Environment=PATH=/usr/local/go/bin:/usr/bin:/bin
Environment=DVARPALA_DB_HOST=localhost
Environment=DVARPALA_DB_PORT=5432
Environment=DVARPALA_DB_NAME=dvarpala
Environment=DVARPALA_DB_USER=dvarpala
Environment=DVARPALA_DB_PASSWORD=dvarpala123
Environment=DVARPALA_REDIS_HOST=localhost
Environment=DVARPALA_REDIS_PORT=6379

[Install]
WantedBy=multi-user.target
SERVICE_EOF

# Enable and start the service
sudo systemctl daemon-reload
sudo systemctl enable dvarpala
echo "✅ Dvarpala application deployed successfully"
`[1:]
}

func (base *BaseCloudProvider) getCaptivePortalNginxCommand() string {
	return `
# Configure nginx as reverse proxy for Dvarpala captive portal
sudo tee /etc/nginx/sites-available/dvarpala-captive > /dev/null << 'NGINX_EOF'
server {
    listen 8080;
    server_name _;
    
    # Main captive portal location
    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    # Health check endpoint
    location /health {
        return 200 '{"status":"healthy","service":"dvarpala"}';
        add_header Content-Type application/json;
    }
    
    # Installation status endpoints  
    location /installation-progress {
        return 200 '{"current_step":"Installation completed","completed_steps":20,"total_steps":20}';
        add_header Content-Type application/json;
    }
    
    location /installation-status {
        return 200 'Installation completed successfully';
        add_header Content-Type text/plain;
    }
}
NGINX_EOF

# Enable the captive portal site
sudo ln -sf /etc/nginx/sites-available/dvarpala-captive /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
echo "✅ Captive portal nginx configuration completed"
`[1:]
}

func (base *BaseCloudProvider) uploadAdminOVPNToBucket(vmIP, keyPath, cloudProvider string, config InstanceConfig) error {
	// Upload admin.ovpn to cloud storage bucket
	bucketPath := fmt.Sprintf("dvarpala/installations/%s/admin.ovpn", config.AdminEmail)
	
	// Download from VM and upload to bucket using cloud CLI
	var uploadCmd string
	switch cloudProvider {
	case "aws":
		uploadCmd = fmt.Sprintf("curl -s http://localhost/admin.ovpn | aws s3 cp - s3://friggalabs/%s", bucketPath)
	case "gcp":
		uploadCmd = fmt.Sprintf("curl -s http://localhost/admin.ovpn | gsutil cp - gs://friggalabs/%s", bucketPath)
	case "azure":
		uploadCmd = fmt.Sprintf("curl -s http://localhost/admin.ovpn | az storage blob upload --account-name friggalabs --container-name dvarpala --name %s --file -", bucketPath)
	default:
		return fmt.Errorf("unsupported cloud provider for bucket upload: %s", cloudProvider)
	}
	
	return base.executeSSHCommand(vmIP, uploadCmd, keyPath)
}

func (base *BaseCloudProvider) generateMinimalStartupScript(config InstanceConfig, cloudProvider string) string {
	return fmt.Sprintf(`#!/bin/bash
# Minimal %s Instance Setup Script - Just basic system prep
set -euo pipefail

# Logging
exec > >(tee /var/log/dvarpala-startup.log)
exec 2>&1

echo "Starting minimal system setup at $(date)"

# Update system packages
apt-get update -y

# Install essential dependencies only
apt-get install -y curl wget openssh-server

# Ensure SSH is running for installer to connect
systemctl enable ssh
systemctl start ssh

# Set environment variables for later use
export ADMIN_EMAIL='%s'
export ADMIN_NAME='%s'
export CLOUD_PROVIDER='%s'

# Create marker that basic setup is complete
mkdir -p /var/log/dvarpala
touch /var/log/dvarpala/startup-complete
echo "VM startup preparation completed at $(date)" > /var/log/dvarpala/startup-status.txt

echo "Minimal setup completed. Ready for installer connection."
`, strings.ToUpper(cloudProvider), config.AdminEmail, config.AdminName, cloudProvider)
}

// Main installation function - common across all providers
func (base *BaseCloudProvider) InstallDvarpalaDirectly(instanceInfo InstanceInfo, config InstanceConfig, cloudProvider string) error {
	vmIP := instanceInfo.GetPublicIP()
	keyPath := instanceInfo.GetSSHKeyPath()

	fmt.Printf("🔗 Connecting to VM for direct installation...\n")
	fmt.Printf("🔑 Using SSH key: %s\n", keyPath)

	// Wait for VM to be SSH accessible
	if err := base.waitForSSHAccess(vmIP, keyPath); err != nil {
		return fmt.Errorf("failed to establish SSH connection: %v", err)
	}

	fmt.Println("✅ SSH connection established")

	// STEP 4: SSH and install dependencies
	fmt.Println("\n📦 Step 4: Installing dependencies...")
	dependencies := []struct {
		name string
		cmd  string
	}{
		{"nginx", "sudo apt-get install -y nginx"},
		{"PostgreSQL", "sudo apt-get install -y postgresql postgresql-contrib"},
		{"Redis", "sudo apt-get install -y redis-server"},
		{"OpenVPN", "sudo apt-get install -y openvpn easy-rsa"},
		{"Go", "curl -fsSL https://go.dev/dl/go1.21.0.linux-amd64.tar.gz | sudo tar -C /usr/local -xzf -"},
		{"Git", "sudo apt-get install -y git"},
		{"directories", "mkdir -p /home/$(whoami)/dvarpala /home/$(whoami)/dvarpala/certs && sudo mkdir -p /var/lib/dvarpala /opt/dvarpala/scripts"},
		{"services", "sudo systemctl enable nginx postgresql redis-server && sudo systemctl start nginx postgresql redis-server"},
	}
	
	for i, dep := range dependencies {
		fmt.Printf("  [%d/%d] Installing %s...\n", i+1, len(dependencies), dep.name)
		if err := base.executeSSHCommand(vmIP, dep.cmd, keyPath); err != nil {
			return fmt.Errorf("failed to install %s: %v", dep.name, err)
		}
	}
	fmt.Println("✅ Dependencies installed successfully")

	// STEP 5: Create admin.ovpn
	fmt.Println("\n📄 Step 5: Creating admin.ovpn...")
	certSteps := []struct {
		name string
		cmd  string
	}{
		{"Setting up Easy-RSA", "make-cadir /home/$(whoami)/dvarpala/easy-rsa"},
		{"Configuring Easy-RSA vars", base.getEasyRSAVarsCommand(config)},
		{"Building Certificate Authority", "cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa init-pki && ./easyrsa --batch build-ca nopass"},
		{"Generating server certificate", "cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa --batch build-server-full server nopass"},
		{"Generating admin client certificate", "cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa --batch build-client-full admin nopass"},
		{"Generating TLS auth key", "cd /home/$(whoami)/dvarpala/easy-rsa && openvpn --genkey --secret pki/ta.key"},
		{"Copying certificates", "sudo cp /home/$(whoami)/dvarpala/easy-rsa/pki/ca.crt /home/$(whoami)/dvarpala/easy-rsa/pki/issued/server.crt /home/$(whoami)/dvarpala/easy-rsa/pki/private/server.key /home/$(whoami)/dvarpala/easy-rsa/pki/ta.key /etc/openvpn/server/"},
	}
	
	for i, step := range certSteps {
		fmt.Printf("  [%d/%d] %s...\n", i+1, len(certSteps), step.name)
		if err := base.executeSSHCommand(vmIP, step.cmd, keyPath); err != nil {
			return fmt.Errorf("failed at %s: %v", step.name, err)
		}
	}
	
	// Generate the actual admin.ovpn file
	if err := base.generateAdminOVPN(vmIP, keyPath); err != nil {
		return fmt.Errorf("failed to generate admin.ovpn: %v", err)
	}
	fmt.Println("✅ admin.ovpn created successfully")

	// STEP 6: Copy admin.ovpn to bucket
	fmt.Println("\n☁️ Step 6: Uploading admin.ovpn to cloud storage...")
	if err := base.uploadAdminOVPNToBucket(vmIP, keyPath, cloudProvider, config); err != nil {
		fmt.Printf("⚠️ Failed to upload admin.ovpn to bucket: %v\n", err)
	} else {
		fmt.Printf("✅ admin.ovpn uploaded to cloud storage\n")
	}

	// STEP 7: Initialize database with tables
	fmt.Println("\n🗄️ Step 7: Initializing database with tables...")
	if err := base.setupPostgreSQLDatabase(vmIP, keyPath, config); err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	fmt.Println("✅ Database initialized with tables")

	// STEP 8: Create first admin user in database  
	fmt.Println("\n👤 Step 8: Creating first admin user...")
	// Admin user creation is handled in the setupPostgreSQLDatabase method above
	fmt.Printf("✅ Admin user created: %s\n", config.AdminEmail)

	// STEP 9: Host captive portal code via nginx
	fmt.Println("\n🌐 Step 9: Deploying captive portal application...")
	if err := base.executeSSHCommand(vmIP, base.getDvarpalaDeploymentCommand(), keyPath); err != nil {
		return fmt.Errorf("failed to deploy captive portal: %v", err)
	}
	
	// Start the Dvarpala service
	if err := base.executeSSHCommand(vmIP, "sudo systemctl start dvarpala && sudo systemctl status dvarpala --no-pager", keyPath); err != nil {
		return fmt.Errorf("failed to start Dvarpala service: %v", err)
	}
	fmt.Println("✅ Captive portal application deployed and running")

	// STEP 10: Rebuild nginx sites-available file
	fmt.Println("\n🔧 Step 10: Configuring nginx for captive portal...")
	if err := base.executeSSHCommand(vmIP, base.getCaptivePortalNginxCommand(), keyPath); err != nil {
		return fmt.Errorf("failed to configure nginx: %v", err)
	}
	fmt.Println("✅ Nginx configured for captive portal")

	// STEP 11: Making admin.ovpn available for download (moved before iptables config)
	fmt.Println("\n📥 Step 11: Preparing admin.ovpn for download...")
	if err := base.executeSSHCommand(vmIP, "sudo cp /home/$(whoami)/dvarpala/certs/admin.ovpn /var/www/html/admin.ovpn && sudo chmod 644 /var/www/html/admin.ovpn", keyPath); err != nil {
		return fmt.Errorf("failed to make admin.ovpn downloadable: %v", err)
	}
	
	// Schedule cleanup after 2 minutes
	cleanupCommand := "sleep 120 && sudo rm -f /var/www/html/admin.ovpn"
	if err := base.executeSSHCommand(vmIP, fmt.Sprintf("nohup bash -c '%s' > /dev/null 2>&1 &", cleanupCommand), keyPath); err != nil {
		return fmt.Errorf("failed to schedule admin.ovpn cleanup: %v", err)
	}
	fmt.Println("✅ admin.ovpn ready for download (will auto-delete in 2 minutes)")

	// Return here - Steps 12-13 handled by installer.go
	fmt.Println("\n🎉 VM setup completed!")
	fmt.Println("📋 Installer will now download admin.ovpn and verify installation")
	fmt.Println("⚠️ IMPORTANT: 2-step VPN configuration will be applied AFTER download completes")
	
	return nil
}

// Configure2StepVPNAccess configures OpenVPN and iptables for 2-step authentication
// This is called AFTER admin.ovpn has been downloaded to avoid breaking the connection
func (base *BaseCloudProvider) Configure2StepVPNAccess(instanceInfo InstanceInfo, config InstanceConfig) error {
	vmIP := instanceInfo.GetPublicIP()
	keyPath := instanceInfo.GetSSHKeyPath()

	fmt.Println("\n🔒 Step 14: Configuring 2-step VPN access...")
	fmt.Println("⚠️ WARNING: This will restrict VPN access to captive portal only")

	// Configure OpenVPN for 2-step access
	if err := base.executeSSHCommand(vmIP, base.getOpenVPNServerConfigCommand(), keyPath); err != nil {
		return fmt.Errorf("failed to configure OpenVPN: %v", err)
	}
	
	// Restart OpenVPN server with new configuration
	if err := base.executeSSHCommand(vmIP, "sudo systemctl restart openvpn-server@server && sudo systemctl status openvpn-server@server --no-pager", keyPath); err != nil {
		return fmt.Errorf("failed to restart OpenVPN server: %v", err)
	}
	
	// Configure OAuth firewall rules
	if err := base.executeSSHCommand(vmIP, "sudo bash -c 'curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/main/scripts/installation/configure-oauth-firewall.sh | bash'", keyPath); err != nil {
		fmt.Printf("⚠️ Failed to configure OAuth firewall rules: %v\n", err)
	}
	
	fmt.Println("✅ 2-step VPN access configured successfully")
	fmt.Println("🔐 VPN users will now be redirected to captive portal for authentication")
	
	return nil
}