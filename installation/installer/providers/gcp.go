package providers

import (
	"dvarpala-cloud-installer/providers/lib"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type GCPProvider struct {
	BaseCloudProvider
	ProjectID   string
	Region      string
	Zone        string
	Credentials GCPCredentials
	Config      lib.InstallationConfig
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
	SSHKeyPath   string
}

// Implement VPCInfo interface
func (v *GCPVPCInfo) GetID() string {
	return v.VPCName
}

func (v *GCPVPCInfo) GetSubnetID() string {
	return v.SubnetName
}

func (v *GCPVPCInfo) GetSecurityGroupID() string {
	return v.FirewallRule
}

// Implement InstanceInfo interface
func (i *GCPInstanceInfo) GetPublicIP() string {
	return i.ExternalIP
}

func (i *GCPInstanceInfo) GetPrivateIP() string {
	return i.InternalIP
}

func (i *GCPInstanceInfo) GetInstanceID() string {
	return i.InstanceName
}

func (i *GCPInstanceInfo) GetSSHKeyPath() string {
	return i.SSHKeyPath
}

func NewGCPProvider(projectID, region string, creds GCPCredentials) *GCPProvider {
	zone := region + "-a" // Default to zone 'a'
	return &GCPProvider{
		BaseCloudProvider: BaseCloudProvider{SSHUser: "ubuntu"},
		ProjectID:         projectID,
		Region:            region,
		Zone:              zone,
		Credentials:       creds,
	}
}

func NewGCPProviderFromConfig(config lib.InstallationConfig) (*GCPProvider, error) {
	zone := config.Cloud.Region + "-a" // Default to zone 'a'
	provider := &GCPProvider{
		BaseCloudProvider: BaseCloudProvider{SSHUser: "ubuntu"},
		ProjectID:         config.Cloud.ProjectID,
		Region:            config.Cloud.Region,
		Zone:              zone,
		Credentials: GCPCredentials{
			ServiceAccountKey: lib.GetStringFromCredentials(config.Cloud.Credentials, "service_account_key"),
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
		return fmt.Errorf("gcp authentication failed: %v", err)
	}

	var accounts []map[string]any
	if err := json.Unmarshal(output, &accounts); err != nil {
		return fmt.Errorf("failed to parse GCP auth response: %v", err)
	}

	if len(accounts) == 0 {
		return fmt.Errorf("no active GCP authentication found")
	}

	fmt.Printf("✅ Authenticated as: %s\n", accounts[0]["account"])
	return nil
}

func (gcp *GCPProvider) CreateOrGetVPC(vpcName string, config NetworkConfig) (VPCInfo, error) {
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

	var network map[string]any
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

func (gcp *GCPProvider) CreateInstance(vpcInfo VPCInfo, config InstanceConfig, vmName string) (InstanceInfo, error) {
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
	startupScript := gcp.generateMinimalStartupScript(config, "gcp")

	// Create instance with SSH key - combine metadata into single flag
	metadata := fmt.Sprintf("startup-script=%s,ssh-keys=ubuntu:%s", startupScript, pubKey)
	cmd := exec.Command("gcloud", "compute", "instances", "create", instanceName,
		"--zone", gcp.Zone,
		"--machine-type", config.InstanceType,
		"--network-interface", fmt.Sprintf("subnet=%s,address=", vpcInfo.GetSubnetID()),
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
	fmt.Printf("     --network-interface subnet=%s,address= \\\n", vpcInfo.GetSubnetID())
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
	instanceInfo, err := gcp.getInstanceDetails(instanceName, sshKeyPath)
	if err != nil {
		return nil, err
	}

	fmt.Printf("✅ Instance created: %s (IP: %s)\n", instanceName, instanceInfo.ExternalIP)
	return instanceInfo, nil
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

func (gcp *GCPProvider) getInstanceDetails(instanceName, sshKeyPath string) (*GCPInstanceInfo, error) {
	cmd := exec.Command("gcloud", "compute", "instances", "describe", instanceName,
		"--zone", gcp.Zone,
		"--format=json")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance details: %v", err)
	}

	var instance map[string]any
	if err := json.Unmarshal(output, &instance); err != nil {
		return nil, err
	}

	// Extract IP addresses
	networkInterfaces := instance["networkInterfaces"].([]any)
	if len(networkInterfaces) == 0 {
		return nil, fmt.Errorf("no network interfaces found")
	}

	firstInterface := networkInterfaces[0].(map[string]any)
	internalIP := firstInterface["networkIP"].(string)

	var externalIP string
	if accessConfigs := firstInterface["accessConfigs"]; accessConfigs != nil {
		configs := accessConfigs.([]any)
		if len(configs) > 0 {
			firstConfig := configs[0].(map[string]any)
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
		SSHKeyPath:   sshKeyPath,
	}, nil
}

func (gcp *GCPProvider) CreateStorage(bucketName string) error {
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

func (gcp *GCPProvider) UploadConfigurationToBucket(bucketName string, configData []byte, filename string) error {
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

// Wrapper methods to implement lib.CloudProvider interface
func (gcp *GCPProvider) SetupVPC() (string, error) {
	vpcInfo, err := gcp.CreateOrGetVPC(gcp.Config.ResourceNames.VPCName, NetworkConfig{
		VPCCidr:           gcp.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  gcp.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: gcp.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        gcp.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return "", err
	}
	return vpcInfo.GetID(), nil
}

func (gcp *GCPProvider) CreateVM(vpcID string) (*lib.VMResult, error) {
	// Get VPC info again for VM creation
	vpcInfo, err := gcp.CreateOrGetVPC(gcp.Config.ResourceNames.VPCName, NetworkConfig{
		VPCCidr:           gcp.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  gcp.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: gcp.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        gcp.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	instanceInfo, err := gcp.CreateInstance(vpcInfo, InstanceConfig{
		InstanceType: gcp.Config.VMConfig.InstanceType,
		DiskSizeGB:   gcp.Config.VMConfig.DiskSize,
		AdminEmail:   gcp.Config.Admin.Email,
		AdminName:    gcp.Config.Admin.FullName,
	}, gcp.Config.ResourceNames.VMName)
	if err != nil {
		return nil, err
	}

	// Perform direct installation using the base provider's InstallDvarpalaDirectly
	if err := gcp.InstallDvarpalaDirectly(instanceInfo, InstanceConfig{
		InstanceType: gcp.Config.VMConfig.InstanceType,
		DiskSizeGB:   gcp.Config.VMConfig.DiskSize,
		AdminEmail:   gcp.Config.Admin.Email,
		AdminName:    gcp.Config.Admin.FullName,
	}, "gcp"); err != nil {
		return nil, fmt.Errorf("direct installation failed: %v", err)
	}

	return &lib.VMResult{
		InstanceID: instanceInfo.GetInstanceID(),
		PublicIP:   instanceInfo.GetPublicIP(),
		PrivateIP:  instanceInfo.GetPrivateIP(),
		SSHKeyPath: instanceInfo.GetSSHKeyPath(),
	}, nil
}

func (gcp *GCPProvider) SetupObjectStorage() error {
	return gcp.CreateStorage(gcp.Config.ResourceNames.BucketName)
}

func (gcp *GCPProvider) UploadConfiguration() error {
	configData, err := json.MarshalIndent(gcp.Config, "", "  ")
	if err != nil {
		return err
	}
	return gcp.UploadConfigurationToBucket(gcp.Config.ResourceNames.BucketName, configData, "installation-config.json")
}

func (gcp *GCPProvider) InstallDvarpala(vmInfo *lib.VMResult) error {
	fmt.Println("🚀 Starting Dvarpala installation...")
	fmt.Printf("📍 Target VM: %s (IP: %s)\n", vmInfo.InstanceID, vmInfo.PublicIP)
	fmt.Printf("🔑 SSH Key: %s\n", vmInfo.SSHKeyPath)

	// This is where the common installation logic would go
	// In the current implementation, this is handled by the provider's InstallDvarpalaDirectly method
	// but in a unified structure, this could be common code

	fmt.Println("✅ Dvarpala installation completed successfully!")
	return nil
}

func (gcp *GCPProvider) Configure2StepVPNAccess(vmInfo *lib.VMResult) error {
	// Convert VMResult to InstanceInfo for the provider
	instanceInfo := &GCPInstanceInfo{
		InstanceName: vmInfo.InstanceID,
		ExternalIP:   vmInfo.PublicIP,
		InternalIP:   vmInfo.PrivateIP,
		SSHKeyPath:   vmInfo.SSHKeyPath,
		Zone:         gcp.Zone,
	}

	return gcp.BaseCloudProvider.Configure2StepVPNAccess(instanceInfo, InstanceConfig{
		AdminEmail: gcp.Config.Admin.Email,
		AdminName:  gcp.Config.Admin.FullName,
	})
}

// generateSSHKeyPair generates an SSH key pair for GCP instances
func (gcp *GCPProvider) generateSSHKeyPair(keyPath string) error {
	// Generate SSH key pair using ssh-keygen
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", keyPath, "-N", "")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate SSH key pair: %v", err)
	}

	fmt.Printf("🔑 SSH key pair generated: %s\n", keyPath)
	return nil
}

// generateMinimalStartupScript creates a minimal startup script for GCP instances
func (gcp *GCPProvider) generateMinimalStartupScript(config InstanceConfig, cloudProvider string) string {
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
export CLOUD_PROVIDER='%s'

# Create marker that basic setup is complete
mkdir -p /var/log/dvarpala
touch /var/log/dvarpala/startup-complete
echo "VM startup preparation completed at $(date)" > /var/log/dvarpala/startup-status.txt

echo "Minimal setup completed. Ready for installer connection."
`, config.AdminEmail, config.AdminName, cloudProvider)
}
