package main

import (
	_ "dvarpala-cloud-installer/installer"
	"dvarpala-cloud-installer/installer/lib"
	_ "dvarpala-cloud-installer/installer/providers"
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
		fmt.Println("  go run main.go --config=config.json")
		fmt.Println()
		fmt.Println("📋 Create a config.json file with your cloud provider settings.")
		fmt.Println("📖 See examples/ directory for sample configuration files.")
		fmt.Println()
		flag.Usage()
		os.Exit(1)
	}

	var config lib.InstallationConfig
	if err := lib.LoadConfigFromFile(*configFile, &config); err != nil {
		log.Fatalf("❌ Failed to load config file: %v", err)
	}
	fmt.Printf("✅ Configuration loaded from: %s\n", *configFile)

	// Generate Frigga resource names
	lib.GenerateResourceNames(&config)

	// Validate configuration
	if err := lib.ValidateConfig(config); err != nil {
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
	if err := lib.InstallCloudTools(config.Cloud.Provider); err != nil {
		log.Fatalf("❌ Failed to install cloud tools: %v", err)
	}
	fmt.Printf("✅ Cloud provider tools installed successfully\n")

	// Authenticate with cloud provider
	fmt.Println("\n🔐 Authenticating with cloud provider...")
	if err := lib.AuthenticateCloudProvider(config); err != nil {
		log.Fatalf("❌ Cloud authentication failed: %v", err)
	}
	fmt.Printf("✅ Cloud authentication successful\n")

	// STEP 3: Create VPC, VM and bucket
	fmt.Println("\n🌐 Step 3: Setting up cloud infrastructure...")

	// Create VPC
	vpcID, err := lib.SetupVPC(config)
	if err != nil {
		log.Fatalf("❌ VPC setup failed: %v", err)
	}
	fmt.Printf("✅ VPC ready: %s\n", vpcID)

	// Create storage bucket (if enabled)
	if config.BackupEnabled {
		fmt.Println("☁️ Creating object storage bucket...")
		if err := lib.SetupObjectStorage(config, nil); err != nil {
			log.Printf("⚠️ Object storage setup failed: %v", err)
		} else {
			fmt.Println("✅ Object storage bucket created")
		}
	}

	// Create VM and run installation (Steps 4-11)
	fmt.Println("\n💻 Creating VM and running installation...")
	vmInfo, err := lib.CreateVM(config, vpcID)
	if err != nil {
		log.Fatalf("❌ VM creation and installation failed: %v", err)
	}
	fmt.Printf("✅ VM created and installation completed: %s (IP: %s)\n", vmInfo.InstanceID, vmInfo.PublicIP)

	// STEP 12: Download admin.ovpn file to local
	fmt.Println("\n📄 Step 12: Downloading admin.ovpn to local...")
	if err := lib.DownloadAdminOVPN(config, vmInfo); err != nil {
		log.Printf("⚠️ Failed to download admin.ovpn: %v", err)
	} else {
		fmt.Println("✅ Admin OpenVPN configuration downloaded")
	}

	// STEP 13: Check installation and exit
	fmt.Println("\n🔍 Step 13: Verifying installation...")
	if err := lib.VerifyInstallation(vmInfo.PublicIP); err != nil {
		log.Printf("⚠️ Installation verification failed: %v", err)
	} else {
		fmt.Println("✅ Installation verification successful")
	}

	// STEP 14: Configure 2-step VPN access (AFTER download is complete)
	fmt.Println("\n🔒 Applying 2-step VPN configuration...")
	cloudProvider, err := lib.NewCloudProvider(config)
	if err != nil {
		log.Printf("⚠️ Failed to initialize cloud provider for 2-step config: %v", err)
	} else {
		if err := cloudProvider.Configure2StepVPNAccess(vmInfo); err != nil {
			log.Printf("⚠️ Failed to configure 2-step VPN access: %v", err)
		} else {
			fmt.Println("✅ 2-step VPN access configured")
		}
	}

	// Generate output files
	fmt.Println("\n📁 Generating output files...")
	if err := lib.GenerateOutputFiles(config, vmInfo); err != nil {
		log.Fatalf("❌ Failed to generate output files: %v", err)
	}

	// Final summary
	lib.PrintInstallationSummary(config, vmInfo)

	// Print SSH connection details for manual access
	lib.PrintSSHConnectionInfo(vmInfo)
}

func main() {
	cloudInstaller()
}
