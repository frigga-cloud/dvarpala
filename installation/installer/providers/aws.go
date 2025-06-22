package providers

import (
	"dvarpala-cloud-installer/installer/lib"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type AWSProvider struct {
	BaseCloudProvider
	Region      string
	Credentials AWSCredentials
	Config      lib.InstallationConfig
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

// Implement VPCInfo interface
func (v *AWSVPCInfo) GetID() string {
	return v.VPCID
}

func (v *AWSVPCInfo) GetSubnetID() string {
	return v.PublicSubnetID
}

func (v *AWSVPCInfo) GetSecurityGroupID() string {
	return v.SecurityGroupID
}

type AWSInstanceInfo struct {
	InstanceID      string
	PublicIP        string
	PrivateIP       string
	KeyPairName     string
	SecurityGroupID string
}

// Implement InstanceInfo interface
func (i *AWSInstanceInfo) GetPublicIP() string {
	return i.PublicIP
}

func (i *AWSInstanceInfo) GetPrivateIP() string {
	return i.PrivateIP
}

func (i *AWSInstanceInfo) GetInstanceID() string {
	return i.InstanceID
}

func (i *AWSInstanceInfo) GetSSHKeyPath() string {
	// AWS stores the key pair name, need to construct path
	return fmt.Sprintf("./dvarpala-deployment/%s.pem", i.KeyPairName)
}

func NewAWSProvider(region string, creds AWSCredentials) *AWSProvider {
	return &AWSProvider{
		Region:      region,
		Credentials: creds,
	}
}

func NewAWSProviderFromConfig(config lib.InstallationConfig) (*AWSProvider, error) {
	provider := &AWSProvider{
		Region: config.Cloud.Region,
		Credentials: AWSCredentials{
			AccessKeyID:     lib.GetStringFromCredentials(config.Cloud.Credentials, "access_key"),
			SecretAccessKey: lib.GetStringFromCredentials(config.Cloud.Credentials, "secret_key"),
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
	fmt.Printf("🔍 Validating AWS authentication in region %s...\n", aws.Region)
	
	cmd := exec.Command("aws", "sts", "get-caller-identity")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ AWS CLI command failed: %v\n", err)
		fmt.Printf("📋 Raw output: %s\n", string(output))
		return fmt.Errorf("AWS authentication failed: %v\nOutput: %s", err, output)
	}

	fmt.Printf("🔍 AWS STS response received, parsing identity...\n")
	var identity map[string]interface{}
	if err := json.Unmarshal(output, &identity); err != nil {
		fmt.Printf("❌ Failed to parse AWS identity JSON: %v\n", err)
		fmt.Printf("📋 Raw response: %s\n", string(output))
		return fmt.Errorf("failed to parse AWS identity response: %v", err)
	}

	fmt.Printf("✅ Authenticated as: %s\n", identity["Arn"])
	fmt.Printf("🆔 Account ID: %s\n", identity["Account"])
	return nil
}

func (aws *AWSProvider) CreateOrGetVPC(vpcName string, config NetworkConfig) (VPCInfo, error) {
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
	fmt.Printf("🔍 Searching for existing VPC with name: %s\n", vpcName)
	
	cmd := exec.Command("aws", "ec2", "describe-vpcs",
		"--filters", fmt.Sprintf("Name=tag:Name,Values=%s", vpcName),
		"--query", "Vpcs[0].VpcId",
		"--output", "text")

	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("❌ Failed to search for VPC: %v\n", err)
		return nil, err
	}

	vpcID := strings.TrimSpace(string(output))
	fmt.Printf("🔍 VPC search result: %s\n", vpcID)
	
	if vpcID == "None" || vpcID == "" {
		fmt.Printf("📋 No existing VPC found with name: %s\n", vpcName)
		return nil, nil
	}

	fmt.Printf("✅ Found existing VPC: %s\n", vpcID)
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

func (aws *AWSProvider) CreateInstance(vpcInfo VPCInfo, config InstanceConfig, vmName string) (InstanceInfo, error) {
	fmt.Printf("🚀 Starting AWS instance creation for VM: %s\n", vmName)
	fmt.Printf("📋 Instance type: %s, Disk size: %dGB\n", config.InstanceType, config.DiskSizeGB)
	
	// Use Frigga Labs naming convention for key pair
	keyPairName := vmName + "-keypair"
	fmt.Printf("🔑 Creating SSH key pair: %s\n", keyPairName)

	// Create key pair
	cmd := exec.Command("aws", "ec2", "create-key-pair",
		"--key-name", keyPairName,
		"--query", "KeyMaterial",
		"--output", "text")

	keyMaterial, err := cmd.Output()
	if err != nil {
		fmt.Printf("❌ Failed to create AWS key pair: %v\n", err)
		return nil, fmt.Errorf("failed to create key pair: %v", err)
	}
	
	fmt.Printf("✅ SSH key pair created successfully\n")

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
	fmt.Printf("🔍 Finding latest Ubuntu 22.04 AMI...\n")
	amiID, err := aws.getLatestUbuntuAMI()
	if err != nil {
		fmt.Printf("❌ Failed to find Ubuntu AMI: %v\n", err)
		return nil, err
	}
	fmt.Printf("✅ Using Ubuntu AMI: %s\n", amiID)

	// Create minimal user data script - just basic system prep
	fmt.Printf("📝 Generating minimal startup script...\n")
	userData := aws.generateMinimalUserData(config)

	// Launch instance
	fmt.Printf("🚀 Launching EC2 instance...\n")
	fmt.Printf("📋 VPC: %s, Subnet: %s, Security Group: %s\n", 
		vpcInfo.GetID(), vpcInfo.GetSubnetID(), vpcInfo.GetSecurityGroupID())
	
	cmd = exec.Command("aws", "ec2", "run-instances",
		"--image-id", amiID,
		"--count", "1",
		"--instance-type", config.InstanceType,
		"--key-name", keyPairName,
		"--security-group-ids", vpcInfo.GetSecurityGroupID(),
		"--subnet-id", vpcInfo.GetSubnetID(),
		"--user-data", userData,
		"--block-device-mappings", fmt.Sprintf(`[{"DeviceName":"/dev/sda1","Ebs":{"VolumeSize":%d,"VolumeType":"gp3","DeleteOnTermination":true}}]`, config.DiskSizeGB),
		"--tag-specifications", fmt.Sprintf(`ResourceType=instance,Tags=[{Key=Name,Value=dvarpala-server},{Key=Project,Value=dvarpala},{Key=ManagedBy,Value=frigga-labs}]`),
		"--query", "Instances[0].InstanceId",
		"--output", "text")

	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("❌ Failed to launch EC2 instance: %v\n", err)
		return nil, fmt.Errorf("failed to launch instance: %v", err)
	}

	instanceID := strings.TrimSpace(string(output))
	fmt.Printf("✅ EC2 instance launched with ID: %s\n", instanceID)

	// Wait for instance to be running
	fmt.Printf("⏳ Waiting for instance %s to be running...\n", instanceID)
	cmd = exec.Command("aws", "ec2", "wait", "instance-running", "--instance-ids", instanceID)
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Timeout waiting for instance to be running: %v\n", err)
		return nil, fmt.Errorf("timeout waiting for instance to be running: %v", err)
	}
	fmt.Printf("✅ Instance is now running\n")

	// Get instance details
	fmt.Printf("🔍 Retrieving instance network details...\n")
	instanceInfo, err := aws.getInstanceDetails(instanceID)
	if err != nil {
		fmt.Printf("❌ Failed to get instance details: %v\n", err)
		return nil, err
	}

	instanceInfo.KeyPairName = keyPairName
	instanceInfo.SecurityGroupID = vpcInfo.GetSecurityGroupID()

	fmt.Printf("✅ Instance launched: %s (Public IP: %s, Private IP: %s)\n", 
		instanceID, instanceInfo.PublicIP, instanceInfo.PrivateIP)
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
	// Use friggalabs as the standard bucket name
	friggaBucketName := "friggalabs"
	
	// Check if bucket exists
	cmd := exec.Command("aws", "s3api", "head-bucket", "--bucket", friggaBucketName)
	if cmd.Run() == nil {
		fmt.Printf("✅ Using existing S3 bucket: %s\n", friggaBucketName)
		// Ensure dvarpala folder exists
		exec.Command("aws", "s3api", "put-object", "--bucket", friggaBucketName, "--key", "dvarpala/").Run()
		return nil
	}

	// Create bucket
	cmd = exec.Command("aws", "s3api", "create-bucket", "--bucket", friggaBucketName)
	if aws.Region != "us-east-1" {
		cmd.Args = append(cmd.Args, "--create-bucket-configuration", fmt.Sprintf("LocationConstraint=%s", aws.Region))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create S3 bucket: %v", err)
	}

	// Enable versioning
	exec.Command("aws", "s3api", "put-bucket-versioning",
		"--bucket", friggaBucketName,
		"--versioning-configuration", "Status=Enabled").Run()

	// Create dvarpala folder
	cmd = exec.Command("aws", "s3api", "put-object",
		"--bucket", friggaBucketName,
		"--key", "dvarpala/")
	cmd.Run()

	fmt.Printf("✅ S3 bucket created: %s\n", friggaBucketName)
	return nil
}

func (aws *AWSProvider) UploadConfigurationToBucket(bucketName string, configData []byte, filename string) error {
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

	// Upload to S3 in friggalabs bucket, dvarpala directory
	cmd := exec.Command("aws", "s3", "cp", tempFile, fmt.Sprintf("s3://friggalabs/dvarpala/%s", filename))
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
}

// Wrapper methods to implement lib.CloudProvider interface
func (aws *AWSProvider) SetupVPC() (string, error) {
	vpcInfo, err := aws.CreateOrGetVPC(aws.Config.ResourceNames.VPCName, NetworkConfig{
		VPCCidr:           aws.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  aws.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: aws.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        aws.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return "", err
	}
	return vpcInfo.GetID(), nil
}

func (aws *AWSProvider) CreateVM(vpcID string) (*lib.VMResult, error) {
	// Get VPC info again for VM creation
	vpcInfo, err := aws.CreateOrGetVPC(aws.Config.ResourceNames.VPCName, NetworkConfig{
		VPCCidr:           aws.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  aws.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: aws.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        aws.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	instanceInfo, err := aws.CreateInstance(vpcInfo, InstanceConfig{
		InstanceType: aws.Config.VMConfig.InstanceType,
		DiskSizeGB:   aws.Config.VMConfig.DiskSize,
		AdminEmail:   aws.Config.Admin.Email,
		AdminName:    aws.Config.Admin.FullName,
	}, aws.Config.ResourceNames.VMName)
	if err != nil {
		return nil, err
	}

	// Perform direct installation using the base provider's InstallDvarpalaDirectly
	if err := aws.BaseCloudProvider.InstallDvarpalaDirectly(instanceInfo, InstanceConfig{
		InstanceType: aws.Config.VMConfig.InstanceType,
		DiskSizeGB:   aws.Config.VMConfig.DiskSize,
		AdminEmail:   aws.Config.Admin.Email,
		AdminName:    aws.Config.Admin.FullName,
	}, "aws"); err != nil {
		return nil, fmt.Errorf("direct installation failed: %v", err)
	}

	return &lib.VMResult{
		InstanceID: instanceInfo.GetInstanceID(),
		PublicIP:   instanceInfo.GetPublicIP(),
		PrivateIP:  instanceInfo.GetPrivateIP(),
		SSHKeyPath: instanceInfo.GetSSHKeyPath(),
	}, nil
}

func (aws *AWSProvider) SetupObjectStorage() error {
	return aws.CreateS3Bucket(aws.Config.ResourceNames.BucketName)
}

func (aws *AWSProvider) UploadConfiguration() error {
	configData, err := json.MarshalIndent(aws.Config, "", "  ")
	if err != nil {
		return err
	}
	return aws.UploadConfigurationToBucket(aws.Config.ResourceNames.BucketName, configData, "installation-config.json")
}

func (aws *AWSProvider) InstallDvarpala(vmInfo *lib.VMResult) error {
	fmt.Println("🚀 Starting Dvarpala installation...")
	fmt.Printf("📍 Target VM: %s (IP: %s)\n", vmInfo.InstanceID, vmInfo.PublicIP)
	fmt.Printf("🔑 SSH Key: %s\n", vmInfo.SSHKeyPath)

	// This is where the common installation logic would go
	// In the current implementation, this is handled by the provider's InstallDvarpalaDirectly method
	// but in a unified structure, this could be common code

	fmt.Println("✅ Dvarpala installation completed successfully!")
	return nil
}

func (aws *AWSProvider) Configure2StepVPNAccess(vmInfo *lib.VMResult) error {
	// Convert VMResult to InstanceInfo for the provider
	instanceInfo := &AWSInstanceInfo{
		InstanceID:      vmInfo.InstanceID,
		PublicIP:        vmInfo.PublicIP,
		PrivateIP:       vmInfo.PrivateIP,
		KeyPairName:     vmInfo.SSHKeyPath,
		SecurityGroupID: "", // Not needed for this operation
	}

	return aws.BaseCloudProvider.Configure2StepVPNAccess(instanceInfo, InstanceConfig{
		AdminEmail: aws.Config.Admin.Email,
		AdminName:  aws.Config.Admin.FullName,
	})
}
