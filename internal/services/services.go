// Package services holds the application's business logic.
//
// Services are transport-agnostic: the same UserService backs the REST API,
// the CLI and (from Phase 3) the OAuth callback. Anything that decides
// something belongs here rather than in an HTTP handler.
package services

import (
	"log"
	"os"
	"time"

	"dvarpala/internal/config"
	"dvarpala/internal/vpn"

	"gorm.io/gorm"
)

// Services is the application's composition root. Build it once at startup
// and pass it to whichever transport needs it.
type Services struct {
	Audit       *AuditService
	Users       *UserService
	Groups      *GroupService
	Resources   *ResourceService
	Permissions *PermissionService
	VPNConfigs  *VPNConfigService
	Domains     *DomainService
}

// New wires up every service against a single database handle.
//
// cfg may be nil, in which case VPN certificate issuing is unavailable but
// every other service works. That keeps tests and tools from needing a full
// configuration.
func New(db *gorm.DB, cfg *config.Config) *Services {
	audit := NewAuditService(db)
	users := NewUserService(db, audit)
	groups := NewGroupService(db, audit, users)
	resources := NewResourceService(db, audit)

	return &Services{
		Audit:       audit,
		Users:       users,
		Groups:      groups,
		Resources:   resources,
		Permissions: NewPermissionService(db, audit, users, groups, resources),
		VPNConfigs:  NewVPNConfigService(db, audit, users, loadCA(cfg), serverDetails(cfg)),
		Domains:     NewDomainService(db, audit),
	}
}

// loadCA opens the certificate authority, creating one if configured to.
// A missing or unreadable CA is not fatal: issuing simply becomes unavailable.
func loadCA(cfg *config.Config) *vpn.CA {
	if cfg == nil || cfg.OpenVPN.PKI.CACert == "" {
		return nil
	}

	pki := cfg.OpenVPN.PKI
	validity := 10 * 365 * 24 * time.Hour

	var (
		ca  *vpn.CA
		err error
	)
	if pki.AutoCreate {
		ca, err = vpn.LoadOrCreateCA(pki.CACert, pki.CAKey, "Dvarpala CA", validity)
	} else {
		ca, err = vpn.LoadCA(pki.CACert, pki.CAKey)
	}
	if err != nil {
		log.Printf("vpn: certificate issuing unavailable: %v", err)
		return nil
	}
	return ca
}

func serverDetails(cfg *config.Config) ServerDetails {
	if cfg == nil {
		return ServerDetails{}
	}

	taKey := ""
	if path := cfg.OpenVPN.PKI.TAKey; path != "" {
		if b, err := os.ReadFile(path); err == nil {
			taKey = string(b)
		} else {
			log.Printf("vpn: could not read ta_key %s: %v", path, err)
		}
	}

	return ServerDetails{
		Host:      cfg.OpenVPN.Server.Host,
		Port:      cfg.OpenVPN.Server.Port,
		Proto:     cfg.OpenVPN.Server.Proto,
		TLSCryptK: taKey,
	}
}
