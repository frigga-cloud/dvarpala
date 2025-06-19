package main

import (
	"bufio"
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
	Provider    CloudProvider          `json:"provider"`
	Region      string                 `json:"region"`
	Credentials map[string]interface{} `json:"credentials"`
	ProjectID   string                 `json:"project_id,omitempty"`
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
	VPCName      string `json:"vpc_name"`
	VMName       string `json:"vm_name"`
	BucketName   string `json:"bucket_name"`
	KeyPairName  string `json:"keypair_name"`
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
		configFile  = flag.String("config", "", "Configuration file (JSON) with cloud and admin settings")
		interactive = flag.Bool("interactive", true, "Run in interactive mode")
		provider    = flag.String("provider", "", "Cloud provider: aws, gcp, azure")
		region      = flag.String("region", "", "Cloud region")
		outputDir   = flag.String("output", "./dvarpala-deployment", "Output directory for configuration files")
	)
	flag.Parse()

	fmt.Println("🚀 Dvarpala Cloud Installer - Frigga Labs")
	fmt.Println("==========================================")
	fmt.Println()

	var config InstallationConfig

	if *configFile != "" {
		if err := loadConfigFromFile(*configFile, &config); err != nil {
			log.Fatalf("❌ Failed to load config file: %v", err)
		}
		fmt.Printf("✅ Configuration loaded from: %s\n", *configFile)
	} else if *interactive {
		config = runInteractiveSetup()
	} else {
		config = createConfigFromFlags(*provider, *region, *outputDir)
	}

	// Generate Frigga resource names
	generateResourceNames(&config)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		log.Fatalf("❌ Configuration validation failed: %v", err)
	}

	// Create output directory
	if err := os.MkdirAll(config.OutputDirectory, 0755); err != nil {
		log.Fatalf("❌ Failed to create output directory: %v", err)
	}

	// Install cloud provider tools
	fmt.Println("\n🔧 Installing cloud provider tools...")
	if err := installCloudTools(config.Cloud.Provider); err != nil {
		log.Fatalf("❌ Failed to install cloud tools: %v", err)
	}

	// Authenticate with cloud provider
	fmt.Println("\n🔐 Authenticating with cloud provider...")
	if err := authenticateCloudProvider(config); err != nil {
		log.Fatalf("❌ Cloud authentication failed: %v", err)
	}

	// Create or use existing VPC
	fmt.Println("\n🌐 Setting up VPC infrastructure...")
	vpcID, err := setupVPC(config)
	if err != nil {
		log.Fatalf("❌ VPC setup failed: %v", err)
	}
	fmt.Printf("✅ VPC ready: %s\n", vpcID)

	// Create VM and install dvarpala
	fmt.Println("\n💻 Creating VM and installing dvarpala...")
	vmInfo, err := createAndConfigureVM(config, vpcID)
	if err != nil {
		log.Fatalf("❌ VM creation failed: %v", err)
	}
	fmt.Printf("✅ VM created: %s (IP: %s)\n", vmInfo.InstanceID, vmInfo.PublicIP)

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
}

