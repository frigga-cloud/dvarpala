package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// awsOutput runs an AWS CLI command and returns its output.
//
// exec.Cmd.Output discards standard error, which is the only place the AWS CLI
// puts the reason for a refusal. Losing it turns every failure into "exit
// status 254" - a number that says a client error occurred and nothing about
// which one, leaving an operator with no way to tell an unavailable instance
// type from a missing permission from a bad subnet.
// exitCode returns the status a failed command exited with, or -1 when the
// failure was something else - the command not being found, or the connection
// dropping. Distinguishing those matters: an installer that reports "needs
// configuring" is not one that crashed.
func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func awsOutput(cmd *exec.Cmd) ([]byte, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}

	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		return out, err
	}
	return out, fmt.Errorf("%s: %s", err, detail)
}

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
	existingVPC, err := aws.findVPCByName(vpcName, config)
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

func (aws *AWSProvider) findVPCByName(vpcName string, config NetworkConfig) (*AWSVPCInfo, error) {
	cmd := exec.Command("aws", "ec2", "describe-vpcs",
		"--filters", fmt.Sprintf("Name=tag:Name,Values=%s", vpcName),
		"--query", "Vpcs[0].VpcId",
		"--output", "text")

	output, err := awsOutput(cmd)
	if err != nil {
		return nil, err
	}

	vpcID := strings.TrimSpace(string(output))
	if vpcID == "None" || vpcID == "" {
		return nil, nil
	}

	// Get VPC details
	return aws.getVPCDetails(vpcName, vpcID, config)
}

// getVPCDetails describes a VPC that already exists, which is what every
// re-run after a failure finds.
//
// It must return either a complete description or an error. Returning a
// half-filled one is how an empty security group id reached RunInstances,
// where AWS refused it with "you must specify a group id for each item" -
// several steps away from the lookup that came back with nothing.
func (aws *AWSProvider) getVPCDetails(vpcName, vpcID string, config NetworkConfig) (*AWSVPCInfo, error) {
	vpcInfo := &AWSVPCInfo{VPCID: vpcID}

	// Get subnets
	cmd := exec.Command("aws", "ec2", "describe-subnets",
		"--filters", fmt.Sprintf("Name=vpc-id,Values=%s", vpcID),
		"--query", "Subnets[?MapPublicIpOnLaunch==`true`].SubnetId",
		"--output", "text")

	if output, err := awsOutput(cmd); err == nil {
		if subnetID := strings.TrimSpace(string(output)); subnetID != "" {
			vpcInfo.PublicSubnetID = subnetID
		}
	}

	if vpcInfo.PublicSubnetID == "" {
		return nil, fmt.Errorf("vpc %s has no public subnet; delete it and run again, "+
			"or pass a different name", vpcID)
	}

	// The group is looked up by the name it was created with. This used to
	// interpolate a hardcoded "frigga-labs" instead of the actual name, so it
	// matched nothing unless the deployment happened to be called that.
	sgID, err := aws.ensureSecurityGroup(vpcName, vpcID, config)
	if err != nil {
		return nil, err
	}
	vpcInfo.SecurityGroupID = sgID

	return vpcInfo, nil
}

// ensureSecurityGroup returns the deployment's security group, creating it if
// it is not there.
//
// Creating it here matters: a run that failed after making the VPC but before
// making the group would otherwise leave a VPC that every later run finds and
// no run can complete.
func (aws *AWSProvider) ensureSecurityGroup(vpcName, vpcID string, config NetworkConfig) (string, error) {
	groupName := vpcName + "-dvarpala-sg"

	cmd := exec.Command("aws", "ec2", "describe-security-groups",
		"--filters", fmt.Sprintf("Name=vpc-id,Values=%s", vpcID),
		fmt.Sprintf("Name=group-name,Values=%s", groupName),
		"--query", "SecurityGroups[0].GroupId",
		"--output", "text")

	if output, err := awsOutput(cmd); err == nil {
		if id := strings.TrimSpace(string(output)); id != "None" && id != "" {
			return id, nil
		}
	}

	fmt.Printf("🔒 Creating security group: %s\n", groupName)
	cmd = exec.Command("aws", "ec2", "create-security-group",
		"--group-name", groupName,
		"--description", "Security group for Dvarpala VPN server",
		"--vpc-id", vpcID,
		"--query", "GroupId",
		"--output", "text")

	output, err := awsOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to create security group: %v", err)
	}

	id := strings.TrimSpace(string(output))
	if id == "" {
		return "", fmt.Errorf("aws returned no id for security group %s", groupName)
	}

	aws.tagResource(id, groupName, "Security Group")
	aws.addSecurityGroupRules(id, config.AllowedIPs)
	return id, nil
}

