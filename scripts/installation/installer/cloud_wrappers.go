package main

import (
	"fmt"
	"dvarpala-cloud-installer/providers"
)

// AWS Wrapper
type AWSWrapper struct {
	provider *providers.AWSProvider
	config   InstallationConfig
}

func NewAWSWrapper(config InstallationConfig) (*AWSWrapper, error) {
	awsProvider := providers.NewAWSProvider(config.Cloud.Region, providers.AWSCredentials{
		AccessKeyID:     getStringFromCredentials(config.Cloud.Credentials, "access_key"),
		SecretAccessKey: getStringFromCredentials(config.Cloud.Credentials, "secret_key"),
	})

	if err := awsProvider.SetupEnvironment(); err != nil {
		return nil, err
	}

	return &AWSWrapper{
		provider: awsProvider,
		config:   config,
	}, nil
}

func (aw *AWSWrapper) SetupVPC(name string) (string, error) {
	// Use Frigga naming convention for VPC
	vpcName := aw.config.ResourceNames.VPCName
	vpcInfo, err := aw.provider.CreateOrGetVPC(vpcName, providers.NetworkConfig{
		VPCCidr:           aw.config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  aw.config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: aw.config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        aw.config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return "", err
	}
	return vpcInfo.VPCID, nil
}

func (aw *AWSWrapper) CreateVM(vpcID string, instanceConfig InstanceConfig) (*VMInfo, error) {
	// Use existing VPC (already created by SetupVPC)
	vpcName := aw.config.ResourceNames.VPCName
	vpcInfo, err := aw.provider.CreateOrGetVPC(vpcName, providers.NetworkConfig{
		VPCCidr:           aw.config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  aw.config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: aw.config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        aw.config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	// Use Frigga naming convention for VM instance
	instanceInfo, err := aw.provider.CreateInstance(vpcInfo, providers.InstanceConfig{
		InstanceType: instanceConfig.InstanceType,
		DiskSizeGB:   instanceConfig.DiskSizeGB,
		AdminEmail:   instanceConfig.AdminEmail,
		AdminName:    instanceConfig.AdminName,
	}, aw.config.ResourceNames.VMName)
	if err != nil {
		return nil, err
	}

	return &VMInfo{
		InstanceID: instanceInfo.InstanceID,
		PublicIP:   instanceInfo.PublicIP,
		PrivateIP:  instanceInfo.PrivateIP,
		SSHKeyPath: instanceInfo.KeyPairName + ".pem",
	}, nil
}

func (aw *AWSWrapper) SetupStorage(bucketName string) error {
	// Use Frigga naming convention for S3 bucket
	return aw.provider.CreateS3Bucket(aw.config.ResourceNames.BucketName)
}

func (aw *AWSWrapper) UploadConfig(bucketName string, data []byte, filename string) error {
	// Use Frigga naming convention for bucket operations
	return aw.provider.UploadConfiguration(aw.config.ResourceNames.BucketName, data, filename)
}

// GCP Wrapper
type GCPWrapper struct {
	provider *providers.GCPProvider
	config   InstallationConfig
}

func NewGCPWrapper(config InstallationConfig) (*GCPWrapper, error) {
	gcpProvider := providers.NewGCPProvider(config.Cloud.ProjectID, config.Cloud.Region, providers.GCPCredentials{
		ServiceAccountKey: getStringFromCredentials(config.Cloud.Credentials, "service_account_key"),
	})

	if err := gcpProvider.SetupEnvironment(); err != nil {
		return nil, err
	}

	return &GCPWrapper{
		provider: gcpProvider,
		config:   config,
	}, nil
}

func (gw *GCPWrapper) SetupVPC(name string) (string, error) {
	// Use Frigga naming convention for VPC
	vpcName := gw.config.ResourceNames.VPCName
	vpcInfo, err := gw.provider.CreateOrGetVPC(vpcName, providers.NetworkConfig{
		VPCCidr:           gw.config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  gw.config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: gw.config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        gw.config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return "", err
	}
	return vpcInfo.VPCName, nil
}

func (gw *GCPWrapper) CreateVM(vpcID string, instanceConfig InstanceConfig) (*VMInfo, error) {
	// Use existing VPC (already created by SetupVPC)
	vpcName := gw.config.ResourceNames.VPCName
	vpcInfo, err := gw.provider.CreateOrGetVPC(vpcName, providers.NetworkConfig{
		VPCCidr:           gw.config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  gw.config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: gw.config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        gw.config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	// Use Frigga naming convention for VM instance
	instanceInfo, err := gw.provider.CreateInstance(vpcInfo, providers.InstanceConfig{
		InstanceType: instanceConfig.InstanceType,
		DiskSizeGB:   instanceConfig.DiskSizeGB,
		AdminEmail:   instanceConfig.AdminEmail,
		AdminName:    instanceConfig.AdminName,
	}, gw.config.ResourceNames.VMName)
	if err != nil {
		return nil, err
	}

	// Perform direct installation instead of relying on cloud-init
	fmt.Println("🚀 Starting direct installation on VM...")
	if err := gw.provider.InstallDvarpalaDirectly(instanceInfo, providers.InstanceConfig{
		InstanceType: instanceConfig.InstanceType,
		DiskSizeGB:   instanceConfig.DiskSizeGB,
		AdminEmail:   instanceConfig.AdminEmail,
		AdminName:    instanceConfig.AdminName,
	}); err != nil {
		return nil, fmt.Errorf("direct installation failed: %v", err)
	}

	return &VMInfo{
		InstanceID: instanceInfo.InstanceName,
		PublicIP:   instanceInfo.ExternalIP,
		PrivateIP:  instanceInfo.InternalIP,
	}, nil
}

func (gw *GCPWrapper) SetupStorage(bucketName string) error {
	// Use Frigga naming convention for Cloud Storage bucket
	return gw.provider.CreateStorageBucket(gw.config.ResourceNames.BucketName)
}

func (gw *GCPWrapper) UploadConfig(bucketName string, data []byte, filename string) error {
	// Use Frigga naming convention for bucket operations
	return gw.provider.UploadConfiguration(gw.config.ResourceNames.BucketName, data, filename)
}

// Azure Wrapper
type AzureWrapper struct {
	provider *providers.AzureProvider
	config   InstallationConfig
}

func NewAzureWrapper(config InstallationConfig) (*AzureWrapper, error) {
	azureProvider := providers.NewAzureProvider("", config.Cloud.Region, providers.AzureCredentials{
		ClientID:     getStringFromCredentials(config.Cloud.Credentials, "client_id"),
		ClientSecret: getStringFromCredentials(config.Cloud.Credentials, "client_secret"),
		TenantID:     getStringFromCredentials(config.Cloud.Credentials, "tenant_id"),
	})

	if err := azureProvider.SetupEnvironment(); err != nil {
		return nil, err
	}

	return &AzureWrapper{
		provider: azureProvider,
		config:   config,
	}, nil
}

func (azw *AzureWrapper) SetupVPC(name string) (string, error) {
	// Use Frigga naming convention for VNet
	vpcName := azw.config.ResourceNames.VPCName
	vpcInfo, err := azw.provider.CreateOrGetVPC(vpcName, providers.NetworkConfig{
		VPCCidr:           azw.config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  azw.config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: azw.config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        azw.config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return "", err
	}
	return vpcInfo.ResourceGroup, nil
}

func (azw *AzureWrapper) CreateVM(vpcID string, instanceConfig InstanceConfig) (*VMInfo, error) {
	// Use existing VNet (already created by SetupVPC)
	vpcName := azw.config.ResourceNames.VPCName
	vpcInfo, err := azw.provider.CreateOrGetVPC(vpcName, providers.NetworkConfig{
		VPCCidr:           azw.config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  azw.config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: azw.config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        azw.config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	// Use Frigga naming convention for VM instance
	instanceInfo, err := azw.provider.CreateInstance(vpcInfo, providers.InstanceConfig{
		InstanceType: instanceConfig.InstanceType,
		DiskSizeGB:   instanceConfig.DiskSizeGB,
		AdminEmail:   instanceConfig.AdminEmail,
		AdminName:    instanceConfig.AdminName,
	}, azw.config.ResourceNames.VMName)
	if err != nil {
		return nil, err
	}

	return &VMInfo{
		InstanceID: instanceInfo.VMName,
		PublicIP:   instanceInfo.PublicIP,
		PrivateIP:  instanceInfo.PrivateIP,
		SSHKeyPath: instanceInfo.SSHKeyPath,
	}, nil
}

func (azw *AzureWrapper) SetupStorage(bucketName string) error {
	// Use Frigga naming convention for Storage Account
	return azw.provider.CreateStorageAccount(azw.config.ResourceNames.BucketName)
}

func (azw *AzureWrapper) UploadConfig(bucketName string, data []byte, filename string) error {
	// Use Frigga naming convention for storage operations
	return azw.provider.UploadConfiguration(azw.config.ResourceNames.BucketName, data, filename)
}