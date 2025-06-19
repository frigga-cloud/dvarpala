package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type GCPProvider struct {
	ProjectID   string
	Region      string
	Zone        string
	Credentials GCPCredentials
}

type GCPCredentials struct {
	ServiceAccountKey string `json:"service_account_key"`
}

type GCPVPCInfo struct {
	VPCName      string
	SubnetName   string
	FirewallRule string
	ExternalIP   string
}

type GCPInstanceInfo struct {
	InstanceName string
	Zone         string
	ExternalIP   string
	InternalIP   string
}

func NewGCPProvider(projectID, region string, creds GCPCredentials) *GCPProvider {
	zone := region + "-a" // Default to zone 'a'
	return &GCPProvider{
		ProjectID:   projectID,
		Region:      region,
		Zone:        zone,
		Credentials: creds,
	}
}

func (gcp *GCPProvider) SetupEnvironment() error {
	if gcp.Credentials.ServiceAccountKey != "" {
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", gcp.Credentials.ServiceAccountKey)
	}

	// Set project
	cmd := exec.Command("gcloud", "config", "set", "project", gcp.ProjectID)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set GCP project: %v", err)
	}

	return nil
}

func (gcp *GCPProvider) ValidateAuthentication() error {
	cmd := exec.Command("gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("GCP authentication failed: %v", err)
	}

	var accounts []map[string]interface{}
	if err := json.Unmarshal(output, &accounts); err != nil {
		return fmt.Errorf("failed to parse GCP auth response: %v", err)
	}

	if len(accounts) == 0 {
		return fmt.Errorf("no active GCP authentication found")
	}

	fmt.Printf("✅ Authenticated as: %s\n", accounts[0]["account"])
	return nil
}

func (gcp *GCPProvider) CreateOrGetVPC(vpcName string, config NetworkConfig) (*GCPVPCInfo, error) {
	// Check if VPC already exists
	existingVPC, err := gcp.findVPCByName(vpcName)
	if err != nil {
		return nil, err
	}

	if existingVPC != nil {
		fmt.Printf("✅ Using existing VPC: %s\n", existingVPC.VPCName)
		return existingVPC, nil
	}

	fmt.Printf("🏗️ Creating new VPC: %s\n", vpcName)
	return gcp.createNewVPC(vpcName, config)
}

func (gcp *GCPProvider) findVPCByName(vpcName string) (*GCPVPCInfo, error) {
	cmd := exec.Command("gcloud", "compute", "networks", "describe", vpcName,
		"--format=json")

	output, err := cmd.Output()
	if err != nil {
		// VPC doesn't exist
		return nil, nil
	}

	var network map[string]interface{}
	if err := json.Unmarshal(output, &network); err != nil {
		return nil, err
	}

	// Get subnet details
	subnetName := vpcName + "-subnet"
	vpcInfo := &GCPVPCInfo{
		VPCName:    vpcName,
		SubnetName: subnetName,
	}

	// Check for existing firewall rule
	cmd = exec.Command("gcloud", "compute", "firewall-rules", "describe", vpcName+"-allow-dvarpala",
		"--format=json")
	if cmd.Run() == nil {
		vpcInfo.FirewallRule = vpcName + "-allow-dvarpala"
	}

	return vpcInfo, nil
}

func (gcp *GCPProvider) createNewVPC(vpcName string, config NetworkConfig) (*GCPVPCInfo, error) {
	vpcInfo := &GCPVPCInfo{
		VPCName:    vpcName,
		SubnetName: vpcName + "-subnet",
	}

	// Create VPC network
	cmd := exec.Command("gcloud", "compute", "networks", "create", vpcName,
		"--subnet-mode=custom",
		"--description=Frigga Labs VPC for dvarpala")

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create VPC: %v", err)
	}

	// Create subnet
	cmd = exec.Command("gcloud", "compute", "networks", "subnets", "create", vpcInfo.SubnetName,
		"--network", vpcName,
		"--range", config.PublicSubnetCidr,
		"--region", gcp.Region,
		"--description=Subnet for dvarpala instances")

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create subnet: %v", err)
	}

	// Create firewall rules
	firewallName := vpcName + "-allow-dvarpala"
	cmd = exec.Command("gcloud", "compute", "firewall-rules", "create", firewallName,
		"--network", vpcName,
		"--allow", "tcp:22,tcp:8080,tcp:443,udp:1194",
		"--source-ranges", "0.0.0.0/0",
		"--description=Allow dvarpala VPN and web access")

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create firewall rules: %v", err)
	}
	vpcInfo.FirewallRule = firewallName

	fmt.Printf("✅ VPC created successfully: %s\n", vpcName)
	return vpcInfo, nil
}

