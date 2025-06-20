package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

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
		fmt.Println("  go run installer/installer.go installer/installer_lib.go installer/cloud_service.go installer/cloud_wrappers.go --config=config.json")
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

func main() {
	cloudInstaller()
}
