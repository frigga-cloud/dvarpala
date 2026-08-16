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

type AWSProvider struct {
	Region      string
	Credentials AWSCredentials
}

type AWSCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token,omitempty"`
}

type AWSVPCInfo struct {
	VPCID           string
	PublicSubnetID  string
	PrivateSubnetID string
	InternetGateway string
	SecurityGroupID string
	RouteTableID    string
}

type AWSInstanceInfo struct {
	InstanceID      string
	PublicIP        string
	PrivateIP       string
	KeyPairName     string
	SecurityGroupID string
}

func NewAWSProvider(region string, creds AWSCredentials) *AWSProvider {
	return &AWSProvider{
		Region:      region,
		Credentials: creds,
	}
}

func (aws *AWSProvider) SetupEnvironment() error {
	if aws.Credentials.AccessKeyID != "" {
		os.Setenv("AWS_ACCESS_KEY_ID", aws.Credentials.AccessKeyID)
		os.Setenv("AWS_SECRET_ACCESS_KEY", aws.Credentials.SecretAccessKey)
		if aws.Credentials.SessionToken != "" {
			os.Setenv("AWS_SESSION_TOKEN", aws.Credentials.SessionToken)
		}
	}
	os.Setenv("AWS_DEFAULT_REGION", aws.Region)
	return nil
}

func (aws *AWSProvider) ValidateAuthentication() error {
	cmd := exec.Command("aws", "sts", "get-caller-identity")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("AWS authentication failed: %v\nOutput: %s", err, output)
	}

	var identity map[string]interface{}
	if err := json.Unmarshal(output, &identity); err != nil {
		return fmt.Errorf("failed to parse AWS identity response: %v", err)
	}

	fmt.Printf("✅ Authenticated as: %s\n", identity["Arn"])
	return nil
}

func (aws *AWSProvider) CreateOrGetVPC(vpcName string, config NetworkConfig) (*AWSVPCInfo, error) {
	// Check if VPC already exists
	existingVPC, err := aws.findVPCByName(vpcName)
	if err != nil {
		return nil, err
	}

	if existingVPC != nil {
		fmt.Printf("✅ Using existing VPC: %s\n", existingVPC.VPCID)
		return existingVPC, nil
	}

	fmt.Printf("🏗️ Creating new VPC: %s\n", vpcName)
	return aws.createNewVPC(vpcName, config)
}

func (aws *AWSProvider) findVPCByName(vpcName string) (*AWSVPCInfo, error) {
	cmd := exec.Command("aws", "ec2", "describe-vpcs",
		"--filters", fmt.Sprintf("Name=tag:Name,Values=%s", vpcName),
		"--query", "Vpcs[0].VpcId",
		"--output", "text")

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	vpcID := strings.TrimSpace(string(output))
	if vpcID == "None" || vpcID == "" {
		return nil, nil
	}

	// Get VPC details
	return aws.getVPCDetails(vpcID)
}

func (aws *AWSProvider) getVPCDetails(vpcID string) (*AWSVPCInfo, error) {
	vpcInfo := &AWSVPCInfo{VPCID: vpcID}

	// Get subnets
	cmd := exec.Command("aws", "ec2", "describe-subnets",
		"--filters", fmt.Sprintf("Name=vpc-id,Values=%s", vpcID),
		"--query", "Subnets[?MapPublicIpOnLaunch==`true`].SubnetId",
		"--output", "text")

	if output, err := cmd.Output(); err == nil {
		if subnetID := strings.TrimSpace(string(output)); subnetID != "" {
			vpcInfo.PublicSubnetID = subnetID
		}
	}

	// Get security groups
	cmd = exec.Command("aws", "ec2", "describe-security-groups",
		"--filters", fmt.Sprintf("Name=vpc-id,Values=%s", vpcID),
		fmt.Sprintf("Name=group-name,Values=%s-dvarpala-sg", "frigga-labs"),
		"--query", "SecurityGroups[0].GroupId",
		"--output", "text")

	if output, err := cmd.Output(); err == nil {
		if sgID := strings.TrimSpace(string(output)); sgID != "None" && sgID != "" {
			vpcInfo.SecurityGroupID = sgID
		}
	}

	return vpcInfo, nil
}

