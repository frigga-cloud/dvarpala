// Package services provides resource service functionality
package services

import (
	"context"
	"errors"
	"fmt"
	"net"
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

	// An address that cannot be turned into a route is refused here rather
	// than stored. Anything at all used to be accepted: a typo, or a prefix
	// length the route builder did not recognise, produced a resource that
	// read as granted in the console and in the CLI and reached nothing.
	req.IPAddress = strings.TrimSpace(req.IPAddress)
	if req.IPAddress != "" {
		if _, _, ok := SplitCIDR(req.IPAddress); !ok {
			return nil, fmt.Errorf("%w: %q is not an IPv4 address or range (for example 10.20.1.55, or 10.20.0.0/16 for a whole network)",
				ErrInvalidResource, req.IPAddress)
		}
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

// SplitCIDR converts a resource address into the network/netmask pair OpenVPN
// wants, reporting whether it could.
//
// A bare address becomes a single-host route. A range is reduced to its
// network address first, because OpenVPN refuses a route carrying host bits
// below the netmask - 10.20.1.5/24 has to be pushed as 10.20.1.0 255.255.255.0.
//
// It lives here, next to the validation that uses it, so the check made when a
// resource is registered and the conversion made when a route is pushed cannot
// disagree about what is routable.
//
// An address it cannot convert is refused rather than approximated. A fixed
// table of prefix lengths used to sit here, and anything outside it fell back
// to a single host: /20 silently became /32, so a grant covering four thousand
// addresses reached exactly one and said nothing anywhere.
func SplitCIDR(addr string) (network, netmask string, ok bool) {
	addr = strings.TrimSpace(addr)

	if !strings.Contains(addr, "/") {
		ip := net.ParseIP(addr)
		if ip == nil || ip.To4() == nil {
			return "", "", false
		}
		return ip.To4().String(), "255.255.255.255", true
	}

	// ParseCIDR returns the masked network, which is exactly what is wanted:
	// it turns a host address written with a prefix into the network it names.
	_, ipnet, err := net.ParseCIDR(addr)
	if err != nil || ipnet.IP.To4() == nil {
		return "", "", false
	}
	return ipnet.IP.String(), net.IP(ipnet.Mask).String(), true
}
