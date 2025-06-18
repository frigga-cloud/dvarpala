package main

import (
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
	vpcInfo, err := aw.provider.CreateOrGetVPC(name, providers.NetworkConfig{
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
	vpcInfo, err := aw.provider.CreateOrGetVPC("frigga-labs", providers.NetworkConfig{
		VPCCidr:           aw.config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  aw.config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: aw.config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        aw.config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	instanceInfo, err := aw.provider.CreateInstance(vpcInfo, providers.InstanceConfig{
		InstanceType: instanceConfig.InstanceType,
		DiskSizeGB:   instanceConfig.DiskSizeGB,
		AdminEmail:   instanceConfig.AdminEmail,
		AdminName:    instanceConfig.AdminName,
	})
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
	return aw.provider.CreateS3Bucket(bucketName)
}

func (aw *AWSWrapper) UploadConfig(bucketName string, data []byte, filename string) error {
	return aw.provider.UploadConfiguration(bucketName, data, filename)
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
	vpcInfo, err := gw.provider.CreateOrGetVPC(name, providers.NetworkConfig{
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
	vpcInfo, err := gw.provider.CreateOrGetVPC("frigga-labs", providers.NetworkConfig{
		VPCCidr:           gw.config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  gw.config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: gw.config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        gw.config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	instanceInfo, err := gw.provider.CreateInstance(vpcInfo, providers.InstanceConfig{
		InstanceType: instanceConfig.InstanceType,
		DiskSizeGB:   instanceConfig.DiskSizeGB,
		AdminEmail:   instanceConfig.AdminEmail,
		AdminName:    instanceConfig.AdminName,
	})
	if err != nil {
		return nil, err
	}

	return &VMInfo{
		InstanceID: instanceInfo.InstanceName,
		PublicIP:   instanceInfo.ExternalIP,
		PrivateIP:  instanceInfo.InternalIP,
	}, nil
}

func (gw *GCPWrapper) SetupStorage(bucketName string) error {
	return gw.provider.CreateStorageBucket(bucketName)
}

func (gw *GCPWrapper) UploadConfig(bucketName string, data []byte, filename string) error {
	return gw.provider.UploadConfiguration(bucketName, data, filename)
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
	vpcInfo, err := azw.provider.CreateOrGetVPC(name, providers.NetworkConfig{
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
	vpcInfo, err := azw.provider.CreateOrGetVPC("frigga-labs", providers.NetworkConfig{
		VPCCidr:           azw.config.NetworkConfig.VPCCidr,
		PublicSubnetCidr:  azw.config.NetworkConfig.PublicSubnetCidr,
		PrivateSubnetCidr: azw.config.NetworkConfig.PrivateSubnetCidr,
		AllowedIPs:        azw.config.NetworkConfig.AllowedIPs,
	})
	if err != nil {
		return nil, err
	}

	instanceInfo, err := azw.provider.CreateInstance(vpcInfo, providers.InstanceConfig{
		InstanceType: instanceConfig.InstanceType,
		DiskSizeGB:   instanceConfig.DiskSizeGB,
		AdminEmail:   instanceConfig.AdminEmail,
		AdminName:    instanceConfig.AdminName,
	})
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
	return azw.provider.CreateStorageAccount(bucketName)
}

func (azw *AzureWrapper) UploadConfig(bucketName string, data []byte, filename string) error {
	return azw.provider.UploadConfiguration(bucketName, data, filename)
}