func (aws *AWSProvider) createNewVPC(vpcName string, config NetworkConfig) (*AWSVPCInfo, error) {
	vpcInfo := &AWSVPCInfo{}

	// Create VPC
	cmd := exec.Command("aws", "ec2", "create-vpc",
		"--cidr-block", config.VPCCidr,
		"--query", "Vpc.VpcId",
		"--output", "text")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC: %v", err)
	}
	vpcInfo.VPCID = strings.TrimSpace(string(output))

	// Tag VPC
	aws.tagResource(vpcInfo.VPCID, vpcName, "VPC")

	// Enable DNS hostname and resolution
	exec.Command("aws", "ec2", "modify-vpc-attribute",
		"--vpc-id", vpcInfo.VPCID,
		"--enable-dns-hostnames").Run()
	exec.Command("aws", "ec2", "modify-vpc-attribute",
		"--vpc-id", vpcInfo.VPCID,
		"--enable-dns-support").Run()

	// Create Internet Gateway
	cmd = exec.Command("aws", "ec2", "create-internet-gateway",
		"--query", "InternetGateway.InternetGatewayId",
		"--output", "text")

	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create internet gateway: %v", err)
	}
	vpcInfo.InternetGateway = strings.TrimSpace(string(output))

	// Tag and attach Internet Gateway
	aws.tagResource(vpcInfo.InternetGateway, vpcName+"-igw", "Internet Gateway")
	exec.Command("aws", "ec2", "attach-internet-gateway",
		"--vpc-id", vpcInfo.VPCID,
		"--internet-gateway-id", vpcInfo.InternetGateway).Run()

	// Create Public Subnet
	cmd = exec.Command("aws", "ec2", "create-subnet",
		"--vpc-id", vpcInfo.VPCID,
		"--cidr-block", config.PublicSubnetCidr,
		"--availability-zone", aws.Region+"a",
		"--query", "Subnet.SubnetId",
		"--output", "text")

	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create public subnet: %v", err)
	}
	vpcInfo.PublicSubnetID = strings.TrimSpace(string(output))

	// Tag subnet and enable auto-assign public IP
	aws.tagResource(vpcInfo.PublicSubnetID, vpcName+"-public", "Public Subnet")
	exec.Command("aws", "ec2", "modify-subnet-attribute",
		"--subnet-id", vpcInfo.PublicSubnetID,
		"--map-public-ip-on-launch").Run()

	// Create Private Subnet
	cmd = exec.Command("aws", "ec2", "create-subnet",
		"--vpc-id", vpcInfo.VPCID,
		"--cidr-block", config.PrivateSubnetCidr,
		"--availability-zone", aws.Region+"b",
		"--query", "Subnet.SubnetId",
		"--output", "text")

	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create private subnet: %v", err)
	}
	vpcInfo.PrivateSubnetID = strings.TrimSpace(string(output))
	aws.tagResource(vpcInfo.PrivateSubnetID, vpcName+"-private", "Private Subnet")

	// Create Route Table for public subnet
	cmd = exec.Command("aws", "ec2", "create-route-table",
		"--vpc-id", vpcInfo.VPCID,
		"--query", "RouteTable.RouteTableId",
		"--output", "text")

	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create route table: %v", err)
	}
	vpcInfo.RouteTableID = strings.TrimSpace(string(output))

	// Tag route table and add route to internet gateway
	aws.tagResource(vpcInfo.RouteTableID, vpcName+"-public-rt", "Route Table")
	exec.Command("aws", "ec2", "create-route",
		"--route-table-id", vpcInfo.RouteTableID,
		"--destination-cidr-block", "0.0.0.0/0",
		"--gateway-id", vpcInfo.InternetGateway).Run()

	// Associate route table with public subnet
	exec.Command("aws", "ec2", "associate-route-table",
		"--subnet-id", vpcInfo.PublicSubnetID,
		"--route-table-id", vpcInfo.RouteTableID).Run()

	// Create Security Group
	cmd = exec.Command("aws", "ec2", "create-security-group",
		"--group-name", vpcName+"-dvarpala-sg",
		"--description", "Security group for Dvarpala VPN server",
		"--vpc-id", vpcInfo.VPCID,
		"--query", "GroupId",
		"--output", "text")

	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create security group: %v", err)
	}
	vpcInfo.SecurityGroupID = strings.TrimSpace(string(output))
	aws.tagResource(vpcInfo.SecurityGroupID, vpcName+"-sg", "Security Group")

	// Add security group rules
	aws.addSecurityGroupRules(vpcInfo.SecurityGroupID, config.AllowedIPs)

	fmt.Printf("✅ VPC created successfully: %s\n", vpcInfo.VPCID)
	return vpcInfo, nil
}

