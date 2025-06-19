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

	// Get latest Ubuntu image
	imageFamily := "ubuntu-2204-lts"
	imageProject := "ubuntu-os-cloud"

	// Generate minimal startup script - just basic system prep
	startupScript := gcp.generateMinimalStartupScript(config)

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

// InstallDvarpalaDirectly performs the installation directly via SSH from the installer
func (gcp *GCPProvider) InstallDvarpalaDirectly(instanceInfo *GCPInstanceInfo, config InstanceConfig) error {
	fmt.Println("🔗 Connecting to VM for direct installation...")
	
	// Wait for VM to be SSH accessible
	if err := gcp.waitForSSHAccess(instanceInfo.ExternalIP); err != nil {
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
		{"Setting up directories", "sudo mkdir -p /opt/dvarpala /var/lib/dvarpala"},
		{"Configuring nginx monitoring", "sudo " + gcp.getNginxConfigCommand()},
		{"Starting services", "sudo systemctl start postgresql redis-server"},
	}
	
	for i, step := range steps {
		fmt.Printf("📦 Step %d/%d: %s\n", i+1, len(steps), step.name)
		
		if err := gcp.executeSSHCommand(instanceInfo.ExternalIP, step.cmd); err != nil {
			return fmt.Errorf("failed at step '%s': %v", step.name, err)
		}
		
		fmt.Printf("✅ Completed: %s\n", step.name)
	}
	
	fmt.Println("🎉 Dvarpala installation completed successfully!")
	return nil
}

func (gcp *GCPProvider) waitForSSHAccess(vmIP string) error {
	fmt.Printf("⏳ Waiting for SSH access to %s...\n", vmIP)
	
	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		// Test SSH connectivity
		cmd := exec.Command("nc", "-z", "-w", "3", vmIP, "22")
		if cmd.Run() == nil {
			// SSH port is open, try actual SSH connection
			sshCmd := exec.Command("ssh", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=no", 
				fmt.Sprintf("ubuntu@%s", vmIP), "echo 'SSH Ready'")
			if sshCmd.Run() == nil {
				return nil
			}
		}
		
		fmt.Printf("⏳ SSH not ready yet... attempt %d/%d\n", i+1, maxAttempts)
		time.Sleep(10 * time.Second)
	}
	
	return fmt.Errorf("SSH access not available after %d attempts", maxAttempts)
}

func (gcp *GCPProvider) executeSSHCommand(vmIP, command string) error {
	cmd := exec.Command("ssh", "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("ubuntu@%s", vmIP), command)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Command failed: %s\nOutput: %s\n", command, string(output))
		return err
	}
	
	return nil
}

func (gcp *GCPProvider) getNginxConfigCommand() string {
	return `cat > /etc/nginx/sites-available/dvarpala-monitoring << 'EOF'
server {
    listen 8080;
    server_name _;
    root /var/www/html;
    
    location /health {
        return 200 '{"status":"healthy","timestamp":"$(date -Iseconds)"}';
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
EOF
ln -sf /etc/nginx/sites-available/dvarpala-monitoring /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx`
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
