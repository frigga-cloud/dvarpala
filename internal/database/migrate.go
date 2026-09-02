package database

import (
	"dvarpala/internal/database/models"

	"gorm.io/gorm"
)

// AutoMigrate creates/updates all database tables
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		// Core entities
		&models.User{},
		&models.Group{},
		&models.Resource{},
		&models.AuditLog{},

		// VPN-related entities
		&models.VPNSession{},
		&models.VPNConfig{},
		&models.NetworkRoute{},

		// Authentication entities
		&models.Session{},
		&models.OAuthState{},
		&models.OAuthProvider{},

		// Security entities
		&models.IPWhitelist{},
		&models.AllowedDomain{},

		// Junction tables (many-to-many relationships)
		&models.UserGroup{},
		&models.GroupPermission{},
		&models.GroupNetworkRoute{},
	)
}

// DropAllTables drops all tables (use with caution)
func DropAllTables(db *gorm.DB) error {
	return db.Migrator().DropTable(
		&models.GroupNetworkRoute{},
		&models.GroupPermission{},
		&models.UserGroup{},
		&models.AllowedDomain{},
		&models.IPWhitelist{},
		&models.OAuthProvider{},
		&models.OAuthState{},
		&models.Session{},
		&models.NetworkRoute{},
		&models.VPNConfig{},
		&models.VPNSession{},
		&models.AuditLog{},
		&models.Resource{},
		&models.Group{},
		&models.User{},
	)
}