func (aws *AWSProvider) addSecurityGroupRules(sgID string, allowedIPs []string) {
	// SSH access (restricted after installation)
	exec.Command("aws", "ec2", "authorize-security-group-ingress",
		"--group-id", sgID,
		"--protocol", "tcp",
		"--port", "22",
		"--cidr", "0.0.0.0/0").Run()

	// OpenVPN port
	exec.Command("aws", "ec2", "authorize-security-group-ingress",
		"--group-id", sgID,
		"--protocol", "udp",
		"--port", "1194",
		"--cidr", "0.0.0.0/0").Run()

	// Dvarpala web interface
	exec.Command("aws", "ec2", "authorize-security-group-ingress",
		"--group-id", sgID,
		"--protocol", "tcp",
		"--port", "8080",
		"--cidr", "0.0.0.0/0").Run()

	// HTTPS for Let's Encrypt (optional)
	exec.Command("aws", "ec2", "authorize-security-group-ingress",
		"--group-id", sgID,
		"--protocol", "tcp",
		"--port", "443",
		"--cidr", "0.0.0.0/0").Run()
}

func (aws *AWSProvider) CreateInstance(vpcInfo *AWSVPCInfo, config InstanceConfig, vmName string) (*AWSInstanceInfo, error) {
	// Use Frigga Labs naming convention for key pair
	keyPairName := vmName + "-keypair"

	// Create key pair
	cmd := exec.Command("aws", "ec2", "create-key-pair",
		"--key-name", keyPairName,
		"--query", "KeyMaterial",
		"--output", "text")

	keyMaterial, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create key pair: %v", err)
	}

	// Create deployment directory if it doesn't exist
	deploymentDir := "./dvarpala-deployment"
	if err := os.MkdirAll(deploymentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create deployment directory: %v", err)
	}

	// Save private key to deployment directory
	keyPath := fmt.Sprintf("%s/%s.pem", deploymentDir, keyPairName)
	if err := os.WriteFile(keyPath, keyMaterial, 0600); err != nil {
		return nil, fmt.Errorf("failed to save private key: %v", err)
	}

	fmt.Printf("🔑 SSH private key saved to: %s\n", keyPath)

	// Get latest Ubuntu AMI
	amiID, err := aws.getLatestUbuntuAMI()
	if err != nil {
		return nil, err
	}

	// Create minimal user data script - just basic system prep
	userData := aws.generateMinimalUserData(config)

	// Launch instance
	cmd = exec.Command("aws", "ec2", "run-instances",
		"--image-id", amiID,
		"--count", "1",
		"--instance-type", config.InstanceType,
		"--key-name", keyPairName,
		"--security-group-ids", vpcInfo.SecurityGroupID,
		"--subnet-id", vpcInfo.PublicSubnetID,
		"--user-data", userData,
		"--block-device-mappings", fmt.Sprintf(`[{"DeviceName":"/dev/sda1","Ebs":{"VolumeSize":%d,"VolumeType":"gp3","DeleteOnTermination":true}}]`, config.DiskSizeGB),
		"--tag-specifications", fmt.Sprintf(`ResourceType=instance,Tags=[{Key=Name,Value=dvarpala-server},{Key=Project,Value=dvarpala},{Key=ManagedBy,Value=frigga-labs}]`),
		"--query", "Instances[0].InstanceId",
		"--output", "text")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to launch instance: %v", err)
	}

	instanceID := strings.TrimSpace(string(output))

	// Wait for instance to be running
	fmt.Printf("⏳ Waiting for instance %s to be running...\n", instanceID)
	cmd = exec.Command("aws", "ec2", "wait", "instance-running", "--instance-ids", instanceID)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("timeout waiting for instance to be running: %v", err)
	}

	// Get instance details
	instanceInfo, err := aws.getInstanceDetails(instanceID)
	if err != nil {
		return nil, err
	}

	instanceInfo.KeyPairName = keyPairName
	instanceInfo.SecurityGroupID = vpcInfo.SecurityGroupID

	fmt.Printf("✅ Instance launched: %s (IP: %s)\n", instanceID, instanceInfo.PublicIP)
	return instanceInfo, nil
}