func runInteractiveSetup() InstallationConfig {
	scanner := bufio.NewScanner(os.Stdin)
	config := InstallationConfig{
		BackupEnabled:   true,
		OutputDirectory: "./dvarpala-deployment",
		VMConfig: VMConfig{
			DiskSize: 50,
			Tags: map[string]string{
				"Project":     "dvarpala",
				"Environment": "production",
				"ManagedBy":   "frigga-labs-installer",
			},
		},
		NetworkConfig: NetworkConfig{
			VPCCidr:           "172.30.0.0/26",
			PublicSubnetCidr:  "172.30.0.0/27",
			PrivateSubnetCidr: "172.30.0.32/27",
		},
	}

	// Cloud provider selection
	fmt.Println("☁️ Select Cloud Provider:")
	fmt.Println("1. Amazon Web Services (AWS)")
	fmt.Println("2. Google Cloud Platform (GCP)")
	fmt.Println("3. Microsoft Azure")
	fmt.Print("Enter choice (1-3): ")
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	switch choice {
	case "1":
		config.Cloud.Provider = AWS
		config = setupAWSConfig(config, scanner)
	case "2":
		config.Cloud.Provider = GCP
		config = setupGCPConfig(config, scanner)
	case "3":
		config.Cloud.Provider = Azure
		config = setupAzureConfig(config, scanner)
	default:
		log.Fatal("❌ Invalid choice")
	}

	// Admin configuration
	fmt.Println("\n👤 Administrator Configuration:")
	fmt.Print("Administrator email: ")
	scanner.Scan()
	config.Admin.Email = strings.TrimSpace(scanner.Text())

	fmt.Print("Administrator full name (optional): ")
	scanner.Scan()
	config.Admin.FullName = strings.TrimSpace(scanner.Text())
	if config.Admin.FullName == "" {
		parts := strings.Split(config.Admin.Email, "@")
		config.Admin.FullName = parts[0]
	}

	// Object storage bucket name will be generated later with resource names

	// Output directory
	fmt.Printf("\nOutput directory [%s]: ", config.OutputDirectory)
	scanner.Scan()
	if input := strings.TrimSpace(scanner.Text()); input != "" {
		config.OutputDirectory = input
	}

	return config
}

func setupAWSConfig(config InstallationConfig, scanner *bufio.Scanner) InstallationConfig {
	fmt.Println("\n🔧 AWS Configuration:")

	// Check for AWS CLI
	if !commandExists("aws") {
		fmt.Println("⚠️ AWS CLI not found. Install it first: https://aws.amazon.com/cli/")
		fmt.Print("Continue after installation? (y/N): ")
		scanner.Scan()
		if !strings.EqualFold(scanner.Text(), "y") {
			os.Exit(1)
		}
	}

	fmt.Print("AWS Region [us-east-1]: ")
	scanner.Scan()
	region := strings.TrimSpace(scanner.Text())
	if region == "" {
		region = "us-east-1"
	}
	config.Cloud.Region = region

	// Instance type selection
	fmt.Println("\nSelect instance type:")
	fmt.Println("1. t3.small (2 vCPU, 2GB RAM) - Basic")
	fmt.Println("2. t3.medium (2 vCPU, 4GB RAM) - Recommended")
	fmt.Println("3. t3.large (2 vCPU, 8GB RAM) - High traffic")
	fmt.Print("Enter choice (1-3) [2]: ")
	scanner.Scan()
	instanceChoice := strings.TrimSpace(scanner.Text())
	if instanceChoice == "" {
		instanceChoice = "2"
	}

	instanceTypes := map[string]string{
		"1": "t3.small",
		"2": "t3.medium",
		"3": "t3.large",
	}
	config.VMConfig.InstanceType = instanceTypes[instanceChoice]

	// Authentication method
	fmt.Println("\nAWS Authentication:")
	fmt.Println("1. Use existing AWS CLI profile")
	fmt.Println("2. Enter Access Key and Secret Key")
	fmt.Println("3. Use IAM role (for EC2/Lambda execution)")
	fmt.Print("Enter choice (1-3): ")
	scanner.Scan()
	authChoice := strings.TrimSpace(scanner.Text())

	switch authChoice {
	case "1":
		// Check for existing AWS configuration
		if !fileExists(filepath.Join(os.Getenv("HOME"), ".aws", "credentials")) {
			fmt.Println("⚠️ No AWS credentials found. Run 'aws configure' first.")
			os.Exit(1)
		}
		fmt.Println("✅ Using existing AWS CLI profile")
	case "2":
		fmt.Print("AWS Access Key ID: ")
		scanner.Scan()
		accessKey := strings.TrimSpace(scanner.Text())

		fmt.Print("AWS Secret Access Key: ")
		secretKey := readPassword()

		config.Cloud.Credentials = map[string]interface{}{
			"access_key": accessKey,
			"secret_key": secretKey,
		}
	case "3":
		fmt.Println("✅ Using IAM role authentication")
	default:
		log.Fatal("❌ Invalid choice")
	}

	return config
}

