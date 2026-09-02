package models

import "time"

// AllowedDomain is an email domain whose users may sign in.
//
// This lives in the database rather than the configuration file so an
// administrator can change it from the console without editing YAML on the
// server and restarting. The installer seeds it from the first
// administrator's own domain; the config file is consulted only when the
// table is empty on first start.
//
// An empty table means any domain is accepted, which is the historic
// behaviour and is what a deployment with no allow-list configured has always
// done. The console says so plainly rather than leaving it implied.
type AllowedDomain struct {
	ID uint `gorm:"primaryKey"`

	// Domain is stored lower-cased and without a leading "@", so the
	// comparison at sign-in time is a plain equality check.
	Domain string `gorm:"not null;size:253;uniqueIndex"`

	// AddedBy is the email of the administrator who added it, or empty when
	// it was seeded from configuration at first start.
	AddedBy string `gorm:"size:255"`

	// Note is optional: why this domain is here.
	Note string `gorm:"size:500"`

	CreatedAt time.Time
}

// TableName keeps the table name explicit rather than pluralised by
// convention, which is what the rest of this schema does.
func (AllowedDomain) TableName() string { return "allowed_domains" }
