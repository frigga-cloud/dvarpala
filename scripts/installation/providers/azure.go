package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type AzureProvider struct {
	SubscriptionID string
	Region         string
	Credentials    AzureCredentials
}

type AzureCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	TenantID     string `json:"tenant_id"`
}

type AzureVPCInfo struct {
	ResourceGroup string
	VNetName      string
	SubnetName    string
	NSGName       string
	PublicIPName  string
}

type AzureInstanceInfo struct {
	VMName        string
	ResourceGroup string
	PublicIP      string
	PrivateIP     string
	SSHKeyPath    string
}

func NewAzureProvider(subscriptionID, region string, creds AzureCredentials) *AzureProvider {
	return &AzureProvider{
		SubscriptionID: subscriptionID,
		Region:         region,
		Credentials:    creds,
	}
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
	// The Dvarpala installer does everything that is the same on every cloud:
	// PostgreSQL, Redis, OpenVPN with the Dvarpala hooks, the walled-garden
	// firewall, and the Dvarpala application itself. Previously this file
	// listed those steps inline, three times over, and never installed the
	// application at all.
	hostForCerts := instanceInfo.PublicIP

	steps := []struct {
		name string
		cmd  string
	}{
		{"Fetching Dvarpala source", fmt.Sprintf("sudo cloud-init status --wait >/dev/null 2>&1 || true; "+
			"sudo apt-get -o DPkg::Lock::Timeout=600 update -qq && "+
			"sudo apt-get -o DPkg::Lock::Timeout=600 install -y -qq git && "+
			"sudo rm -rf /opt/dvarpala/src && "+
			"sudo git clone --depth 1 --branch %s %s /opt/dvarpala/src", defaultRepoRef, defaultRepoURL)},
		{"Installing Dvarpala", "sudo chmod +x /opt/dvarpala/src/scripts/install/install-dvarpala.sh && sudo DVARPALA_ADMIN_EMAIL='" + config.AdminEmail + "' /opt/dvarpala/src/scripts/install/install-dvarpala.sh --source /opt/dvarpala/src --host " + hostForCerts},
	}

	for i, step := range steps {
		fmt.Printf("📦 Step %d/%d: %s\n", i+1, len(steps), step.name)

		if err := az.executeSSHCommand(instanceInfo.PublicIP, step.cmd, keyPath); err != nil {
			return fmt.Errorf("failed at step '%s': %v", step.name, err)
		}

		fmt.Printf("✅ Completed: %s\n", step.name)
	}

	// Retrieve the administrator's VPN profile over the SSH session that is
	// already open. The previous approach published it on the machine's public
	// web root for two minutes; the file contains the client private key, so
	// anyone who fetched it in that window gained permanent VPN access.
	fmt.Println("📄 Retrieving the administrator VPN profile")
	if config.AdminEmail == "" {
		fmt.Println("   (no admin email supplied, so no profile was issued)")
	} else if err := az.fetchAdminProfile(instanceInfo.PublicIP, keyPath, config.OutputDir); err != nil {
		fmt.Printf("⚠️  Could not retrieve admin.ovpn: %v\n", err)
		fmt.Printf("   Fetch it later with:\n     scp -i %s azureuser@%s:/tmp/admin.ovpn .\n",
			keyPath, instanceInfo.PublicIP)
	}

	fmt.Println("🎉 Dvarpala installation completed successfully!")
	return nil
}

// fetchAdminProfile copies the administrator's .ovpn to the operator's machine.
//
// The profile is readable only by the dvarpala service user, so it is staged
// briefly into the login user's home - never anywhere served over the network -
// and removed afterwards.
func (az *AzureProvider) fetchAdminProfile(vmIP, keyPath, outputDir string) error {
	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	const staged = "/tmp/admin.ovpn"
	stage := fmt.Sprintf(
		"sudo cp /opt/dvarpala/certs/admin.ovpn %s && sudo chown $(whoami) %s && chmod 600 %s",
		staged, staged, staged)
	if err := az.executeSSHCommand(vmIP, stage, keyPath); err != nil {
		return fmt.Errorf("staging the profile: %w", err)
	}

	local := filepath.Join(outputDir, "admin.ovpn")
	cmd := exec.Command("scp", "-i", keyPath,
		"-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("azureuser@%s:%s", vmIP, staged), local)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp: %v: %s", err, strings.TrimSpace(string(out)))
	}

	_ = az.executeSSHCommand(vmIP, fmt.Sprintf("shred -u %s 2>/dev/null || rm -f %s", staged, staged), keyPath)

	if err := os.Chmod(local, 0o600); err != nil {
		return err
	}
	fmt.Printf("✅ Saved %s (mode 0600 - contains a private key)\n", local)
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

func (az *AzureProvider) UploadConfiguration(accountName string, configData []byte, filename string) error {
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
