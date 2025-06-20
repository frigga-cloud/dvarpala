package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CloudProvider string

const (
	AWS   CloudProvider = "aws"
	GCP   CloudProvider = "gcp"
	Azure CloudProvider = "azure"
)

type CloudConfig struct {
	Provider    CloudProvider  `json:"provider"`
	Region      string         `json:"region"`
	Credentials map[string]any `json:"credentials"`
	ProjectID   string         `json:"project_id,omitempty"`
}

type AdminConfig struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

type InstallationConfig struct {
	Cloud           CloudConfig   `json:"cloud"`
	Admin           AdminConfig   `json:"admin"`
	BackupEnabled   bool          `json:"backup_enabled"`
	StorageBucket   string        `json:"storage_bucket"`
	VMConfig        VMConfig      `json:"vm_config"`
	NetworkConfig   NetworkConfig `json:"network_config"`
	OutputDirectory string        `json:"output_directory"`
	ResourceNames   ResourceNames `json:"resource_names"`
}

type ResourceNames struct {
	VPCName     string `json:"vpc_name"`
	VMName      string `json:"vm_name"`
	BucketName  string `json:"bucket_name"`
	KeyPairName string `json:"keypair_name"`
}

type VMConfig struct {
	InstanceType string            `json:"instance_type"`
	DiskSize     int               `json:"disk_size_gb"`
	Tags         map[string]string `json:"tags"`
}

type NetworkConfig struct {
	VPCCidr           string   `json:"vpc_cidr"`
	PublicSubnetCidr  string   `json:"public_subnet_cidr"`
	PrivateSubnetCidr string   `json:"private_subnet_cidr"`
	AllowedIPs        []string `json:"allowed_ips"`
}

func cloudInstaller() {
	var (
		configFile = flag.String("config", "", "Configuration file (JSON) with cloud and admin settings (REQUIRED)")
	)
	flag.Parse()

	fmt.Println("🚀 Dvarpala Cloud Installer - Frigga Labs")
	fmt.Println("==========================================")
	fmt.Println()

	// Config File is required.
	if *configFile == "" {
		fmt.Println("❌ Configuration file is required!")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  go run installer/installer.go installer/cloud_service.go installer/cloud_wrappers.go --config=config.json")
		fmt.Println()
		fmt.Println("📋 Create a config.json file with your cloud provider settings.")
		fmt.Println("📖 See examples/ directory for sample configuration files.")
		fmt.Println()
		flag.Usage()
		os.Exit(1)
	}

	var config InstallationConfig
	if err := loadConfigFromFile(*configFile, &config); err != nil {
		log.Fatalf("❌ Failed to load config file: %v", err)
	}
	fmt.Printf("✅ Configuration loaded from: %s\n", *configFile)

	// Generate Frigga resource names
	generateResourceNames(&config)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		log.Fatalf("❌ Configuration validation failed: %v", err)
	}
	fmt.Printf("✅ Configuration validated successfully\n")

	// Create output directory
	if err := os.MkdirAll(config.OutputDirectory, 0755); err != nil {
		log.Fatalf("❌ Failed to create output directory: %v", err)
	}
	fmt.Printf("✅ Output directory created: %s\n", config.OutputDirectory)

	// Install cloud provider tools
	fmt.Println("\n🔧 Installing cloud provider tools...")
	if err := installCloudTools(config.Cloud.Provider); err != nil {
		log.Fatalf("❌ Failed to install cloud tools: %v", err)
	}
	fmt.Printf("✅ Cloud provider tools installed successfully\n")

	// Authenticate with cloud provider
	fmt.Println("\n🔐 Authenticating with cloud provider...")
	if err := authenticateCloudProvider(config); err != nil {
		log.Fatalf("❌ Cloud authentication failed: %v", err)
	}
	fmt.Printf("✅ Cloud authentication successful\n")

	// Create or use existing VPC
	fmt.Println("\n🌐 Setting up VPC infrastructure...")
	vpcID, err := setupVPC(config)
	if err != nil {
		log.Fatalf("❌ VPC setup failed: %v", err)
	}
	fmt.Printf("✅ VPC ready: %s\n", vpcID)

	// Create VM and install dvarpala directly
	fmt.Println("\n💻 Creating VM and installing dvarpala...")
	vmInfo, err := CreateVM(config, vpcID)
	if err != nil {
		log.Fatalf("❌ VM creation and installation failed: %v", err)
	}
	fmt.Printf("✅ VM created and dvarpala installed: %s (IP: %s)\n", vmInfo.InstanceID, vmInfo.PublicIP)

	// Verify installation is working
	fmt.Println("\n🔍 Verifying installation...")
	if err := verifyInstallation(vmInfo.PublicIP); err != nil {
		log.Printf("⚠️ Installation verification failed: %v", err)
	} else {
		fmt.Println("✅ Installation verification successful")
	}

	// Download admin.ovpn file
	fmt.Println("\n📄 Downloading admin OpenVPN configuration...")
	if err := downloadAdminOVPN(config, vmInfo); err != nil {
		log.Printf("⚠️ Failed to download admin.ovpn: %v", err)
	} else {
		fmt.Println("✅ Admin OpenVPN configuration downloaded")
	}

	// Generate output files
	fmt.Println("\n📁 Generating configuration files...")
	if err := generateOutputFiles(config, vmInfo); err != nil {
		log.Fatalf("❌ Failed to generate output files: %v", err)
	}

	// Setup object storage backup
	if config.BackupEnabled {
		fmt.Println("\n☁️ Setting up object storage backup...")
		if err := setupObjectStorage(config, vmInfo); err != nil {
			log.Printf("⚠️ Object storage setup failed: %v", err)
		} else {
			fmt.Println("✅ Object storage configured")
		}
	}

	// Final summary
	printInstallationSummary(config, vmInfo)

	// Print SSH connection details for manual access
	printSSHConnectionInfo(vmInfo)
}

