package providers

import (
	"dvarpala-cloud-installer/lib"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type AzureProvider struct {
	BaseCloudProvider
	SubscriptionID string
	Region         string
	Credentials    AzureCredentials
	Config         lib.InstallationConfig
}

type AzureCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	TenantID     string `json:"tenant_id"`
}

type AzureVPCInfo struct {
	ResourceGroup   string
	VNetName        string
	SubnetName      string
	NSGName         string
	PublicIPName    string
}

type AzureInstanceInfo struct {
	VMName        string
	ResourceGroup string
	PublicIP      string
	PrivateIP     string
	SSHKeyPath    string
}

// Implement InstanceInfo interface
func (i *AzureInstanceInfo) GetPublicIP() string {
	return i.PublicIP
}

func (i *AzureInstanceInfo) GetPrivateIP() string {
	return i.PrivateIP
}

func (i *AzureInstanceInfo) GetInstanceID() string {
	return i.VMName
}

func (i *AzureInstanceInfo) GetSSHKeyPath() string {
	return i.SSHKeyPath
}

func NewAzureProvider(subscriptionID, region string, creds AzureCredentials) *AzureProvider {
	return &AzureProvider{
		SubscriptionID: subscriptionID,
		Region:         region,
		Credentials:    creds,
	}
}

func NewAzureProviderFromConfig(config lib.InstallationConfig) (*AzureProvider, error) {
	provider := &AzureProvider{
		SubscriptionID: "", // Will be set from auth or defaults
		Region:         config.Cloud.Region,
		Credentials: AzureCredentials{
			ClientID:     lib.GetStringFromCredentials(config.Cloud.Credentials, "client_id"),
			ClientSecret: lib.GetStringFromCredentials(config.Cloud.Credentials, "client_secret"),
			TenantID:     lib.GetStringFromCredentials(config.Cloud.Credentials, "tenant_id"),
		},
		Config: config,
	}

	if err := provider.SetupEnvironment(); err != nil {
		return nil, err
	}

	if err := provider.ValidateAuthentication(); err != nil {
		return nil, err
	}

	return provider, nil
}

func (az *AzureProvider) SetupEnvironment() error {
	if az.Credentials.ClientID != "" {
		// Login with service principal
		cmd := exec.Command("az", "login", "--service-principal",
			"--username", az.Credentials.ClientID,
			"--password", az.Credentials.ClientSecret,
			"--tenant", az.Credentials.TenantID)
		
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to login with service principal: %v", err)
		}
	}
	
	// Set subscription
	if az.SubscriptionID != "" {
		cmd := exec.Command("az", "account", "set", "--subscription", az.SubscriptionID)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set subscription: %v", err)
		}
	}
	
	return nil
}

func (az *AzureProvider) ValidateAuthentication() error {
	cmd := exec.Command("az", "account", "show", "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Azure authentication failed: %v", err)
	}
	
	var account map[string]interface{}
	if err := json.Unmarshal(output, &account); err != nil {
		return fmt.Errorf("failed to parse Azure account response: %v", err)
	}
	
	fmt.Printf("✅ Authenticated as: %s\n", account["user"].(map[string]interface{})["name"])
	return nil
}

func (az *AzureProvider) CreateOrGetVPC(vpcName string, config NetworkConfig) (*AzureVPCInfo, error) {
	resourceGroup := vpcName + "-rg"
	
	// Check if resource group exists
	existingVPC, err := az.findResourceGroup(resourceGroup)
	if err != nil {
		return nil, err
	}
	
	if existingVPC != nil {
		fmt.Printf("✅ Using existing resource group: %s\n", existingVPC.ResourceGroup)
		return existingVPC, nil
	}
	
	fmt.Printf("🏗️ Creating new resource group: %s\n", resourceGroup)
	return az.createNewVPC(vpcName, resourceGroup, config)
}

func (az *AzureProvider) findResourceGroup(resourceGroup string) (*AzureVPCInfo, error) {
	cmd := exec.Command("az", "group", "show", "--name", resourceGroup, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		// Resource group doesn't exist
		return nil, nil
	}
	
	var rg map[string]interface{}
	if err := json.Unmarshal(output, &rg); err != nil {
		return nil, err
	}
	
	vpcInfo := &AzureVPCInfo{
		ResourceGroup: resourceGroup,
		VNetName:      "frigga-labs-vnet",
		SubnetName:    "dvarpala-subnet",
		NSGName:       "dvarpala-nsg",
	}
	
	return vpcInfo, nil
}

