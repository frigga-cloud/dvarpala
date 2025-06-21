package main

import (
	"encoding/json"
	"fmt"
	"dvarpala-cloud-installer/providers"
	"dvarpala-cloud-installer/installer/lib"
)


// CloudProvider type alias for the interface defined in lib
type CloudProvider = lib.CloudProvider

// BaseCloudProvider provides common functionality for all cloud providers
type BaseCloudProvider struct {
	Config lib.InstallationConfig
}

// Common installation method - same across all providers
func (base *BaseCloudProvider) InstallDvarpala(vmInfo *lib.VMResult) error {
	fmt.Println("🚀 Starting Dvarpala installation...")
	fmt.Printf("📍 Target VM: %s (IP: %s)\n", vmInfo.InstanceID, vmInfo.PublicIP)
	fmt.Printf("🔑 SSH Key: %s\n", vmInfo.SSHKeyPath)
	
	// This is where the common installation logic would go
	// In the current implementation, this is handled by the provider's InstallDvarpalaDirectly method
	// but in a unified structure, this could be common code
	
	fmt.Println("✅ Dvarpala installation completed successfully!")
	return nil
}

// AWSCloudProvider implements CloudProvider for AWS
type AWSCloudProvider struct {
	BaseCloudProvider
	provider *providers.AWSProvider
}

func NewAWSCloudProvider(config lib.InstallationConfig) (*AWSCloudProvider, error) {
	awsProvider := providers.NewAWSProvider(config.Cloud.Region, providers.AWSCredentials{
		AccessKeyID:     lib.GetStringFromCredentials(config.Cloud.Credentials, "access_key"),
		SecretAccessKey: lib.GetStringFromCredentials(config.Cloud.Credentials, "secret_key"),
	})

	if err := awsProvider.SetupEnvironment(); err != nil {
		return nil, err
	}

	if err := awsProvider.ValidateAuthentication(); err != nil {
		return nil, err
	}

	return &AWSCloudProvider{
		BaseCloudProvider: BaseCloudProvider{Config: config},
		provider:          awsProvider,
	}, nil
}

