// Package services provides VPN client configuration functionality
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dvarpala/internal/database/models"
	"dvarpala/internal/vpn"

	"gorm.io/gorm"
)

// Errors returned by VPNConfigService.
var (
	ErrConfigNotFound = errors.New("vpn configuration not found")
	ErrNoCA           = errors.New("no certificate authority configured")
)

// ServerDetails describes the VPN endpoint clients should connect to.
type ServerDetails struct {
	Host      string
	Port      int
	Proto     string
	TLSCryptK string // contents of ta.key, if the server uses tls-crypt
}

// VPNConfigService issues and revokes per-user VPN credentials.
//
// Every user gets their own certificate, with their email as the common name.
// OpenVPN reports that name to the connect hook, so this is what allows the
// VPN to tell one person from another. A single shared certificate - which is
// what this deployment had before - makes every client anonymous and
// interchangeable.
type VPNConfigService struct {
	db     *gorm.DB
	audit  *AuditService
	users  *UserService
	ca     *vpn.CA
	server ServerDetails
}

// NewVPNConfigService creates the service. ca may be nil, in which case
// issuing returns ErrNoCA and everything else still works.
func NewVPNConfigService(db *gorm.DB, audit *AuditService, users *UserService,
	ca *vpn.CA, server ServerDetails) *VPNConfigService {
	return &VPNConfigService{db: db, audit: audit, users: users, ca: ca, server: server}
}

// Issue creates a VPN credential for a user and stores it.
//
// Any previously issued configuration for that user is revoked first: a person
// should have one active credential, so that revoking it revokes their access
// rather than one of several copies.
func (s *VPNConfigService) Issue(ctx context.Context, email, configName string,
	validity time.Duration) (*models.VPNConfig, error) {
	if s.ca == nil {
		return nil, ErrNoCA
	}

	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if err := s.revokeExisting(ctx, user.ID, "superseded by a new certificate"); err != nil {
		return nil, err
	}

	cred, err := s.ca.IssueClient(user.Email, validity)
	if err != nil {
		return nil, fmt.Errorf("issuing certificate: %w", err)
	}

	profile := s.buildProfile(cred)
	expires := cred.NotAfter

	if configName == "" {
		configName = "default"
	}

	config := models.VPNConfig{
		UserID:     user.ID,
		ConfigName: configName,
		ClientCert: cred.Certificate,
		ClientKey:  cred.PrivateKey,
		CACert:     s.ca.CertificatePEM(),
		ConfigData: profile,
		Status:     models.VPNConfigStatusActive,
		ExpiresAt:  &expires,
	}
	if err := s.db.WithContext(ctx).Create(&config).Error; err != nil {
		return nil, fmt.Errorf("storing vpn configuration: %w", err)
	}

	s.audit.Log(ctx, Entry{
		UserID:       &user.ID,
		Action:       "vpn_config_issued",
		ResourceType: "vpn_config",
		ResourceID:   &config.ID,
		Details: map[string]interface{}{
			"email": user.Email, "common_name": cred.CommonName,
			"serial": cred.Serial, "expires": expires.Format(time.RFC3339),
		},
	})

	return &config, nil
}

// ListForUser returns a user's configurations, newest first.
func (s *VPNConfigService) ListForUser(ctx context.Context, email string) ([]models.VPNConfig, error) {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	var configs []models.VPNConfig
	if err := s.db.WithContext(ctx).Where("user_id = ?", user.ID).
		Order("id desc").Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("listing vpn configurations: %w", err)
	}
	return configs, nil
}

// ListAll returns every configuration, with the owning user preloaded.
func (s *VPNConfigService) ListAll(ctx context.Context) ([]models.VPNConfig, error) {
	var configs []models.VPNConfig
	if err := s.db.WithContext(ctx).Preload("User").
		Order("id desc").Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("listing vpn configurations: %w", err)
	}
	return configs, nil
}

// Revoke marks a user's active configuration as revoked.
//
// Note what this does not do: OpenVPN checks a certificate revocation list,
// which this does not yet maintain, so a revoked certificate can still
// complete a TLS handshake. Access is still refused, because authentication
// checks the database - but generating a CRL is worth doing before this is
// relied upon as the only barrier.
func (s *VPNConfigService) Revoke(ctx context.Context, email string) error {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	return s.revokeExisting(ctx, user.ID, "revoked by an administrator")
}

func (s *VPNConfigService) revokeExisting(ctx context.Context, userID uint, reason string) error {
	var active []models.VPNConfig
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, models.VPNConfigStatusActive).
		Find(&active).Error; err != nil {
		return fmt.Errorf("finding active configurations: %w", err)
	}

	for i := range active {
		if err := s.db.WithContext(ctx).Model(&active[i]).
			Update("status", models.VPNConfigStatusRevoked).Error; err != nil {
			return fmt.Errorf("revoking configuration %d: %w", active[i].ID, err)
		}
		s.audit.Log(ctx, Entry{
			UserID:       &userID,
			Action:       "vpn_config_revoked",
			ResourceType: "vpn_config",
			ResourceID:   &active[i].ID,
			Details:      map[string]interface{}{"reason": reason},
		})
	}
	return nil
}

// buildProfile assembles the .ovpn file a user imports into their client.
func (s *VPNConfigService) buildProfile(cred *vpn.ClientCredential) string {
	var b strings.Builder

	proto := s.server.Proto
	if proto == "" {
		proto = "udp"
	}
	port := s.server.Port
	if port == 0 {
		port = 1194
	}

	fmt.Fprintf(&b, "# Dvarpala VPN profile for %s\n", cred.CommonName)
	fmt.Fprintf(&b, "# Issued %s, expires %s\n\n",
		time.Now().Format("2006-01-02"), cred.NotAfter.Format("2006-01-02"))

	b.WriteString("client\ndev tun\n")
	fmt.Fprintf(&b, "proto %s\n", proto)
	fmt.Fprintf(&b, "remote %s %d\n", s.server.Host, port)
	b.WriteString(`resolv-retry infinite
nobind
persist-key
persist-tun
remote-cert-tls server
cipher AES-256-GCM
verb 3

# Ask the server to tell us when we disconnect, so access is revoked promptly
# rather than after a keepalive timeout.
explicit-exit-notify 1

`)

	fmt.Fprintf(&b, "<ca>\n%s</ca>\n\n", s.ca.CertificatePEM())
	fmt.Fprintf(&b, "<cert>\n%s</cert>\n\n", cred.Certificate)
	fmt.Fprintf(&b, "<key>\n%s</key>\n", cred.PrivateKey)

	if s.server.TLSCryptK != "" {
		fmt.Fprintf(&b, "\n<tls-crypt>\n%s</tls-crypt>\n", s.server.TLSCryptK)
	}

	return b.String()
}
