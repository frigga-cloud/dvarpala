package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CloudProviderType string

const (
	AWS   CloudProviderType = "aws"
	GCP   CloudProviderType = "gcp"
	Azure CloudProviderType = "azure"
)

type CloudConfig struct {
	Provider    CloudProviderType  `json:"provider"`
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

type VMResult struct {
	InstanceID  string
	PublicIP    string
	PrivateIP   string
	SSHKeyPath  string
	VPNConfig   string
	AdminConfig string
}

func ValidateConfig(config InstallationConfig) error {
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

func LoadConfigFromFile(filename string, config *InstallationConfig) error {
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

func GenerateOutputFiles(config InstallationConfig, vmInfo *VMResult) error {
	configData, _ := json.MarshalIndent(config, "", "  ")
	configPath := filepath.Join(config.OutputDirectory, "installation-config.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return err
	}

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

func PrintInstallationSummary(config InstallationConfig, vmInfo *VMResult) {
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

func PrintSSHConnectionInfo(vmInfo *VMResult) {
	fmt.Println("\n🔗 SSH Connection Information")
	fmt.Println("=============================")
	fmt.Printf("🌐 Server IP: %s\n", vmInfo.PublicIP)
	if vmInfo.SSHKeyPath != "" {
		fmt.Printf("🔑 SSH Key: %s\n", vmInfo.SSHKeyPath)

		var sshUser string
		if strings.Contains(vmInfo.SSHKeyPath, "aws") || strings.Contains(vmInfo.InstanceID, "i-") {
			sshUser = "ubuntu"
		} else if strings.Contains(vmInfo.SSHKeyPath, "gcp") || strings.Contains(vmInfo.InstanceID, "friggalabs-vm") {
			sshUser = "ubuntu"
		} else if strings.Contains(vmInfo.SSHKeyPath, "azure") {
			sshUser = "azureuser"
		} else {
			sshUser = "ubuntu"
		}

		fmt.Printf("\n📋 To connect manually:\n")
		fmt.Printf("   ssh -i %s %s@%s\n", vmInfo.SSHKeyPath, sshUser, vmInfo.PublicIP)
		fmt.Printf("\n🌐 Access Dvarpala web interface:\n")
		fmt.Printf("   http://%s:8080/health\n", vmInfo.PublicIP)
	}
}

func GenerateFriggaResourceName(resourceType string) string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	suffix := make([]byte, 5)
	for i := range suffix {
		suffix[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(1000)
	}
	return fmt.Sprintf("friggalabs-%s-%s", resourceType, string(suffix))
}

func GenerateResourceNames(config *InstallationConfig) {
	config.ResourceNames.VPCName = GenerateFriggaResourceName("vpc")
	config.ResourceNames.VMName = GenerateFriggaResourceName("vm")

	config.ResourceNames.BucketName = "friggalabs"
	config.StorageBucket = config.ResourceNames.BucketName

	config.ResourceNames.KeyPairName = GenerateFriggaResourceName("keypair")

	fmt.Printf("🏷️ Generated resource names:\n")
	fmt.Printf("   VPC: %s\n", config.ResourceNames.VPCName)
	fmt.Printf("   VM: %s\n", config.ResourceNames.VMName)
	fmt.Printf("   Storage: %s (shared Frigga bucket)\n", config.ResourceNames.BucketName)
	fmt.Printf("   KeyPair: %s\n", config.ResourceNames.KeyPairName)
}

func GetStringFromCredentials(credentials map[string]any, key string) string {
	if val, ok := credentials[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}