func (aws *AWSCloudProvider) SetupVPC() (string, error) {
	vpcInfo, err := aws.provider.CreateOrGetVPC(aws.Config.ResourceNames.VPCName, providers.NetworkConfig{
		VPCCidr:           aws.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  aws.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: aws.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        aws.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return "", err
	}
	return vpcInfo.VPCID, nil
}

func (aws *AWSCloudProvider) CreateVM(vpcID string) (*lib.VMResult, error) {
	// Get VPC info again for VM creation
	vpcInfo, err := aws.provider.CreateOrGetVPC(aws.Config.ResourceNames.VPCName, providers.NetworkConfig{
		VPCCidr:           aws.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  aws.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: aws.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        aws.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	instanceInfo, err := aws.provider.CreateInstance(vpcInfo, providers.InstanceConfig{
		InstanceType: aws.Config.VMConfig.InstanceType,
		DiskSizeGB:   aws.Config.VMConfig.DiskSize,
		AdminEmail:   aws.Config.Admin.Email,
		AdminName:    aws.Config.Admin.FullName,
	}, aws.Config.ResourceNames.VMName)
	if err != nil {
		return nil, err
	}

	// Perform direct installation
	if err := aws.provider.InstallDvarpalaDirectly(instanceInfo, providers.InstanceConfig{
		InstanceType: aws.Config.VMConfig.InstanceType,
		DiskSizeGB:   aws.Config.VMConfig.DiskSize,
		AdminEmail:   aws.Config.Admin.Email,
		AdminName:    aws.Config.Admin.FullName,
	}, "aws"); err != nil {
		return nil, fmt.Errorf("direct installation failed: %v", err)
	}

	return &lib.VMResult{
		InstanceID: instanceInfo.InstanceID,
		PublicIP:   instanceInfo.PublicIP,
		PrivateIP:  instanceInfo.PrivateIP,
		SSHKeyPath: instanceInfo.KeyPairName, // AWS uses key pair name
	}, nil
}

func (aws *AWSCloudProvider) SetupObjectStorage() error {
	return aws.provider.CreateS3Bucket(aws.Config.ResourceNames.BucketName)
}

func (aws *AWSCloudProvider) UploadConfiguration() error {
	configData, err := json.MarshalIndent(aws.Config, "", "  ")
	if err != nil {
		return err
	}
	return aws.provider.UploadConfiguration(aws.Config.ResourceNames.BucketName, configData, "installation-config.json")
}

func (aws *AWSCloudProvider) Configure2StepVPNAccess(vmInfo *lib.VMResult) error {
	// Convert VMResult to InstanceInfo for the provider
	instanceInfo := &providers.AWSInstanceInfo{
		InstanceID:      vmInfo.InstanceID,
		PublicIP:        vmInfo.PublicIP,
		PrivateIP:       vmInfo.PrivateIP,
		KeyPairName:     vmInfo.SSHKeyPath,
		SecurityGroupID: "", // Not needed for this operation
	}
	
	return aws.provider.Configure2StepVPNAccess(instanceInfo, providers.InstanceConfig{
		AdminEmail: aws.Config.Admin.Email,
		AdminName:  aws.Config.Admin.FullName,
	})
}

// GCPCloudProvider implements CloudProvider for Google Cloud Platform
type GCPCloudProvider struct {
	BaseCloudProvider
	provider *providers.GCPProvider
}

func NewGCPCloudProvider(config lib.InstallationConfig) (*GCPCloudProvider, error) {
	gcpProvider := providers.NewGCPProvider(config.Cloud.ProjectID, config.Cloud.Region, providers.GCPCredentials{
		ServiceAccountKey: lib.GetStringFromCredentials(config.Cloud.Credentials, "service_account_key"),
	})

	if err := gcpProvider.SetupEnvironment(); err != nil {
		return nil, err
	}

	if err := gcpProvider.ValidateAuthentication(); err != nil {
		return nil, err
	}

	return &GCPCloudProvider{
		BaseCloudProvider: BaseCloudProvider{Config: config},
		provider:          gcpProvider,
	}, nil
}

func (gcp *GCPCloudProvider) SetupVPC() (string, error) {
	vpcInfo, err := gcp.provider.CreateOrGetVPC(gcp.Config.ResourceNames.VPCName, providers.NetworkConfig{
		VPCCidr:           gcp.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  gcp.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: gcp.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        gcp.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return "", err
	}
	return vpcInfo.GetID(), nil
}

func (gcp *GCPCloudProvider) CreateVM(vpcID string) (*lib.VMResult, error) {
	// Get VPC info again for VM creation
	vpcInfo, err := gcp.provider.CreateOrGetVPC(gcp.Config.ResourceNames.VPCName, providers.NetworkConfig{
		VPCCidr:           gcp.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  gcp.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: gcp.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        gcp.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	instanceInfo, err := gcp.provider.CreateInstance(vpcInfo, providers.InstanceConfig{
		InstanceType: gcp.Config.VMConfig.InstanceType,
		DiskSizeGB:   gcp.Config.VMConfig.DiskSize,
		AdminEmail:   gcp.Config.Admin.Email,
		AdminName:    gcp.Config.Admin.FullName,
	}, gcp.Config.ResourceNames.VMName)
	if err != nil {
		return nil, err
	}

	// Perform direct installation using the base provider's InstallDvarpalaDirectly
	if err := gcp.provider.InstallDvarpalaDirectly(instanceInfo, providers.InstanceConfig{
		InstanceType: gcp.Config.VMConfig.InstanceType,
		DiskSizeGB:   gcp.Config.VMConfig.DiskSize,
		AdminEmail:   gcp.Config.Admin.Email,
		AdminName:    gcp.Config.Admin.FullName,
	}, "gcp"); err != nil {
		return nil, fmt.Errorf("direct installation failed: %v", err)
	}

	return &lib.VMResult{
		InstanceID: instanceInfo.GetInstanceID(),
		PublicIP:   instanceInfo.GetPublicIP(),
		PrivateIP:  instanceInfo.GetPrivateIP(),
		SSHKeyPath: instanceInfo.GetSSHKeyPath(),
	}, nil
}

func (gcp *GCPCloudProvider) SetupObjectStorage() error {
	return gcp.provider.CreateStorage(gcp.Config.ResourceNames.BucketName)
}

func (gcp *GCPCloudProvider) UploadConfiguration() error {
	configData, err := json.MarshalIndent(gcp.Config, "", "  ")
	if err != nil {
		return err
	}
	return gcp.provider.UploadConfiguration(gcp.Config.ResourceNames.BucketName, configData, "installation-config.json")
}

func (gcp *GCPCloudProvider) Configure2StepVPNAccess(vmInfo *lib.VMResult) error {
	// Convert VMResult to InstanceInfo for the provider
	instanceInfo := &providers.GCPInstanceInfo{
		InstanceName: vmInfo.InstanceID,
		ExternalIP:   vmInfo.PublicIP,
		InternalIP:   vmInfo.PrivateIP,
		SSHKeyPath:   vmInfo.SSHKeyPath,
		Zone:         gcp.provider.Zone,
	}
	
	return gcp.provider.Configure2StepVPNAccess(instanceInfo, providers.InstanceConfig{
		AdminEmail: gcp.Config.Admin.Email,
		AdminName:  gcp.Config.Admin.FullName,
	})
}

// AzureCloudProvider implements CloudProvider for Microsoft Azure
type AzureCloudProvider struct {
	BaseCloudProvider
	provider *providers.AzureProvider
}

func NewAzureCloudProvider(config lib.InstallationConfig) (*AzureCloudProvider, error) {
	azureProvider := providers.NewAzureProvider("", config.Cloud.Region, providers.AzureCredentials{
		ClientID:     lib.GetStringFromCredentials(config.Cloud.Credentials, "client_id"),
		ClientSecret: lib.GetStringFromCredentials(config.Cloud.Credentials, "client_secret"),
		TenantID:     lib.GetStringFromCredentials(config.Cloud.Credentials, "tenant_id"),
	})

	if err := azureProvider.SetupEnvironment(); err != nil {
		return nil, err
	}

	if err := azureProvider.ValidateAuthentication(); err != nil {
		return nil, err
	}

	return &AzureCloudProvider{
		BaseCloudProvider: BaseCloudProvider{Config: config},
		provider:          azureProvider,
	}, nil
}

func (azure *AzureCloudProvider) SetupVPC() (string, error) {
	vpcInfo, err := azure.provider.CreateOrGetVPC(azure.Config.ResourceNames.VPCName, providers.NetworkConfig{
		VPCCidr:           azure.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  azure.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: azure.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        azure.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return "", err
	}
	return vpcInfo.ResourceGroup, nil
}

func (azure *AzureCloudProvider) CreateVM(vpcID string) (*lib.VMResult, error) {
	// Get resource group info again for VM creation
	resourceGroupInfo, err := azure.provider.CreateOrGetVPC(azure.Config.ResourceNames.VPCName, providers.NetworkConfig{
		VPCCidr:           azure.Config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  azure.Config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: azure.Config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        azure.Config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	instanceInfo, err := azure.provider.CreateInstance(resourceGroupInfo, providers.InstanceConfig{
		InstanceType: azure.Config.VMConfig.InstanceType,
		DiskSizeGB:   azure.Config.VMConfig.DiskSize,
		AdminEmail:   azure.Config.Admin.Email,
		AdminName:    azure.Config.Admin.FullName,
	}, azure.Config.ResourceNames.VMName)
	if err != nil {
		return nil, err
	}

	// Perform direct installation
	if err := azure.provider.InstallDvarpalaDirectly(instanceInfo, providers.InstanceConfig{
		InstanceType: azure.Config.VMConfig.InstanceType,
		DiskSizeGB:   azure.Config.VMConfig.DiskSize,
		AdminEmail:   azure.Config.Admin.Email,
		AdminName:    azure.Config.Admin.FullName,
	}, "azure"); err != nil {
		return nil, fmt.Errorf("direct installation failed: %v", err)
	}

	return &lib.VMResult{
		InstanceID: instanceInfo.VMName,
		PublicIP:   instanceInfo.PublicIP,
		PrivateIP:  instanceInfo.PrivateIP,
		SSHKeyPath: instanceInfo.SSHKeyPath,
	}, nil
}

func (azure *AzureCloudProvider) SetupObjectStorage() error {
	return azure.provider.CreateStorageAccount(azure.Config.ResourceNames.BucketName)
}

func (azure *AzureCloudProvider) UploadConfiguration() error {
	configData, err := json.MarshalIndent(azure.Config, "", "  ")
	if err != nil {
		return err
	}
	return azure.provider.UploadConfiguration(azure.Config.ResourceNames.BucketName, configData, "installation-config.json")
}

func (azure *AzureCloudProvider) Configure2StepVPNAccess(vmInfo *lib.VMResult) error {
	// Convert VMResult to InstanceInfo for the provider
	instanceInfo := &providers.AzureInstanceInfo{
		VMName:        vmInfo.InstanceID,
		PublicIP:      vmInfo.PublicIP,
		PrivateIP:     vmInfo.PrivateIP,
		SSHKeyPath:    vmInfo.SSHKeyPath,
		ResourceGroup: azure.Config.ResourceNames.VPCName, // Using VPC name as resource group
	}
	
	return azure.provider.Configure2StepVPNAccess(instanceInfo, providers.InstanceConfig{
		AdminEmail: azure.Config.Admin.Email,
		AdminName:  azure.Config.Admin.FullName,
	})
}

// newCloudProvider creates the appropriate cloud provider based on configuration
func newCloudProvider(config lib.InstallationConfig) (lib.CloudProvider, error) {
	switch config.Cloud.Provider {
	case lib.AWS:
		return NewAWSCloudProvider(config)
	case lib.GCP:
		return NewGCPCloudProvider(config)
	case lib.Azure:
		return NewAzureCloudProvider(config)
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", config.Cloud.Provider)
	}
}

// init registers the cloud provider factory with the lib package
func init() {
	lib.SetCloudProviderFactory(newCloudProvider)
}