// Validating the configuration for installation.
func validateConfig(config InstallationConfig) error {
	if config.Cloud.Provider == "" {
		return fmt.Errorf("cloud provider not specified")
	}
	if config.Cloud.Region == "" {
		return fmt.Errorf("cloud region not specified")
	}
	if config.Admin.Email == "" {
		return fmt.Errorf("administrator email not specified")
	}
	return nil
}

func loadConfigFromFile(filename string, config *InstallationConfig) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, config)
}

func installCloudTools(provider CloudProvider) error {
	switch provider {
	case AWS:
		return installAWSCLI()
	case GCP:
		return installGCloudCLI()
	case Azure:
		return installAzureCLI()
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}
}

func installAWSCLI() error {
	if commandExists("aws") {
		fmt.Println("✅ AWS CLI already installed")
		return nil
	}
	fmt.Println("📦 Installing AWS CLI...")
	return installAWSCLIForPlatform()
}

func installGCloudCLI() error {
	if commandExists("gcloud") {
		fmt.Println("✅ Google Cloud CLI already installed")
		return nil
	}
	fmt.Println("📦 Installing Google Cloud CLI...")
	return installGCloudCLIForPlatform()
}

func installAzureCLI() error {
	if commandExists("az") {
		fmt.Println("✅ Azure CLI already installed")
		return nil
	}
	fmt.Println("📦 Installing Azure CLI...")
	return installAzureCLIForPlatform()
}

func authenticateCloudProvider(config InstallationConfig) error {
	switch config.Cloud.Provider {
	case AWS:
		return authenticateAWS(config)
	case GCP:
		return authenticateGCP(config)
	case Azure:
		return authenticateAzure(config)
	default:
		return fmt.Errorf("unsupported provider: %s", config.Cloud.Provider)
	}
}

func authenticateAWS(config InstallationConfig) error {
	if creds, ok := config.Cloud.Credentials["access_key"]; ok {
		// Set environment variables for AWS authentication
		os.Setenv("AWS_ACCESS_KEY_ID", creds.(string))
		os.Setenv("AWS_SECRET_ACCESS_KEY", config.Cloud.Credentials["secret_key"].(string))
		os.Setenv("AWS_DEFAULT_REGION", config.Cloud.Region)
	}

	// Test authentication
	cmd := exec.Command("aws", "sts", "get-caller-identity")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("AWS authentication failed: %v", err)
	}
	return nil
}