func (gcp *GCPProvider) CreateInstance(vpcInfo *GCPVPCInfo, config InstanceConfig, vmName string) (*GCPInstanceInfo, error) {
	// Use the provided VM name with Frigga Labs naming convention
	instanceName := vmName

	// Create deployment directory if it doesn't exist
	deploymentDir := "./dvarpala-deployment"
	if err := os.MkdirAll(deploymentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create deployment directory: %v", err)
	}

	// Generate SSH key pair for this instance
	sshKeyPath := fmt.Sprintf("%s/%s-key", deploymentDir, instanceName)
	if err := gcp.generateSSHKeyPair(sshKeyPath); err != nil {
		return nil, fmt.Errorf("failed to generate SSH key pair: %v", err)
	}

	// Read public key for instance metadata
	pubKeyData, err := os.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %v", err)
	}
	pubKey := strings.TrimSpace(string(pubKeyData))

	// Get latest Ubuntu image
	imageFamily := "ubuntu-2204-lts"
	imageProject := "ubuntu-os-cloud"

	// Test gcloud authentication and project setup
	fmt.Printf("🔍 Testing gcloud configuration...\n")
	testCmd := exec.Command("gcloud", "config", "get-value", "project")
	if projectOutput, err := testCmd.Output(); err != nil {
		return nil, fmt.Errorf("gcloud configuration error: %v", err)
	} else {
		fmt.Printf("✅ GCP project: %s\n", strings.TrimSpace(string(projectOutput)))
	}

	// Generate minimal startup script - just basic system prep
	startupScript := gcp.generateMinimalStartupScript(config)

	// Create instance with SSH key - combine metadata into single flag
	metadata := fmt.Sprintf("startup-script=%s,ssh-keys=ubuntu:%s", startupScript, pubKey)
	cmd := exec.Command("gcloud", "compute", "instances", "create", instanceName,
		"--zone", gcp.Zone,
		"--machine-type", config.InstanceType,
		"--network-interface", fmt.Sprintf("subnet=%s,address=", vpcInfo.SubnetName),
		"--image-family", imageFamily,
		"--image-project", imageProject,
		"--boot-disk-size", fmt.Sprintf("%dGB", config.DiskSizeGB),
		"--boot-disk-type", "pd-standard",
		"--boot-disk-device-name", instanceName,
		"--metadata", metadata,
		"--tags", "dvarpala-server",
		"--labels", "project=dvarpala,managed-by=frigga-labs",
		"--scopes", "https://www.googleapis.com/auth/cloud-platform")
	
	fmt.Printf("🔍 Creating GCP instance with command:\n")
	fmt.Printf("   gcloud compute instances create %s \\\n", instanceName)
	fmt.Printf("     --zone %s \\\n", gcp.Zone)
	fmt.Printf("     --machine-type %s \\\n", config.InstanceType)
	fmt.Printf("     --network-interface subnet=%s,address= \\\n", vpcInfo.SubnetName)
	fmt.Printf("     --image-family %s \\\n", imageFamily)
	fmt.Printf("     --image-project %s \\\n", imageProject)
	fmt.Printf("     --boot-disk-size %dGB\n", config.DiskSizeGB)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %v\nOutput: %s", err, string(output))
	}
	
	fmt.Printf("✅ GCP instance creation command completed successfully\n")

	// Wait for instance to be running
	fmt.Printf("⏳ Waiting for instance %s to be running...\n", instanceName)
	for i := 0; i < 30; i++ {
		if gcp.isInstanceRunning(instanceName) {
			break
		}
		time.Sleep(10 * time.Second)
	}

	// Get instance details
	instanceInfo, err := gcp.getInstanceDetails(instanceName)
	if err != nil {
		return nil, err
	}

	fmt.Printf("✅ Instance created: %s (IP: %s)\n", instanceName, instanceInfo.ExternalIP)
	return instanceInfo, nil
}