func (az *AzureProvider) createNewVPC(vpcName, resourceGroup string, config NetworkConfig) (*AzureVPCInfo, error) {
	vpcInfo := &AzureVPCInfo{
		ResourceGroup: resourceGroup,
		VNetName:      vpcName + "-vnet",
		SubnetName:    "dvarpala-subnet",
		NSGName:       "dvarpala-nsg",
	}
	
	// Create resource group
	cmd := exec.Command("az", "group", "create",
		"--name", resourceGroup,
		"--location", az.Region,
		"--tags", "Project=dvarpala", "ManagedBy=frigga-labs")
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create resource group: %v", err)
	}
	
	// Create virtual network
	cmd = exec.Command("az", "network", "vnet", "create",
		"--resource-group", resourceGroup,
		"--name", vpcInfo.VNetName,
		"--address-prefix", config.VPCCidr,
		"--subnet-name", vpcInfo.SubnetName,
		"--subnet-prefix", config.PublicSubnetCidr,
		"--location", az.Region)
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create virtual network: %v", err)
	}
	
	// Create network security group
	cmd = exec.Command("az", "network", "nsg", "create",
		"--resource-group", resourceGroup,
		"--name", vpcInfo.NSGName,
		"--location", az.Region)
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create network security group: %v", err)
	}
	
	// Add security rules
	az.addSecurityRules(resourceGroup, vpcInfo.NSGName)
	
	// Associate NSG with subnet
	cmd = exec.Command("az", "network", "vnet", "subnet", "update",
		"--resource-group", resourceGroup,
		"--vnet-name", vpcInfo.VNetName,
		"--name", vpcInfo.SubnetName,
		"--network-security-group", vpcInfo.NSGName)
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to associate NSG with subnet: %v", err)
	}
	
	fmt.Printf("✅ Virtual network created successfully: %s\n", vpcInfo.VNetName)
	return vpcInfo, nil
}

func (az *AzureProvider) addSecurityRules(resourceGroup, nsgName string) {
	rules := []struct {
		name     string
		priority int
		port     string
		protocol string
	}{
		{"SSH", 1000, "22", "Tcp"},
		{"OpenVPN", 1001, "1194", "Udp"},
		{"DvarpalaWeb", 1002, "8080", "Tcp"},
		{"HTTPS", 1003, "443", "Tcp"},
	}
	
	for _, rule := range rules {
		exec.Command("az", "network", "nsg", "rule", "create",
			"--resource-group", resourceGroup,
			"--nsg-name", nsgName,
			"--name", rule.name,
			"--protocol", rule.protocol,
			"--priority", fmt.Sprintf("%d", rule.priority),
			"--destination-port-ranges", rule.port,
			"--access", "Allow",
			"--direction", "Inbound").Run()
	}
}