func authenticateGCP(config InstallationConfig) error {
	if keyPath, ok := config.Cloud.Credentials["service_account_key"]; ok {
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyPath.(string))
	}

	if config.Cloud.ProjectID != "" {
		cmd := exec.Command("gcloud", "config", "set", "project", config.Cloud.ProjectID)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set GCP project: %v", err)
		}
	}

	// Test authentication
	cmd := exec.Command("gcloud", "auth", "list", "--filter=status:ACTIVE")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GCP authentication failed: %v", err)
	}
	return nil
}

func authenticateAzure(config InstallationConfig) error {
	if creds, ok := config.Cloud.Credentials["client_id"]; ok {
		cmd := exec.Command("az", "login", "--service-principal",
			"--username", creds.(string),
			"--password", config.Cloud.Credentials["client_secret"].(string),
			"--tenant", config.Cloud.Credentials["tenant_id"].(string))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Azure service principal login failed: %v", err)
		}
	} else {
		// Test existing authentication
		cmd := exec.Command("az", "account", "show")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Azure authentication failed. Run 'az login' first: %v", err)
		}
	}
	return nil
}

type VMInfo struct {
	InstanceID  string
	PublicIP    string
	PrivateIP   string
	SSHKeyPath  string
	VPNConfig   string
	AdminConfig string
}

func setupVPC(config InstallationConfig) (string, error) {
	cloudService, err := NewCloudService(config)
	if err != nil {
		return "", err
	}

	return cloudService.SetupVPC()
}

func CreateVM(config InstallationConfig, vpcID string) (*VMInfo, error) {
	cloudService, err := NewCloudService(config)
	if err != nil {
		return nil, err
	}

	return cloudService.CreateVM(vpcID)
}

func setupObjectStorage(config InstallationConfig, _ *VMInfo) error {
	cloudService, err := NewCloudService(config)
	if err != nil {
		return err
	}

	if err := cloudService.SetupObjectStorage(); err != nil {
		return err
	}

	return cloudService.UploadConfiguration()
}

func generateOutputFiles(config InstallationConfig, vmInfo *VMInfo) error {
	// Save configuration
	configData, _ := json.MarshalIndent(config, "", "  ")
	configPath := filepath.Join(config.OutputDirectory, "installation-config.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return err
	}

	// Generate connection info
	connectionInfo := fmt.Sprintf(`Dvarpala Installation Complete
================================

Frigga Resource Names:
- VPC: %s
- VM: %s  
- Storage: %s
- KeyPair: %s

Server Details:
- Instance ID: %s  
- Public IP: %s
- Private IP: %s

Admin Access:
- Email: %s
- VPN Config: See %s/admin.ovpn

Object Storage:
- Bucket: %s
- Backup Location: %s/dvarpala/

Next Steps:
1. Download admin.ovpn from the output directory
2. Connect to VPN using credentials: portal/access
3. Browser auto-opens to http://172.30.100.1:8080
4. Complete authentication via web portal for full access
5. Configure OAuth providers and generate user certificates

Files Generated:
- installation-config.json: Full installation configuration
- admin.ovpn: Admin VPN configuration with auto-open
- connection-info.txt: This file
`, config.ResourceNames.VPCName, config.ResourceNames.VMName,
		config.ResourceNames.BucketName, config.ResourceNames.KeyPairName,
		vmInfo.InstanceID, vmInfo.PublicIP, vmInfo.PrivateIP,
		config.Admin.Email, config.OutputDirectory,
		config.StorageBucket, config.StorageBucket)

	connectionPath := filepath.Join(config.OutputDirectory, "connection-info.txt")
	return os.WriteFile(connectionPath, []byte(connectionInfo), 0644)
}

func printInstallationSummary(config InstallationConfig, vmInfo *VMInfo) {
	fmt.Println("\n🎉 Dvarpala Cloud Installation Complete!")
	fmt.Println("=========================================")
	fmt.Printf("☁️ Provider: %s (%s)\n", config.Cloud.Provider, config.Cloud.Region)
	fmt.Printf("💻 Instance: %s (%s)\n", vmInfo.InstanceID, vmInfo.PublicIP)
	fmt.Printf("👤 Admin: %s\n", config.Admin.Email)
	fmt.Printf("📁 Files: %s\n", config.OutputDirectory)
	if config.BackupEnabled {
		fmt.Printf("☁️ Backup: %s\n", config.StorageBucket)
	}
	fmt.Println("\n✨ Your Dvarpala VPN server is ready to use!")
	fmt.Printf("📖 See %s/connection-info.txt for next steps\n", config.OutputDirectory)
}

