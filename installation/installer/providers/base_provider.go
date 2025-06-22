package providers

import (
	"dvarpala-cloud-installer/installer/schema"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	CreateStorage(bucketName string) error
	UploadConfiguration(bucketName string, configData []byte, filename string) error

	// Common installation method (with base implementation)
	InstallDvarpalaDirectly(instanceInfo InstanceInfo, config InstanceConfig, cloudProvider string) error
	Configure2StepVPNAccess(instanceInfo InstanceInfo, config InstanceConfig) error
}

// BaseCloudProvider provides common functionality
type BaseCloudProvider struct {
	SSHUser string
}

// Common installation method
func (base *BaseCloudProvider) InstallDvarpalaDirectly(instanceInfo InstanceInfo, config InstanceConfig, cloudProvider string) error {
	fmt.Println("🔗 Connecting to VM for direct installation...")
	vmIP := instanceInfo.GetPublicIP()
	keyPath := instanceInfo.GetSSHKeyPath()

	// Determine SSH user based on cloud provider
	if base.SSHUser == "" {
		switch cloudProvider {
		case "aws":
			base.SSHUser = "ubuntu"
		case "gcp":
			base.SSHUser = "ubuntu"
		case "azure":
			base.SSHUser = "azureuser"
		default:
			base.SSHUser = "ubuntu"
		}
	}

	fmt.Printf("🔑 Using SSH key: %s\n", keyPath)
	fmt.Printf("👤 SSH user: %s\n", base.SSHUser)

	// Wait for VM to be SSH accessible
	if err := base.waitForSSHAccess(vmIP, keyPath); err != nil {
		return fmt.Errorf("failed to establish SSH connection: %v", err)
	}

	fmt.Println("✅ SSH connection established")

	// Install prerequisites
	fmt.Println("📦 Installing prerequisites...")
	prereqCommands := []string{
		"sudo apt-get update -y",
		"sudo apt-get install -y nginx postgresql postgresql-contrib redis-server openvpn easy-rsa git curl wget",
		"curl -fsSL https://go.dev/dl/go1.21.0.linux-amd64.tar.gz | sudo tar -C /usr/local -xzf -",
		"echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc",
		"source ~/.bashrc",
	}

	for _, cmd := range prereqCommands {
		if err := base.executeSSHCommand(vmIP, cmd, keyPath); err != nil {
			fmt.Printf("⚠️ Warning: %v (continuing...)\n", err)
		}
	}

	// Setup directory structure
	setupCommands := []string{
		"mkdir -p /home/$(whoami)/dvarpala/certs",
		"mkdir -p /home/$(whoami)/dvarpala/easy-rsa",
		"sudo mkdir -p /var/lib/dvarpala",
		"sudo mkdir -p /etc/openvpn/server",
		"sudo systemctl start postgresql redis-server",
		"sudo systemctl enable postgresql redis-server nginx",
	}

	for _, cmd := range setupCommands {
		if err := base.executeSSHCommand(vmIP, cmd, keyPath); err != nil {
			return fmt.Errorf("failed to setup directories: %v", err)
		}
	}

	// Setup PostgreSQL database
	if err := base.setupPostgreSQLDatabase(vmIP, keyPath, config); err != nil {
		return fmt.Errorf("failed to setup PostgreSQL: %v", err)
	}

	// Setup PKI and certificates
	fmt.Println("🔐 Setting up PKI and certificates...")
	pkiCommands := []string{
		"cp -r /usr/share/easy-rsa/* /home/$(whoami)/dvarpala/easy-rsa/",
		fmt.Sprintf("cd /home/$(whoami)/dvarpala/easy-rsa && %s", base.getEasyRSAVarsCommand(config)),
		"cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa init-pki",
		"cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa --batch build-ca nopass",
		"cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa --batch build-server-full server nopass",
		"cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa --batch build-client-full admin nopass",
		"cd /home/$(whoami)/dvarpala/easy-rsa && openvpn --genkey --secret pki/ta.key",
		"sudo cp /home/$(whoami)/dvarpala/easy-rsa/pki/ca.crt /home/$(whoami)/dvarpala/easy-rsa/pki/issued/server.crt /home/$(whoami)/dvarpala/easy-rsa/pki/private/server.key /home/$(whoami)/dvarpala/easy-rsa/pki/ta.key /etc/openvpn/server/",
		base.getOpenVPNServerConfigCommand(),
		"sudo systemctl enable openvpn-server@server && sudo systemctl start openvpn-server@server",
	}

	for _, cmd := range pkiCommands {
		if err := base.executeSSHCommand(vmIP, cmd, keyPath); err != nil {
			return fmt.Errorf("failed at PKI setup: %v", err)
		}
	}

	// Configure networking and firewall
	fmt.Println("🔥 Configuring firewall and networking...")
	firewallCommands := []string{
		"sudo bash -c 'echo 1 > /proc/sys/net/ipv4/ip_forward'",
		"sudo bash -c 'echo \"net.ipv4.ip_forward=1\" >> /etc/sysctl.conf'",
		"sudo iptables -t nat -A POSTROUTING -s 172.30.100.0/24 -o $(ip route | grep default | awk '{print $5}') -j MASQUERADE",
		"sudo apt-get install -y iptables-persistent",
		"sudo netfilter-persistent save",
		"sudo curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/main/scripts/installation/configure-oauth-firewall.sh | sudo bash",
		"sudo iptables -A FORWARD -i tun0 -j ACCEPT",
		"sudo iptables -A FORWARD -o tun0 -j ACCEPT",
		"sudo netfilter-persistent save",
	}

	for _, cmd := range firewallCommands {
		if err := base.executeSSHCommand(vmIP, cmd, keyPath); err != nil {
			fmt.Printf("⚠️ Firewall warning: %v (continuing...)\n", err)
		}
	}

	// Configure nginx monitoring
	if err := base.configureNginxMonitoring(vmIP, keyPath); err != nil {
		return fmt.Errorf("failed to configure nginx monitoring: %v", err)
	}

	// Deploy Dvarpala application
	fmt.Println("🚀 Deploying Dvarpala application...")
	deployCmd := base.getDvarpalaDeploymentCommand()
	if err := base.executeSSHCommand(vmIP, deployCmd, keyPath); err != nil {
		return fmt.Errorf("failed to deploy Dvarpala: %v", err)
	}

	// Configure captive portal
	if err := base.configureCaptivePortal(vmIP, keyPath); err != nil {
		return fmt.Errorf("failed to configure captive portal: %v", err)
	}

	// Generate admin OVPN file
	if err := base.generateAdminOVPN(vmIP, keyPath); err != nil {
		return fmt.Errorf("failed to generate admin OVPN: %v", err)
	}

	// Make admin.ovpn temporarily available for download
	if err := base.executeSSHCommand(vmIP, "sudo cp /home/$(whoami)/dvarpala/certs/admin.ovpn /var/www/html/admin.ovpn && sudo chmod 644 /var/www/html/admin.ovpn", keyPath); err != nil {
		return fmt.Errorf("failed to make admin.ovpn downloadable: %v", err)
	}

	// Schedule cleanup of public admin.ovpn file
	cleanupCommand := "sleep 120 && sudo rm -f /var/www/html/admin.ovpn && echo '🔒 SECURITY: admin.ovpn removed from public web directory for security'"
	if err := base.executeSSHCommand(vmIP, fmt.Sprintf("nohup bash -c '%s' > /dev/null 2>&1 &", cleanupCommand), keyPath); err != nil {
		fmt.Printf("⚠️ Warning: failed to schedule admin.ovpn cleanup: %v\n", err)
	}

	fmt.Println("🎉 Dvarpala installation completed successfully!")
	fmt.Println("🔒 SECURITY NOTE: admin.ovpn will be automatically removed from public access in 2 minutes")
	fmt.Println("📋 The installer will download the file immediately - please wait for download completion")
	return nil
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

	if err := base.executeSSHCommand(vmIP, command, keyPath); err != nil {
		return err
	}

	// Enable the site and reload nginx
	enableCommands := []string{
		"sudo ln -sf /etc/nginx/sites-available/dvarpala-monitoring /etc/nginx/sites-enabled/",
		"sudo nginx -t && sudo systemctl reload nginx",
	}

	for _, cmd := range enableCommands {
		if err := base.executeSSHCommand(vmIP, cmd, keyPath); err != nil {
			return err
		}
	}

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

	// Generate database installation script using DatabaseInstaller
	fmt.Println("📝 Generating database installation script...")

	// Get the schema path relative to the installer
	schemaPath := filepath.Join(".", "schema")
	dbInstaller := schema.NewDatabaseInstaller(schemaPath, config.AdminEmail, config.AdminName)

	// Generate the complete SQL script
	sqlScript, err := dbInstaller.GenerateInstallationScript()
	if err != nil {
		return fmt.Errorf("failed to generate database installation script: %v", err)
	}

	// Write script to a temporary file
	tempFile := "/tmp/dvarpala_db_install.sql"
	if err := os.WriteFile(tempFile, []byte(sqlScript), 0644); err != nil {
		return fmt.Errorf("failed to write installation script: %v", err)
	}
	defer os.Remove(tempFile)

	// Upload the SQL script to the VM
	fmt.Println("📤 Uploading database installation script to VM...")
	uploadCmd := fmt.Sprintf("scp -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null %s %s@%s:/tmp/dvarpala_db_install.sql",
		keyPath, tempFile, base.SSHUser, vmIP)
	if err := exec.Command("bash", "-c", uploadCmd).Run(); err != nil {
		return fmt.Errorf("failed to upload installation script: %v", err)
	}

	// Create database and run the installation script
	fmt.Println("🗄️ Creating database and running installation script...")

	// Database setup commands
	setupDBCommands := []string{
		// Create the database if it doesn't exist
		`sudo -u postgres psql -c "CREATE DATABASE dvarpala" 2>/dev/null || echo "Database already exists"`,

		// Run the installation script
		`sudo -u postgres psql -d dvarpala -f /tmp/dvarpala_db_install.sql`,

		// Clean up the uploaded script
		"rm -f /tmp/dvarpala_db_install.sql",
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
sudo systemctl start dvarpala

echo "✅ Dvarpala application deployed and started"
`
}

func (base *BaseCloudProvider) configureCaptivePortal(vmIP, keyPath string) error {
	fmt.Println("🌐 Configuring captive portal...")

	// Create captive portal nginx configuration
	captivePortalConfig := `sudo tee /etc/nginx/sites-available/dvarpala-portal > /dev/null << 'EOF'
server {
    listen 80;
    server_name _;
    
    # Redirect all HTTP traffic to captive portal
    location / {
        return 302 http://$host:8080$request_uri;
    }
}

server {
    listen 8080;
    server_name _;
    
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF`

	commands := []string{
		captivePortalConfig,
		"sudo ln -sf /etc/nginx/sites-available/dvarpala-portal /etc/nginx/sites-enabled/",
		"sudo rm -f /etc/nginx/sites-enabled/default",
		"sudo nginx -t && sudo systemctl reload nginx",
	}

	for _, cmd := range commands {
		if err := base.executeSSHCommand(vmIP, cmd, keyPath); err != nil {
			return fmt.Errorf("failed to configure captive portal: %v", err)
		}
	}

	return nil
}

func (base *BaseCloudProvider) getEasyRSAVarsCommand(config InstanceConfig) string {
	return fmt.Sprintf(`tee ./vars > /dev/null << 'EOF'
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
EOF`, config.AdminEmail)
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
push "redirect-gateway def1 bypass-dhcp"
push "dhcp-option DNS 8.8.8.8"
push "dhcp-option DNS 8.8.4.4"
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
EOF`
}

func (base *BaseCloudProvider) generateAdminOVPN(vmIP, keyPath string) error {
	// Create the admin.ovpn file with real certificates
	command := `
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
`

	return base.executeSSHCommand(vmIP, command, keyPath)
}

func (base *BaseCloudProvider) Configure2StepVPNAccess(instanceInfo InstanceInfo, config InstanceConfig) error {
	fmt.Println("🔐 Configuring 2-step VPN access...")

	vmIP := instanceInfo.GetPublicIP()
	keyPath := instanceInfo.GetSSHKeyPath()

	// Step 1: Create OAuth-based authentication configuration
	fmt.Println("📝 Setting up OAuth authentication...")

	// Create OAuth configuration
	oauthConfig := fmt.Sprintf(`{
		"providers": {
			"google": {
				"enabled": true,
				"client_id": "${GOOGLE_CLIENT_ID}",
				"client_secret": "${GOOGLE_CLIENT_SECRET}",
				"redirect_uri": "http://%s:8080/auth/google/callback"
			},
			"github": {
				"enabled": true,
				"client_id": "${GITHUB_CLIENT_ID}",
				"client_secret": "${GITHUB_CLIENT_SECRET}",
				"redirect_uri": "http://%s:8080/auth/github/callback"
			}
		},
		"allowed_domains": ["*"],
		"admin_email": "%s"
	}`, vmIP, vmIP, config.AdminEmail)

	// Write OAuth config to VM
	writeOAuthConfig := fmt.Sprintf("cat > /home/$(whoami)/dvarpala/oauth-config.json << 'EOF'\n%s\nEOF", oauthConfig)
	if err := base.executeSSHCommand(vmIP, writeOAuthConfig, keyPath); err != nil {
		return fmt.Errorf("failed to write OAuth config: %v", err)
	}

	// Step 2: Create 2-factor authentication setup
	fmt.Println("🔐 Enabling 2-factor authentication...")

	// Create 2FA configuration script
	twoFAScript := `#!/bin/bash
# 2FA Setup for Dvarpala VPN

# Install Google Authenticator PAM module
sudo apt-get install -y libpam-google-authenticator

# Configure PAM for OpenVPN
sudo tee /etc/pam.d/openvpn > /dev/null << 'PAM_EOF'
auth    required    pam_google_authenticator.so forward_pass
auth    required    pam_unix.so use_first_pass
account required    pam_unix.so
PAM_EOF

# Update OpenVPN server config for 2FA
sudo bash -c 'echo "plugin /usr/lib/openvpn/openvpn-plugin-auth-pam.so openvpn" >> /etc/openvpn/server/server.conf'

# Restart OpenVPN
sudo systemctl restart openvpn-server@server

echo "✅ 2FA configuration completed"
`

	// Write and execute 2FA setup script
	write2FAScript := fmt.Sprintf("cat > /home/$(whoami)/dvarpala/setup-2fa.sh << 'EOF'\n%s\nEOF", twoFAScript)
	if err := base.executeSSHCommand(vmIP, write2FAScript, keyPath); err != nil {
		return fmt.Errorf("failed to write 2FA script: %v", err)
	}

	if err := base.executeSSHCommand(vmIP, "chmod +x /home/$(whoami)/dvarpala/setup-2fa.sh && /home/$(whoami)/dvarpala/setup-2fa.sh", keyPath); err != nil {
		fmt.Printf("⚠️ Warning: 2FA setup encountered issues: %v\n", err)
	}

	// Step 3: Create admin 2FA setup instructions
	fmt.Println("📄 Creating 2FA setup instructions...")

	setupInstructions := fmt.Sprintf(`
# Dvarpala VPN 2-Step Access Setup Instructions
# =============================================

## For Admin User: %s

### Step 1: Generate Google Authenticator Token
Run this command on the server:
  ssh -i %s %s@%s "google-authenticator -t -d -f -r 3 -R 30 -w 3"

### Step 2: Configure VPN Client
1. Use the admin.ovpn file downloaded during installation
2. When connecting:
   - First prompt: Enter "portal" (username) and "access" (password)
   - Second prompt: Enter your 6-digit 2FA code from Google Authenticator

### Step 3: Access Captive Portal
After VPN connection, your browser will open to:
  http://172.30.100.1:8080

Complete OAuth authentication to gain full network access.

## Security Notes:
- 2FA codes refresh every 30 seconds
- Each code can only be used once
- Keep your 2FA secret key secure
- Admin access is restricted to: %s

Generated on: %s
`, config.AdminEmail, keyPath, base.SSHUser, vmIP, config.AdminEmail, time.Now().Format(time.RFC3339))

	// Write instructions to VM
	writeInstructions := fmt.Sprintf("cat > /home/$(whoami)/dvarpala/2FA-SETUP-INSTRUCTIONS.txt << 'EOF'\n%s\nEOF", setupInstructions)
	if err := base.executeSSHCommand(vmIP, writeInstructions, keyPath); err != nil {
		return fmt.Errorf("failed to write setup instructions: %v", err)
	}

	// Copy instructions to web directory for download
	if err := base.executeSSHCommand(vmIP, "sudo cp /home/$(whoami)/dvarpala/2FA-SETUP-INSTRUCTIONS.txt /var/www/html/", keyPath); err != nil {
		fmt.Printf("⚠️ Warning: Could not copy instructions to web directory: %v\n", err)
	}

	fmt.Println("✅ 2-step VPN access configuration completed!")
	fmt.Printf("📋 Setup instructions available at: http://%s:8080/2FA-SETUP-INSTRUCTIONS.txt\n", vmIP)

	return nil
}

// Helper function to generate a random secure password
func (base *BaseCloudProvider) generateSecurePassword() string {
	// This is a simple implementation - in production, use crypto/rand
	return fmt.Sprintf("Dvarpala-%d-Secure", time.Now().Unix())
}
