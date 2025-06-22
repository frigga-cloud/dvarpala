package installer

import (
	"dvarpala-cloud-installer/installer/lib"
	"dvarpala-cloud-installer/installer/providers"
	"fmt"
)

// newCloudProvider creates the appropriate cloud provider based on configuration
func newCloudProvider(config lib.InstallationConfig) (lib.CloudProvider, error) {
	switch config.Cloud.Provider {
	case lib.AWS:
		return providers.NewAWSProviderFromConfig(config)
	case lib.GCP:
		return providers.NewGCPProviderFromConfig(config)
	case lib.Azure:
		return providers.NewAzureProviderFromConfig(config)
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", config.Cloud.Provider)
	}
}

// init registers the cloud provider factory with the lib package
func init() {
	lib.SetCloudProviderFactory(newCloudProvider)
}