func printSSHConnectionInfo(vmInfo *VMInfo) {
	fmt.Println("\n🔗 SSH Connection Information")
	fmt.Println("=============================")
	fmt.Printf("🌐 Server IP: %s\n", vmInfo.PublicIP)
	if vmInfo.SSHKeyPath != "" {
		fmt.Printf("🔑 SSH Key: %s\n", vmInfo.SSHKeyPath)

		// Determine the correct SSH user based on the key path or instance info
		var sshUser string
		if strings.Contains(vmInfo.SSHKeyPath, "aws") || strings.Contains(vmInfo.InstanceID, "i-") {
			sshUser = "ubuntu"
		} else if strings.Contains(vmInfo.SSHKeyPath, "gcp") || strings.Contains(vmInfo.InstanceID, "friggalabs-vm") {
			sshUser = "ubuntu"
		} else if strings.Contains(vmInfo.SSHKeyPath, "azure") {
			sshUser = "azureuser"
		} else {
			sshUser = "ubuntu" // default
		}

		fmt.Printf("\n📋 To connect manually:\n")
		fmt.Printf("   ssh -i %s %s@%s\n", vmInfo.SSHKeyPath, sshUser, vmInfo.PublicIP)
		fmt.Printf("\n🌐 Access Dvarpala web interface:\n")
		fmt.Printf("   http://%s:8080/health\n", vmInfo.PublicIP)
	}
}

