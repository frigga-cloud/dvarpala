// Package services holds the application's business logic.
//
// Services are transport-agnostic: the same UserService backs the REST API,
// the CLI and (from Phase 3) the OAuth callback. Anything that decides
// something belongs here rather than in an HTTP handler.
package services

import "gorm.io/gorm"

// Services is the application's composition root. Build it once at startup
// and pass it to whichever transport needs it.
type Services struct {
	Audit  *AuditService
	Users  *UserService
	Groups *GroupService
}

// New wires up every service against a single database handle.
func New(db *gorm.DB) *Services {
	audit := NewAuditService(db)
	users := NewUserService(db, audit)

	return &Services{
		Audit:  audit,
		Users:  users,
		Groups: NewGroupService(db, audit, users),
	}
}