func setupGCPConfig(config InstallationConfig, scanner *bufio.Scanner) InstallationConfig {
	fmt.Println("\n🔧 Google Cloud Configuration:")

	if !commandExists("gcloud") {
		fmt.Println("⚠️ gcloud CLI not found. Install it first: https://cloud.google.com/sdk/docs/install")
		fmt.Print("Continue after installation? (y/N): ")
		scanner.Scan()
		if !strings.EqualFold(scanner.Text(), "y") {
			os.Exit(1)
		}
	}

	fmt.Print("GCP Project ID: ")
	scanner.Scan()
	config.Cloud.ProjectID = strings.TrimSpace(scanner.Text())

	fmt.Print("GCP Region [us-central1]: ")
	scanner.Scan()
	region := strings.TrimSpace(scanner.Text())
	if region == "" {
		region = "us-central1"
	}
	config.Cloud.Region = region

	// Machine type selection
	fmt.Println("\nSelect machine type:")
	fmt.Println("1. e2-small (2 vCPU, 2GB RAM) - Basic")
	fmt.Println("2. e2-medium (1 vCPU, 4GB RAM) - Recommended")
	fmt.Println("3. e2-standard-2 (2 vCPU, 8GB RAM) - High traffic")
	fmt.Print("Enter choice (1-3) [2]: ")
	scanner.Scan()
	machineChoice := strings.TrimSpace(scanner.Text())
	if machineChoice == "" {
		machineChoice = "2"
	}

	machineTypes := map[string]string{
		"1": "e2-small",
		"2": "e2-medium",
		"3": "e2-standard-2",
	}
	config.VMConfig.InstanceType = machineTypes[machineChoice]

	fmt.Println("\nAuthentication:")
	fmt.Println("1. Use existing gcloud authentication")
	fmt.Println("2. Use service account key file")
	fmt.Print("Enter choice (1-2): ")
	scanner.Scan()
	authChoice := strings.TrimSpace(scanner.Text())

	switch authChoice {
	case "1":
		fmt.Println("✅ Using existing gcloud authentication")
	case "2":
		fmt.Print("Service account key file path: ")
		scanner.Scan()
		keyPath := strings.TrimSpace(scanner.Text())
		if !fileExists(keyPath) {
			log.Fatal("❌ Service account key file not found")
		}
		config.Cloud.Credentials = map[string]interface{}{
			"service_account_key": keyPath,
		}
	default:
		log.Fatal("❌ Invalid choice")
	}

	return config
}