func (az *AzureProvider) CreateInstance(vpcInfo *AzureVPCInfo, config InstanceConfig, vmName string) (*AzureInstanceInfo, error) {
	// Use the provided VM name with Frigga Labs naming convention
	publicIPName := vmName + "-ip"
	
	// Create public IP
	cmd := exec.Command("az", "network", "public-ip", "create",
		"--resource-group", vpcInfo.ResourceGroup,
		"--name", publicIPName,
		"--location", az.Region,
		"--allocation-method", "Static",
		"--sku", "Standard")
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create public IP: %v", err)
	}
	
	// Create deployment directory if it doesn't exist
	deploymentDir := "./dvarpala-deployment"
	if err := os.MkdirAll(deploymentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create deployment directory: %v", err)
	}

	// Generate SSH key
	sshKeyPath := fmt.Sprintf("%s/%s-key", deploymentDir, vmName)
	cmd = exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", sshKeyPath, "-N", "")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to generate SSH key: %v", err)
	}
	
	fmt.Printf("🔑 SSH key pair generated: %s\n", sshKeyPath)
	
	// Read public key
	pubKeyData, err := os.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %v", err)
	}
	pubKey := strings.TrimSpace(string(pubKeyData))
	
	// Generate minimal cloud-init script - just basic system prep
	cloudInit := az.generateMinimalCloudInit(config)
	
	// Create VM
	cmd = exec.Command("az", "vm", "create",
		"--resource-group", vpcInfo.ResourceGroup,
		"--name", vmName,
		"--image", "Ubuntu2204",
		"--size", config.InstanceType,
		"--vnet-name", vpcInfo.VNetName,
		"--subnet", vpcInfo.SubnetName,
		"--public-ip-address", publicIPName,
		"--nsg", vpcInfo.NSGName,
		"--ssh-key-values", pubKey,
		"--custom-data", cloudInit,
		"--location", az.Region,
		"--tags", "Project=dvarpala", "ManagedBy=frigga-labs")
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create VM: %v", err)
	}
	
	// Wait for VM to be running
	fmt.Printf("⏳ Waiting for VM %s to be running...\n", vmName)
	for i := 0; i < 30; i++ {
		if az.isVMRunning(vpcInfo.ResourceGroup, vmName) {
			break
		}
		time.Sleep(10 * time.Second)
	}
	
	// Get VM details
	instanceInfo, err := az.getVMDetails(vpcInfo.ResourceGroup, vmName, publicIPName)
	if err != nil {
		return nil, err
	}
	
	instanceInfo.SSHKeyPath = sshKeyPath
	
	fmt.Printf("✅ VM created: %s (IP: %s)\n", vmName, instanceInfo.PublicIP)
	fmt.Printf("🔑 SSH private key saved to: %s\n", sshKeyPath)
	return instanceInfo, nil
}

func (az *AzureProvider) generateMinimalCloudInit(config InstanceConfig) string {
	return fmt.Sprintf(`#!/bin/bash
# Minimal Azure Instance Setup Script - Just basic system prep
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
export CLOUD_PROVIDER='azure'

# Create marker that basic setup is complete
mkdir -p /var/log/dvarpala
touch /var/log/dvarpala/startup-complete
echo "VM startup preparation completed at $(date)" > /var/log/dvarpala/startup-status.txt

echo "Minimal setup completed. Ready for installer connection."
`, config.AdminEmail, config.AdminName)
}

