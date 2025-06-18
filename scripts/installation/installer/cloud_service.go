package main

import (
	"encoding/json"
	"fmt"
)

// CloudService provides a unified interface for cloud operations
type CloudService struct {
	config   InstallationConfig
	provider CloudProviderWrapper
}

// CloudProviderWrapper wraps the different provider types with a common interface
type CloudProviderWrapper interface {
	SetupVPC(name string) (string, error)
	CreateVM(vpcID string, instanceConfig InstanceConfig) (*VMInfo, error)
	SetupStorage(bucketName string) error
	UploadConfig(bucketName string, data []byte, filename string) error
}

// InstanceConfig represents VM configuration for service layer
type InstanceConfig struct {
	InstanceType string
	DiskSizeGB   int
	AdminEmail   string
	AdminName    string
}

// NewCloudService creates a new cloud service
func NewCloudService(config InstallationConfig) (*CloudService, error) {
	service := &CloudService{config: config}

	var wrapper CloudProviderWrapper
	var err error

	switch config.Cloud.Provider {
	case AWS:
		wrapper, err = NewAWSWrapper(config)
	case GCP:
		wrapper, err = NewGCPWrapper(config)
	case Azure:
		wrapper, err = NewAzureWrapper(config)
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", config.Cloud.Provider)
	}

	if err != nil {
		return nil, err
	}

	service.provider = wrapper
	return service, nil
}

// SetupVPC creates or gets VPC
func (cs *CloudService) SetupVPC() (string, error) {
	return cs.provider.SetupVPC("frigga-labs")
}

// CreateVM creates a virtual machine
func (cs *CloudService) CreateVM(vpcID string) (*VMInfo, error) {
	instanceConfig := InstanceConfig{
		InstanceType: cs.config.VMConfig.InstanceType,
		DiskSizeGB:   cs.config.VMConfig.DiskSize,
		AdminEmail:   cs.config.Admin.Email,
		AdminName:    cs.config.Admin.FullName,
	}

	return cs.provider.CreateVM(vpcID, instanceConfig)
}

// SetupObjectStorage sets up object storage
func (cs *CloudService) SetupObjectStorage() error {
	return cs.provider.SetupStorage(cs.config.StorageBucket)
}

// UploadConfiguration uploads configuration to object storage
func (cs *CloudService) UploadConfiguration() error {
	configData, err := json.MarshalIndent(cs.config, "", "  ")
	if err != nil {
		return err
	}

	return cs.provider.UploadConfig(cs.config.StorageBucket, configData, "installation-config.json")
}