func (aws *AWSProvider) getLatestUbuntuAMI() (string, error) {
	cmd := exec.Command("aws", "ec2", "describe-images",
		"--owners", "099720109477", // Canonical
		"--filters",
		"Name=name,Values=ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*",
		"Name=state,Values=available",
		"--query", "Images|sort_by(@, &CreationDate)[-1].ImageId",
		"--output", "text")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get Ubuntu AMI: %v", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (aws *AWSProvider) generateMinimalUserData(config InstanceConfig) string {
	return fmt.Sprintf(`#!/bin/bash
# Minimal AWS Instance Setup Script - Just basic system prep
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
export CLOUD_PROVIDER='aws'

# Create marker that basic setup is complete
mkdir -p /var/log/dvarpala
touch /var/log/dvarpala/startup-complete
echo "VM startup preparation completed at $(date)" > /var/log/dvarpala/startup-status.txt

echo "Minimal setup completed. Ready for installer connection."
`, config.AdminEmail, config.AdminName)
}

// InstallDvarpalaDirectly performs the installation directly via SSH from the installer
func (aws *AWSProvider) InstallDvarpalaDirectly(instanceInfo *AWSInstanceInfo, config InstanceConfig, keyPath string) error {
	fmt.Println("🔗 Connecting to AWS VM for direct installation...")
	fmt.Printf("🔑 Using SSH key: %s\n", keyPath)

	// Wait for VM to be SSH accessible
	if err := aws.waitForSSHAccess(instanceInfo.PublicIP, keyPath); err != nil {
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
		{"Fetching Dvarpala source", "sudo apt-get update -qq && sudo apt-get install -y -qq git && sudo rm -rf /opt/dvarpala/src && sudo git clone --depth 1 https://github.com/frigga-cloud/dvarpala.git /opt/dvarpala/src"},
		{"Installing Dvarpala", "sudo chmod +x /opt/dvarpala/src/scripts/install/install-dvarpala.sh && sudo DVARPALA_ADMIN_EMAIL='" + config.AdminEmail + "' /opt/dvarpala/src/scripts/install/install-dvarpala.sh --source /opt/dvarpala/src --host " + hostForCerts},
	}

	for i, step := range steps {
		fmt.Printf("📦 Step %d/%d: %s\n", i+1, len(steps), step.name)

		if err := aws.executeSSHCommand(instanceInfo.PublicIP, step.cmd, keyPath); err != nil {
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
	} else if err := aws.fetchAdminProfile(instanceInfo.PublicIP, keyPath, config.OutputDir); err != nil {
		fmt.Printf("⚠️  Could not retrieve admin.ovpn: %v\n", err)
		fmt.Printf("   Fetch it later with:\n     scp -i %s ubuntu@%s:/tmp/admin.ovpn .\n",
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
func (aws *AWSProvider) fetchAdminProfile(vmIP, keyPath, outputDir string) error {
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
	if err := aws.executeSSHCommand(vmIP, stage, keyPath); err != nil {
		return fmt.Errorf("staging the profile: %w", err)
	}

	local := filepath.Join(outputDir, "admin.ovpn")
	cmd := exec.Command("scp", "-i", keyPath,
		"-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("ubuntu@%s:%s", vmIP, staged), local)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp: %v: %s", err, strings.TrimSpace(string(out)))
	}

	_ = aws.executeSSHCommand(vmIP, fmt.Sprintf("shred -u %s 2>/dev/null || rm -f %s", staged, staged), keyPath)

	if err := os.Chmod(local, 0o600); err != nil {
		return err
	}
	fmt.Printf("✅ Saved %s (mode 0600 - contains a private key)\n", local)
	return nil
}

func (aws *AWSProvider) waitForSSHAccess(vmIP, keyPath string) error {
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

func (aws *AWSProvider) executeSSHCommand(vmIP, command, keyPath string) error {
	cmd := exec.Command("ssh", "-i", keyPath, "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("ubuntu@%s", vmIP), command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Command failed: %s\nOutput: %s\n", command, string(output))
		return err
	}

	return nil
}

func (aws *AWSProvider) getInstanceDetails(instanceID string) (*AWSInstanceInfo, error) {
	cmd := exec.Command("aws", "ec2", "describe-instances",
		"--instance-ids", instanceID,
		"--query", "Reservations[0].Instances[0].[PublicIpAddress,PrivateIpAddress]",
		"--output", "text")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance details: %v", err)
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid instance details response")
	}

	return &AWSInstanceInfo{
		InstanceID: instanceID,
		PublicIP:   parts[0],
		PrivateIP:  parts[1],
	}, nil
}

func (aws *AWSProvider) CreateS3Bucket(bucketName string) error {
	// Check if bucket exists
	cmd := exec.Command("aws", "s3api", "head-bucket", "--bucket", bucketName)
	if cmd.Run() == nil {
		fmt.Printf("✅ Using existing S3 bucket: %s\n", bucketName)
		return nil
	}

	// Create bucket
	cmd = exec.Command("aws", "s3api", "create-bucket", "--bucket", bucketName)
	if aws.Region != "us-east-1" {
		cmd.Args = append(cmd.Args, "--create-bucket-configuration", fmt.Sprintf("LocationConstraint=%s", aws.Region))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create S3 bucket: %v", err)
	}

	// Enable versioning
	exec.Command("aws", "s3api", "put-bucket-versioning",
		"--bucket", bucketName,
		"--versioning-configuration", "Status=Enabled").Run()

	// Create dvarpala folder
	cmd = exec.Command("aws", "s3api", "put-object",
		"--bucket", bucketName,
		"--key", "dvarpala/")
	cmd.Run()

	fmt.Printf("✅ S3 bucket created: %s\n", bucketName)
	return nil
}

func (aws *AWSProvider) UploadConfiguration(bucketName string, configData []byte, filename string) error {
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

	// Upload to S3
	cmd := exec.Command("aws", "s3", "cp", tempFile, fmt.Sprintf("s3://%s/dvarpala/%s", bucketName, filename))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to upload configuration to S3: %v", err)
	}

	return nil
}

func (aws *AWSProvider) tagResource(resourceID, name, resourceType string) {
	exec.Command("aws", "ec2", "create-tags",
		"--resources", resourceID,
		"--tags", fmt.Sprintf("Key=Name,Value=%s", name),
		fmt.Sprintf("Key=Type,Value=%s", resourceType),
		"Key=Project,Value=dvarpala",
		"Key=ManagedBy,Value=frigga-labs").Run()
}

// Common types used across providers
type NetworkConfig struct {
	VPCCidr           string
	PublicSubnetCidr  string
	PrivateSubnetCidr string
	AllowedIPs        []string
}

type InstanceConfig struct {
	InstanceType string
	DiskSizeGB   int
	AdminEmail   string
	AdminName    string

	// OutputDir is where the operator's copy of admin.ovpn is written. Empty
	// means the current directory.
	OutputDir string
}
