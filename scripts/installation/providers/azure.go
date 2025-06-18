package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

func (az *AzureProvider) CreateInstance(vpcInfo *AzureVPCInfo, config InstanceConfig) (*AzureInstanceInfo, error) {
	vmName := fmt.Sprintf("dvarpala-vm-%d", time.Now().Unix())
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
	
	// Generate SSH key
	sshKeyPath := fmt.Sprintf("./dvarpala-deployment/%s-key", vmName)
	cmd = exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", sshKeyPath, "-N", "")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to generate SSH key: %v", err)
	}
	
	// Read public key
	pubKeyData, err := os.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %v", err)
	}
	pubKey := strings.TrimSpace(string(pubKeyData))
	
	// Generate cloud-init script
	cloudInit := az.generateCloudInit(config)
	
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
	return instanceInfo, nil
}

func (az *AzureProvider) generateCloudInit(config InstanceConfig) string {
	return fmt.Sprintf(`#!/bin/bash
# Update system
apt-get update -y
apt-get upgrade -y

# Install dependencies
apt-get install -y curl wget unzip

# Download and run dvarpala installation
cd /tmp
curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/setup-server.sh | bash

# Configure admin user
echo '%s' > /opt/dvarpala/config/admin-email.txt
echo '%s' > /opt/dvarpala/config/admin-name.txt

# Signal completion
logger "Dvarpala installation completed"
`, config.AdminEmail, config.AdminName)
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
		return nil
	}
	
	// Create storage account
	cmd = exec.Command("az", "storage", "account", "create",
		"--name", accountName,
		"--resource-group", resourceGroup,
		"--location", az.Region,
		"--sku", "Standard_LRS",
		"--kind", "StorageV2",
		"--tags", "Project=dvarpala", "ManagedBy=frigga-labs")
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create storage account: %v", err)
	}
	
	// Create container
	cmd = exec.Command("az", "storage", "container", "create",
		"--name", "dvarpala",
		"--account-name", accountName)
	cmd.Run()
	
	fmt.Printf("✅ Storage account created: %s\n", accountName)
	return nil
}

func (az *AzureProvider) UploadConfiguration(accountName string, configData []byte, filename string) error {
	// Write config to temp file
	tempFile := fmt.Sprintf("/tmp/%s", filename)
	if err := os.WriteFile(tempFile, configData, 0644); err != nil {
		return err
	}
	defer os.Remove(tempFile)
	
	// Upload to Azure Storage
	cmd := exec.Command("az", "storage", "blob", "upload",
		"--account-name", accountName,
		"--container-name", "dvarpala",
		"--name", filename,
		"--file", tempFile)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to upload configuration to Azure Storage: %v", err)
	}
	
	return nil
}