// InstallDvarpalaDirectly performs the installation directly via SSH from the installer
func (az *AzureProvider) InstallDvarpalaDirectly(instanceInfo *AzureInstanceInfo, config InstanceConfig, keyPath string) error {
	fmt.Println("🔗 Connecting to Azure VM for direct installation...")
	fmt.Printf("🔑 Using SSH key: %s\n", keyPath)
	
	// Wait for VM to be SSH accessible
	if err := az.waitForSSHAccess(instanceInfo.PublicIP, keyPath); err != nil {
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
		{"Setting up Easy-RSA", "make-cadir /home/$(whoami)/dvarpala/easy-rsa"},
		{"Configuring Easy-RSA vars", az.getEasyRSAVarsCommand(config)},
		{"Building Certificate Authority", "cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa init-pki && ./easyrsa --batch build-ca nopass"},
		{"Generating server certificate", "cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa --batch build-server-full server nopass"},
		{"Generating admin client certificate", "cd /home/$(whoami)/dvarpala/easy-rsa && ./easyrsa --batch build-client-full admin nopass"},
		{"Generating TLS authentication key", "cd /home/$(whoami)/dvarpala/easy-rsa && openvpn --genkey --secret pki/ta.key"},
		{"Copying certificates to OpenVPN directory", "sudo cp /home/$(whoami)/dvarpala/easy-rsa/pki/ca.crt /home/$(whoami)/dvarpala/easy-rsa/pki/issued/server.crt /home/$(whoami)/dvarpala/easy-rsa/pki/private/server.key /home/$(whoami)/dvarpala/easy-rsa/pki/ta.key /etc/openvpn/server/"},
		{"Creating OpenVPN server configuration", az.getOpenVPNServerConfigCommand()},
		{"Starting OpenVPN server", "sudo systemctl enable openvpn-server@server && sudo systemctl start openvpn-server@server"},
		{"Configuring OAuth firewall rules", "sudo bash -c 'curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/main/scripts/installation/configure-oauth-firewall.sh | bash'"},
	}
	
	for i, step := range steps {
		fmt.Printf("📦 Step %d/%d: %s\n", i+1, len(steps), step.name)
		
		if err := az.executeSSHCommand(instanceInfo.PublicIP, step.cmd, keyPath); err != nil {
			return fmt.Errorf("failed at step '%s': %v", step.name, err)
		}
		
		fmt.Printf("✅ Completed: %s\n", step.name)
	}
	
	// Configure nginx monitoring as separate steps with proper sudo handling
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+1, len(steps)+5, "Creating nginx monitoring config")
	if err := az.configureNginxMonitoring(instanceInfo.PublicIP, keyPath); err != nil {
		return fmt.Errorf("failed to configure nginx monitoring: %v", err)
	}
	fmt.Printf("✅ Completed: Creating nginx monitoring config\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+2, len(steps)+5, "Enabling nginx monitoring site")
	if err := az.executeSSHCommand(instanceInfo.PublicIP, "sudo ln -sf /etc/nginx/sites-available/dvarpala-monitoring /etc/nginx/sites-enabled/", keyPath); err != nil {
		return fmt.Errorf("failed to enable nginx site: %v", err)
	}
	fmt.Printf("✅ Completed: Enabling nginx monitoring site\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+3, len(steps)+5, "Reloading nginx configuration")
	if err := az.executeSSHCommand(instanceInfo.PublicIP, "sudo nginx -t && sudo systemctl reload nginx", keyPath); err != nil {
		return fmt.Errorf("failed to reload nginx: %v", err)
	}
	fmt.Printf("✅ Completed: Reloading nginx configuration\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+4, len(steps)+5, "Generating admin OpenVPN configuration")
	if err := az.generateAdminOVPN(instanceInfo.PublicIP, keyPath); err != nil {
		return fmt.Errorf("failed to generate admin OVPN: %v", err)
	}
	fmt.Printf("✅ Completed: Generating admin OpenVPN configuration\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+5, len(steps)+6, "Making admin.ovpn temporarily available for download")
	if err := az.executeSSHCommand(instanceInfo.PublicIP, "sudo cp /home/$(whoami)/dvarpala/certs/admin.ovpn /var/www/html/admin.ovpn && sudo chmod 644 /var/www/html/admin.ovpn", keyPath); err != nil {
		return fmt.Errorf("failed to make admin.ovpn downloadable: %v", err)
	}
	fmt.Printf("✅ Completed: Making admin.ovpn temporarily available for download\n")
	
	fmt.Printf("📦 Step %d/%d: %s\n", len(steps)+6, len(steps)+6, "Cleaning up public admin.ovpn file")
	// Give the installer 2 minutes to download the file, then remove it from public access
	cleanupCommand := "sleep 120 && sudo rm -f /var/www/html/admin.ovpn && echo '🔒 SECURITY: admin.ovpn removed from public web directory for security'"
	if err := az.executeSSHCommand(instanceInfo.PublicIP, fmt.Sprintf("nohup bash -c '%s' > /dev/null 2>&1 &", cleanupCommand), keyPath); err != nil {
		return fmt.Errorf("failed to schedule admin.ovpn cleanup: %v", err)
	}
	fmt.Printf("✅ Completed: Scheduled cleanup of public admin.ovpn file in 2 minutes\n")
	
	fmt.Println("🎉 Dvarpala installation completed successfully!")
	fmt.Println("🔒 SECURITY NOTE: admin.ovpn will be automatically removed from public access in 2 minutes")
	fmt.Println("📋 The installer will download the file immediately - please wait for download completion")
	return nil
}

func (az *AzureProvider) waitForSSHAccess(vmIP, keyPath string) error {
	fmt.Printf("⏳ Waiting for SSH access to %s...\n", vmIP)
	
	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		// Test SSH connectivity with key
		sshCmd := exec.Command("ssh", "-i", keyPath, "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
			fmt.Sprintf("azureuser@%s", vmIP), "echo 'SSH Ready'")
		if sshCmd.Run() == nil {
			return nil
		}
		
		fmt.Printf("⏳ SSH not ready yet... attempt %d/%d\n", i+1, maxAttempts)
		time.Sleep(10 * time.Second)
	}
	
	return fmt.Errorf("SSH access not available after %d attempts", maxAttempts)
}

func (az *AzureProvider) executeSSHCommand(vmIP, command, keyPath string) error {
	cmd := exec.Command("ssh", "-i", keyPath, "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("azureuser@%s", vmIP), command)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Command failed: %s\nOutput: %s\n", command, string(output))
		return err
	}
	
	return nil
}

func (az *AzureProvider) configureNginxMonitoring(vmIP, keyPath string) error {
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
	
	return az.executeSSHCommand(vmIP, command, keyPath)
}

func (az *AzureProvider) getEasyRSAVarsCommand(config InstanceConfig) string {
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
EOF`, config.AdminEmail)
}

func (az *AzureProvider) getOpenVPNServerConfigCommand() string {
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

func (az *AzureProvider) generateAdminOVPN(vmIP, keyPath string) error {
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
	
	return az.executeSSHCommand(vmIP, command, keyPath)
}


func (az *AzureProvider) isVMRunning(resourceGroup, vmName string) bool {
	cmd := exec.Command("az", "vm", "get-instance-view",
		"--resource-group", resourceGroup,
		"--name", vmName,
		"--query", "instanceView.statuses[1].displayStatus",
		"--output", "tsv")
	
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	return strings.TrimSpace(string(output)) == "VM running"
}

func (az *AzureProvider) getVMDetails(resourceGroup, vmName, publicIPName string) (*AzureInstanceInfo, error) {
	// Get public IP
	cmd := exec.Command("az", "network", "public-ip", "show",
		"--resource-group", resourceGroup,
		"--name", publicIPName,
		"--query", "ipAddress",
		"--output", "tsv")
	
	publicIPOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get public IP: %v", err)
	}
	publicIP := strings.TrimSpace(string(publicIPOutput))
	
	// Get private IP
	cmd = exec.Command("az", "vm", "show",
		"--resource-group", resourceGroup,
		"--name", vmName,
		"--show-details",
		"--query", "privateIps",
		"--output", "tsv")
	
	privateIPOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get private IP: %v", err)
	}
	privateIP := strings.TrimSpace(string(privateIPOutput))
	
	return &AzureInstanceInfo{
		VMName:        vmName,
		ResourceGroup: resourceGroup,
		PublicIP:      publicIP,
		PrivateIP:     privateIP,
	}, nil
}

func (az *AzureProvider) CreateStorageAccount(accountName string) error {
	resourceGroup := "frigga-labs-rg"
	
	// Check if storage account exists
	cmd := exec.Command("az", "storage", "account", "show",
		"--name", accountName,
		"--resource-group", resourceGroup)
	if cmd.Run() == nil {
		fmt.Printf("✅ Using existing storage account: %s\n", accountName)
		// Ensure friggalabs container exists
		cmd = exec.Command("az", "storage", "container", "create",
			"--name", "friggalabs",
			"--account-name", accountName)
		cmd.Run() // Create container if it doesn't exist
		return nil
	}
	
	// Create storage account
	cmd = exec.Command("az", "storage", "account", "create",
		"--name", accountName,
		"--resource-group", resourceGroup,
		"--location", az.Region,
		"--sku", "Standard_LRS",
		"--kind", "StorageV2",
		"--tags", "Project=frigga-tools", "ManagedBy=frigga-labs")
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create storage account: %v", err)
	}
	
	// Create shared friggalabs container
	cmd = exec.Command("az", "storage", "container", "create",
		"--name", "friggalabs",
		"--account-name", accountName)
	cmd.Run()
	
	// Create dvarpala directory marker (Azure blob)
	cmd = exec.Command("az", "storage", "blob", "upload",
		"--account-name", accountName,
		"--container-name", "friggalabs",
		"--name", "dvarpala/.gitkeep",
		"--file", "/dev/null")
	cmd.Run()
	
	fmt.Printf("✅ Storage account created: %s (with shared friggalabs container)\n", accountName)
	return nil
}

func (az *AzureProvider) UploadConfigurationToBucket(accountName string, configData []byte, filename string) error {
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
	
	// Upload to Azure Storage in shared friggalabs container, dvarpala directory
	cmd := exec.Command("az", "storage", "blob", "upload",
		"--account-name", accountName,
		"--container-name", "friggalabs",
		"--name", fmt.Sprintf("dvarpala/%s", filename),
		"--file", tempFile)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to upload configuration to Azure Storage: %v", err)
	}
	
	return nil
}

// Wrapper methods to implement lib.CloudProvider interface
func (azure *AzureProvider) SetupVPC() (string, error) {
	vpcInfo, err := azure.CreateOrGetVPC(azure.Config.ResourceNames.VPCName, NetworkConfig{
		VPCCidr:           azure.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  azure.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: azure.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        azure.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return "", err
	}
	return vpcInfo.ResourceGroup, nil
}

func (azure *AzureProvider) CreateVM(vpcID string) (*lib.VMResult, error) {
	// Get resource group info again for VM creation
	resourceGroupInfo, err := azure.CreateOrGetVPC(azure.Config.ResourceNames.VPCName, NetworkConfig{
		VPCCidr:           azure.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  azure.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: azure.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        azure.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	instanceInfo, err := azure.CreateInstance(resourceGroupInfo, InstanceConfig{
		InstanceType: azure.Config.VMConfig.InstanceType,
		DiskSizeGB:   azure.Config.VMConfig.DiskSize,
		AdminEmail:   azure.Config.Admin.Email,
		AdminName:    azure.Config.Admin.FullName,
	}, azure.Config.ResourceNames.VMName)
	if err != nil {
		return nil, err
	}

	// Perform direct installation
	if err := azure.InstallDvarpalaDirectly(instanceInfo, InstanceConfig{
		InstanceType: azure.Config.VMConfig.InstanceType,
		DiskSizeGB:   azure.Config.VMConfig.DiskSize,
		AdminEmail:   azure.Config.Admin.Email,
		AdminName:    azure.Config.Admin.FullName,
	}, "azure"); err != nil {
		return nil, fmt.Errorf("direct installation failed: %v", err)
	}

	return &lib.VMResult{
		InstanceID: instanceInfo.VMName,
		PublicIP:   instanceInfo.PublicIP,
		PrivateIP:  instanceInfo.PrivateIP,
		SSHKeyPath: instanceInfo.SSHKeyPath,
	}, nil
}

func (azure *AzureProvider) SetupObjectStorage() error {
	return azure.CreateStorageAccount(azure.Config.ResourceNames.BucketName)
}

func (azure *AzureProvider) UploadConfiguration() error {
	configData, err := json.MarshalIndent(azure.Config, "", "  ")
	if err != nil {
		return err
	}
	return azure.UploadConfigurationToBucket(azure.Config.ResourceNames.BucketName, configData, "installation-config.json")
}

func (azure *AzureProvider) InstallDvarpala(vmInfo *lib.VMResult) error {
	fmt.Println("🚀 Starting Dvarpala installation...")
	fmt.Printf("📍 Target VM: %s (IP: %s)\n", vmInfo.InstanceID, vmInfo.PublicIP)
	fmt.Printf("🔑 SSH Key: %s\n", vmInfo.SSHKeyPath)

	// This is where the common installation logic would go
	// In the current implementation, this is handled by the provider's InstallDvarpalaDirectly method
	// but in a unified structure, this could be common code

	fmt.Println("✅ Dvarpala installation completed successfully!")
	return nil
}

func (azure *AzureProvider) Configure2StepVPNAccess(vmInfo *lib.VMResult) error {
	// Convert VMResult to InstanceInfo for the provider
	instanceInfo := &AzureInstanceInfo{
		VMName:        vmInfo.InstanceID,
		PublicIP:      vmInfo.PublicIP,
		PrivateIP:     vmInfo.PrivateIP,
		SSHKeyPath:    vmInfo.SSHKeyPath,
		ResourceGroup: azure.Config.ResourceNames.VPCName, // Using VPC name as resource group
	}

	return azure.BaseCloudProvider.Configure2StepVPNAccess(instanceInfo, InstanceConfig{
		AdminEmail: azure.Config.Admin.Email,
		AdminName:  azure.Config.Admin.FullName,
	})
}