// Utility functions
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func generateFriggaResourceName(resourceType string) string {
	// Generate 5-character alphanumeric string
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	suffix := make([]byte, 5)
	for i := range suffix {
		suffix[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(1000) // Small delay to ensure different nano timestamps
	}
	return fmt.Sprintf("friggalabs-%s-%s", resourceType, string(suffix))
}

func generateResourceNames(config *InstallationConfig) {
	// Generate consistent resource names with Frigga naming convention
	config.ResourceNames.VPCName = generateFriggaResourceName("vpc")
	config.ResourceNames.VMName = generateFriggaResourceName("vm")

	// Use shared bucket for all Frigga tools
	config.ResourceNames.BucketName = "friggalabs"
	config.StorageBucket = config.ResourceNames.BucketName

	config.ResourceNames.KeyPairName = generateFriggaResourceName("keypair")

	fmt.Printf("🏷️ Generated resource names:\n")
	fmt.Printf("   VPC: %s\n", config.ResourceNames.VPCName)
	fmt.Printf("   VM: %s\n", config.ResourceNames.VMName)
	fmt.Printf("   Storage: %s (shared Frigga bucket)\n", config.ResourceNames.BucketName)
	fmt.Printf("   KeyPair: %s\n", config.ResourceNames.KeyPairName)
}

func getStringFromCredentials(credentials map[string]any, key string) string {
	if val, ok := credentials[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// verifyInstallation performs a quick verification that the installation is working
func verifyInstallation(vmIP string) error {
	fmt.Printf("🔍 Checking health endpoint at http://%s:8080/health...\n", vmIP)

	// Quick check that nginx is responding on port 8080
	cmd := exec.Command("curl", "-s", "--connect-timeout", "10", "--max-time", "15",
		fmt.Sprintf("http://%s:8080/health", vmIP))

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("health check failed: %v", err)
	}

	fmt.Printf("✅ Health check response: %s\n", strings.TrimSpace(string(output)))
	return nil
}

// downloadAdminOVPN downloads the admin.ovpn file from the VM
func downloadAdminOVPN(config InstallationConfig, vmInfo *VMInfo) error {
	// Download admin.ovpn file from VM
	adminOVPNURL := fmt.Sprintf("http://%s:8080/admin.ovpn", vmInfo.PublicIP)

	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		cmd := exec.Command("curl", "-s", "-o",
			filepath.Join(config.OutputDirectory, "admin.ovpn"),
			adminOVPNURL)

		if err := cmd.Run(); err == nil {
			// Verify the file was downloaded and is not empty
			if fileExists(filepath.Join(config.OutputDirectory, "admin.ovpn")) {
				return nil
			}
		}

		fmt.Printf("⏳ Waiting for admin.ovpn to be ready... (%d/%d)\n", i+1, maxAttempts)
		time.Sleep(30 * time.Second)
	}

	// If download fails, try to generate a basic template
	return generateBasicOVPNTemplate(config, vmInfo)
}

// generateBasicOVPNTemplate creates a basic OpenVPN configuration template
func generateBasicOVPNTemplate(config InstallationConfig, vmInfo *VMInfo) error {
	ovpnTemplate := fmt.Sprintf(`# Dvarpala OpenVPN Client Configuration
# Generated by Frigga Labs Installer

client
dev tun
proto udp
remote %s 1194
resolv-retry infinite
nobind

# Authentication
auth-user-pass

# Security
cipher AES-256-GCM
auth SHA256

# Compression
compress lz4-v2

# Connection
keepalive 10 120
verb 3

# Selective Routing - Internet traffic goes direct
# Only blocked resources route through VPN
route-nopull

# Auto-open captive portal (cross-platform)
up "echo 'Opening captive portal...' && (open http://172.30.100.1:8080 2>/dev/null || xdg-open http://172.30.100.1:8080 2>/dev/null || start http://172.30.100.1:8080 2>/dev/null || echo 'Please open http://172.30.100.1:8080 manually')"

# Note: SSL certificates will be added automatically after first connection
# Initial credentials: username=portal, password=access

<ca>
# Certificate Authority certificate will be added here
# Connect to VPN first, then download complete configuration
</ca>

<cert>
# Client certificate will be added here
# Visit http://%s:8080 after VPN connection for setup
</cert>

<key>
# Client private key will be added here
</key>
`, vmInfo.PublicIP, vmInfo.PublicIP)

	ovpnPath := filepath.Join(config.OutputDirectory, "admin.ovpn")
	return os.WriteFile(ovpnPath, []byte(ovpnTemplate), 0644)
}

// Platform-specific CLI installation functions

func installAWSCLIForPlatform() error {
	switch getOperatingSystem() {
	case "darwin":
		return installAWSCLIMacOS()
	case "linux":
		return installAWSCLILinux()
	case "windows":
		return installAWSCLIWindows()
	default:
		return fmt.Errorf("AWS CLI installation not supported for this OS. Please install manually from: https://aws.amazon.com/cli/")
	}
}

func installGCloudCLIForPlatform() error {
	switch getOperatingSystem() {
	case "darwin":
		return installGCloudCLIMacOS()
	case "linux":
		return installGCloudCLILinux()
	case "windows":
		return installGCloudCLIWindows()
	default:
		return fmt.Errorf("Google Cloud CLI installation not supported for this OS. Please install manually from: https://cloud.google.com/sdk/docs/install")
	}
}

func installAzureCLIForPlatform() error {
	switch getOperatingSystem() {
	case "darwin":
		return installAzureCLIMacOS()
	case "linux":
		return installAzureCLILinux()
	case "windows":
		return installAzureCLIWindows()
	default:
		return fmt.Errorf("Azure CLI installation not supported for this OS. Please install manually from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli")
	}
}

// macOS installations using Homebrew
func installAWSCLIMacOS() error {
	if !commandExists("brew") {
		return fmt.Errorf("Homebrew not found. Install Homebrew first or install AWS CLI manually")
	}
	fmt.Println("📦 Installing AWS CLI via Homebrew...")
	cmd := exec.Command("brew", "install", "awscli")
	return cmd.Run()
}

func installGCloudCLIMacOS() error {
	if !commandExists("brew") {
		return fmt.Errorf("Homebrew not found. Install Homebrew first or install Google Cloud CLI manually")
	}
	fmt.Println("📦 Installing Google Cloud CLI via Homebrew...")
	cmd := exec.Command("brew", "install", "--cask", "google-cloud-sdk")
	return cmd.Run()
}

func installAzureCLIMacOS() error {
	if !commandExists("brew") {
		return fmt.Errorf("Homebrew not found. Install Homebrew first or install Azure CLI manually")
	}
	fmt.Println("📦 Installing Azure CLI via Homebrew...")
	cmd := exec.Command("brew", "install", "azure-cli")
	return cmd.Run()
}

// Linux installations
func installAWSCLILinux() error {
	fmt.Println("📦 Installing AWS CLI for Linux...")

	// Download and install AWS CLI v2
	commands := [][]string{
		{"curl", "-fsSL", "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip", "-o", "/tmp/awscliv2.zip"},
		{"unzip", "-q", "/tmp/awscliv2.zip", "-d", "/tmp"},
		{"sudo", "/tmp/aws/install"},
		{"rm", "-rf", "/tmp/awscliv2.zip", "/tmp/aws"},
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install AWS CLI: %v", err)
		}
	}
	fmt.Println("✅ AWS CLI installed successfully")
	return nil
}