func (gcp *GCPProvider) generateMinimalStartupScript(config InstanceConfig) string {
	return fmt.Sprintf(`#!/bin/bash
# Minimal GCP Instance Setup Script - Just basic system prep
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
export CLOUD_PROVIDER='gcp'

# Create marker that basic setup is complete
mkdir -p /var/log/dvarpala
touch /var/log/dvarpala/startup-complete
echo "VM startup preparation completed at $(date)" > /var/log/dvarpala/startup-status.txt

echo "Minimal setup completed. Ready for installer connection."
`, config.AdminEmail, config.AdminName)
}

// generateSSHKeyPair creates an SSH key pair for the instance
func (gcp *GCPProvider) generateSSHKeyPair(keyPath string) error {
	// Generate SSH key pair
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", keyPath, "-N", "")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate SSH key pair: %v", err)
	}
	
	fmt.Printf("🔑 SSH key pair generated: %s\n", keyPath)
	return nil
}

// InstallDvarpalaDirectly performs the installation directly via SSH from the installer
func (gcp *GCPProvider) InstallDvarpalaDirectly(instanceInfo *GCPInstanceInfo, config InstanceConfig, keyPath string) error {
	fmt.Println("🔗 Connecting to GCP VM for direct installation...")
	fmt.Printf("🔑 Using SSH key: %s\n", keyPath)
	
	// Wait for VM to be SSH accessible
	if err := gcp.waitForSSHAccess(instanceInfo.ExternalIP, keyPath); err != nil {
		return fmt.Errorf("failed to establish SSH connection: %v", err)
	}
	
	fmt.Println("✅ SSH connection established")
	
	// Install components step by step with real-time tracking
	steps := []struct {
		name string
		cmd  string
	}{
		{"Installing nginx", "sudo apt-get install -y nginx"},
		{"Configuring nginx", "sudo systemctl enable nginx && sudo systemctl start nginx"},
		{"Installing PostgreSQL", "sudo apt-get install -y postgresql postgresql-contrib"},
		{"Installing Redis", "sudo apt-get install -y redis-server"},
		{"Installing OpenVPN", "sudo apt-get install -y openvpn easy-rsa"},
		{"Installing Go", "curl -fsSL https://go.dev/dl/go1.21.0.linux-amd64.tar.gz | sudo tar -C /usr/local -xzf -"},
		{"Setting up directories", "mkdir -p /home/$(whoami)/dvarpala /home/$(whoami)/dvarpala/certs && sudo mkdir -p /var/lib/dvarpala"},
		{"Starting basic services", "sudo systemctl start postgresql redis-server"},
		{"Setting up Easy-RSA", "make-cadir /home/$(whoami)/dvarpala/easy-rsa && echo '🔍 DEBUG: Easy-RSA directory created' && ls -la /home/$(whoami)/dvarpala/"},
		{"Configuring Easy-RSA vars", gcp.getEasyRSAVarsCommand()},
		{"Building Certificate Authority", "cd /home/$(whoami)/dvarpala/easy-rsa && echo '🔍 DEBUG: Starting CA generation' && ./easyrsa init-pki && echo '🔍 DEBUG: PKI initialized' && ./easyrsa --batch build-ca nopass && echo '🔍 DEBUG: CA generated' && ls -la pki/"},
		{"Generating server certificate", "cd /home/$(whoami)/dvarpala/easy-rsa && echo '🔍 DEBUG: Starting server cert generation' && ./easyrsa --batch build-server-full server nopass && echo '🔍 DEBUG: Server cert generated' && ls -la pki/issued/ && ls -la pki/private/"},
		{"Generating admin client certificate", "cd /home/$(whoami)/dvarpala/easy-rsa && echo '🔍 DEBUG: Starting admin cert generation' && ./easyrsa --batch build-client-full admin nopass && echo '🔍 DEBUG: Admin cert generated' && ls -la pki/issued/ && ls -la pki/private/"},
		{"Generating TLS authentication key", "cd /home/$(whoami)/dvarpala/easy-rsa && echo '🔍 DEBUG: Starting TLS auth key generation' && openvpn --genkey --secret pki/ta.key && echo '🔍 DEBUG: TLS auth key generated' && ls -la pki/ta.key"},
		{"Copying certificates to OpenVPN directory", "sudo cp /home/$(whoami)/dvarpala/easy-rsa/pki/ca.crt /home/$(whoami)/dvarpala/easy-rsa/pki/issued/server.crt /home/$(whoami)/dvarpala/easy-rsa/pki/private/server.key /home/$(whoami)/dvarpala/easy-rsa/pki/ta.key /etc/openvpn/server/ && echo '🔍 DEBUG: Certificates copied to OpenVPN directory' && sudo ls -la /etc/openvpn/server/"},
		{"Creating OpenVPN server configuration", gcp.getOpenVPNServerConfigCommand()},
		{"Starting OpenVPN server", "sudo systemctl enable openvpn-server@server && sudo systemctl start openvpn-server@server && echo '🔍 DEBUG: OpenVPN server status:' && sudo systemctl status openvpn-server@server --no-pager"},
		{"Configuring OAuth firewall rules", "sudo bash -c 'curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/main/scripts/installation/configure-oauth-firewall.sh | bash'"},
	}
	
	for i, step := range steps {
		fmt.Printf("📦 Step %d/%d: %s\n", i+1, len(steps), step.name)
		
		if err := gcp.executeSSHCommand(instanceInfo.ExternalIP, step.cmd, keyPath); err != nil {
			return fmt.Errorf("failed at step '%s': %v", step.name, err)
		}
		
		fmt.Printf("✅ Completed: %s\n", step.name)
	}
	
	// Configure nginx monitoring as separate steps with proper sudo handling
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+1, len(steps)+5, "Creating nginx monitoring config")
	if err := gcp.configureNginxMonitoring(instanceInfo.ExternalIP, keyPath); err != nil {
		return fmt.Errorf("failed to configure nginx monitoring: %v", err)
	}
	fmt.Printf("✅ Completed: Creating nginx monitoring config\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+2, len(steps)+5, "Enabling nginx monitoring site")
	if err := gcp.executeSSHCommand(instanceInfo.ExternalIP, "sudo ln -sf /etc/nginx/sites-available/dvarpala-monitoring /etc/nginx/sites-enabled/", keyPath); err != nil {
		return fmt.Errorf("failed to enable nginx site: %v", err)
	}
	fmt.Printf("✅ Completed: Enabling nginx monitoring site\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+3, len(steps)+5, "Reloading nginx configuration")
	if err := gcp.executeSSHCommand(instanceInfo.ExternalIP, "sudo nginx -t && sudo systemctl reload nginx && echo '🔍 DEBUG: Nginx configuration test passed and reloaded'", keyPath); err != nil {
		return fmt.Errorf("failed to reload nginx: %v", err)
	}
	fmt.Printf("✅ Completed: Reloading nginx configuration\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+4, len(steps)+5, "Generating admin OpenVPN configuration")
	if err := gcp.generateAdminOVPN(instanceInfo.ExternalIP, keyPath); err != nil {
		return fmt.Errorf("failed to generate admin OVPN: %v", err)
	}
	fmt.Printf("✅ Completed: Generating admin OpenVPN configuration\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+5, len(steps)+6, "Making admin.ovpn temporarily available for download")
	if err := gcp.executeSSHCommand(instanceInfo.ExternalIP, "echo '🔍 DEBUG: Copying admin.ovpn to web directory' && sudo cp /home/$(whoami)/dvarpala/certs/admin.ovpn /var/www/html/admin.ovpn && sudo chmod 644 /var/www/html/admin.ovpn && echo '🔍 DEBUG: File copied. Checking web directory:' && ls -la /var/www/html/admin.ovpn && wc -l /var/www/html/admin.ovpn && echo '🔍 DEBUG: Testing HTTP access:' && curl -s -I http://localhost/admin.ovpn", keyPath); err != nil {
		return fmt.Errorf("failed to make admin.ovpn downloadable: %v", err)
	}
	fmt.Printf("✅ Completed: Making admin.ovpn temporarily available for download\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+6, len(steps)+6, "Cleaning up public admin.ovpn file")
	// Give the installer 2 minutes to download the file, then remove it from public access
	cleanupCommand := "sleep 120 && sudo rm -f /var/www/html/admin.ovpn && echo '🔒 SECURITY: admin.ovpn removed from public web directory for security'"
	if err := gcp.executeSSHCommand(instanceInfo.ExternalIP, fmt.Sprintf("nohup bash -c '%s' > /dev/null 2>&1 &", cleanupCommand), keyPath); err != nil {
		return fmt.Errorf("failed to schedule admin.ovpn cleanup: %v", err)
	}
	fmt.Printf("✅ Completed: Scheduled cleanup of public admin.ovpn file in 2 minutes\n")
	
	fmt.Println("🎉 Dvarpala installation completed successfully!")
	fmt.Println("🔒 SECURITY NOTE: admin.ovpn will be automatically removed from public access in 2 minutes")
	fmt.Println("📋 The installer will download the file immediately - please wait for download completion")
	return nil
}

func (gcp *GCPProvider) waitForSSHAccess(vmIP, keyPath string) error {
	fmt.Printf("⏳ Waiting for SSH access to %s...\n", vmIP)
	
	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		// Test SSH connectivity with key
		sshCmd := exec.Command("ssh", "-i", keyPath, "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
			fmt.Sprintf("ubuntu@%s", vmIP), "echo 'SSH Ready'")
		if sshCmd.Run() == nil {
			return nil
		}
		
		fmt.Printf("⏳ SSH not ready yet... attempt %d/%d\n", i+1, maxAttempts)
		time.Sleep(10 * time.Second)
	}
	
	return fmt.Errorf("SSH access not available after %d attempts", maxAttempts)
}

func (gcp *GCPProvider) executeSSHCommand(vmIP, command, keyPath string) error {
	cmd := exec.Command("ssh", "-i", keyPath, "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("ubuntu@%s", vmIP), command)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Command failed: %s\nOutput: %s\n", command, string(output))
		return err
	}
	
	return nil
}

func (gcp *GCPProvider) configureNginxMonitoring(vmIP, keyPath string) error {
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
	
	return gcp.executeSSHCommand(vmIP, command, keyPath)
}

func (gcp *GCPProvider) getEasyRSAVarsCommand() string {
	return `echo "🔍 DEBUG: Creating Easy-RSA vars file" && tee /home/$(whoami)/dvarpala/easy-rsa/vars > /dev/null << 'EOF'
set_var EASYRSA_REQ_COUNTRY    "US"
set_var EASYRSA_REQ_PROVINCE   "CA"
set_var EASYRSA_REQ_CITY       "San Francisco"
set_var EASYRSA_REQ_ORG        "Frigga Labs"
set_var EASYRSA_REQ_EMAIL      "admin@friggalabs.com"
set_var EASYRSA_REQ_OU         "Dvarpala VPN"
set_var EASYRSA_KEY_SIZE       2048
set_var EASYRSA_ALGO           rsa
set_var EASYRSA_CA_EXPIRE      3650
set_var EASYRSA_CERT_EXPIRE    365
EOF
echo "🔍 DEBUG: Easy-RSA vars file created" && ls -la /home/$(whoami)/dvarpala/easy-rsa/vars`
}

func (gcp *GCPProvider) getOpenVPNServerConfigCommand() string {
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

func (gcp *GCPProvider) generateAdminOVPN(vmIP, keyPath string) error {
	// Create the admin.ovpn file with real certificates
	command := `
echo "🔍 DEBUG: Starting admin.ovpn generation"

# Check if certificate files exist before proceeding
echo "🔍 DEBUG: Checking if certificate files exist:"
ls -la /home/$(whoami)/dvarpala/easy-rsa/pki/ca.crt
ls -la /home/$(whoami)/dvarpala/easy-rsa/pki/issued/admin.crt
ls -la /home/$(whoami)/dvarpala/easy-rsa/pki/private/admin.key  
ls -la /home/$(whoami)/dvarpala/easy-rsa/pki/ta.key

echo "🔍 DEBUG: File sizes:"
wc -l /home/$(whoami)/dvarpala/easy-rsa/pki/ca.crt
wc -l /home/$(whoami)/dvarpala/easy-rsa/pki/issued/admin.crt
wc -l /home/$(whoami)/dvarpala/easy-rsa/pki/private/admin.key
wc -l /home/$(whoami)/dvarpala/easy-rsa/pki/ta.key

# Get the external IP address
EXTERNAL_IP=$(curl -s http://checkip.amazonaws.com)
echo "🔍 DEBUG: External IP detected: $EXTERNAL_IP"

# Create admin.ovpn with embedded certificates
echo "🔍 DEBUG: Creating admin.ovpn file..."
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

echo "🔍 DEBUG: Admin.ovpn file created. Checking file:"
ls -la /home/$(whoami)/dvarpala/certs/admin.ovpn
wc -l /home/$(whoami)/dvarpala/certs/admin.ovpn
echo "🔍 DEBUG: First 10 lines of admin.ovpn:"
head -10 /home/$(whoami)/dvarpala/certs/admin.ovpn
echo "🔍 DEBUG: Last 10 lines of admin.ovpn:"
tail -10 /home/$(whoami)/dvarpala/certs/admin.ovpn
echo "🔍 DEBUG: Admin.ovpn generation completed"
`
	
	return gcp.executeSSHCommand(vmIP, command, keyPath)
}


func (gcp *GCPProvider) isInstanceRunning(instanceName string) bool {
	cmd := exec.Command("gcloud", "compute", "instances", "describe", instanceName,
		"--zone", gcp.Zone,
		"--format=value(status)")

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "RUNNING"
}

func (gcp *GCPProvider) getInstanceDetails(instanceName string) (*GCPInstanceInfo, error) {
	cmd := exec.Command("gcloud", "compute", "instances", "describe", instanceName,
		"--zone", gcp.Zone,
		"--format=json")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance details: %v", err)
	}

	var instance map[string]interface{}
	if err := json.Unmarshal(output, &instance); err != nil {
		return nil, err
	}

	// Extract IP addresses
	networkInterfaces := instance["networkInterfaces"].([]interface{})
	if len(networkInterfaces) == 0 {
		return nil, fmt.Errorf("no network interfaces found")
	}

	firstInterface := networkInterfaces[0].(map[string]interface{})
	internalIP := firstInterface["networkIP"].(string)

	var externalIP string
	if accessConfigs := firstInterface["accessConfigs"]; accessConfigs != nil {
		configs := accessConfigs.([]interface{})
		if len(configs) > 0 {
			firstConfig := configs[0].(map[string]interface{})
			if natIP := firstConfig["natIP"]; natIP != nil {
				externalIP = natIP.(string)
			}
		}
	}

	return &GCPInstanceInfo{
		InstanceName: instanceName,
		Zone:         gcp.Zone,
		ExternalIP:   externalIP,
		InternalIP:   internalIP,
	}, nil
}

func (gcp *GCPProvider) CreateStorageBucket(bucketName string) error {
	// Check if bucket exists
	cmd := exec.Command("gsutil", "ls", fmt.Sprintf("gs://%s", bucketName))
	if cmd.Run() == nil {
		fmt.Printf("✅ Using existing GCS bucket: %s\n", bucketName)
		return nil
	}

	// Create bucket
	cmd = exec.Command("gsutil", "mb", "-l", gcp.Region, fmt.Sprintf("gs://%s", bucketName))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create GCS bucket: %v", err)
	}

	// Enable versioning
	cmd = exec.Command("gsutil", "versioning", "set", "on", fmt.Sprintf("gs://%s", bucketName))
	cmd.Run()

	// Create dvarpala folder
	cmd = exec.Command("gsutil", "cp", "/dev/null", fmt.Sprintf("gs://%s/dvarpala/.gitkeep", bucketName))
	cmd.Run()

	fmt.Printf("✅ GCS bucket created: %s\n", bucketName)
	return nil
}

func (gcp *GCPProvider) UploadConfiguration(bucketName string, configData []byte, filename string) error {
	// Write config to user home directory to avoid permission issues
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/home/" + os.Getenv("USER")
	}
	tempFile := fmt.Sprintf("%s/%s", homeDir, filename)
	if err := os.WriteFile(tempFile, configData, 0644); err != nil {
		return err
	}
	defer os.Remove(tempFile)

	// Upload to GCS
	cmd := exec.Command("gsutil", "cp", tempFile, fmt.Sprintf("gs://%s/dvarpala/%s", bucketName, filename))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to upload configuration to GCS: %v", err)
	}

	return nil
}
