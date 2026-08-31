package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dvarpala/internal/database/models"

	"gorm.io/gorm"
)

// Errors returned by DomainService.
var (
	ErrInvalidDomain  = errors.New("invalid domain")
	ErrDomainExists   = errors.New("domain already allowed")
	ErrDomainNotFound = errors.New("domain not in the allow-list")
)

// DomainService is the list of email domains permitted to sign in.
//
// It used to be a slice read from the configuration file once at startup,
// which meant adding a domain required editing YAML on the server as root and
// restarting the service - so in practice the guess the installer made from
// the first administrator's address was the list forever. Holding it in the
// database lets the console change it, and makes every change auditable.
type DomainService struct {
	db    *gorm.DB
	audit *AuditService
}

// NewDomainService creates the allow-list service.
func NewDomainService(db *gorm.DB, audit *AuditService) *DomainService {
	return &DomainService{db: db, audit: audit}
}

// List returns every allowed domain, alphabetically.
func (s *DomainService) List(ctx context.Context) ([]models.AllowedDomain, error) {
	var domains []models.AllowedDomain
	if err := s.db.WithContext(ctx).Order("domain").Find(&domains).Error; err != nil {
		return nil, fmt.Errorf("listing allowed domains: %w", err)
	}
	return domains, nil
}

// Allowed reports whether an email address may sign in.
//
// An empty list accepts everything. That is the historic behaviour of an
// unset allow-list and changing it here would lock out every deployment that
// never configured one; the console warns about the state instead.
func (s *DomainService) Allowed(ctx context.Context, email string) (bool, error) {
	domain, err := domainOf(email)
	if err != nil {
		return false, nil
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&models.AllowedDomain{}).Count(&count).Error; err != nil {
		return false, fmt.Errorf("counting allowed domains: %w", err)
	}
	if count == 0 {
		return true, nil
	}

	var match int64
	if err := s.db.WithContext(ctx).Model(&models.AllowedDomain{}).
		Where("domain = ?", domain).Count(&match).Error; err != nil {
		return false, fmt.Errorf("checking allowed domain: %w", err)
	}
	return match > 0, nil
}

// Add puts a domain on the list. addedBy is the administrator responsible,
// or empty when seeding from configuration.
func (s *DomainService) Add(ctx context.Context, domain, addedBy, note string) (*models.AllowedDomain, error) {
	clean, err := normaliseDomain(domain)
	if err != nil {
		return nil, err
	}

	var existing models.AllowedDomain
	err = s.db.WithContext(ctx).Where("domain = ?", clean).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: %s", ErrDomainExists, clean)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("checking for existing domain: %w", err)
	}

	record := models.AllowedDomain{Domain: clean, AddedBy: addedBy, Note: note}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("adding domain: %w", err)
	}

	s.audit.Log(ctx, Entry{
		Action:       "allowed_domain_added",
		ResourceType: "allowed_domain",
		ResourceID:   &record.ID,
		Details: map[string]interface{}{
			"domain": clean, "added_by": addedBy, "note": note,
		},
	})

	return &record, nil
}

// Remove takes a domain off the list.
//
// Removing the last one is permitted and leaves the server accepting any
// domain. That is a real thing an administrator may want, and refusing it
// would make the open state unreachable from the console - but it is recorded
// as such, because it is not what removing an entry looks like it does.
func (s *DomainService) Remove(ctx context.Context, domain, removedBy string) error {
	clean, err := normaliseDomain(domain)
	if err != nil {
		return err
	}

	var record models.AllowedDomain
	if err := s.db.WithContext(ctx).Where("domain = ?", clean).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrDomainNotFound, clean)
		}
		return fmt.Errorf("looking up domain: %w", err)
	}

	if err := s.db.WithContext(ctx).Delete(&record).Error; err != nil {
		return fmt.Errorf("removing domain: %w", err)
	}

	var remaining int64
	_ = s.db.WithContext(ctx).Model(&models.AllowedDomain{}).Count(&remaining).Error

	s.audit.Log(ctx, Entry{
		Action:       "allowed_domain_removed",
		ResourceType: "allowed_domain",
		Details: map[string]interface{}{
			"domain": clean, "removed_by": removedBy,
			"remaining": remaining,
			// Worth stating outright in the trail: with none left, every
			// domain is accepted.
			"now_accepts_any_domain": remaining == 0,
		},
	})

	return nil
}

// SeedFromConfig fills an empty list from the configured domains, so an
// existing deployment keeps the allow-list it already had the first time it
// starts with this table present.
//
// It does nothing once the table has any entry, including when an
// administrator has deliberately emptied it - configuration is the starting
// point, not a thing that reasserts itself on every restart.
func (s *DomainService) SeedFromConfig(ctx context.Context, domains []string) error {
	if len(domains) == 0 {
		return nil
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&models.AllowedDomain{}).Count(&count).Error; err != nil {
		return fmt.Errorf("counting allowed domains: %w", err)
	}
	if count > 0 {
		return nil
	}

	for _, d := range domains {
		if _, err := s.Add(ctx, d, "", "seeded from configuration at first start"); err != nil {
			// A malformed entry in the config file should not stop the server
			// from starting; the others still apply.
			if !errors.Is(err, ErrDomainExists) {
				return fmt.Errorf("seeding %q: %w", d, err)
			}
		}
	}
	return nil
}

// normaliseDomain accepts what an administrator is likely to type and returns
// the form stored: lower case, no leading "@", no surrounding whitespace.
func normaliseDomain(domain string) (string, error) {
	clean := strings.ToLower(strings.TrimSpace(domain))
	clean = strings.TrimPrefix(clean, "@")

	// An address pasted in whole is a likely mistake, and the domain is
	// unambiguous, so take it rather than refusing.
	if at := strings.LastIndex(clean, "@"); at >= 0 {
		clean = clean[at+1:]
	}

	if clean == "" {
		return "", fmt.Errorf("%w: a domain is required", ErrInvalidDomain)
	}
	if strings.ContainsAny(clean, " \t/\\:") {
		return "", fmt.Errorf("%w: %q contains characters a domain cannot have", ErrInvalidDomain, domain)
	}
	if !strings.Contains(clean, ".") {
		return "", fmt.Errorf("%w: %q has no dot, so it is not a full domain (for example frigga.cloud)",
			ErrInvalidDomain, domain)
	}
	if strings.HasPrefix(clean, ".") || strings.HasSuffix(clean, ".") {
		return "", fmt.Errorf("%w: %q starts or ends with a dot", ErrInvalidDomain, domain)
	}
	return clean, nil
}

// domainOf returns the domain part of an email address.
func domainOf(email string) (string, error) {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return "", fmt.Errorf("%w: %q is not an email address", ErrInvalidDomain, email)
	}
	return strings.ToLower(strings.TrimSpace(email[at+1:])), nil
}