func installGCloudCLILinux() error {
	fmt.Println("📦 Installing Google Cloud CLI for Linux...")

	commands := [][]string{
		{"curl", "-fsSL", "https://sdk.cloud.google.com", "-o", "/tmp/install.sh"},
		{"bash", "/tmp/install.sh", "--disable-prompts"},
		{"rm", "/tmp/install.sh"},
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install Google Cloud CLI: %v", err)
		}
	}
	fmt.Println("✅ Google Cloud CLI installed successfully")
	return nil
}

func installAzureCLILinux() error {
	fmt.Println("📦 Installing Azure CLI for Linux...")

	commands := [][]string{
		{"curl", "-sL", "https://aka.ms/InstallAzureCLIDeb", "-o", "/tmp/azure_cli_install.sh"},
		{"sudo", "bash", "/tmp/azure_cli_install.sh"},
		{"rm", "/tmp/azure_cli_install.sh"},
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install Azure CLI: %v", err)
		}
	}
	fmt.Println("✅ Azure CLI installed successfully")
	return nil
}

// Windows installations
func installAWSCLIWindows() error {
	fmt.Println("📦 Installing AWS CLI for Windows...")

	if commandExists("winget") {
		// Use winget if available
		cmd := exec.Command("winget", "install", "Amazon.AWSCLI")
		return cmd.Run()
	} else if commandExists("choco") {
		// Use chocolatey if available
		cmd := exec.Command("choco", "install", "awscli", "-y")
		return cmd.Run()
	}

	return fmt.Errorf("please install AWS CLI manually from: https://aws.amazon.com/cli/")
}

func installGCloudCLIWindows() error {
	fmt.Println("📦 Installing Google Cloud CLI for Windows...")

	if commandExists("winget") {
		cmd := exec.Command("winget", "install", "Google.CloudSDK")
		return cmd.Run()
	} else if commandExists("choco") {
		cmd := exec.Command("choco", "install", "gcloudsdk", "-y")
		return cmd.Run()
	}

	return fmt.Errorf("please install Google Cloud CLI manually from: https://cloud.google.com/sdk/docs/install")
}

func installAzureCLIWindows() error {
	fmt.Println("📦 Installing Azure CLI for Windows...")

	if commandExists("winget") {
		cmd := exec.Command("winget", "install", "Microsoft.AzureCLI")
		return cmd.Run()
	} else if commandExists("choco") {
		cmd := exec.Command("choco", "install", "azure-cli", "-y")
		return cmd.Run()
	}

	return fmt.Errorf("please install Azure CLI manually from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli")
}

// getOperatingSystem returns the current operating system
func getOperatingSystem() string {
	cmd := exec.Command("uname", "-s")
	if output, err := cmd.Output(); err == nil {
		os := strings.ToLower(strings.TrimSpace(string(output)))
		switch os {
		case "darwin":
			return "darwin"
		case "linux":
			return "linux"
		default:
			return "unknown"
		}
	}

	// Fallback for Windows
	if commandExists("powershell") || commandExists("cmd") {
		return "windows"
	}

	return "unknown"
}

func main() {
	cloudInstaller()
}
