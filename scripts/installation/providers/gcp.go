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
	VPCName         string
	SubnetName      string
	FirewallRule    string
	ExternalIP      string
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

func (gcp *GCPProvider) CreateInstance(vpcInfo *GCPVPCInfo, config InstanceConfig) (*GCPInstanceInfo, error) {
	instanceName := fmt.Sprintf("dvarpala-server-%d", time.Now().Unix())
	
	// Get latest Ubuntu image
	imageFamily := "ubuntu-2204-lts"
	imageProject := "ubuntu-os-cloud"
	
	// Generate startup script
	startupScript := gcp.generateStartupScript(config)
	
	// Create instance
	cmd := exec.Command("gcloud", "compute", "instances", "create", instanceName,
		"--zone", gcp.Zone,
		"--machine-type", config.InstanceType,
		"--network-interface", fmt.Sprintf("subnet=%s,address=", vpcInfo.SubnetName),
		"--image-family", imageFamily,
		"--image-project", imageProject,
		"--boot-disk-size", fmt.Sprintf("%dGB", config.DiskSizeGB),
		"--boot-disk-type", "pd-standard",
		"--boot-disk-device-name", instanceName,
		"--metadata", fmt.Sprintf("startup-script=%s", startupScript),
		"--tags", "dvarpala-server",
		"--labels", "project=dvarpala,managed-by=frigga-labs",
		"--scopes", "https://www.googleapis.com/auth/cloud-platform")
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create instance: %v", err)
	}
	
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

func (gcp *GCPProvider) generateStartupScript(config InstanceConfig) string {
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
	// Write config to temp file
	tempFile := fmt.Sprintf("/tmp/%s", filename)
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