func setupAzureConfig(config InstallationConfig, scanner *bufio.Scanner) InstallationConfig {
	fmt.Println("\n🔧 Azure Configuration:")

	if !commandExists("az") {
		fmt.Println("⚠️ Azure CLI not found. Install it first: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli")
		fmt.Print("Continue after installation? (y/N): ")
		scanner.Scan()
		if !strings.EqualFold(scanner.Text(), "y") {
			os.Exit(1)
		}
	}

	fmt.Print("Azure Region [East US]: ")
	scanner.Scan()
	region := strings.TrimSpace(scanner.Text())
	if region == "" {
		region = "East US"
	}
	config.Cloud.Region = region

	// VM size selection
	fmt.Println("\nSelect VM size:")
	fmt.Println("1. Standard_B1ms (1 vCPU, 2GB RAM) - Basic")
	fmt.Println("2. Standard_B2s (2 vCPU, 4GB RAM) - Recommended")
	fmt.Println("3. Standard_B2ms (2 vCPU, 8GB RAM) - High traffic")
	fmt.Print("Enter choice (1-3) [2]: ")
	scanner.Scan()
	vmChoice := strings.TrimSpace(scanner.Text())
	if vmChoice == "" {
		vmChoice = "2"
	}

	vmSizes := map[string]string{
		"1": "Standard_B1ms",
		"2": "Standard_B2s",
		"3": "Standard_B2ms",
	}
	config.VMConfig.InstanceType = vmSizes[vmChoice]

	fmt.Println("\nAuthentication:")
	fmt.Println("1. Use existing Azure CLI login")
	fmt.Println("2. Use service principal")
	fmt.Print("Enter choice (1-2): ")
	scanner.Scan()
	authChoice := strings.TrimSpace(scanner.Text())

	switch authChoice {
	case "1":
		fmt.Println("✅ Using existing Azure CLI authentication")
	case "2":
		fmt.Print("Client ID: ")
		scanner.Scan()
		clientID := strings.TrimSpace(scanner.Text())

		fmt.Print("Client Secret: ")
		clientSecret := readPassword()

		fmt.Print("Tenant ID: ")
		scanner.Scan()
		tenantID := strings.TrimSpace(scanner.Text())

		config.Cloud.Credentials = map[string]interface{}{
			"client_id":     clientID,
			"client_secret": clientSecret,
			"tenant_id":     tenantID,
		}
	default:
		log.Fatal("❌ Invalid choice")
	}

	return config
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

func createConfigFromFlags(provider, region, outputDir string) InstallationConfig {
	if provider == "" {
		log.Fatal("❌ Provider must be specified when not in interactive mode")
	}
	if region == "" {
		log.Fatal("❌ Region must be specified when not in interactive mode")
	}

	return InstallationConfig{
		Cloud: CloudConfig{
			Provider: CloudProvider(provider),
			Region:   region,
		},
		OutputDirectory: outputDir,
		BackupEnabled:   true,
		VMConfig: VMConfig{
			InstanceType: getDefaultInstanceType(CloudProvider(provider)),
			DiskSize:     50,
			Tags: map[string]string{
				"Project":     "dvarpala",
				"Environment": "production",
				"ManagedBy":   "frigga-labs-installer",
			},
		},
		NetworkConfig: NetworkConfig{
			VPCCidr:           "172.30.0.0/26",
			PublicSubnetCidr:  "172.30.0.0/27",
			PrivateSubnetCidr: "172.30.0.32/27",
		},
	}
}

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
		return nil
	}
	fmt.Println("Installing AWS CLI...")
	// Implementation depends on OS
	return fmt.Errorf("AWS CLI not found. Please install from: https://aws.amazon.com/cli/")
}

func installGCloudCLI() error {
	if commandExists("gcloud") {
		return nil
	}
	fmt.Println("Installing Google Cloud CLI...")
	return fmt.Errorf("gcloud CLI not found. Please install from: https://cloud.google.com/sdk/docs/install")
}

func installAzureCLI() error {
	if commandExists("az") {
		return nil
	}
	fmt.Println("Installing Azure CLI...")
	return fmt.Errorf("Azure CLI not found. Please install from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli")
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

func createAndConfigureVM(config InstallationConfig, vpcID string) (*VMInfo, error) {
	cloudService, err := NewCloudService(config)
	if err != nil {
		return nil, err
	}

	return cloudService.CreateVM(vpcID)
}

func setupObjectStorage(config InstallationConfig, vmInfo *VMInfo) error {
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

// Utility functions
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readPassword() string {
	fmt.Print("Password: ")
	var password string
	_, err := fmt.Scanln(&password)
	if err != nil {
		log.Fatal("Failed to read password")
	}
	return password
}

func generateRandomSuffix() string {
	return fmt.Sprintf("%d", time.Now().Unix()%100000)
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
	config.ResourceNames.BucketName = generateFriggaResourceName("storage")
	config.ResourceNames.KeyPairName = generateFriggaResourceName("keypair")
	
	// Update storage bucket name to use new naming convention
	config.StorageBucket = config.ResourceNames.BucketName
	
	fmt.Printf("🏷️ Generated resource names:\n")
	fmt.Printf("   VPC: %s\n", config.ResourceNames.VPCName)
	fmt.Printf("   VM: %s\n", config.ResourceNames.VMName)
	fmt.Printf("   Storage: %s\n", config.ResourceNames.BucketName)
	fmt.Printf("   KeyPair: %s\n", config.ResourceNames.KeyPairName)
}

func getDefaultInstanceType(provider CloudProvider) string {
	defaults := map[CloudProvider]string{
		AWS:   "t3.medium",
		GCP:   "e2-medium",
		Azure: "Standard_B2s",
	}
	return defaults[provider]
}

func getStringFromCredentials(credentials map[string]interface{}, key string) string {
	if val, ok := credentials[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}


func main() {
	cloudInstaller()
}
