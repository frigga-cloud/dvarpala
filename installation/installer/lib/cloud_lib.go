package lib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CloudProvider interface defines the interface for all cloud providers
type CloudProvider interface {
	// Provider-specific methods that must be implemented
	SetupVPC() (string, error)
	CreateVM(vpcID string) (*VMResult, error)
	SetupObjectStorage() error
	UploadConfiguration() error

	// Common methods with base implementation
	InstallDvarpala(vmInfo *VMResult) error
	Configure2StepVPNAccess(vmInfo *VMResult) error
}

// NewCloudProviderFunc is a function type for creating cloud providers
type NewCloudProviderFunc func(config InstallationConfig) (CloudProvider, error)

// cloudProviderFactory holds the function to create cloud providers
var cloudProviderFactory NewCloudProviderFunc

// SetCloudProviderFactory sets the factory function for creating cloud providers
func SetCloudProviderFactory(factory NewCloudProviderFunc) {
	cloudProviderFactory = factory
}

// NewCloudProvider creates a cloud provider using the registered factory
func NewCloudProvider(config InstallationConfig) (CloudProvider, error) {
	if cloudProviderFactory == nil {
		return nil, fmt.Errorf("cloud provider factory not registered")
	}
	return cloudProviderFactory(config)
}

func InstallCloudTools(provider CloudProviderType) error {
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

func AuthenticateCloudProvider(config InstallationConfig) error {
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
		os.Setenv("AWS_ACCESS_KEY_ID", creds.(string))
		os.Setenv("AWS_SECRET_ACCESS_KEY", config.Cloud.Credentials["secret_key"].(string))
		os.Setenv("AWS_DEFAULT_REGION", config.Cloud.Region)
	}

	cmd := exec.Command("aws", "sts", "get-caller-identity")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws authentication failed: %v", err)
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
			return fmt.Errorf("failed to set gcp project: %v", err)
		}
	}

	cmd := exec.Command("gcloud", "auth", "list", "--filter=status:ACTIVE")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gcp authentication failed: %v", err)
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
			return fmt.Errorf("azure service principal login failed: %v", err)
		}
	} else {
		cmd := exec.Command("az", "account", "show")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("azure authentication failed. run 'az login' first: %v", err)
		}
	}
	return nil
}

func SetupVPC(config InstallationConfig) (string, error) {
	cloudProvider, err := NewCloudProvider(config)
	if err != nil {
		return "", err
	}

	return cloudProvider.SetupVPC()
}

func CreateVM(config InstallationConfig, vpcID string) (*VMResult, error) {
	cloudProvider, err := NewCloudProvider(config)
	if err != nil {
		return nil, err
	}

	vmInfo, err := cloudProvider.CreateVM(vpcID)
	if err != nil {
		return nil, err
	}

	return &VMResult{
		InstanceID: vmInfo.InstanceID,
		PublicIP:   vmInfo.PublicIP,
		PrivateIP:  vmInfo.PrivateIP,
		SSHKeyPath: vmInfo.SSHKeyPath,
	}, nil
}

func SetupObjectStorage(config InstallationConfig, _ *VMResult) error {
	cloudProvider, err := NewCloudProvider(config)
	if err != nil {
		return err
	}

	if err := cloudProvider.SetupObjectStorage(); err != nil {
		return err
	}

	return cloudProvider.UploadConfiguration()
}

func VerifyInstallation(vmIP string) error {
	fmt.Printf("🔍 Checking health endpoint at http://%s:8080/health...\n", vmIP)

	cmd := exec.Command("curl", "-s", "--connect-timeout", "10", "--max-time", "15",
		fmt.Sprintf("http://%s:8080/health", vmIP))

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("health check failed: %v", err)
	}

	fmt.Printf("✅ Health check response: %s\n", strings.TrimSpace(string(output)))
	return nil
}

func DownloadAdminOVPN(config InstallationConfig, vmInfo *VMResult) error {
	adminOVPNURL := fmt.Sprintf("http://%s:8080/admin.ovpn", vmInfo.PublicIP)

	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		cmd := exec.Command("curl", "-s", "-o",
			filepath.Join(config.OutputDirectory, "admin.ovpn"),
			adminOVPNURL)

		if err := cmd.Run(); err == nil {
			if fileExists(filepath.Join(config.OutputDirectory, "admin.ovpn")) {
				return nil
			}
		}

		fmt.Printf("⏳ Waiting for admin.ovpn to be ready... (%d/%d)\n", i+1, maxAttempts)
		time.Sleep(30 * time.Second)
	}

	return generateBasicOVPNTemplate(config, vmInfo)
}

func generateBasicOVPNTemplate(config InstallationConfig, vmInfo *VMResult) error {
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

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func installAWSCLIForPlatform() error {
	switch getOperatingSystem() {
	case "darwin":
		return installAWSCLIMacOS()
	case "linux":
		return installAWSCLILinux()
	case "windows":
		return installAWSCLIWindows()
	default:
		return fmt.Errorf("aws CLI installation not supported for this OS. please install manually from: https://aws.amazon.com/cli/")
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
		return fmt.Errorf("google Cloud CLI installation not supported for this OS. please install manually from: https://cloud.google.com/sdk/docs/install")
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
		return fmt.Errorf("azure CLI installation not supported for this OS. please install manually from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli")
	}
}

func installAWSCLIMacOS() error {
	if !commandExists("brew") {
		return fmt.Errorf("homebrew not found. install Homebrew first or install AWS CLI manually")
	}
	fmt.Println("📦 Installing AWS CLI via Homebrew...")
	cmd := exec.Command("brew", "install", "awscli")
	return cmd.Run()
}

func installGCloudCLIMacOS() error {
	if !commandExists("brew") {
		return fmt.Errorf("homebrew not found. install Homebrew first or install Google Cloud CLI manually")
	}
	fmt.Println("📦 Installing Google Cloud CLI via Homebrew...")
	cmd := exec.Command("brew", "install", "--cask", "google-cloud-sdk")
	return cmd.Run()
}

func installAzureCLIMacOS() error {
	if !commandExists("brew") {
		return fmt.Errorf("homebrew not found. install Homebrew first or install Azure CLI manually")
	}
	fmt.Println("📦 Installing Azure CLI via Homebrew...")
	cmd := exec.Command("brew", "install", "azure-cli")
	return cmd.Run()
}

func installAWSCLILinux() error {
	fmt.Println("📦 Installing AWS CLI for Linux...")

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

func installAWSCLIWindows() error {
	fmt.Println("📦 Installing AWS CLI for Windows...")

	if commandExists("winget") {
		cmd := exec.Command("winget", "install", "Amazon.AWSCLI")
		return cmd.Run()
	} else if commandExists("choco") {
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

	if commandExists("powershell") || commandExists("cmd") {
		return "windows"
	}

	return "unknown"
}
