// Package services provides resource service functionality
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dvarpala/internal/database/models"

	"gorm.io/gorm"
)

// Errors returned by ResourceService.
var (
	ErrResourceNotFound = errors.New("resource not found")
	ErrResourceExists   = errors.New("resource already exists")
	ErrInvalidResource  = errors.New("invalid resource")
)

// ResourceService handles the protected things a user might reach: dashboards,
// VMs, databases and services on the customer's private network.
type ResourceService struct {
	db    *gorm.DB
	audit *AuditService
}

// NewResourceService creates a resource service.
func NewResourceService(db *gorm.DB, audit *AuditService) *ResourceService {
	return &ResourceService{db: db, audit: audit}
}

// CreateResourceRequest is the input to CreateResource.
type CreateResourceRequest struct {
	Name        string `json:"name" binding:"required"`
	Type        string `json:"type" binding:"required"`
	URL         string `json:"url"`
	IPAddress   string `json:"ip_address"`
	Port        int    `json:"port"`
	Description string `json:"description"`
}

// validResourceTypes mirrors models.ResourceType.
var validResourceTypes = map[string]models.ResourceType{
	"dashboard": models.ResourceTypeDashboard,
	"vm":        models.ResourceTypeVM,
	"database":  models.ResourceTypeDatabase,
	"service":   models.ResourceTypeService,
}

// CreateResource registers something that access can be granted to.
//
// Routing (phase 4) needs an address, so vm and database resources require an
// IP. Dashboards and services may be identified by URL instead.
func (s *ResourceService) CreateResource(ctx context.Context, req CreateResourceRequest) (*models.Resource, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidResource)
	}

	kind, ok := validResourceTypes[strings.ToLower(strings.TrimSpace(req.Type))]
	if !ok {
		return nil, fmt.Errorf("%w: type must be one of dashboard, vm, database, service (got %q)",
			ErrInvalidResource, req.Type)
	}

	if (kind == models.ResourceTypeVM || kind == models.ResourceTypeDatabase) && req.IPAddress == "" {
		return nil, fmt.Errorf("%w: %s resources need --ip so routes can be pushed",
			ErrInvalidResource, kind)
	}

	var existing models.Resource
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: %s", ErrResourceExists, name)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("checking for existing resource: %w", err)
	}

	resource := models.Resource{
		Name:        name,
		Type:        kind,
		URL:         req.URL,
		IPAddress:   req.IPAddress,
		Port:        req.Port,
		Description: req.Description,
	}
	if err := s.db.WithContext(ctx).Create(&resource).Error; err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	s.audit.Log(ctx, Entry{
		Action:       "resource_created",
		ResourceType: "resource",
		ResourceID:   &resource.ID,
		Details: map[string]interface{}{
			"name": resource.Name, "type": string(resource.Type),
			"ip": resource.IPAddress, "url": resource.URL,
		},
	})

	return &resource, nil
}

// ListResources returns every registered resource.
func (s *ResourceService) ListResources(ctx context.Context) ([]models.Resource, error) {
	var resources []models.Resource
	if err := s.db.WithContext(ctx).Order("id").Find(&resources).Error; err != nil {
		return nil, fmt.Errorf("listing resources: %w", err)
	}
	return resources, nil
}

// GetResourceByName looks up one resource.
func (s *ResourceService) GetResourceByName(ctx context.Context, name string) (*models.Resource, error) {
	var resource models.Resource
	err := s.db.WithContext(ctx).Where("name = ?", strings.TrimSpace(name)).First(&resource).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrResourceNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("looking up resource: %w", err)
	}
	return &resource, nil
}

// Address returns how a resource is reached, for display and routing.
func Address(r models.Resource) string {
	switch {
	case r.IPAddress != "" && r.Port != 0:
		return fmt.Sprintf("%s:%d", r.IPAddress, r.Port)
	case r.IPAddress != "":
		return r.IPAddress
	case r.URL != "":
		return r.URL
	default:
		return "-"
	}
}