func (aws *AWSProvider) createNewVPC(vpcName string, config NetworkConfig) (*AWSVPCInfo, error) {
	vpcInfo := &AWSVPCInfo{}

	// Create VPC
	cmd := exec.Command("aws", "ec2", "create-vpc",
		"--cidr-block", config.VPCCidr,
		"--query", "Vpc.VpcId",
		"--output", "text")

	output, err := awsOutput(cmd)
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

	output, err = awsOutput(cmd)
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

	output, err = awsOutput(cmd)
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

	output, err = awsOutput(cmd)
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

	output, err = awsOutput(cmd)
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

	// Create Security Group, through the same function the reuse path calls,
	// so the name it is created with and the name it is found by cannot drift
	// apart again.
	sgID, err := aws.ensureSecurityGroup(vpcName, vpcInfo.VPCID, config)
	if err != nil {
		return nil, err
	}
	vpcInfo.SecurityGroupID = sgID

	fmt.Printf("✅ VPC created successfully: %s\n", vpcInfo.VPCID)
	return vpcInfo, nil
}

// addSecurityGroupRules opens the two ports a Dvarpala server needs, and no
// others.
//
// The portal is deliberately absent. It listens on 8080, but only ever needs
// reaching from inside the tunnel, at 172.30.100.1 - which arrives on tun0
// and never crosses the security group at all. Opening 8080 to the internet
// published the sign-in page to anyone who found the address: the form that
// sends codes to real employees, the endpoint that redeems emergency access
// links, and the administration console. None of that is a way in on its own,
// and none of it should be reachable from outside either.
//
// Port 443 is absent for a simpler reason: nothing listens on it.
func (aws *AWSProvider) addSecurityGroupRules(sgID string, allowedIPs []string) {
	// SSH, for installing and administering the machine.
	exec.Command("aws", "ec2", "authorize-security-group-ingress",
		"--group-id", sgID,
		"--protocol", "tcp",
		"--port", "22",
		"--cidr", "0.0.0.0/0").Run()

	// The VPN itself. This is how a client reaches everything else.
	exec.Command("aws", "ec2", "authorize-security-group-ingress",
		"--group-id", sgID,
		"--protocol", "udp",
		"--port", "1194",
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

	keyMaterial, err := awsOutput(cmd)
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

	// Everything the launch needs, checked here rather than discovered from
	// AWS's reply. A blank value produces "MissingParameter: when specifying a
	// security group you must specify a group id for each item", which names
	// neither the value that was empty nor where it should have come from.
	for _, required := range []struct{ name, value string }{
		{"security group", vpcInfo.SecurityGroupID},
		{"public subnet", vpcInfo.PublicSubnetID},
		{"key pair", keyPairName},
		{"machine image", amiID},
	} {
		if strings.TrimSpace(required.value) == "" {
			return nil, fmt.Errorf("cannot launch: no %s was found or created", required.name)
		}
	}

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

	output, err := awsOutput(cmd)
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

	output, err := awsOutput(cmd)
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
		{"Fetching Dvarpala source", fmt.Sprintf("sudo cloud-init status --wait >/dev/null 2>&1 || true; "+
			"sudo apt-get -o DPkg::Lock::Timeout=600 update -qq && "+
			"sudo apt-get -o DPkg::Lock::Timeout=600 install -y -qq git && "+
			"sudo rm -rf /opt/dvarpala/src && "+
			"sudo git clone --depth 1 --branch %s %s /opt/dvarpala/src", defaultRepoRef, defaultRepoURL)},
		{"Installing Dvarpala", "sudo chmod +x /opt/dvarpala/src/scripts/install/install-dvarpala.sh && sudo DVARPALA_ADMIN_EMAIL='" + config.AdminEmail + "' /opt/dvarpala/src/scripts/install/install-dvarpala.sh --source /opt/dvarpala/src --host " + hostForCerts},
	}

	needsSignIn := false

	for i, step := range steps {
		fmt.Printf("📦 Step %d/%d: %s\n", i+1, len(steps), step.name)

		if err := aws.executeSSHCommand(instanceInfo.PublicIP, step.cmd, keyPath); err != nil {
			// Exit 2 from the installer means the machine is built and every
			// service is running, and only a sign-in method is still to be
			// chosen. Treating that as a failure reported a working system as
			// a crash, over output that had a tick against all fifteen steps.
			if exitCode(err) == 2 {
				fmt.Printf("⚠️  %s completed, but no sign-in method is configured yet\n", step.name)
				needsSignIn = true
				continue
			}
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

	if needsSignIn {
		// Said plainly, because a machine nobody can sign in to is not
		// finished, and the profile just fetched cannot be used until it is.
		fmt.Println()
		fmt.Println("⚠️  Dvarpala is installed and running, but NOBODY CAN SIGN IN YET.")
		fmt.Println("   Anyone who connects reaches the sign-in page and nothing else.")
		fmt.Println()
		fmt.Println("   Configure a sign-in method on the server:")
		fmt.Println("     sudo nano /opt/dvarpala/config/environment.yaml   # auth.otp + auth.smtp")
		fmt.Println("     sudo systemctl edit dvarpala                      # AUTH_SMTP_PASSWORD")
		fmt.Println("     sudo systemctl restart dvarpala")
		fmt.Println("     dvarpala-cli mail test you@your-domain")
		return nil
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

	output, err := awsOutput(cmd)
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
